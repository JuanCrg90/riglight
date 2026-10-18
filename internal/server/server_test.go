package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"riglight/internal/metrics"
)

func TestHealthReturnsProcessStatus(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, allowedRequest(http.MethodGet, "/healthz"))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if got, want := response.Body.String(), `{"status":"OK"}`; got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

func TestSnapshotReturnsAllowlistedLatestSample(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	server := newTestServerAt(t, at)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, allowedRequest(http.MethodGet, "/api/v1/snapshot"))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	const want = `{"schemaVersion":1,"sampledAt":"2026-01-01T00:00:02Z","mode":"DEMO","cpu":{"status":"AVAILABLE","sampledAt":"2026-01-01T00:00:02Z","ageMs":0,"utilizationPercent":42.5},"memory":{"status":"AVAILABLE","sampledAt":"2026-01-01T00:00:02Z","ageMs":0,"usedBytes":8,"totalBytes":16}}`
	if got := strings.TrimSpace(response.Body.String()); got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
	if got := response.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
}

func TestRoutesRejectUnsupportedMethodsWithAllowHeader(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	for _, path := range []string{"/healthz", "/api/v1/snapshot", "/api/v1/events"} {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, allowedRequest(http.MethodPost, path))
		if response.Code != http.StatusMethodNotAllowed {
			t.Errorf("POST %s status = %d, want %d", path, response.Code, http.StatusMethodNotAllowed)
		}
		if got := response.Header().Get("Allow"); got != http.MethodGet {
			t.Errorf("POST %s Allow = %q, want GET", path, got)
		}
	}
}

func TestAPIRejectsQueryParameters(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, allowedRequest(http.MethodGet, "/api/v1/snapshot?ignored=value"))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if got := response.Body.String(); !strings.Contains(got, `"code":"INVALID_REQUEST"`) {
		t.Errorf("response = %s, want INVALID_REQUEST", got)
	}
}

func TestUnknownRouteReturnsGenericNotFound(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, allowedRequest(http.MethodGet, "/private/path"))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
	if strings.Contains(response.Body.String(), "private/path") {
		t.Errorf("response leaks path: %s", response.Body.String())
	}
}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	return newTestServerAt(t, time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC))
}

func newTestServerAt(t *testing.T, at time.Time) *Server {
	t.Helper()
	sample := metrics.Sample{
		SchemaVersion: metrics.SchemaVersion,
		SampledAt:     at,
		Mode:          metrics.ModeDemo,
		CPU:           metrics.AvailableCPU(at, at, 42.5),
		Memory:        metrics.AvailableMemory(at, at, 8, 16),
	}
	hub, err := metrics.NewHub(sample)
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	t.Cleanup(hub.Close)
	server, err := New(Config{
		Hub:           hub,
		AllowedHost:   "127.0.0.1:9090",
		AllowedOrigin: "http://127.0.0.1:9090",
		Now:           func() time.Time { return at },
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	return server
}

func allowedRequest(method, target string) *http.Request {
	request := httptest.NewRequest(method, target, nil)
	request.Host = "127.0.0.1:9090"
	return request
}
