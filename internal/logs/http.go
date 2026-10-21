package logs

import (
	"context"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const maxHeaderBytes = 8 << 10

var openingTimeout = 10 * time.Second

// CanonicalBaseURL accepts exactly one literal loopback origin, with no path,
// credentials, query, fragment, hostname, or proxy behavior.
func CanonicalBaseURL(raw string) (string, error) {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil || parsed.Scheme == "" || parsed.Opaque != "" || parsed.User != nil || parsed.ForceQuery || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") || parsed.RawPath != "" {
		return "", ErrInvalid
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", ErrInvalid
	}
	host := parsed.Hostname()
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() || strings.Contains(host, "%") {
		return "", ErrInvalid
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil || port < 1 || port > 65535 {
		return "", ErrInvalid
	}
	return (&url.URL{Scheme: parsed.Scheme, Host: net.JoinHostPort(ip.String(), strconv.Itoa(port))}).String(), nil
}

func newHTTPClient(dial func(context.Context, string, string) (net.Conn, error)) *http.Client {
	transport := &http.Transport{
		Proxy:                  nil,
		DisableCompression:     true,
		MaxResponseHeaderBytes: maxHeaderBytes,
		ResponseHeaderTimeout:  openingTimeout,
		TLSHandshakeTimeout:    openingTimeout,
		DialContext:            dial,
	}
	return &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

// OpenHTTP enforces one absolute opening deadline for dialing, TLS, and
// response headers. Its returned finish function must run after Body.Close;
// stopping the timer leaves a successfully opened stream bound only to ctx.
func OpenHTTP(ctx context.Context, client *http.Client, request *http.Request) (*http.Response, func(), error) {
	opening, cancel := context.WithCancel(ctx)
	timer := time.AfterFunc(openingTimeout, cancel)
	response, err := client.Do(request.WithContext(opening))
	stopped := timer.Stop()
	timedOut := !stopped && ctx.Err() == nil
	if err != nil || timedOut || opening.Err() != nil {
		cancel()
		if response != nil {
			_ = response.Body.Close()
		}
		if timedOut {
			return nil, func() {}, context.DeadlineExceeded
		}
		if err != nil {
			return nil, func() {}, err
		}
		return nil, func() {}, context.Canceled
	}
	return response, cancel, nil
}

// NewUnixHTTPClient creates a proxy-free client restricted by a caller-owned
// Unix-socket dialer. It is used only by the privileged helper package.
func NewUnixHTTPClient(socket string) *http.Client {
	dialer := &net.Dialer{Timeout: openingTimeout}
	return newHTTPClient(func(ctx context.Context, _, _ string) (net.Conn, error) {
		return dialer.DialContext(ctx, "unix", socket)
	})
}

type PlainHTTPProvider struct {
	endpoint string
	client   *http.Client
	budget   *InputBudget
}

func NewPlainHTTPProvider(endpoint string) *PlainHTTPProvider {
	return NewPlainHTTPProviderWithBudget(endpoint, nil)
}
func NewPlainHTTPProviderWithBudget(endpoint string, budget *InputBudget) *PlainHTTPProvider {
	dialer := &net.Dialer{Timeout: openingTimeout}
	return &PlainHTTPProvider{endpoint: endpoint, client: newHTTPClient(dialer.DialContext), budget: budget}
}

func (p *PlainHTTPProvider) CloseIdleConnections() { p.client.CloseIdleConnections() }
func (p *PlainHTTPProvider) Stream(ctx context.Context, ready func() error, emit Emit) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, p.endpoint, nil)
	if err != nil {
		return ErrInvalid
	}
	response, finish, err := OpenHTTP(ctx, p.client, request)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return ErrUnavailable
	}
	defer finish()
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusMethodNotAllowed || response.StatusCode == http.StatusNotImplemented {
		return ErrUnsupported
	}
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") == "" || !isPlainContentType(response.Header.Get("Content-Type")) {
		return ErrInvalid
	}
	if err := ready(); err != nil {
		return err
	}
	if p.budget != nil {
		return ConsumePlainWithBudget(ctx, response.Body, StreamOutput, emit, p.budget)
	}
	return ConsumePlain(ctx, response.Body, StreamOutput, emit)
}

func isPlainContentType(value string) bool {
	mediaType, _, err := mimeParseMediaType(value)
	return err == nil && mediaType == "text/plain"
}

var mimeParseMediaType = mime.ParseMediaType
