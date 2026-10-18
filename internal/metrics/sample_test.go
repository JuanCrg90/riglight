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
	utilization := 42.5
	used := uint64(8_589_934_592)
	total := uint64(17_179_869_184)
	sample := Sample{
		SchemaVersion: SchemaVersion,
		SampledAt:     at,
		Mode:          ModeDemo,
		CPU: CPU{
			Status:             StatusAvailable,
			SampledAt:          &at,
			UtilizationPercent: &utilization,
		},
		Memory: Memory{
			Status:     StatusAvailable,
			SampledAt:  &at,
			UsedBytes:  &used,
			TotalBytes: &total,
		},
	}

	got, err := json.Marshal(sample)
	if err != nil {
		t.Fatalf("marshal sample: %v", err)
	}
	const want = `{"schemaVersion":1,"sampledAt":"2026-01-01T00:00:02Z","mode":"DEMO","cpu":{"status":"AVAILABLE","sampledAt":"2026-01-01T00:00:02Z","utilizationPercent":42.5},"memory":{"status":"AVAILABLE","sampledAt":"2026-01-01T00:00:02Z","usedBytes":8589934592,"totalBytes":17179869184}}`
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

	got, err := json.Marshal(sample)
	if err != nil {
		t.Fatalf("marshal sample: %v", err)
	}
	const want = `{"schemaVersion":1,"sampledAt":"2026-01-01T00:00:02Z","mode":"LIVE","cpu":{"status":"NOT_READY","sampledAt":null,"utilizationPercent":null},"memory":{"status":"UNAVAILABLE","sampledAt":null,"usedBytes":null,"totalBytes":null}}`
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
	} {
		if _, err := json.Marshal(sample); err == nil {
			t.Errorf("json.Marshal(%+v) succeeded for invalid sample", sample)
		}
	}
}

func float64Pointer(value float64) *float64 { return &value }

func uint64Pointer(value uint64) *uint64 { return &value }
