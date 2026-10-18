package server

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
	"time"

	"riglight/internal/metrics"
)

func TestAssetsServeOnlyEmbeddedAllowlistedFiles(t *testing.T) {
	t.Parallel()

	server := newAssetServer(t)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, allowedRequest(http.MethodGet, "/"))
	if response.Code != http.StatusOK {
		t.Fatalf("root status = %d, want %d", response.Code, http.StatusOK)
	}
	if got := response.Body.String(); got != "<main>synthetic</main>" {
		t.Errorf("root body = %q", got)
	}

	response = httptest.NewRecorder()
	server.ServeHTTP(response, allowedRequest(http.MethodGet, "/assets/app.js"))
	if response.Code != http.StatusOK {
		t.Fatalf("script status = %d, want %d", response.Code, http.StatusOK)
	}
	if got := response.Header().Get("Content-Type"); got != "text/javascript; charset=utf-8" {
		t.Errorf("script Content-Type = %q", got)
	}
}

func TestAssetsRejectTraversalDirectoriesMapsAndFallbacks(t *testing.T) {
	t.Parallel()

	server := newAssetServer(t)
	for _, target := range []string{
		"/assets/",
		"/assets/app.js.map",
		"/assets/../index.html",
		"/not-a-route",
		"/api/v1/unknown",
	} {
		request := allowedRequest(http.MethodGet, target)
		if target == "/assets/../index.html" {
			request.URL.Path = target
		}
		response := httptest.NewRecorder()
		server.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want %d", target, response.Code, http.StatusNotFound)
		}
	}
}

func TestAssetsRejectNonGETBeforeReadingInput(t *testing.T) {
	t.Parallel()

	server := newAssetServer(t)
	request := allowedRequest(http.MethodPost, "/")
	request.ContentLength = 1
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
	if got := response.Header().Get("Allow"); got != http.MethodGet {
		t.Errorf("Allow = %q, want GET", got)
	}
}

func newAssetServer(t *testing.T) *Server {
	t.Helper()
	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	hub, err := metrics.NewHub(testSample(at))
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	t.Cleanup(hub.Close)
	assets := fstest.MapFS{
		"dist/index.html":        {Data: []byte("<main>synthetic</main>")},
		"dist/assets/app.js":     {Data: []byte("console.log('synthetic')")},
		"dist/assets/app.js.map": {Data: []byte("{}")},
	}
	server, err := New(Config{
		Hub:           hub,
		AllowedHost:   "127.0.0.1:9090",
		AllowedOrigin: "http://127.0.0.1:9090",
		Now:           func() time.Time { return at },
		Assets:        fs.FS(assets),
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	return server
}
