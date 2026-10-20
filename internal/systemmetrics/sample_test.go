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

func TestAvailableSwapAllowsNoConfiguredSwap(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	sample := Sample{SchemaVersion: SchemaVersion, SampledAt: at, Mode: ModeLive, Swap: AvailableSwap(at, at, 0, 0), Disk: AvailableDisk(at, at, 0, 0, 1), Network: AvailableNetwork(at, at, 0, 0)}
	if err := sample.Validate(); err != nil {
		t.Fatalf("zero swap sample: %v", err)
	}
}
