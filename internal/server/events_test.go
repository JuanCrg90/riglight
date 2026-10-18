package server

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"riglight/internal/metrics"
)

func TestEventsSendInitialSnapshotThenPublishedUpdate(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	server, hub := newStreamServer(t, at, func() time.Time { return at })
	writer := newSSEWriter()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := serveEvents(server, writer, ctx)

	writer.waitFlush(t)
	if got := writer.String(); !bytes.Contains([]byte(got), []byte("retry: 2000\nevent: snapshot\ndata: ")) || !bytes.Contains([]byte(got), []byte(`"ageMs":0`)) {
		t.Fatalf("initial frame = %q", got)
	}
	if got := writer.Header().Get("Content-Type"); got != "text/event-stream; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
	if err := hub.Publish(testSample(at.Add(2 * time.Second))); err != nil {
		t.Fatalf("publish: %v", err)
	}
	writer.waitFlush(t)

	cancel()
	waitDone(t, done)
	if got := hub.SubscriberCount(); got != 0 {
		t.Errorf("subscriber count = %d, want 0", got)
	}
}

func TestEventsEnforceConnectionLimitAndCleanup(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	server, hub := newStreamServer(t, at, func() time.Time { return at })
	cancels := make([]context.CancelFunc, 0, maxSSEClients)
	done := make([]<-chan struct{}, 0, maxSSEClients)
	for range maxSSEClients {
		writer := newSSEWriter()
		ctx, cancel := context.WithCancel(context.Background())
		cancels = append(cancels, cancel)
		done = append(done, serveEvents(server, writer, ctx))
		writer.waitFlush(t)
	}
	if got := hub.SubscriberCount(); got != maxSSEClients {
		t.Fatalf("subscriber count = %d, want %d", got, maxSSEClients)
	}

	overLimit := newSSEWriter()
	server.ServeHTTP(overLimit, allowedRequest(http.MethodGet, "/api/v1/events"))
	if got := overLimit.statusCode(); got != http.StatusServiceUnavailable {
		t.Errorf("over-limit status = %d, want %d", got, http.StatusServiceUnavailable)
	}
	if got := hub.SubscriberCount(); got != maxSSEClients {
		t.Errorf("over-limit subscriber count = %d, want %d", got, maxSSEClients)
	}

	for _, cancel := range cancels {
		cancel()
	}
	for _, streamDone := range done {
		waitDone(t, streamDone)
	}
	if got := hub.SubscriberCount(); got != 0 {
		t.Errorf("subscriber count after cancellation = %d, want 0", got)
	}
}

func TestEventsRejectFailureBeforeSubscribing(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	server, hub := newStreamServer(t, at, func() time.Time { return at })
	for _, request := range []*http.Request{
		allowedRequest(http.MethodPost, "/api/v1/events"),
		func() *http.Request {
			request := allowedRequest(http.MethodGet, "/api/v1/events")
			request.Host = "attacker.invalid"
			return request
		}(),
		func() *http.Request {
			request := allowedRequest(http.MethodGet, "/api/v1/events")
			request.Header.Set("Origin", "https://attacker.invalid")
			return request
		}(),
	} {
		writer := newSSEWriter()
		server.ServeHTTP(writer, request)
		if got := hub.SubscriberCount(); got != 0 {
			t.Errorf("subscriber count after rejected request = %d, want 0", got)
		}
	}
}

func TestEventsExitOnApplicationCancellation(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	appContext, cancelApp := context.WithCancel(context.Background())
	defer cancelApp()
	server, hub := newStreamServerWithContext(t, at, func() time.Time { return at }, appContext)
	writer := newSSEWriter()
	requestContext, cancelRequest := context.WithCancel(context.Background())
	defer cancelRequest()
	done := serveEvents(server, writer, requestContext)
	writer.waitFlush(t)

	cancelApp()
	waitDone(t, done)
	if got := hub.SubscriberCount(); got != 0 {
		t.Errorf("subscriber count = %d, want 0", got)
	}
}

