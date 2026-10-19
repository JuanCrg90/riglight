package containers

import (
	"context"
	"testing"
	"time"
)

func TestHubCopiesLatestAndBoundsSlowSubscriber(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.January, 1, 0, 0, 5, 0, time.UTC)
	initial, err := NewAvailableSample(at, at, ModeDemo, []Container{demoContainer(at, "demo")})
	if err != nil {
		t.Fatal(err)
	}
	hub, err := NewHub(initial)
	if err != nil {
		t.Fatal(err)
	}
	defer hub.Close()
	subscription, err := hub.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	defer subscription.Unsubscribe()
	for i := 1; i < 20; i++ {
		sample, _ := NewAvailableSample(at.Add(time.Duration(i)*time.Second), at, ModeDemo, []Container{demoContainer(at, "demo")})
		if err := hub.Publish(sample); err != nil {
			t.Fatal(err)
		}
	}
	got := <-subscription.Updates()
	if !got.SampledAt.Equal(at.Add(19 * time.Second)) {
		t.Fatalf("sampledAt = %s", got.SampledAt)
	}
	*got.Containers[0].Memory.UsedBytes = 99
	if *hub.Latest().Containers[0].Memory.UsedBytes == 99 {
		t.Fatal("subscriber mutation aliases hub")
	}
}

func TestDemoPublishesSyntheticSampleWithoutSource(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.January, 1, 0, 0, 5, 0, time.UTC)
	hub, err := NewHub(NotReadySample(at, ModeDemo))
	if err != nil {
		t.Fatal(err)
	}
	defer hub.Close()
	if err := NewDemo(hub).Publish(context.Background(), at); err != nil {
		t.Fatal(err)
	}
	sample := hub.Latest()
	if sample.Mode != ModeDemo || sample.Status != OverviewAvailable || len(sample.Containers) == 0 {
		t.Fatalf("sample = %#v", sample)
	}
}
