package containers

import (
	"strings"
	"testing"
	"time"
)

func TestAvailableSampleSortsAliasesAndMarshalsIndependentAges(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.January, 1, 0, 0, 5, 0, time.UTC)
	container := demoContainer(at, "zeta")
	container.CPU = NotReadyCPU()
	sample, err := NewAvailableSample(at, at, ModeDemo, []Container{container, demoContainer(at.Add(-time.Second), "alpha")})
	if err != nil {
		t.Fatal(err)
	}
	body, err := sample.MarshalAt(at.Add(1500 * time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		`"schemaVersion":1`, `"mode":"DEMO"`, `"status":"AVAILABLE"`,
		`"ageMs":1500`, `"alias":"alpha"`, `"alias":"zeta"`,
		`"cpu":{"status":"NOT_READY","sampledAt":null,"ageMs":null,"percent":null}`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("body missing %s: %s", want, text)
		}
	}
	if strings.Index(text, `"alias":"alpha"`) > strings.Index(text, `"alias":"zeta"`) {
		t.Errorf("aliases not sorted: %s", text)
	}
}

func TestSampleRejectsUnsafeOrInconsistentValues(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.January, 1, 0, 0, 5, 0, time.UTC)
	for _, test := range []struct {
		name   string
		mutate func(*Sample)
	}{
		{"duplicate aliases", func(sample *Sample) { sample.Containers = append(sample.Containers, sample.Containers[0]) }},
		{"unsafe memory", func(sample *Sample) { value := uint64(1 << 53); sample.Containers[0].Memory.TotalBytes = &value }},
		{"available cpu on stopped state", func(sample *Sample) { sample.Containers[0].Metadata.State = StateExited }},
		{"nonavailable overview rows", func(sample *Sample) {
			sample.Status = OverviewUnavailable
			sample.ReasonCode = reasonPointer(ReasonQueryFailed)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			sample, err := NewAvailableSample(at, at, ModeLive, []Container{demoContainer(at, "one")})
			if err != nil {
				t.Fatal(err)
			}
			test.mutate(&sample)
			if err := sample.Validate(); err == nil {
				t.Fatal("invalid sample was accepted")
			}
		})
	}
}

func TestDisabledAndUnavailableSamplesHaveNullAgeAndNoRows(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.January, 1, 0, 0, 5, 0, time.UTC)
	for _, sample := range []Sample{DisabledSample(at), UnavailableSample(at, ModeLive, ReasonTimeout), UnsupportedSample(at, ModeLive)} {
		body, err := sample.MarshalAt(at.Add(time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), `"ageMs":null`) || !strings.Contains(string(body), `"containers":[]`) {
			t.Errorf("non-available output = %s", body)
		}
	}
}

func demoContainer(at time.Time, alias string) Container {
	return Container{
		Alias:    alias,
		Metadata: AvailableMetadata(at, at, StateRunning, HealthHealthy, uint64Pointer(2)),
		CPU:      AvailableCPU(at, at, 125),
		Memory:   AvailableMemory(at, at, 8, 16),
	}
}
