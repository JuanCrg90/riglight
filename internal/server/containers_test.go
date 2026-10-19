package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"riglight/internal/containers"
)

func TestContainerRouteIsOptionalAndAllowlisted(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.January, 1, 0, 0, 5, 0, time.UTC)
	server := newTestServerAt(t, at)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, allowedRequest(http.MethodGet, "/api/v1/containers"))
	if response.Code != http.StatusNotFound {
		t.Fatalf("disabled route = %d", response.Code)
	}
	sample, err := containers.NewAvailableSample(at, at, containers.ModeDemo, []containers.Container{{Alias: "app", Metadata: containers.AvailableMetadata(at, at, containers.StateRunning, containers.HealthHealthy, nil), CPU: containers.AvailableCPU(at, at, 1), Memory: containers.AvailableMemory(at, at, 1, 2)}})
	if err != nil {
		t.Fatal(err)
	}
	hub, err := containers.NewHub(sample)
	if err != nil {
		t.Fatal(err)
	}
	defer hub.Close()
	server.containersHub = hub
	response = httptest.NewRecorder()
	server.ServeHTTP(response, allowedRequest(http.MethodGet, "/api/v1/containers"))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"alias":"app"`) {
		t.Fatalf("response = %d %s", response.Code, response.Body.String())
	}
}
