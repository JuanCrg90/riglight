package gpu

import (
	"strings"
	"testing"
	"time"
)

func TestSampleMarshalsExactAvailableContractWithDynamicAge(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	sample := DemoSample(at, 1)
	body, err := sample.MarshalAt(at.Add(1500 * time.Millisecond))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{
		`"schemaVersion":1`, `"mode":"DEMO"`, `"status":"AVAILABLE"`,
		`"reasonCode":null`, `"isTruncated":false`, `"index":0`, `"ageMs":1500`,
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("body missing %s: %s", want, body)
		}
	}
}

func TestSampleRejectsInvalidSuccessfulMeasurementsAndOverviewCombinations(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	sample := DemoSample(at, 1)
	sample.Devices[0].Memory.UsedBytes = uint64Pointer(17_000_000_000)
	if err := sample.Validate(); err == nil {
		t.Fatal("accepted memory used beyond total")
	}

	sample = DisabledSample(at)
	sample.Devices = []Device{{Index: 0}}
	if err := sample.Validate(); err == nil {
		t.Fatal("accepted disabled inventory")
	}
}

func TestHubCopiesSampleAndBoundsSlowSubscriber(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	hub, err := NewHub(DemoSample(at, 1))
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	defer hub.Close()
	subscription, err := hub.Subscribe()
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer subscription.Unsubscribe()
	for step := uint64(2); step < 20; step++ {
		if err := hub.Publish(DemoSample(at.Add(time.Duration(step)*time.Second), step)); err != nil {
			t.Fatalf("publish: %v", err)
		}
	}
	value := <-subscription.Updates()
	if value.SampledAt != at.Add(19*time.Second) {
		t.Errorf("slow subscription time = %s", value.SampledAt)
	}
	value.Devices[0].Utilization.Percent = float64Pointer(99)
	if *hub.Latest().Devices[0].Utilization.Percent == 99 {
		t.Fatal("subscriber mutation aliases hub")
	}
}
