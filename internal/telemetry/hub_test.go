package telemetry

import (
	"context"
	"errors"
	"testing"
)

type testValue struct{ value int }

func TestHubQueuesLatestValueAndClonesBoundaries(t *testing.T) {
	t.Parallel()

	hub, err := New(testValue{1}, func(value testValue) testValue { return value }, func(testValue) error { return nil })
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	defer hub.Close()
	subscription, err := hub.Subscribe()
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer subscription.Unsubscribe()
	if err := hub.Publish(testValue{2}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	value, err := subscription.Next(context.Background())
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if value.value != 2 {
		t.Errorf("value = %d, want 2", value.value)
	}
}

func TestHubRejectsInvalidValuesAndClose(t *testing.T) {
	t.Parallel()

	hub, err := New(1, func(value int) int { return value }, func(value int) error {
		if value < 0 {
			return errors.New("invalid")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	if err := hub.Publish(-1); err == nil {
		t.Fatal("invalid publish succeeded")
	}
	hub.Close()
	if _, err := hub.Subscribe(); !errors.Is(err, ErrHubClosed) {
		t.Fatalf("subscribe after close = %v", err)
	}
}
