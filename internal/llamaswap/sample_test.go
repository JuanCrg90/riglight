package llamaswap

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestAvailableSampleSortsAndSerializesExactContract(t *testing.T) {
	t.Parallel()

	completed := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	sample, err := NewAvailableSample(completed, completed, ModeDemo, []Model{
		{ID: "example-embed", State: StateUnloaded},
		{ID: "example-chat", State: StateLoaded},
	})
	if err != nil {
		t.Fatalf("new available sample: %v", err)
	}
	body, err := sample.MarshalAt(completed.Add(1500 * time.Millisecond))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	const want = `{"schemaVersion":1,"sampledAt":"2026-01-01T00:00:02Z","mode":"DEMO","status":"AVAILABLE","reasonCode":null,"ageMs":1500,"isTruncated":false,"models":[{"id":"example-chat","state":"LOADED"},{"id":"example-embed","state":"UNLOADED"}]}`
	if got := string(body); got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

func TestAvailableSampleBoundsOnlyAfterValidatingAllModels(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	models := make([]Model, MaxModels+1)
	for index := range models {
		models[index] = Model{ID: "model-" + strconv.Itoa(index), State: StateLoaded}
	}
	models[MaxModels] = Model{ID: "../unsafe", State: StateLoaded}
	if _, err := NewAvailableSample(at, at, ModeLive, models); err == nil {
		t.Fatal("unsafe overflow model was accepted")
	}

	models[MaxModels] = Model{ID: "model-overflow", State: StateUnknown}
	sample, err := NewAvailableSample(at, at, ModeLive, models)
	if err != nil {
		t.Fatalf("new available sample: %v", err)
	}
	if !sample.IsTruncated || len(sample.Models) != MaxModels {
		t.Fatalf("truncation = %t, models = %d", sample.IsTruncated, len(sample.Models))
	}
}

func TestAvailableSampleRejectsUnsafeAndDuplicateIDs(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	for _, id := range []string{"", " model", "model ", "../model", "/model", "~/model", "C:/model", "https://example.invalid/model", "model\\path", "model\u200b", string([]byte{0xff})} {
		if _, err := NewAvailableSample(at, at, ModeLive, []Model{{ID: id, State: StateLoaded}}); err == nil {
			t.Errorf("unsafe id %q was accepted", id)
		}
	}
	if _, err := NewAvailableSample(at, at, ModeLive, []Model{{ID: "org/model", State: StateLoaded}, {ID: "org/model", State: StateUnloaded}}); err == nil {
		t.Error("duplicate ids were accepted")
	}
}

func TestSamplesValidateModesReasonsAndOwnership(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	sample := UnavailableSample(at, ModeLive, ReasonTimeout)
	if err := sample.Validate(); err != nil {
		t.Fatalf("unavailable sample: %v", err)
	}
	if body, err := sample.MarshalAt(at); err != nil || !strings.Contains(string(body), `"ageMs":null`) || !strings.Contains(string(body), `"models":[]`) {
		t.Fatalf("unavailable body = %s, %v", body, err)
	}
	if err := UnsupportedSample(at, ModeLive).Validate(); err != nil {
		t.Fatalf("unsupported sample: %v", err)
	}
	if err := DisabledSample(at).Validate(); err != nil {
		t.Fatalf("disabled sample: %v", err)
	}

	available, err := NewAvailableSample(at, at, ModeLive, []Model{{ID: "model", State: StateLoaded}})
	if err != nil {
		t.Fatal(err)
	}
	hub, err := NewHub(available)
	if err != nil {
		t.Fatal(err)
	}
	defer hub.Close()
	available.Models[0].ID = "changed"
	latest := hub.Latest()
	if latest.Models[0].ID != "model" {
		t.Errorf("hub retained caller mutation: %#v", latest.Models)
	}
	latest.Models[0].ID = "changed-again"
	if got := hub.Latest().Models[0].ID; got != "model" {
		t.Errorf("hub exposed internal mutation: %q", got)
	}

	var wire map[string]json.RawMessage
	body, err := hub.Latest().MarshalAt(at)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatal(err)
	}
	if len(wire) != 8 {
		t.Errorf("wire keys = %d, want 8: %s", len(wire), body)
	}
}

func TestMarshalCanonicalizesNilModelListsWithoutPrivateFields(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	available, err := NewAvailableSample(at, at, ModeLive, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, sample := range []Sample{
		DisabledSample(at),
		NotReadySample(at, ModeLive),
		UnavailableSample(at, ModeLive, ReasonQueryFailed),
		UnsupportedSample(at, ModeLive),
		available,
	} {
		sample.Models = nil
		body, err := sample.MarshalAt(at)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), `"models":[]`) {
			t.Errorf("models were not an array: %s", body)
		}
		for _, forbidden := range []string{"completedAt", "inner", "endpoint"} {
			if strings.Contains(string(body), forbidden) {
				t.Errorf("wire body leaked %q: %s", forbidden, body)
			}
		}
	}
}

func TestAvailableSampleFreshnessUsesPrivateCompletionTime(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	sample, err := NewAvailableSample(at, at, ModeLive, []Model{{ID: "model", State: StateLoaded}})
	if err != nil {
		t.Fatal(err)
	}
	if remaining, ok := sample.Freshness(at.Add(5*time.Second), 6*time.Second); !ok || remaining != time.Second {
		t.Fatalf("freshness = %s, %t", remaining, ok)
	}
	if _, ok := sample.Freshness(at.Add(6*time.Second), 6*time.Second); ok {
		t.Fatal("six-second-old sample was fresh")
	}
	if _, ok := sample.Freshness(at.Add(-time.Nanosecond), 6*time.Second); ok {
		t.Fatal("future completion was fresh")
	}
}

func TestDemoSampleIsSyntheticAndAvailable(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	sample := DemoSample(at)
	if sample.Mode != ModeDemo || sample.Status != OverviewAvailable || len(sample.Models) != 2 {
		t.Fatalf("demo sample = %#v", sample)
	}
	if err := sample.Validate(); err != nil {
		t.Fatalf("demo sample invalid: %v", err)
	}
}
