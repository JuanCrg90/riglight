package containers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestHTTPSourceRequiresLoopbackProjectedLiveSnapshotAndRetainsAge(t *testing.T) {
	t.Parallel()
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/containers" || r.URL.RawQuery != "" {
			t.Errorf("request = %s %s", r.Method, r.URL)
		}
		time.Sleep(10 * time.Millisecond)
		_, _ = io.WriteString(w, projectedJSON(1000))
	}))
	defer fixture.Close()
	source, err := NewHTTPSource(fixture.URL)
	if err != nil {
		t.Fatal(err)
	}
	sample, err := source.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	body, err := sample.MarshalAt(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"mode":"LIVE"`) || !strings.Contains(string(body), `"alias":"app"`) || strings.Contains(string(body), "PRIVATE_SENTINEL") {
		t.Fatalf("body = %s", body)
	}
	if !strings.Contains(string(body), `"ageMs":`) || strings.Contains(string(body), `"ageMs":0`) {
		t.Fatalf("cached age reset: %s", body)
	}
}
func TestHTTPSourceRejectsUnsafeURLsAndMalformedProjection(t *testing.T) {
	t.Parallel()
	for _, url := range []string{"http://localhost:9123", "http://127.0.0.1", "http://127.0.0.1:9123/base", "http://127.0.0.1:9123?x=1"} {
		if _, err := NewHTTPSource(url); err == nil {
			t.Errorf("accepted %s", url)
		}
	}
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{ "schemaVersion":1, "sampledAt":"2026-01-01T00:00:05Z", "mode":"DEMO" }`)
	}))
	defer fixture.Close()
	source, _ := NewHTTPSource(fixture.URL)
	sample, err := source.Collect(context.Background())
	if err != nil || sample.Status != OverviewUnavailable || sample.ReasonCode == nil || *sample.ReasonCode != ReasonInvalidResponse {
		t.Fatalf("sample = %#v, %v", sample, err)
	}
}
func TestHTTPSourceRejectsAgeThatCannotFitMonotonicDuration(t *testing.T) {
	t.Parallel()
	age := int64(^uint64(0)>>1)/int64(time.Millisecond) + 1
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, projectedJSON(age)) }))
	defer fixture.Close()
	source, err := NewHTTPSource(fixture.URL)
	if err != nil {
		t.Fatal(err)
	}
	sample, err := source.Collect(context.Background())
	if err != nil || sample.Status != OverviewUnavailable || sample.ReasonCode == nil || *sample.ReasonCode != ReasonInvalidResponse {
		t.Fatalf("sample = %#v, %v", sample, err)
	}
}
func projectedJSON(age int64) string {
	value := strconv.FormatInt(age, 10)
	return `{"schemaVersion":1,"sampledAt":"2026-01-01T00:00:05Z","mode":"LIVE","status":"AVAILABLE","reasonCode":null,"ageMs":` + value + `,"containers":[{"alias":"app","metadata":{"status":"AVAILABLE","sampledAt":"2026-01-01T00:00:05Z","ageMs":` + value + `,"state":"RUNNING","health":"HEALTHY","restartCount":0},"cpu":{"status":"AVAILABLE","sampledAt":"2026-01-01T00:00:05Z","ageMs":` + value + `,"percent":10},"memory":{"status":"AVAILABLE","sampledAt":"2026-01-01T00:00:05Z","ageMs":` + value + `,"usedBytes":1,"totalBytes":2}}]}`
}
