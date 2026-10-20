package systemmetrics

import (
	"context"
	"testing"
	"time"
)

func TestHubPublishesLatestImmutableSystemSample(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	hub, err := NewHub(DisabledSample(at))
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	defer hub.Close()
	subscription, err := hub.Subscribe()
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer subscription.Unsubscribe()
	initial, err := subscription.Next(context.Background())
	if err != nil {
		t.Fatalf("initial: %v", err)
	}
	if initial.Mode != ModeDisabled {
		t.Fatalf("initial mode = %q", initial.Mode)
	}
	sample := demoSample(at, 1)
	if err := hub.Publish(sample); err != nil {
		t.Fatalf("publish: %v", err)
	}
	latest := hub.Latest()
	*latest.Swap.UsedBytes = 999
	if got := *hub.Latest().Swap.UsedBytes; got == 999 {
		t.Fatal("latest aliases caller data")
	}
}
