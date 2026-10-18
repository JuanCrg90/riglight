package server

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"riglight/internal/metrics"
)

func TestSSEOverHTTPDeliversInitialAndUpdatedSamples(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	server, hub, endpoint := newNetworkSSEServer(t, context.Background(), at)
	connection := openSSE(t, endpoint)
	defer connection.close()

	if frame := connection.nextFrame(t); !strings.Contains(frame, `"ageMs":0`) {
		t.Fatalf("initial frame = %q", frame)
	}
	if err := hub.Publish(testSample(at.Add(2 * time.Second))); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if frame := connection.nextFrame(t); !strings.Contains(frame, `"sampledAt":"2026-01-01T00:00:04Z"`) {
		t.Fatalf("updated frame = %q", frame)
	}
	if got := server.hub.SubscriberCount(); got != 1 {
		t.Errorf("subscriber count = %d, want 1", got)
	}
}

func TestSSEOverHTTPReleasesLimitAfterDisconnect(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	server, hub, endpoint := newNetworkSSEServer(t, context.Background(), at)
	connections := make([]*networkSSE, 0, maxSSEClients)
	for range maxSSEClients {
		connection := openSSE(t, endpoint)
		connection.nextFrame(t)
		connections = append(connections, connection)
	}
	if got := hub.SubscriberCount(); got != maxSSEClients {
		t.Fatalf("subscriber count = %d, want %d", got, maxSSEClients)
	}

	request, cancel := networkRequest(t, endpoint)
	defer cancel()
	response, err := networkClient.Do(request)
	if err != nil {
		t.Fatalf("open over-limit stream: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("over-limit status = %d, want %d", response.StatusCode, http.StatusServiceUnavailable)
	}

	connections[0].close()
	waitFor(t, func() bool {
		return hub.SubscriberCount() == maxSSEClients-1 && len(server.streamSlots) == maxSSEClients-1
	})
	replacement := openSSE(t, endpoint)
	defer replacement.close()
	replacement.nextFrame(t)

	for _, connection := range connections[1:] {
		connection.close()
	}
	waitFor(t, func() bool { return hub.SubscriberCount() == 1 && len(server.streamSlots) == 1 })
}

func TestSSEOverHTTPExitsOnApplicationCancellation(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	application, cancelApplication := context.WithCancel(context.Background())
	defer cancelApplication()
	server, hub, endpoint := newNetworkSSEServer(t, application, at)
	connection := openSSE(t, endpoint)
	defer connection.close()
	connection.nextFrame(t)

	cancelApplication()
	if _, err := connection.reader.ReadString('\n'); err != io.EOF {
		t.Fatalf("stream read after application cancellation = %v, want EOF", err)
	}
	if got := hub.SubscriberCount(); got != 0 {
		t.Errorf("subscriber count = %d, want 0", got)
	}
	if got := len(server.streamSlots); got != 0 {
		t.Errorf("stream slots = %d, want 0", got)
	}
}

var networkClient = &http.Client{Transport: &http.Transport{Proxy: nil}}

type networkSSE struct {
	cancel context.CancelFunc
	body   io.ReadCloser
	reader *bufio.Reader
}

func (connection *networkSSE) close() {
	connection.cancel()
	_ = connection.body.Close()
}

func (connection *networkSSE) nextFrame(t *testing.T) string {
	t.Helper()
	var frame strings.Builder
	for {
		line, err := connection.reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read SSE frame: %v", err)
		}
		if line == "\n" {
			return frame.String()
		}
		frame.WriteString(line)
	}
}

func newNetworkSSEServer(t *testing.T, application context.Context, at time.Time) (*Server, *metrics.Hub, *httptest.Server) {
	t.Helper()
	hub, err := metrics.NewHub(testSample(at))
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	server, err := New(Config{
		Hub:           hub,
		AllowedHost:   "127.0.0.1:9090",
		AllowedOrigin: "http://127.0.0.1:9090",
		Now:           func() time.Time { return at },
		Context:       application,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	endpoint := httptest.NewServer(server)
	t.Cleanup(func() {
		endpoint.Close()
		hub.Close()
	})
	return server, hub, endpoint
}

func networkRequest(t *testing.T, endpoint *httptest.Server) (*http.Request, context.CancelFunc) {
	t.Helper()
	context, cancel := context.WithTimeout(context.Background(), time.Second)
	request, err := http.NewRequestWithContext(context, http.MethodGet, endpoint.URL+"/api/v1/events", nil)
	if err != nil {
		cancel()
		t.Fatalf("new SSE request: %v", err)
	}
	request.Host = "127.0.0.1:9090"
	request.Header.Set("Origin", "http://127.0.0.1:9090")
	return request, cancel
}

func openSSE(t *testing.T, endpoint *httptest.Server) *networkSSE {
	t.Helper()
	request, cancel := networkRequest(t, endpoint)
	response, err := networkClient.Do(request)
	if err != nil {
		cancel()
		t.Fatalf("open SSE stream: %v", err)
	}
	if response.StatusCode != http.StatusOK {
		response.Body.Close()
		cancel()
		t.Fatalf("open SSE status = %d", response.StatusCode)
	}
	return &networkSSE{cancel: cancel, body: response.Body, reader: bufio.NewReader(response.Body)}
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		if condition() {
			return
		}
		select {
		case <-deadline.C:
			t.Fatal("condition did not become true")
		case <-ticker.C:
		}
	}
}
