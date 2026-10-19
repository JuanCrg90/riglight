package server

import (
	"bufio"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

func TestAllowedClientsConfigIsFailClosedAndNormalizesMappedAddresses(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		clients []netip.Addr
		wantErr bool
	}{
		{name: "nil preserves loopback mode"},
		{name: "empty fails closed", clients: []netip.Addr{}, wantErr: true},
		{name: "invalid fails closed", clients: []netip.Addr{{}}, wantErr: true},
		{name: "public fails closed", clients: []netip.Addr{netip.MustParseAddr("192.0.2.20")}, wantErr: true},
		{name: "mapped tailnet address", clients: []netip.Addr{netip.MustParseAddr("::ffff:100.64.0.20")}},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := newTailnetServer(t, test.clients)
			if (err != nil) != test.wantErr {
				t.Fatalf("New() error = %v, want error %t", err, test.wantErr)
			}
		})
	}
}

func TestAllowedClientsProtectEveryRouteAndIgnoreForwardedHeaders(t *testing.T) {
	t.Parallel()

	server, err := newTailnetServer(t, []netip.Addr{netip.MustParseAddr("100.64.0.20")})
	if err != nil {
		t.Fatalf("new tailnet server: %v", err)
	}
	for _, path := range []string{"/", "/api/v1/snapshot", "/api/v1/events"} {
		request := tailnetRequest(http.MethodGet, path, "100.64.0.21:4321")
		request.Header.Set("X-Forwarded-For", "100.64.0.20")
		request.Header.Set("Forwarded", "for=100.64.0.20")
		response := httptest.NewRecorder()
		server.ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Errorf("denied %s status = %d, want %d", path, response.Code, http.StatusForbidden)
		}
	}

	for _, remoteAddress := range []string{"malformed", "[::ffff:100.64.0.20]:4321"} {
		request := tailnetRequest(http.MethodGet, "/api/v1/snapshot", remoteAddress)
		response := httptest.NewRecorder()
		server.ServeHTTP(response, request)
		want := http.StatusForbidden
		if remoteAddress != "malformed" {
			want = http.StatusOK
		}
		if response.Code != want {
			t.Errorf("RemoteAddr %q status = %d, want %d", remoteAddress, response.Code, want)
		}
	}

	request := tailnetRequest(http.MethodGet, "/api/v1/snapshot", "100.64.0.20:4321")
	request.Header.Set("X-Forwarded-For", "192.0.2.20")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Errorf("allowed RemoteAddr status = %d, want %d", response.Code, http.StatusOK)
	}
}

func TestTailnetHostAndOriginAreExact(t *testing.T) {
	t.Parallel()

	server, err := newTailnetServer(t, []netip.Addr{netip.MustParseAddr("100.64.0.20")})
	if err != nil {
		t.Fatalf("new tailnet server: %v", err)
	}
	for _, test := range []struct {
		name   string
		host   string
		origin string
		want   int
	}{
		{name: "exact", host: "workstation.example.ts.net:9090", origin: "http://workstation.example.ts.net:9090", want: http.StatusOK},
		{name: "listen address is not host", host: "100.64.0.10:9090", want: http.StatusForbidden},
		{name: "foreign origin", host: "workstation.example.ts.net:9090", origin: "http://100.64.0.10:9090", want: http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := tailnetRequest(http.MethodGet, "/healthz", "100.64.0.20:4321")
			request.Host = test.host
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			response := httptest.NewRecorder()
			server.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Errorf("status = %d, want %d", response.Code, test.want)
			}
		})
	}
}

func TestRawHTTPDuplicateHostIsRejectedByParser(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(newTestServer(t))
	defer server.Close()
	address := strings.TrimPrefix(server.URL, "http://")
	connection, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatalf("dial test server: %v", err)
	}
	defer connection.Close()
	if _, err := connection.Write([]byte("GET /healthz HTTP/1.1\r\nHost: 127.0.0.1:9090\r\nHost: attacker.invalid\r\nConnection: close\r\n\r\n")); err != nil {
		t.Fatalf("write raw request: %v", err)
	}
	response, err := http.ReadResponse(bufio.NewReader(connection), nil)
	if err != nil {
		t.Fatalf("read raw response: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", response.StatusCode, http.StatusBadRequest)
	}
}

func newTailnetServer(t *testing.T, clients []netip.Addr) (*Server, error) {
	t.Helper()
	sampleServer := newTestServer(t)
	return New(Config{
		Hub:            sampleServer.hub,
		AllowedHost:    "workstation.example.ts.net:9090",
		AllowedOrigin:  "http://workstation.example.ts.net:9090",
		AllowedClients: clients,
		Now:            sampleServer.now,
	})
}

func tailnetRequest(method, path, remoteAddress string) *http.Request {
	request := httptest.NewRequest(method, path, nil)
	request.Host = "workstation.example.ts.net:9090"
	request.RemoteAddr = remoteAddress
	return request
}
