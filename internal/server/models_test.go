package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"riglight/internal/llamaswap"
	"riglight/internal/metrics"
)

func TestModelsEndpointUsesExactContractAndNilHubRemainsHidden(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	withoutModels := newTestServerAt(t, at)
	notFound := httptest.NewRecorder()
	withoutModels.ServeHTTP(notFound, allowedRequest(http.MethodGet, "/api/v1/models"))
	if notFound.Code != http.StatusNotFound {
		t.Fatalf("nil hub status = %d, want 404", notFound.Code)
	}

	server, _ := newModelStreamServer(t, at)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, allowedRequest(http.MethodGet, "/api/v1/models"))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	const want = `{"schemaVersion":1,"sampledAt":"2026-01-01T00:00:02Z","mode":"DEMO","status":"AVAILABLE","reasonCode":null,"ageMs":0,"isTruncated":false,"models":[{"id":"example-chat","state":"LOADED"},{"id":"example-embed","state":"UNLOADED"}]}`
	if got := strings.TrimSpace(response.Body.String()); got != want {
		t.Errorf("body = %s, want %s", got, want)
	}

	blocked := httptest.NewRecorder()
	server.ServeHTTP(blocked, allowedRequest(http.MethodPost, "/api/v1/models"))
	if blocked.Code != http.StatusMethodNotAllowed || blocked.Header().Get("Allow") != http.MethodGet {
		t.Errorf("method guard = %d / %q", blocked.Code, blocked.Header().Get("Allow"))
	}
}

func TestSSEMuxesNamedModelFramesOnExistingConnectionSlot(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	server, models := newModelStreamServer(t, at)
	writer := newSSEWriter()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := serveEvents(server, writer, ctx)
	writer.waitFlush(t)
	writer.waitFlush(t)
	if body := writer.String(); !bytes.Contains([]byte(body), []byte("event: snapshot")) || !bytes.Contains([]byte(body), []byte("event: models")) {
		t.Fatalf("frames = %q", body)
	}
	if got := models.SubscriberCount(); got != 1 {
		t.Errorf("model subscribers = %d, want 1", got)
	}
	cancel()
	waitDone(t, done)
	if got := models.SubscriberCount(); got != 0 {
		t.Errorf("model subscribers after cancellation = %d, want 0", got)
	}
	if got := len(server.streamSlots); got != 0 {
		t.Errorf("stream slots after cancellation = %d, want 0", got)
	}
}

func newModelStreamServer(t *testing.T, at time.Time) (*Server, *llamaswap.Hub) {
	t.Helper()
	cpuHub, err := metrics.NewHub(testSample(at))
	if err != nil {
		t.Fatal(err)
	}
	modelHub, err := llamaswap.NewHub(llamaswap.DemoSample(at))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cpuHub.Close(); modelHub.Close() })
	server, err := New(Config{
		Hub:           cpuHub,
		ModelsHub:     modelHub,
		AllowedHost:   "127.0.0.1:9090",
		AllowedOrigin: "http://127.0.0.1:9090",
		Now:           func() time.Time { return at },
	})
	if err != nil {
		t.Fatal(err)
	}
	return server, modelHub
}