func TestEventsExitWhenWriteDeadlineCannotBeSet(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	server, hub := newStreamServer(t, at, func() time.Time { return at })
	writer := newSSEWriter()
	writer.deadlineErr = errors.New("deadline failed")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := serveEvents(server, writer, ctx)
	waitDone(t, done)
	if writer.deadlineCalls == 0 {
		t.Error("stream did not set a write deadline")
	}
	if got := hub.SubscriberCount(); got != 0 {
		t.Errorf("subscriber count = %d, want 0", got)
	}
}

func TestCachedSamplesAgeAcrossSnapshotAndReconnectFrames(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	now := at.Add(time.Second)
	server, _ := newStreamServer(t, at, func() time.Time { return now })

	firstSnapshot := newSSEWriter()
	server.ServeHTTP(firstSnapshot, allowedRequest(http.MethodGet, "/api/v1/snapshot"))
	if got := firstSnapshot.String(); !bytes.Contains([]byte(got), []byte(`"ageMs":1000`)) {
		t.Fatalf("first snapshot = %q", got)
	}
	now = at.Add(3 * time.Second)
	secondSnapshot := newSSEWriter()
	server.ServeHTTP(secondSnapshot, allowedRequest(http.MethodGet, "/api/v1/snapshot"))
	if got := secondSnapshot.String(); !bytes.Contains([]byte(got), []byte(`"ageMs":3000`)) {
		t.Fatalf("second snapshot = %q", got)
	}

	now = at.Add(5 * time.Second)
	writer := newSSEWriter()
	ctx, cancel := context.WithCancel(context.Background())
	done := serveEvents(server, writer, ctx)
	writer.waitFlush(t)
	if got := writer.String(); !bytes.Contains([]byte(got), []byte(`"ageMs":5000`)) {
		t.Errorf("reconnect frame = %q", got)
	}
	cancel()
	waitDone(t, done)
}

func newStreamServer(t *testing.T, at time.Time, now func() time.Time) (*Server, *metrics.Hub) {
	t.Helper()
	return newStreamServerWithContext(t, at, now, context.Background())
}

func newStreamServerWithContext(t *testing.T, at time.Time, now func() time.Time, appContext context.Context) (*Server, *metrics.Hub) {
	t.Helper()
	hub, err := metrics.NewHub(testSample(at))
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	t.Cleanup(hub.Close)
	server, err := New(Config{
		Hub:           hub,
		AllowedHost:   "127.0.0.1:9090",
		AllowedOrigin: "http://127.0.0.1:9090",
		Now:           now,
		Context:       appContext,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	return server, hub
}

func testSample(at time.Time) metrics.Sample {
	return metrics.Sample{
		SchemaVersion: metrics.SchemaVersion,
		SampledAt:     at,
		Mode:          metrics.ModeDemo,
		CPU:           metrics.AvailableCPU(at, at, 42.5),
		Memory:        metrics.AvailableMemory(at, at, 8, 16),
	}
}

func serveEvents(server *Server, writer *sseWriter, ctx context.Context) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		request := allowedRequest(http.MethodGet, "/api/v1/events").WithContext(ctx)
		server.ServeHTTP(writer, request)
	}()
	return done
}

func waitDone(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("stream handler did not exit")
	}
}

type sseWriter struct {
	mu            sync.Mutex
	header        http.Header
	body          bytes.Buffer
	status        int
	flushes       chan struct{}
	deadlineCalls int
	deadlineErr   error
}

func newSSEWriter() *sseWriter {
	return &sseWriter{header: make(http.Header), flushes: make(chan struct{}, 16)}
}

func (w *sseWriter) Header() http.Header { return w.header }

func (w *sseWriter) WriteHeader(status int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.status == 0 {
		w.status = status
	}
}

func (w *sseWriter) Write(body []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(body)
}

func (w *sseWriter) FlushError() error {
	w.flushes <- struct{}{}
	return nil
}

func (w *sseWriter) SetWriteDeadline(time.Time) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.deadlineCalls++
	return w.deadlineErr
}

func (w *sseWriter) waitFlush(t *testing.T) {
	t.Helper()
	select {
	case <-w.flushes:
	case <-time.After(time.Second):
		t.Fatal("stream did not flush")
	}
}

func (w *sseWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.body.String()
}

func (w *sseWriter) statusCode() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.status
}
