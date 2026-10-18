package metrics

import (
	"testing"
	"time"
)

func TestDemoPublishesDeterministicSyntheticSample(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	hub, err := NewHub(notReadySample(at, ModeDemo))
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	defer hub.Close()
	demo := NewDemo(hub)

	if err := demo.Publish(at); err != nil {
		t.Fatalf("publish demo sample: %v", err)
	}
	got := hub.Latest()
	if got.Mode != ModeDemo || got.CPU.Status != StatusAvailable || got.Memory.Status != StatusAvailable {
		t.Errorf("demo statuses = %#v", got)
	}
	if got.CPU.UtilizationPercent == nil || *got.CPU.UtilizationPercent != 41 {
		t.Errorf("demo CPU = %v, want 41", got.CPU.UtilizationPercent)
	}
	if got.Memory.UsedBytes == nil || *got.Memory.UsedBytes != 8_000_000_001 {
		t.Errorf("demo used bytes = %v, want 8000000001", got.Memory.UsedBytes)
	}
}

func TestDemoDoesNotNeedSubscriberToPublish(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	hub, err := NewHub(notReadySample(at, ModeDemo))
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	defer hub.Close()

	demo := NewDemo(hub)
	if err := demo.Publish(at); err != nil {
		t.Fatalf("publish demo sample: %v", err)
	}
	if hub.SubscriberCount() != 0 {
		t.Fatal("demo publication requires no subscribers")
	}
	if got := hub.Latest(); got.CPU.Status != StatusAvailable || got.Memory.Status != StatusAvailable {
		t.Errorf("latest statuses = %s/%s, want AVAILABLE/AVAILABLE", got.CPU.Status, got.Memory.Status)
	}
}
