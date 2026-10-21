package dockerproxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"riglight/internal/containers"
	"riglight/internal/logs"
)

type helperSSERecorder struct{ *httptest.ResponseRecorder }

func (s *helperSSERecorder) SetWriteDeadline(time.Time) error { return nil }

func TestLogCatalogIsOptInAndKeepsOnlyAliases(t *testing.T) {
	at := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	hub, err := containers.NewHub(containers.NotReadySample(at, containers.ModeLive))
	if err != nil {
		t.Fatal(err)
	}
	defer hub.Close()
	disabled, err := NewHTTPServer(hub, "127.0.0.1:9123", func() time.Time { return at })
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/log-sources", nil)
	request.Host = "127.0.0.1:9123"
	response := httptest.NewRecorder()
	disabled.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != `{"schemaVersion":1,"enabled":false,"aliases":[]}` {
		t.Fatalf("disabled = %d %s", response.Code, response.Body.String())
	}
	enabled, err := NewHTTPServerWithLogs(hub, "127.0.0.1:9123", func() time.Time { return at }, &LogProvider{targets: map[string]Target{"api": {Alias: "api", Reference: "private-service"}}})
	if err != nil {
		t.Fatal(err)
	}
	response = httptest.NewRecorder()
	enabled.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"enabled":true`) || strings.Contains(response.Body.String(), "private-service") {
		t.Fatalf("enabled = %d %s", response.Code, response.Body.String())
	}
}

func TestLogStreamRejectsUnknownConfiguredAliasBeforeProvider(t *testing.T) {
	var daemonRequests atomic.Int32
	socket, stop := fakeDaemon(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { daemonRequests.Add(1) }))
	defer stop()
	config, _ := NewConfig(socket, "127.0.0.1:9123", []string{"api=service"})
	provider, _ := NewLogProvider(config)
	defer provider.CloseIdleConnections()
	at := time.Now()
	hub, err := containers.NewHub(containers.NotReadySample(at, containers.ModeLive))
	if err != nil {
		t.Fatal(err)
	}
	defer hub.Close()
	server, err := NewHTTPServerWithLogs(hub, "127.0.0.1:9123", nil, provider)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/logs/missing", nil)
	request.Host = "127.0.0.1:9123"
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound || daemonRequests.Load() != 0 {
		t.Fatalf("status = %d, daemon requests = %d", response.Code, daemonRequests.Load())
	}
}

func TestLogStreamStopsAtEmittedLineLimit(t *testing.T) {
	socket, stop := fakeDaemon(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1.44/containers/service/json":
			_, _ = io.WriteString(w, `{"Id":"`+testID+`","Name":"/service","Config":{"Tty":false}}`)
		case "/v1.44/containers/" + testID + "/logs":
			w.Header().Set("Content-Type", "application/vnd.docker.raw-stream")
			for range logs.MaxEmittedLines + 1 {
				_, _ = w.Write(dockerFrame(1, []byte("\n")))
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer stop()
	config, _ := NewConfig(socket, "127.0.0.1:9123", []string{"api=service"})
	provider, _ := NewLogProvider(config)
	defer provider.CloseIdleConnections()
	hub, err := containers.NewHub(containers.NotReadySample(time.Now(), containers.ModeLive))
	if err != nil {
		t.Fatal(err)
	}
	defer hub.Close()
	server, err := NewHTTPServerWithLogs(hub, "127.0.0.1:9123", nil, provider)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/logs/api", nil)
	request.Host = "127.0.0.1:9123"
	response := helperSSERecorder{ResponseRecorder: httptest.NewRecorder()}
	server.ServeHTTP(&response, request)
	body := response.Body.String()
	if !strings.Contains(body, `"reasonCode":"LINE_LIMIT"`) || strings.Contains(body, `"seq":4097`) {
		t.Fatalf("line limit body = %s", body)
	}
}
