package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityRejectsForeignAndMalformedHosts(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	for _, host := range []string{"attacker.invalid", "127.0.0.1", "127.0.0.1:9091", "127.0.0.1:9090, attacker.invalid"} {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/snapshot", nil)
		request.Host = host
		response := httptest.NewRecorder()

		server.ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Errorf("Host %q status = %d, want %d", host, response.Code, http.StatusForbidden)
		}
		if strings.Contains(response.Body.String(), host) {
			t.Errorf("Host %q leaked in response %q", host, response.Body.String())
		}
	}
}

func TestSecurityRejectsHostileAndNullOrigins(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	for _, origin := range []string{"https://attacker.invalid", "null", "://bad", "http://127.0.0.1:9090,https://attacker.invalid"} {
		request := allowedRequest(http.MethodGet, "/api/v1/snapshot")
		request.Header.Set("Origin", origin)
		response := httptest.NewRecorder()

		server.ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Errorf("Origin %q status = %d, want %d", origin, response.Code, http.StatusForbidden)
		}
	}
}

func TestSecurityAllowsMissingOrExactOrigin(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	for _, origin := range []string{"", "http://127.0.0.1:9090"} {
		request := allowedRequest(http.MethodGet, "/healthz")
		if origin != "" {
			request.Header.Set("Origin", origin)
		}
		response := httptest.NewRecorder()

		server.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Errorf("Origin %q status = %d, want %d", origin, response.Code, http.StatusOK)
		}
	}
}

func TestSecuritySetsHeadersAndDoesNotSetCORS(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, allowedRequest(http.MethodGet, "/api/v1/snapshot"))

	for header, want := range map[string]string{
		"Cache-Control":           "no-store",
		"Content-Security-Policy": "default-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'",
		"Referrer-Policy":         "no-referrer",
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
	} {
		if got := response.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want no CORS header", got)
	}
}
