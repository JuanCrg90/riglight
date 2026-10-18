package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"riglight/internal/gpu"
	"riglight/internal/metrics"
)

func TestGPUSnapshotUsesExactContractAndSecurityGuards(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	server := newGPUStreamServer(t, at)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, allowedRequest(http.MethodGet, "/api/v1/gpus"))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	for _, want := range []string{`"mode":"DEMO"`, `"reasonCode":null`, `"devices":[{`} {
		if !strings.Contains(response.Body.String(), want) {
			t.Errorf("body missing %s: %s", want, response.Body.String())
		}
	}
	blocked := httptest.NewRecorder()
	server.ServeHTTP(blocked, allowedRequest(http.MethodPost, "/api/v1/gpus"))
	if blocked.Code != http.StatusMethodNotAllowed || blocked.Header().Get("Allow") != http.MethodGet {
		t.Fatalf("method guard = %d / %q", blocked.Code, blocked.Header().Get("Allow"))
	}
}

func TestSSEMuxesNamedGPUFramesOnOneSlot(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	server := newGPUStreamServer(t, at)
	writer := newSSEWriter()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := serveEvents(server, writer, ctx)
	writer.waitFlush(t)
	writer.waitFlush(t)
	body := writer.String()
	if !bytes.Contains([]byte(body), []byte("event: snapshot")) || !bytes.Contains([]byte(body), []byte("event: gpus")) {
		t.Fatalf("frames = %q", body)
	}
	cancel()
	waitDone(t, done)
	if len(server.streamSlots) != 0 {
		t.Errorf("slots = %d", len(server.streamSlots))
	}
}

func newGPUStreamServer(t *testing.T, at time.Time) *Server {
	t.Helper()
	cpuHub, err := metrics.NewHub(testSample(at))
	if err != nil {
		t.Fatal(err)
	}
	gpuHub, err := gpu.NewHub(gpu.DemoSample(at, 1))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cpuHub.Close(); gpuHub.Close() })
	server, err := New(Config{Hub: cpuHub, GPUHub: gpuHub, AllowedHost: "127.0.0.1:9090", AllowedOrigin: "http://127.0.0.1:9090", Now: func() time.Time { return at }})
	if err != nil {
		t.Fatal(err)
	}
	return server
}
