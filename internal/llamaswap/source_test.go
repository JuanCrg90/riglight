package llamaswap

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCanonicalURLAcceptsOnlyLiteralLoopbackRootWithPort(t *testing.T) {
	t.Parallel()

	for input, want := range map[string]string{
		"http://127.0.0.1:1234":      "http://127.0.0.1:1234/v1/models",
		"https://[::1]:1234/":        "https://[::1]:1234/v1/models",
		"http://[0:0:0:0:0:0:0:1]:5": "http://[::1]:5/v1/models",
	} {
		got, err := CanonicalURL(input)
		if err != nil || got != want {
			t.Errorf("CanonicalURL(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
	for _, input := range []string{
		"http://localhost:1234", "http://127.0.0.1", "ftp://127.0.0.1:1234",
		"http://127.0.0.1:1234/base", "http://127.0.0.1:1234/?query=1",
		"http://user@127.0.0.1:1234", "http://127.0.0.1:1234/#fragment",
		"http://[::1%25lo]:1234", "http://0.0.0.0:1234", "http://192.0.2.1:1234",
	} {
		if _, err := CanonicalURL(input); err == nil {
			t.Errorf("CanonicalURL(%q) accepted unsafe URL", input)
		}
	}
}

func TestHTTPSourceProjectsOnlyAllowedFieldsAndStates(t *testing.T) {
	t.Parallel()

	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/v1/models" || request.URL.RawQuery != "" {
			t.Fatalf("request = %s %s", request.Method, request.URL)
		}
		_, _ = io.WriteString(w, `{"object":"list","data":[{"id":"zeta","status":{"value":"loaded","cmd":"private"},"meta":{"secret":"private"}},{"id":"alpha","status":{"value":"unloaded"},"description":"discard"},{"id":"unknown","status":{"value":"starting"}}]}`)
	}))
	defer fixture.Close()
	source, err := NewHTTPSource(fixture.URL)
	if err != nil {
		t.Fatal(err)
	}
	sample, err := source.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if sample.Status != OverviewAvailable || len(sample.Models) != 3 {
		t.Fatalf("sample = %#v", sample)
	}
	if got := sample.Models; got[0] != (Model{ID: "alpha", State: StateUnloaded}) || got[1] != (Model{ID: "unknown", State: StateUnknown}) || got[2] != (Model{ID: "zeta", State: StateLoaded}) {
		t.Errorf("models = %#v", got)
	}
	body, err := sample.MarshalAt(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"cmd", "meta", "secret", "description", "starting"} {
		if strings.Contains(string(body), forbidden) {
			t.Errorf("wire body leaked %q: %s", forbidden, body)
		}
	}
}

func TestHTTPSourceRejectsLossyEscapedIdentifierUnicode(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		id   string
		want string
	}{
		{"lone high surrogate", `"\uD800"`, ""},
		{"lone low surrogate", `"\uDC00"`, ""},
		{"high followed by high", `"\uD800\uD801"`, ""},
		{"paired astral unicode", `"org/\uD83D\uDE00"`, "org/😀"},
		{"literal replacement unicode", `"org/�"`, "org/�"},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				_, _ = io.WriteString(w, `{"data":[{"id":`+test.id+`,"status":{"value":"loaded"}}]}`)
			}))
			defer fixture.Close()
			source, err := NewHTTPSource(fixture.URL)
			if err != nil {
				t.Fatal(err)
			}
			sample, err := source.Collect(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if test.want == "" {
				if sample.Status != OverviewUnavailable || sample.ReasonCode == nil || *sample.ReasonCode != ReasonInvalidResponse {
					t.Errorf("sample = %#v", sample)
				}
				return
			}
			if sample.Status != OverviewAvailable || len(sample.Models) != 1 || sample.Models[0].ID != test.want {
				t.Errorf("sample = %#v, want id %q", sample, test.want)
			}
		})
	}

	if !validJSONStringUnicode(json.RawMessage(`"model\\uD800"`)) {
		t.Error("escaped literal backslash-u was treated as a surrogate escape")
	}
}

