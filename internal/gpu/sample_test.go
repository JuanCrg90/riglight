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

func TestMemoryValidationRequiresAnAllOrNonePointerPair(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	statuses := []Status{StatusNotReady, StatusAvailable, StatusUnavailable, StatusUnsupported}
	pointerPairs := []struct {
		name        string
		used, total *uint64
	}{
		{name: "none"},
		{name: "used only", used: uint64Pointer(1)},
		{name: "total only", total: uint64Pointer(2)},
		{name: "both", used: uint64Pointer(1), total: uint64Pointer(2)},
	}
	for _, status := range statuses {
		for _, pointers := range pointerPairs {
			t.Run(string(status)+"/"+pointers.name, func(t *testing.T) {
				memory := Memory{Status: status, UsedBytes: pointers.used, TotalBytes: pointers.total}
				wantError := pointers.used != nil || pointers.total != nil
				if status == StatusAvailable {
					memory.SampledAt, memory.collectedAt = timePointer(at), at
					wantError = pointers.used == nil || pointers.total == nil
				}
				defer func() {
					if recovered := recover(); recovered != nil {
						t.Fatalf("validate panicked: %v", recovered)
					}
				}()
				err := memory.validate()
				if (err != nil) != wantError {
					t.Fatalf("validate error = %v, want error %t", err, wantError)
				}
			})
		}
	}
}

func TestMemoryValidationRejectsInvalidAvailableRanges(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	for _, test := range []struct {
		name        string
		used, total uint64
	}{
		{name: "zero total", used: 0, total: 0},
		{name: "unsafe total", used: maxSafeJSONInteger + 1, total: maxSafeJSONInteger + 1},
		{name: "used exceeds total", used: 3, total: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			memory := AvailableMemory(at, at, test.used, test.total)
			if err := memory.validate(); err == nil {
				t.Fatal("invalid available memory passed validation")
			}
		})
	}
}

func TestSampleEnforcesOverviewInventoryAndSortedSparseIndices(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	if sample := NotReadySample(at, ModeDisabled); sample.Validate() == nil {
		t.Fatal("disabled mode accepted non-disabled overview")
	}
	if sample := func() Sample { sample := DisabledSample(at); sample.Mode = ModeDemo; return sample }(); sample.Validate() == nil {
		t.Fatal("disabled overview accepted non-disabled mode")
	}
	if sample := func() Sample { sample := DemoSample(at, 1); sample.Devices = nil; return sample }(); sample.Validate() == nil {
		t.Fatal("available overview accepted empty inventory")
	}

	sparse := DemoSample(at, 1)
	second := sparse.Devices[0]
	sparse.Devices = []Device{sparse.Devices[0], second}
	sparse.Devices[0].Index, sparse.Devices[1].Index = 1, 4
	if err := sparse.Validate(); err != nil {
		t.Fatalf("sorted sparse indices rejected: %v", err)
	}
	sparse.Devices[1].Index = 1
	if err := sparse.Validate(); err == nil {
		t.Fatal("duplicate indices accepted")
	}
	sparse.Devices[0].Index, sparse.Devices[1].Index = 4, 1
	if err := sparse.Validate(); err == nil {
		t.Fatal("out-of-order indices accepted")
	}

	truncated := DemoSample(at, 1)
	device := truncated.Devices[0]
	truncated.Devices = make([]Device, MaxDevices-1)
	for index := range truncated.Devices {
		truncated.Devices[index] = device
		truncated.Devices[index].Index = index
	}
	truncated.IsTruncated, truncated.ReasonCode = true, reasonPointer(ReasonDeviceLimit)
	if err := truncated.Validate(); err == nil {
		t.Fatal("truncated inventory below cap accepted")
	}
	truncated.Devices = append(truncated.Devices, device)
	truncated.Devices[MaxDevices-1].Index = MaxDevices - 1
	if err := truncated.Validate(); err != nil {
		t.Fatalf("capped truncated inventory rejected: %v", err)
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
