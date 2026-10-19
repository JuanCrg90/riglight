package dockerproxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"riglight/internal/containers"
)

func TestHTTPServerServesOnlyCachedProjectedSnapshot(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.January, 1, 0, 0, 5, 0, time.UTC)
	sample, err := containers.NewAvailableSample(at, at, containers.ModeLive, []containers.Container{{Alias: "app", Metadata: containers.AvailableMetadata(at, at, containers.StateRunning, containers.HealthHealthy, nil), CPU: containers.AvailableCPU(at, at, 1), Memory: containers.AvailableMemory(at, at, 1, 2)}})
	if err != nil {
		t.Fatal(err)
	}
	hub, err := containers.NewHub(sample)
	if err != nil {
		t.Fatal(err)
	}
	defer hub.Close()
	server, err := NewHTTPServer(hub, "127.0.0.1:9123", func() time.Time { return at })
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/containers", nil)
	request.Host = "127.0.0.1:9123"
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"alias":"app"`) {
		t.Fatalf("response = %d %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("security headers = %#v", response.Header())
	}
	for _, request := range []*http.Request{httptest.NewRequest(http.MethodPost, "/api/v1/containers", nil), httptest.NewRequest(http.MethodGet, "/api/v1/containers?x=1", nil), httptest.NewRequest(http.MethodGet, "/api/v1/containers", nil)} {
		request.Host = "127.0.0.1:9123"
		if request.Method == http.MethodGet && request.URL.RawQuery == "" {
			request.Header.Set("Origin", "http://127.0.0.1:9123")
		}
		recorder := httptest.NewRecorder()
		server.ServeHTTP(recorder, request)
		if recorder.Code == http.StatusOK {
			t.Errorf("unsafe request accepted: %s %s", request.Method, request.URL)
		}
	}
}

func TestWorkerPollsIndependentlyAndStopsOnCancellation(t *testing.T) {
	t.Parallel()
	at := time.Now()
	hub, err := containers.NewHub(containers.NotReadySample(at, containers.ModeLive))
	if err != nil {
		t.Fatal(err)
	}
	defer hub.Close()
	worker := newWorker(hub, fakeCollector{sample: containers.UnavailableSample(at, containers.ModeLive, containers.ReasonTimeout)})
	ctx, cancel := context.WithCancel(context.Background())
	worker.Start(ctx)
	defer cancel()
	deadline := time.After(time.Second)
	for hub.Latest().Status != containers.OverviewUnavailable {
		select {
		case <-deadline:
			t.Fatal("worker did not publish")
		case <-time.After(time.Millisecond):
		}
	}
	cancel()
	worker.Wait(context.Background())
}

type fakeCollector struct{ sample containers.Sample }

func (f fakeCollector) Collect(context.Context) (containers.Sample, error) { return f.sample, nil }
