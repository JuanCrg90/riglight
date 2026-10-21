package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"riglight/internal/logs"
	"riglight/internal/metrics"
)

type sseRecorder struct{ *httptest.ResponseRecorder }

func (s *sseRecorder) SetWriteDeadline(time.Time) error { return nil }

type testLogProvider struct{ calls int }

func (p *testLogProvider) Stream(ctx context.Context, ready func() error, emit logs.Emit) error {
	p.calls++
	if err := ready(); err != nil {
		return err
	}
	if err := emit(logs.StreamOutput, "first", false); err != nil {
		return err
	}
	return nil
}

type blockingLogProvider struct{ started chan struct{} }

func (p *blockingLogProvider) Stream(ctx context.Context, ready func() error, _ logs.Emit) error {
	if err := ready(); err != nil {
		return err
	}
	p.started <- struct{}{}
	<-ctx.Done()
	return ctx.Err()
}

type countingLogCatalog struct {
	calls    atomic.Int32
	provider logs.Provider
}

func (c *countingLogCatalog) Catalog(context.Context) (logs.Catalog, error) {
	c.calls.Add(1)
	return logs.Catalog{SchemaVersion: logs.SchemaVersion, Mode: logs.ModeLive, Sources: []logs.Source{{ID: "model.demo", Label: "Demo", Kind: logs.KindModel}}}, nil
}
func (c *countingLogCatalog) Provider(context.Context, string) (logs.Provider, error) {
	return c.provider, nil
}

func TestLogFramesIncludeSSEEnvelopeInWireLimit(t *testing.T) {
	recorder := sseRecorder{ResponseRecorder: httptest.NewRecorder()}
	if err := writeLogFrame(&recorder, "log", map[string]string{"text": strings.Repeat("x", logs.MaxWireBytes)}); err == nil {
		t.Fatal("oversized frame accepted")
	}
}

func TestLogAdmissionRejectsThirdRequestBeforeCatalog(t *testing.T) {
	provider := &blockingLogProvider{started: make(chan struct{}, 2)}
	catalog := &countingLogCatalog{provider: provider}
	server := newLogTestServer(t, catalog)
	var done [2]chan struct{}
	var cancel [2]context.CancelFunc
	for index := range done {
		ctx, stop := context.WithCancel(context.Background())
		cancel[index] = stop
		done[index] = make(chan struct{})
		request := allowedRequest(http.MethodGet, "/api/v1/logs/model.demo").WithContext(ctx)
		go func(index int, request *http.Request) {
			defer close(done[index])
			response := sseRecorder{ResponseRecorder: httptest.NewRecorder()}
			server.ServeHTTP(&response, request)
		}(index, request)
	}
	for range 2 {
		select {
		case <-provider.started:
		case <-time.After(time.Second):
			t.Fatal("stream did not start")
		}
	}
	third := httptest.NewRecorder()
	server.ServeHTTP(third, allowedRequest(http.MethodGet, "/api/v1/logs/model.demo"))
	if third.Code != http.StatusTooManyRequests || catalog.calls.Load() != 2 {
		t.Fatalf("third = %d, catalog calls = %d", third.Code, catalog.calls.Load())
	}
	for index := range cancel {
		cancel[index]()
	}
	for index := range done {
		select {
		case <-done[index]:
		case <-time.After(time.Second):
			t.Fatal("slot did not release")
		}
	}
}

func TestLogRoutesAreManualBoundedAndStrict(t *testing.T) {
	provider := &testLogProvider{}
	registry, err := logs.NewRegistry(logs.ModeLive, []logs.Entry{{Source: logs.Source{ID: "model.demo", Label: "Demo", Kind: logs.KindModel}, Provider: provider}})
	if err != nil {
		t.Fatal(err)
	}
	server := newLogTestServer(t, registry)
	catalog := allowedRequest(http.MethodGet, "/api/v1/log-sources")
	catalogResponse := httptest.NewRecorder()
	server.ServeHTTP(catalogResponse, catalog)
	if catalogResponse.Code != http.StatusOK || !strings.Contains(catalogResponse.Body.String(), `"mode":"LIVE"`) {
		t.Fatalf("catalog = %d %s", catalogResponse.Code, catalogResponse.Body.String())
	}
	for _, request := range []*http.Request{
		allowedRequest(http.MethodGet, "/api/v1/logs/missing"),
		allowedRequest(http.MethodGet, "/api/v1/logs/model.demo?"),
	} {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, request)
		if response.Code == http.StatusOK {
			t.Fatalf("unsafe stream accepted: %s", request.URL)
		}
	}
	lastID := allowedRequest(http.MethodGet, "/api/v1/logs/model.demo")
	lastID.Header.Set("Last-Event-ID", "1")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, lastID)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("Last-Event-ID status = %d", response.Code)
	}
	stream := allowedRequest(http.MethodGet, "/api/v1/logs/model.demo")
	streamResponse := sseRecorder{ResponseRecorder: httptest.NewRecorder()}
	server.ServeHTTP(&streamResponse, stream)
	body := streamResponse.Body.String()
	if streamResponse.Code != http.StatusOK {
		t.Fatalf("stream = %d %s", streamResponse.Code, body)
	}
	for _, want := range []string{"event: log-status", `"state":"OPENING"`, `"state":"STREAMING"`, "event: log\n", `"seq":1`, `"state":"ENDED"`, `"reasonCode":"EOF"`} {
		if !strings.Contains(body, want) {
			t.Errorf("stream missing %q: %s", want, body)
		}
	}
	if provider.calls != 1 {
		t.Fatalf("provider calls = %d", provider.calls)
	}
}

func newLogTestServer(t *testing.T, source logs.CatalogProvider) *Server {
	t.Helper()
	at := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	hub, err := metrics.NewHub(testSample(at))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(hub.Close)
	server, err := New(Config{Hub: hub, LogSources: source, AllowedHost: "127.0.0.1:9090", AllowedOrigin: "http://127.0.0.1:9090", Now: func() time.Time { return at }})
	if err != nil {
		t.Fatal(err)
	}
	return server
}
