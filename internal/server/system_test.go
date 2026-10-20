package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"riglight/internal/metrics"
	"riglight/internal/systemmetrics"
)

func TestSystemRouteUsesExistingGuardsAndFrozenBody(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	server, _ := newSystemServer(t, at)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, allowedRequest(http.MethodGet, "/api/v1/system"))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"mode":"DEMO"`) || !strings.Contains(response.Body.String(), `"availableBytes":3`) {
		t.Fatalf("response = %d %s", response.Code, response.Body.String())
	}
	for _, request := range []*http.Request{allowedRequest(http.MethodPost, "/api/v1/system"), allowedRequest(http.MethodGet, "/api/v1/system?x=y")} {
		response = httptest.NewRecorder()
		server.ServeHTTP(response, request)
		if response.Code != http.StatusMethodNotAllowed && response.Code != http.StatusBadRequest {
			t.Errorf("status = %d", response.Code)
		}
	}
}

func TestEventsSendSystemAsFifthNamedSource(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	server, hub := newSystemServer(t, at)
	writer := newSSEWriter()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := serveEvents(server, writer, ctx)
	for range 2 {
		writer.waitFlush(t)
	}
	if got := writer.String(); !strings.Contains(got, "event: system") || !strings.Contains(got, `"mode":"DEMO"`) {
		t.Fatalf("frames = %q", got)
	}
	cancel()
	waitDone(t, done)
	if hub.SubscriberCount() != 0 {
		t.Fatal("system subscription leaked")
	}
}

func newSystemServer(t *testing.T, at time.Time) (*Server, *systemmetrics.Hub) {
	t.Helper()
	metricsHub, err := metrics.NewHub(testSample(at))
	if err != nil {
		t.Fatal(err)
	}
	systemHub, err := systemmetrics.NewHub(systemmetrics.Sample{SchemaVersion: systemmetrics.SchemaVersion, SampledAt: at, Mode: systemmetrics.ModeDemo, Swap: systemmetrics.AvailableSwap(at, at, 1, 4), Disk: systemmetrics.AvailableDisk(at, at, 2, 3, 6), Network: systemmetrics.AvailableNetwork(at, at, 0, 0)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { metricsHub.Close(); systemHub.Close() })
	server, err := New(Config{Hub: metricsHub, SystemHub: systemHub, AllowedHost: "127.0.0.1:9090", AllowedOrigin: "http://127.0.0.1:9090", Now: func() time.Time { return at }})
	if err != nil {
		t.Fatal(err)
	}
	return server, systemHub
}