func TestHTTPSourceClassifiesResponsesAndMalformedBodies(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		status int
		body   string
		want   ReasonCode
	}{
		{"auth", http.StatusUnauthorized, "", ReasonAuthRequired},
		{"unsupported", http.StatusNotFound, "", ReasonAPIUnsupported},
		{"query failure", http.StatusBadGateway, "", ReasonQueryFailed},
		{"missing data", http.StatusOK, `{}`, ReasonInvalidResponse},
		{"null data", http.StatusOK, `{"data":null}`, ReasonInvalidResponse},
		{"trailing JSON", http.StatusOK, `{"data":[]} {}`, ReasonInvalidResponse},
		{"unsafe id", http.StatusOK, `{"data":[{"id":"../unsafe","status":{"value":"loaded"}}]}`, ReasonInvalidResponse},
		{"duplicates", http.StatusOK, `{"data":[{"id":"same","status":{"value":"loaded"}},{"id":"same","status":{"value":"unloaded"}}]}`, ReasonInvalidResponse},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				w.WriteHeader(test.status)
				_, _ = io.WriteString(w, test.body)
			}))
			defer fixture.Close()
			source, err := NewHTTPSource(fixture.URL)
			if err != nil {
				t.Fatal(err)
			}
			sample, err := source.Collect(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if sample.ReasonCode == nil || *sample.ReasonCode != test.want {
				t.Errorf("sample = %#v, want %s", sample, test.want)
			}
			if test.want == ReasonAPIUnsupported && sample.Status != OverviewUnsupported {
				t.Errorf("status = %s, want UNSUPPORTED", sample.Status)
			}
			if test.want != ReasonAPIUnsupported && sample.Status != OverviewUnavailable {
				t.Errorf("status = %s, want UNAVAILABLE", sample.Status)
			}
		})
	}
}

func TestHTTPSourceBoundsRedirectsHeadersAndBody(t *testing.T) {
	t.Parallel()

	var redirects atomic.Int32
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		redirects.Add(1)
		http.Redirect(w, request, "http://example.invalid/", http.StatusFound)
	}))
	defer redirect.Close()
	source, err := NewHTTPSource(redirect.URL)
	if err != nil {
		t.Fatal(err)
	}
	sample, err := source.Collect(context.Background())
	if err != nil || sample.Status != OverviewUnavailable || *sample.ReasonCode != ReasonQueryFailed || redirects.Load() != 1 {
		t.Fatalf("redirect sample = %#v, %v, hits=%d", sample, err, redirects.Load())
	}

	for index, fixture := range []*httptest.Server{
		httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			w.Header().Set("X-Bounded", strings.Repeat("x", maxResponseHeaderBytes))
			_, _ = io.WriteString(w, `{"data":[]}`)
		})),
		httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			_, _ = io.WriteString(w, strings.Repeat("x", maxResponseBodyBytes+1))
		})),
		httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) { _, _ = w.Write([]byte{'{', 0xff, '}'}) })),
	} {
		source, err := NewHTTPSource(fixture.URL)
		if err != nil {
			t.Fatal(err)
		}
		sample, err := source.Collect(context.Background())
		fixture.Close()
		if err != nil || sample.Status != OverviewUnavailable || sample.ReasonCode == nil || *sample.ReasonCode != ReasonInvalidResponse {
			t.Errorf("bounded response %d sample = %#v, %v", index, sample, err)
		}
	}
}

func TestHTTPSourceDisablesProxyAndHonorsDeadlineAndCancellation(t *testing.T) {
	t.Parallel()

	blocked := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		<-request.Context().Done()
	}))
	defer blocked.Close()
	source, err := NewHTTPSource(blocked.URL)
	if err != nil {
		t.Fatal(err)
	}
	transport, ok := source.client.Transport.(*http.Transport)
	if !ok || transport.Proxy != nil || transport.MaxResponseHeaderBytes != maxResponseHeaderBytes {
		t.Fatalf("transport = %#v", source.client.Transport)
	}
	canceledContext, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := source.Collect(canceledContext); err == nil {
		t.Fatal("canceled collection returned nil error")
	}

	start := time.Now()
	sample, err := source.Collect(context.Background())
	if err != nil || sample.Status != OverviewUnavailable || sample.ReasonCode == nil || *sample.ReasonCode != ReasonTimeout || time.Since(start) > 3*time.Second {
		t.Fatalf("timeout sample = %#v, %v", sample, err)
	}
}

func TestHTTPSourceUsesDefaultTLSVerification(t *testing.T) {
	t.Parallel()

	fixture := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) { _, _ = io.WriteString(w, `{"data":[]}`) }))
	defer fixture.Close()
	source, err := NewHTTPSource(fixture.URL)
	if err != nil {
		t.Fatal(err)
	}
	sample, err := source.Collect(context.Background())
	if err != nil || sample.Status != OverviewUnavailable || sample.ReasonCode == nil || *sample.ReasonCode != ReasonConnectFailed {
		t.Fatalf("TLS sample = %#v, %v", sample, err)
	}
}
