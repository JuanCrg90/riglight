package systemmetrics

import (
	"strings"
	"testing"
	"time"
)

func TestMarshalAtAvailableSystemContract(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	sample := Sample{
		SchemaVersion: SchemaVersion,
		SampledAt:     at,
		Mode:          ModeDemo,
		Swap:          AvailableSwap(at, at, 1, 4),
		Disk:          AvailableDisk(at, at, 2, 3, 6),
		Network:       AvailableNetwork(at, at, 1.5, 0),
	}
	body, err := sample.MarshalAt(at.Add(time.Second))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"schemaVersion":1,"sampledAt":"2026-01-01T00:00:02Z","mode":"DEMO","swap":{"status":"AVAILABLE","sampledAt":"2026-01-01T00:00:02Z","ageMs":1000,"usedBytes":1,"totalBytes":4},"disk":{"status":"AVAILABLE","sampledAt":"2026-01-01T00:00:02Z","ageMs":1000,"usedBytes":2,"availableBytes":3,"totalBytes":6},"network":{"status":"AVAILABLE","sampledAt":"2026-01-01T00:00:02Z","ageMs":1000,"rxBytesPerSecond":1.5,"txBytesPerSecond":0}}`
	if got := string(body); got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

func TestDisabledSampleRequiresOnlyDisabledSensors(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	if err := DisabledSample(at).Validate(); err != nil {
		t.Fatalf("disabled sample: %v", err)
	}
	sample := DisabledSample(at)
	sample.Swap = Swap{Status: StatusUnavailable}
	if err := sample.Validate(); err == nil {
		t.Fatal("enabled swap accepted in disabled mode")
	}
}

func TestUnavailableSensorsMarshalAllValuesAsNull(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	sample := Sample{SchemaVersion: SchemaVersion, SampledAt: at, Mode: ModeLive, Swap: Swap{Status: StatusUnavailable}, Disk: Disk{Status: StatusUnsupported}, Network: Network{Status: StatusNotReady}}
	body, err := sample.MarshalAt(at)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, field := range []string{`"sampledAt":null`, `"ageMs":null`, `"usedBytes":null`, `"availableBytes":null`, `"rxBytesPerSecond":null`} {
		if !strings.Contains(string(body), field) {
			t.Errorf("body missing %s: %s", field, body)
		}
	}
}

func TestNonAvailableSensorsRejectEveryPartialMeasurement(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	for _, status := range []Status{StatusDisabled, StatusNotReady, StatusUnavailable, StatusUnsupported} {
		mode := ModeLive
		if status == StatusDisabled {
			mode = ModeDisabled
		}
		for mask := 1; mask < 4; mask++ {
			sample := nonAvailableSample(at, mode, status)
			if mask&1 != 0 {
				sample.Swap.UsedBytes = uint64Pointer(1)
			}
			if mask&2 != 0 {
				sample.Swap.TotalBytes = uint64Pointer(1)
			}
			if _, err := sample.MarshalAt(at); err == nil {
				t.Errorf("swap status %s mask %d marshalled", status, mask)
			}
		}
		for mask := 1; mask < 8; mask++ {
			sample := nonAvailableSample(at, mode, status)
			if mask&1 != 0 {
				sample.Disk.UsedBytes = uint64Pointer(1)
			}
			if mask&2 != 0 {
				sample.Disk.AvailableBytes = uint64Pointer(1)
			}
			if mask&4 != 0 {
				sample.Disk.TotalBytes = uint64Pointer(1)
			}
			if _, err := sample.MarshalAt(at); err == nil {
				t.Errorf("disk status %s mask %d marshalled", status, mask)
			}
		}
		for mask := 1; mask < 4; mask++ {
			sample := nonAvailableSample(at, mode, status)
			if mask&1 != 0 {
				sample.Network.RXBytesPerSecond = float64Pointer(1)
			}
			if mask&2 != 0 {
				sample.Network.TXBytesPerSecond = float64Pointer(1)
			}
			if _, err := sample.MarshalAt(at); err == nil {
				t.Errorf("network status %s mask %d marshalled", status, mask)
			}
		}
	}
}

func nonAvailableSample(at time.Time, mode Mode, status Status) Sample {
	return Sample{SchemaVersion: SchemaVersion, SampledAt: at, Mode: mode, Swap: Swap{Status: status}, Disk: Disk{Status: status}, Network: Network{Status: status}}
}

func TestAvailableSwapAllowsNoConfiguredSwap(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	sample := Sample{SchemaVersion: SchemaVersion, SampledAt: at, Mode: ModeLive, Swap: AvailableSwap(at, at, 0, 0), Disk: AvailableDisk(at, at, 0, 0, 1), Network: AvailableNetwork(at, at, 0, 0)}
	if err := sample.Validate(); err != nil {
		t.Fatalf("zero swap sample: %v", err)
	}
}
