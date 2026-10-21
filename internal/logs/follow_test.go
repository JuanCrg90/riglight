package logs

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"riglight/internal/llamaswap"
)

type followTestClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *followTestClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}
func (c *followTestClock) Set(value time.Time) {
	c.mu.Lock()
	c.now = value
	c.mu.Unlock()
}

type followTestTimer struct{ updates chan time.Time }

func (t *followTestTimer) C() <-chan time.Time { return t.updates }
func (t *followTestTimer) Stop() bool          { return true }

type followReader struct {
	started   chan struct{}
	cancelled chan struct{}
	lines     chan string
	once      sync.Once
}

func newFollowReader() *followReader {
	return &followReader{started: make(chan struct{}), cancelled: make(chan struct{}), lines: make(chan string, 2)}
}
func (r *followReader) Stream(ctx context.Context, ready func() error, emit Emit) error {
	if err := ready(); err != nil {
		return err
	}
	close(r.started)
	for {
		select {
		case line := <-r.lines:
			if err := emit(StreamOutput, line, false); err != nil {
				return err
			}
		case <-ctx.Done():
			r.once.Do(func() { close(r.cancelled) })
			return ctx.Err()
		}
	}
}

func availableLoaded(t *testing.T, at time.Time, mode llamaswap.Mode, ids ...string) llamaswap.Sample {
	t.Helper()
	models := make([]llamaswap.Model, len(ids))
	for i, id := range ids {
		models[i] = llamaswap.Model{ID: id, State: llamaswap.StateLoaded}
	}
	sample, err := llamaswap.NewAvailableSample(at, at, mode, models)
	if err != nil {
		t.Fatal(err)
	}
	return sample
}

