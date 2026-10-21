package dockerproxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"riglight/internal/containers"
)

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
