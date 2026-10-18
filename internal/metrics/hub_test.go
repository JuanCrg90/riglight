package metrics

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestHubSubscriptionReceivesCurrentSampleBeforeNewerPublication(t *testing.T) {
	t.Parallel()

	first := demoSampleAt(time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC), 1)
	hub, err := NewHub(first)
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	defer hub.Close()

	subscription, err := hub.Subscribe()
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer subscription.Unsubscribe()

	gotFirst := nextSample(t, subscription)
	if !gotFirst.SampledAt.Equal(first.SampledAt) {
		t.Fatalf("first sample timestamp = %s, want %s", gotFirst.SampledAt, first.SampledAt)
	}

	second := demoSampleAt(first.SampledAt.Add(2*time.Second), 2)
	if err := hub.Publish(second); err != nil {
		t.Fatalf("publish: %v", err)
	}
	gotSecond := nextSample(t, subscription)
	if !gotSecond.SampledAt.Equal(second.SampledAt) {
		t.Fatalf("second sample timestamp = %s, want %s", gotSecond.SampledAt, second.SampledAt)
	}
}

func TestHubSubscriptionStartsWithLatestPublishedSample(t *testing.T) {
	t.Parallel()

	first := demoSampleAt(time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC), 1)
	hub, err := NewHub(first)
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	defer hub.Close()
	latest := demoSampleAt(first.SampledAt.Add(2*time.Second), 2)
	if err := hub.Publish(latest); err != nil {
		t.Fatalf("publish latest: %v", err)
	}

	subscription, err := hub.Subscribe()
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer subscription.Unsubscribe()
	if got := nextSample(t, subscription); !got.SampledAt.Equal(latest.SampledAt) {
		t.Errorf("initial subscription timestamp = %s, want %s", got.SampledAt, latest.SampledAt)
	}
}

func TestHubSlowSubscriberReceivesLatestSampleWithoutBlockingPublication(t *testing.T) {
	t.Parallel()

	first := demoSampleAt(time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC), 1)
	hub, err := NewHub(first)
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	defer hub.Close()

	slow, err := hub.Subscribe()
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer slow.Unsubscribe()

	for step := uint64(2); step <= 100; step++ {
		if err := hub.Publish(demoSampleAt(first.SampledAt.Add(time.Duration(step)*time.Second), step)); err != nil {
			t.Fatalf("publish %d: %v", step, err)
		}
	}

	got := nextSample(t, slow)
	if got.SampledAt != first.SampledAt.Add(100*time.Second) {
		t.Errorf("slow subscriber got %s, want latest timestamp", got.SampledAt)
	}
}

func TestHubUnsubscribeIsIdempotentAndRaceSafe(t *testing.T) {
	t.Parallel()

	first := demoSampleAt(time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC), 1)
	hub, err := NewHub(first)
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	defer hub.Close()

	const subscribers = 32
	var ready sync.WaitGroup
	ready.Add(subscribers)
	var workers sync.WaitGroup
	for index := 0; index < subscribers; index++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			subscription, subscribeErr := hub.Subscribe()
			if subscribeErr != nil {
				t.Errorf("subscribe: %v", subscribeErr)
				return
			}
			ready.Done()
			subscription.Unsubscribe()
			subscription.Unsubscribe()
		}()
	}
	ready.Wait()
	for step := uint64(2); step < 100; step++ {
		if err := hub.Publish(demoSampleAt(first.SampledAt, step)); err != nil {
			t.Fatalf("publish: %v", err)
		}
	}
	workers.Wait()
	if got := hub.SubscriberCount(); got != 0 {
		t.Errorf("subscriber count = %d, want 0", got)
	}
}

func TestHubCopiesPublishedAndDeliveredSamples(t *testing.T) {
	t.Parallel()

	first := demoSampleAt(time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC), 1)
	hub, err := NewHub(first)
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	defer hub.Close()

	*first.CPU.UtilizationPercent = 99
	latest := hub.Latest()
	if *latest.CPU.UtilizationPercent == 99 {
		t.Fatal("published sample aliases caller-owned measurement")
	}

	subscription, err := hub.Subscribe()
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer subscription.Unsubscribe()
	received := nextSample(t, subscription)
	*received.CPU.UtilizationPercent = 99
	if got := hub.Latest(); *got.CPU.UtilizationPercent == 99 {
		t.Fatal("delivered sample aliases hub state")
	}
}

func demoSampleAt(at time.Time, step uint64) Sample {
	return demoSample(at, step)
}

func notReadySample(at time.Time, mode Mode) Sample {
	return Sample{
		SchemaVersion: SchemaVersion,
		SampledAt:     at,
		Mode:          mode,
		CPU:           CPU{Status: StatusNotReady},
		Memory:        Memory{Status: StatusNotReady},
	}
}

func nextSample(t *testing.T, subscription *Subscription) Sample {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	sample, err := subscription.Next(ctx)
	if err != nil {
		t.Fatalf("next sample: %v", err)
	}
	return sample
}