func TestTargetUsesExactBoundedContract(t *testing.T) {
	model := "example-chat"
	value := Target{SchemaVersion: SchemaVersion, Mode: ModeDemo, SourceID: "model.loaded", Revision: 1, State: TargetFollowing, Model: &model}
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"schemaVersion":1,"mode":"DEMO","sourceId":"model.loaded","revision":1,"state":"FOLLOWING","model":"example-chat"}`
	if string(body) != want || value.Validate() != nil {
		t.Fatalf("target = %s, validation = %v", body, value.Validate())
	}
	for _, invalid := range []Target{
		{SchemaVersion: SchemaVersion, Mode: ModeLive, SourceID: "model.loaded", Revision: 1, State: TargetWaiting, Model: &model},
		{SchemaVersion: SchemaVersion, Mode: ModeLive, SourceID: "model.loaded", Revision: 1, State: TargetFollowing},
		{SchemaVersion: SchemaVersion, Mode: ModeLive, SourceID: "model.loaded", Revision: maxSafeRevision + 1, State: TargetWaiting},
	} {
		if invalid.Validate() == nil {
			t.Fatalf("accepted target %#v", invalid)
		}
	}
}

func TestFollowLoadedTransitionsJoinOldReaderBeforeNewTarget(t *testing.T) {
	at := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	clock := &followTestClock{now: at}
	hub, err := llamaswap.NewHub(availableLoaded(t, at, llamaswap.ModeLive, "alpha"))
	if err != nil {
		t.Fatal(err)
	}
	defer hub.Close()

	var readers []*followReader
	provider, err := NewFollowLoadedProvider(FollowLoadedOptions{
		Hub: hub, Mode: ModeLive, Now: clock.Now,
		NewReader: func(string, *InputBudget) (Provider, error) {
			reader := newFollowReader()
			readers = append(readers, reader)
			return reader, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	targets := make(chan Target, 4)
	done := make(chan error, 1)
	go func() {
		done <- provider.StreamWithTarget(ctx, func() error { return nil }, func(target Target) error { targets <- target; return nil }, func(Stream, string, bool) error { return nil })
	}()

	first := <-targets
	if first.State != TargetFollowing || first.Model == nil || *first.Model != "alpha" || first.Revision != 1 {
		t.Fatalf("first target = %#v", first)
	}
	select {
	case <-readers[0].started:
	case <-time.After(time.Second):
		t.Fatal("alpha reader did not start")
	}
	clock.Set(at.Add(time.Second))
	if err := hub.Publish(availableLoaded(t, at.Add(time.Second), llamaswap.ModeLive)); err != nil {
		t.Fatal(err)
	}
	var waiting Target
	select {
	case waiting = <-targets:
	case <-time.After(time.Second):
		select {
		case err := <-done:
			t.Fatalf("provider stopped: %v; session context: %v", err, ctx.Err())
		default:
			t.Fatalf("waiting target not emitted; subscriptions = %d", hub.SubscriberCount())
		}
	}
	if waiting.State != TargetWaiting || waiting.Model != nil || waiting.Revision != 2 {
		t.Fatalf("waiting target = %#v", waiting)
	}
	select {
	case <-readers[0].cancelled:
	case <-time.After(time.Second):
		t.Fatal("alpha reader was not joined before waiting")
	}
	clock.Set(at.Add(2 * time.Second))
	if err := hub.Publish(availableLoaded(t, at.Add(2*time.Second), llamaswap.ModeLive, "bravo")); err != nil {
		t.Fatal(err)
	}
	second := <-targets
	if second.State != TargetFollowing || second.Model == nil || *second.Model != "bravo" || second.Revision != 3 {
		t.Fatalf("second target = %#v", second)
	}
	select {
	case <-readers[1].started:
	case <-time.After(time.Second):
		t.Fatal("bravo reader did not start")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("shutdown unexpectedly succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("provider did not stop")
	}
	if got := hub.SubscriberCount(); got != 0 {
		t.Fatalf("subscriptions = %d", got)
	}
}

func TestFollowLoadedExpiresWithoutInventoryPublication(t *testing.T) {
	at := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	clock := &followTestClock{now: at}
	hub, err := llamaswap.NewHub(availableLoaded(t, at, llamaswap.ModeLive, "alpha"))
	if err != nil {
		t.Fatal(err)
	}
	defer hub.Close()
	timer := &followTestTimer{updates: make(chan time.Time, 1)}
	reader := newFollowReader()
	provider, err := NewFollowLoadedProvider(FollowLoadedOptions{
		Hub: hub, Mode: ModeLive, Now: clock.Now,
		NewTimer:  func(time.Duration) FollowLoadedTimer { return timer },
		NewReader: func(string, *InputBudget) (Provider, error) { return reader, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	targets := make(chan Target, 2)
	go func() {
		_ = provider.StreamWithTarget(ctx, func() error { return nil }, func(target Target) error { targets <- target; return nil }, func(Stream, string, bool) error { return nil })
	}()
	<-targets
	clock.Set(at.Add(6 * time.Second))
	timer.updates <- clock.Now()
	expired := <-targets
	if expired.State != TargetUnavailable || expired.Model != nil || expired.Revision != 2 {
		t.Fatalf("expired target = %#v", expired)
	}
	select {
	case <-reader.cancelled:
	case <-time.After(time.Second):
		t.Fatal("reader remained attached after expiry")
	}
}

func TestFollowLoadedKeepsFreshSameTargetAttached(t *testing.T) {
	at := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	clock := &followTestClock{now: at}
	hub, err := llamaswap.NewHub(availableLoaded(t, at, llamaswap.ModeLive, "alpha"))
	if err != nil {
		t.Fatal(err)
	}
	defer hub.Close()
	calls := 0
	reader := newFollowReader()
	provider, err := NewFollowLoadedProvider(FollowLoadedOptions{
		Hub: hub, Mode: ModeLive, Now: clock.Now,
		NewReader: func(string, *InputBudget) (Provider, error) { calls++; return reader, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	targets := make(chan Target, 2)
	go func() {
		_ = provider.StreamWithTarget(ctx, func() error { return nil }, func(target Target) error { targets <- target; return nil }, func(Stream, string, bool) error { return nil })
	}()
	<-targets
	clock.Set(at.Add(time.Second))
	if err := hub.Publish(availableLoaded(t, at.Add(time.Second), llamaswap.ModeLive, "alpha")); err != nil {
		t.Fatal(err)
	}
	select {
	case target := <-targets:
		t.Fatalf("same target republished: %#v", target)
	case <-time.After(20 * time.Millisecond):
	}
	if calls != 1 {
		t.Fatalf("reader calls = %d", calls)
	}
}
