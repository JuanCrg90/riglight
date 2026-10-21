package logs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

type followTestLatest struct {
	mu     sync.Mutex
	sample llamaswap.Sample
}

func (l *followTestLatest) Sample() llamaswap.Sample {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.sample
}
func (l *followTestLatest) Set(sample llamaswap.Sample) {
	l.mu.Lock()
	l.sample = sample
	l.mu.Unlock()
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

type finiteFollowReader struct{}

func (finiteFollowReader) Stream(_ context.Context, ready func() error, emit Emit) error {
	if err := ready(); err != nil {
		return err
	}
	return emit(StreamOutput, "final line", false)
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

func TestFollowLoadedDrainsFiniteReaderLinesBeforeEOF(t *testing.T) {
	at := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	for range 100 {
		hub, err := llamaswap.NewHub(availableLoaded(t, at, llamaswap.ModeLive, "alpha"))
		if err != nil {
			t.Fatal(err)
		}
		provider, err := NewFollowLoadedProvider(FollowLoadedOptions{Hub: hub, Mode: ModeLive, Now: func() time.Time { return at }, NewReader: func(string, *InputBudget) (Provider, error) { return finiteFollowReader{}, nil }})
		if err != nil {
			hub.Close()
			t.Fatal(err)
		}
		emitted := make(chan string, 1)
		err = provider.StreamWithTarget(t.Context(), func() error { return nil }, func(Target) error { return nil }, func(_ Stream, text string, _ bool) error { emitted <- text; return nil })
		hub.Close()
		if err != nil {
			t.Fatal(err)
		}
		select {
		case line := <-emitted:
			if line != "final line" {
				t.Fatalf("line = %q", line)
			}
		default:
			t.Fatal("final line was lost before EOF")
		}
	}
}

func TestFollowLoadedExpiresBeforeQueuedLine(t *testing.T) {
	at := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	clock := &followTestClock{now: at}
	hub, err := llamaswap.NewHub(availableLoaded(t, at, llamaswap.ModeLive, "alpha"))
	if err != nil {
		t.Fatal(err)
	}
	defer hub.Close()
	timer := &followTestTimer{updates: make(chan time.Time, 1)}
	created := make(chan *followReader, 1)
	provider, err := NewFollowLoadedProvider(FollowLoadedOptions{Hub: hub, Mode: ModeLive, Now: clock.Now, NewTimer: func(time.Duration) FollowLoadedTimer { return timer }, NewReader: func(string, *InputBudget) (Provider, error) {
		reader := newFollowReader()
		created <- reader
		return reader, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	events := make(chan string, 3)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go func() {
		_ = provider.StreamWithTarget(ctx, func() error { return nil }, func(target Target) error { events <- string(target.State); return nil }, func(_ Stream, text string, _ bool) error { events <- text; return nil })
	}()
	if event := <-events; event != string(TargetFollowing) {
		t.Fatalf("initial event = %q", event)
	}
	reader := <-created
	<-reader.started
	clock.Set(at.Add(6 * time.Second))
	reader.lines <- "stale line"
	timer.updates <- clock.Now()
	if event := <-events; event != string(TargetUnavailable) {
		t.Fatalf("stale line crossed expiry: %q", event)
	}
	select {
	case event := <-events:
		t.Fatalf("unexpected post-expiry event %q", event)
	case <-time.After(20 * time.Millisecond):
	}
}

func TestFollowLoadedLineRechecksFreshnessAfterReceive(t *testing.T) {
	at := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	clock := &followTestClock{now: at}
	initial := availableLoaded(t, at, llamaswap.ModeLive, "alpha")
	hub, err := llamaswap.NewHub(initial)
	if err != nil {
		t.Fatal(err)
	}
	defer hub.Close()
	created := make(chan *followReader, 1)
	provider, err := NewFollowLoadedProvider(FollowLoadedOptions{Hub: hub, Mode: ModeLive, Now: clock.Now, NewReader: func(string, *InputBudget) (Provider, error) {
		reader := newFollowReader()
		created <- reader
		return reader, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	latest := &followTestLatest{sample: initial}
	checked := make(chan struct{}, 1)
	var hookMu sync.Mutex
	checksUntilSignal := 0
	provider.latest = func() llamaswap.Sample {
		sample := latest.Sample()
		hookMu.Lock()
		if checksUntilSignal > 0 {
			checksUntilSignal--
			if checksUntilSignal == 0 {
				checked <- struct{}{}
			}
		}
		hookMu.Unlock()
		return sample
	}
	events := make(chan string, 3)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go func() {
		_ = provider.StreamWithTarget(ctx, func() error { return nil }, func(target Target) error { events <- string(target.State); return nil }, func(_ Stream, text string, _ bool) error { events <- text; return nil })
	}()
	if event := <-events; event != string(TargetFollowing) {
		t.Fatalf("initial event = %q", event)
	}
	reader := <-created
	<-reader.started
	hookMu.Lock()
	// apply reads latest once; the second read is the line-select precheck.
	checksUntilSignal = 2
	hookMu.Unlock()
	if err := hub.Publish(initial); err != nil {
		t.Fatal(err)
	}
	<-checked // The pre-select freshness check has returned while still fresh.
	clock.Set(at.Add(6 * time.Second))
	reader.lines <- "stale line"
	if event := <-events; event != string(TargetUnavailable) {
		t.Fatalf("stale line crossed post-receive check: %q", event)
	}
}

func TestFollowLoadedRestoresUnopenedAnnouncedTarget(t *testing.T) {
	at := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	clock := &followTestClock{now: at}
	initial := availableLoaded(t, at, llamaswap.ModeLive, "alpha")
	hub, err := llamaswap.NewHub(initial)
	if err != nil {
		t.Fatal(err)
	}
	defer hub.Close()
	created := make(chan *followReader, 2)
	provider, err := NewFollowLoadedProvider(FollowLoadedOptions{Hub: hub, Mode: ModeLive, Now: clock.Now, NewReader: func(string, *InputBudget) (Provider, error) {
		reader := newFollowReader()
		created <- reader
		return reader, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	postCallbackReconciled := make(chan struct{})
	var latestMu sync.Mutex
	latestCalls := 0
	provider.latest = func() llamaswap.Sample {
		latestMu.Lock()
		latestCalls++
		if latestCalls == 2 {
			close(postCallbackReconciled)
		}
		latestMu.Unlock()
		return hub.Latest()
	}
	targets := make(chan Target, 2)
	callbackAdvanced := make(chan struct{})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go func() {
		_ = provider.StreamWithTarget(ctx, func() error { return nil }, func(target Target) error {
			targets <- target
			clock.Set(at.Add(6 * time.Second))
			close(callbackAdvanced)
			return nil
		}, func(Stream, string, bool) error { return nil })
	}()
	first := <-targets
	if first.State != TargetFollowing || first.Revision != 1 {
		t.Fatalf("first target = %#v", first)
	}
	<-callbackAdvanced
	<-postCallbackReconciled
	select {
	case <-created:
		t.Fatal("stale target opened a reader")
	default:
	}
	freshAt := at.Add(6 * time.Second)
	if err := hub.Publish(availableLoaded(t, freshAt, llamaswap.ModeLive, "alpha")); err != nil {
		t.Fatal(err)
	}
	reader := <-created
	select {
	case <-reader.started:
	case <-time.After(time.Second):
		t.Fatal("fresh target did not attach a reader")
	}
	select {
	case target := <-targets:
		t.Fatalf("target repeated while attaching reader %#v", target)
	case <-time.After(20 * time.Millisecond):
	}
}

func TestFollowLoadedExpiryUsesNewestSampleWithoutReopening(t *testing.T) {
	at := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	clock := &followTestClock{now: at}
	initial := availableLoaded(t, at, llamaswap.ModeLive, "alpha")
	hub, err := llamaswap.NewHub(initial)
	if err != nil {
		t.Fatal(err)
	}
	defer hub.Close()
	timer := &followTestTimer{updates: make(chan time.Time, 1)}
	created := make(chan *followReader, 2)
	provider, err := NewFollowLoadedProvider(FollowLoadedOptions{Hub: hub, Mode: ModeLive, Now: clock.Now, NewTimer: func(time.Duration) FollowLoadedTimer { return timer }, NewReader: func(string, *InputBudget) (Provider, error) {
		reader := newFollowReader()
		created <- reader
		return reader, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	latest := &followTestLatest{sample: initial}
	provider.latest = latest.Sample
	targets := make(chan Target, 2)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go func() {
		_ = provider.StreamWithTarget(ctx, func() error { return nil }, func(target Target) error { targets <- target; return nil }, func(Stream, string, bool) error { return nil })
	}()
	<-targets
	reader := <-created
	<-reader.started
	freshAt := at.Add(5 * time.Second)
	latest.Set(availableLoaded(t, freshAt, llamaswap.ModeLive, "alpha"))
	clock.Set(at.Add(6 * time.Second))
	timer.updates <- clock.Now()
	select {
	case target := <-targets:
		t.Fatalf("old expiry reopened unchanged target %#v", target)
	case <-time.After(20 * time.Millisecond):
	}
	select {
	case <-created:
		t.Fatal("old expiry opened another reader")
	default:
	}
	select {
	case <-reader.cancelled:
		t.Fatal("old expiry cancelled the fresh reader")
	default:
	}
}

func TestFollowLoadedCancellationWinsContinuousInventoryUpdates(t *testing.T) {
	at := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	hub, err := llamaswap.NewHub(availableLoaded(t, at, llamaswap.ModeLive, "alpha"))
	if err != nil {
		t.Fatal(err)
	}
	defer hub.Close()
	created := make(chan *followReader, 1)
	provider, err := NewFollowLoadedProvider(FollowLoadedOptions{Hub: hub, Mode: ModeLive, Now: func() time.Time { return at }, NewReader: func(string, *InputBudget) (Provider, error) {
		reader := newFollowReader()
		created <- reader
		return reader, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		done <- provider.StreamWithTarget(ctx, func() error { return nil }, func(Target) error { return nil }, func(Stream, string, bool) error { return nil })
	}()
	reader := <-created
	<-reader.started
	stopUpdates := make(chan struct{})
	go func() {
		defer close(stopUpdates)
		for {
			if err := hub.Publish(availableLoaded(t, at, llamaswap.ModeLive, "alpha")); err != nil {
				return
			}
			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("continuous updates starved cancellation")
	}
	select {
	case <-stopUpdates:
	case <-time.After(time.Second):
		t.Fatal("publisher did not stop")
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

	created := make(chan *followReader, 2)
	provider, err := NewFollowLoadedProvider(FollowLoadedOptions{
		Hub: hub, Mode: ModeLive, Now: clock.Now,
		NewReader: func(string, *InputBudget) (Provider, error) {
			reader := newFollowReader()
			created <- reader
			return reader, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
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
	alphaReader := <-created
	select {
	case <-alphaReader.started:
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
			t.Fatalf("provider stopped: %v", err)
		default:
			t.Fatalf("waiting target not emitted; subscriptions = %d", hub.SubscriberCount())
		}
	}
	if waiting.State != TargetWaiting || waiting.Model != nil || waiting.Revision != 2 {
		t.Fatalf("waiting target = %#v", waiting)
	}
	select {
	case <-alphaReader.cancelled:
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
	bravoReader := <-created
	select {
	case <-bravoReader.started:
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

type chargedFollowReader struct {
	budget  *InputBudget
	bytes   int
	started chan struct{}
}

func (r *chargedFollowReader) Stream(ctx context.Context, ready func() error, _ Emit) error {
	if err := ready(); err != nil {
		return err
	}
	reader := NewBudgetReaderWithBudget(bytes.NewReader(make([]byte, r.bytes)), r.budget)
	if _, err := reader.Read(make([]byte, r.bytes)); err != nil {
		return err
	}
	close(r.started)
	<-ctx.Done()
	return ctx.Err()
}

func TestFollowLoadedRawBudgetEndsOnCoincidentNoTarget(t *testing.T) {
	at := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	clock := &followTestClock{now: at}
	hub, err := llamaswap.NewHub(availableLoaded(t, at, llamaswap.ModeLive, "alpha"))
	if err != nil {
		t.Fatal(err)
	}
	defer hub.Close()
	created := make(chan *chargedFollowReader, 2)
	provider, err := NewFollowLoadedProvider(FollowLoadedOptions{
		Hub: hub, Mode: ModeLive, Now: clock.Now,
		NewReader: func(_ string, budget *InputBudget) (Provider, error) {
			reader := &chargedFollowReader{budget: budget, bytes: MaxInputBytes, started: make(chan struct{})}
			created <- reader
			return reader, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	targets := make(chan Target, 2)
	done := make(chan error, 1)
	go func() {
		done <- provider.StreamWithTarget(ctx, func() error { return nil }, func(target Target) error { targets <- target; return nil }, func(Stream, string, bool) error { return nil })
	}()
	<-targets
	alphaReader := <-created
	<-alphaReader.started
	clock.Set(at.Add(time.Second))
	if err := hub.Publish(availableLoaded(t, at.Add(time.Second), llamaswap.ModeLive)); err != nil {
		t.Fatal(err)
	}
	select {
	case target := <-targets:
		t.Fatalf("exhausted budget announced target %#v", target)
	case err := <-done:
		if !errors.Is(err, ErrByteLimit) {
			t.Fatalf("stream error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("shared byte limit did not end the session")
	}
	select {
	case <-created:
		t.Fatal("exhausted budget opened another reader")
	default:
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
	created := make(chan struct{}, 2)
	reader := newFollowReader()
	provider, err := NewFollowLoadedProvider(FollowLoadedOptions{
		Hub: hub, Mode: ModeLive, Now: clock.Now,
		NewReader: func(string, *InputBudget) (Provider, error) { created <- struct{}{}; return reader, nil },
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
	<-created
	clock.Set(at.Add(time.Second))
	if err := hub.Publish(availableLoaded(t, at.Add(time.Second), llamaswap.ModeLive, "alpha")); err != nil {
		t.Fatal(err)
	}
	select {
	case target := <-targets:
		t.Fatalf("same target republished: %#v", target)
	case <-time.After(20 * time.Millisecond):
	}
	select {
	case <-created:
		t.Fatal("same target opened another reader")
	default:
	}
}
