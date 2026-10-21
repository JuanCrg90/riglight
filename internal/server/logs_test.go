package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestLogRoutesAreManualBoundedAndStrict(t *testing.T) {
	provider := &testLogProvider{}
	registry, err := logs.NewRegistry(logs.ModeDemo, []logs.Entry{{Source: logs.Source{ID: "model.demo", Label: "Demo", Kind: logs.KindModel}, Provider: provider}})
	if err != nil {
		t.Fatal(err)
	}
	server := newLogTestServer(t, registry)
	catalog := allowedRequest(http.MethodGet, "/api/v1/log-sources")
	catalogResponse := httptest.NewRecorder()
	server.ServeHTTP(catalogResponse, catalog)
	if catalogResponse.Code != http.StatusOK || !strings.Contains(catalogResponse.Body.String(), `"mode":"DEMO"`) {
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
