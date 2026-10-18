package metrics

import (
	"encoding/json"
	"math"
	"testing"
	"time"
)

func TestSampleJSONUsesOnlyAllowlistedFields(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	sample := Sample{
		SchemaVersion: SchemaVersion,
		SampledAt:     at,
		Mode:          ModeDemo,
		CPU:           AvailableCPU(at, at, 42.5),
		Memory:        AvailableMemory(at, at, 8_589_934_592, 17_179_869_184),
	}

	got, err := sample.MarshalAt(at)
	if err != nil {
		t.Fatalf("marshal sample: %v", err)
	}
	const want = `{"schemaVersion":1,"sampledAt":"2026-01-01T00:00:02Z","mode":"DEMO","cpu":{"status":"AVAILABLE","sampledAt":"2026-01-01T00:00:02Z","ageMs":0,"utilizationPercent":42.5},"memory":{"status":"AVAILABLE","sampledAt":"2026-01-01T00:00:02Z","ageMs":0,"usedBytes":8589934592,"totalBytes":17179869184}}`
	if string(got) != want {
		t.Errorf("JSON = %s, want %s", got, want)
	}
}

func TestSampleJSONSerializesAbsentMeasurementsAsNull(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	sample := Sample{
		SchemaVersion: SchemaVersion,
		SampledAt:     at,
		Mode:          ModeLive,
		CPU:           CPU{Status: StatusNotReady},
		Memory:        Memory{Status: StatusUnavailable},
	}

	got, err := sample.MarshalAt(at)
	if err != nil {
		t.Fatalf("marshal sample: %v", err)
	}
	const want = `{"schemaVersion":1,"sampledAt":"2026-01-01T00:00:02Z","mode":"LIVE","cpu":{"status":"NOT_READY","sampledAt":null,"ageMs":null,"utilizationPercent":null},"memory":{"status":"UNAVAILABLE","sampledAt":null,"ageMs":null,"usedBytes":null,"totalBytes":null}}`
	if string(got) != want {
		t.Errorf("JSON = %s, want %s", got, want)
	}
}

func TestSampleMarshalAtUsesCollectionAge(t *testing.T) {
	t.Parallel()

	collectedAt := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	sample := Sample{
		SchemaVersion: SchemaVersion,
		SampledAt:     collectedAt,
		Mode:          ModeDemo,
		CPU:           AvailableCPU(collectedAt, collectedAt, 42.5),
		Memory:        AvailableMemory(collectedAt, collectedAt, 8, 16),
	}

	got, err := sample.MarshalAt(collectedAt.Add(2300 * time.Millisecond))
	if err != nil {
		t.Fatalf("marshal sample: %v", err)
	}
	const want = `{"schemaVersion":1,"sampledAt":"2026-01-01T00:00:02Z","mode":"DEMO","cpu":{"status":"AVAILABLE","sampledAt":"2026-01-01T00:00:02Z","ageMs":2300,"utilizationPercent":42.5},"memory":{"status":"AVAILABLE","sampledAt":"2026-01-01T00:00:02Z","ageMs":2300,"usedBytes":8,"totalBytes":16}}`
	if string(got) != want {
		t.Errorf("JSON = %s, want %s", got, want)
	}
}

func TestSampleRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	for _, sample := range []Sample{
		{
			SchemaVersion: SchemaVersion,
			SampledAt:     at,
			Mode:          ModeDemo,
			CPU: CPU{
				Status:             StatusAvailable,
				SampledAt:          &at,
				UtilizationPercent: float64Pointer(math.NaN()),
			},
			Memory: Memory{Status: StatusNotReady},
		},
		{
			SchemaVersion: SchemaVersion,
			SampledAt:     at,
			Mode:          ModeDemo,
			CPU:           CPU{Status: StatusNotReady},
			Memory: Memory{
				Status:     StatusAvailable,
				SampledAt:  &at,
				UsedBytes:  uint64Pointer(2),
				TotalBytes: uint64Pointer(1),
			},
		},
		{
			SchemaVersion: SchemaVersion,
			SampledAt:     at,
			Mode:          ModeDemo,
			CPU: CPU{
				Status:             StatusNotReady,
				UtilizationPercent: float64Pointer(0),
			},
			Memory: Memory{Status: StatusNotReady},
		},
		{
			SchemaVersion: SchemaVersion,
			SampledAt:     at,
			Mode:          ModeDemo,
			CPU:           CPU{Status: StatusNotReady},
			Memory:        AvailableMemory(at, at, maxSafeJSONInteger+1, maxSafeJSONInteger+1),
		},
	} {
		if _, err := json.Marshal(sample); err == nil {
			t.Errorf("json.Marshal(%+v) succeeded for invalid sample", sample)
		}
	}
}

func float64Pointer(value float64) *float64 { return &value }

func uint64Pointer(value uint64) *uint64 { return &value }
