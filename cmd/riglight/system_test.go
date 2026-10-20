package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"riglight/internal/systemmetrics"
)

type blockedSystemPublisher struct {
	started  chan struct{}
	release  chan struct{}
	attempts chan time.Time
	calls    int
}

func (p *blockedSystemPublisher) Publish(_ context.Context, at time.Time) error {
	p.attempts <- at
	if p.calls == 0 {
		p.calls++
		close(p.started)
		<-p.release
	}
	return nil
}

func TestSystemWorkerUsesCollectionTimeNotStaleTick(t *testing.T) {
	t.Parallel()
	first := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	fresh := first.Add(5 * time.Second)
	publisher := &blockedSystemPublisher{started: make(chan struct{}), release: make(chan struct{}), attempts: make(chan time.Time, 2)}
	worker := newSystemWorker(nil, publisher)
	clock := []time.Time{first, fresh}
	worker.now = func() time.Time { value := clock[0]; clock = clock[1:]; return value }
	ticks := make(chan time.Time, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { defer close(done); worker.run(ctx, ticks) }()
	<-publisher.started
	ticks <- first.Add(time.Second)
	close(publisher.release)
	if got := <-publisher.attempts; !got.Equal(first) {
		t.Fatalf("first attempt = %s", got)
	}
	if got := <-publisher.attempts; !got.Equal(fresh) {
		t.Fatalf("attempt used stale tick %s", got)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop")
	}
}

func TestParseConfigRequiresSystemDetailsForInterfaces(t *testing.T) {
	t.Parallel()
	valid, err := parseConfig([]string{"--system-details", "--network-interface", "uplink2", "--network-interface", "uplink1"})
	if err != nil || !valid.systemDetails || len(valid.networkInterfaces) != 2 || valid.networkInterfaces[0] != "uplink1" {
		t.Fatalf("valid config = %#v, %v", valid, err)
	}
	for _, args := range [][]string{{"--network-interface", "uplink1"}, {"--system-details"}, {"--system-details", "--network-interface", "lo"}} {
		if _, err := parseConfig(args); !errors.Is(err, errInvalidArguments) {
			t.Errorf("parseConfig(%q) = %v", args, err)
		}
	}
}

func TestDemoSystemWorkerNeverConstructsNativeSource(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	hub, err := systemmetrics.NewHub(initialSystemSample(at, config{demo: true, systemDetails: true}))
	if err != nil {
		t.Fatal(err)
	}
	defer hub.Close()
	called := false
	worker, err := selectSystemWorker(hub, config{demo: true, systemDetails: true, networkInterfaces: []string{"uplink1"}}, func(*systemmetrics.Hub, []string) (systemPublisher, error) {
		called = true
		return nil, errors.New("must not construct")
	})
	if err != nil || worker == nil || called {
		t.Fatalf("worker = %#v, called = %t, err = %v", worker, called, err)
	}
	worker.collect(context.Background(), at)
	if got := hub.Latest().Mode; got != systemmetrics.ModeDemo {
		t.Fatalf("mode = %s", got)
	}
}
