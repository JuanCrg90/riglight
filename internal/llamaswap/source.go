package llamaswap

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	requestTimeout         = 2 * time.Second
	maxResponseBodyBytes   = 1 << 20
	maxResponseHeaderBytes = 8 << 10
)

// Source acquires an inventory snapshot without exposing upstream details.
type Source interface {
	Collect(context.Context) (Sample, error)
}

// HTTPSource is a fixed, bounded reader for llama-swap's /v1/models API.
type HTTPSource struct {
	endpoint string
	client   *http.Client
}

func NewHTTPSource(rawURL string) (*HTTPSource, error) {
	endpoint, err := CanonicalURL(rawURL)
	if err != nil {
		return nil, errors.New("invalid llama-swap URL")
	}
	transport := &http.Transport{
		Proxy:                  nil,
		MaxResponseHeaderBytes: maxResponseHeaderBytes,
	}
	return &HTTPSource{
		endpoint: endpoint,
		client: &http.Client{
			Transport: transport,
			Timeout:   requestTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

// CanonicalURL accepts only a literal loopback URL and fixes its only path.
func CanonicalURL(rawURL string) (string, error) {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Opaque != "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") || parsed.RawPath != "" {
		return "", errors.New("invalid llama-swap URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("invalid llama-swap URL")
	}
	host := parsed.Hostname()
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() || strings.Contains(host, "%") {
		return "", errors.New("invalid llama-swap URL")
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil || port <= 0 || port > 65535 {
		return "", errors.New("invalid llama-swap URL")
	}
	return (&url.URL{Scheme: parsed.Scheme, Host: net.JoinHostPort(ip.String(), strconv.Itoa(port)), Path: "/v1/models"}).String(), nil
}

func (s *HTTPSource) Collect(ctx context.Context) (Sample, error) {
	requestContext, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestContext, http.MethodGet, s.endpoint, nil)
	if err != nil {
		return Sample{}, err
	}
	response, err := s.client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return Sample{}, ctx.Err()
		}
		if errors.Is(requestContext.Err(), context.DeadlineExceeded) {
			return UnavailableSample(time.Now(), ModeLive, ReasonTimeout), nil
		}
		if strings.Contains(err.Error(), "response header too large") || strings.Contains(err.Error(), "response headers exceeded") {
			return UnavailableSample(time.Now(), ModeLive, ReasonInvalidResponse), nil
		}
		return UnavailableSample(time.Now(), ModeLive, ReasonConnectFailed), nil
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return UnavailableSample(time.Now(), ModeLive, ReasonAuthRequired), nil
	}
	if response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusMethodNotAllowed || response.StatusCode == http.StatusNotImplemented {
		return UnsupportedSample(time.Now(), ModeLive), nil
	}
	if response.StatusCode != http.StatusOK {
		return UnavailableSample(time.Now(), ModeLive, ReasonQueryFailed), nil
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBodyBytes+1))
	if err != nil || len(body) > maxResponseBodyBytes || !utf8.Valid(body) {
		return UnavailableSample(time.Now(), ModeLive, ReasonInvalidResponse), nil
	}
	models, ok := decodeModels(body)
	if !ok {
		return UnavailableSample(time.Now(), ModeLive, ReasonInvalidResponse), nil
	}
	completedAt := time.Now()
	sample, err := NewAvailableSample(completedAt, completedAt, ModeLive, models)
	if err != nil {
		return UnavailableSample(completedAt, ModeLive, ReasonInvalidResponse), nil
	}
	return sample, nil
}

type upstreamEnvelope struct {
	Data json.RawMessage `json:"data"`
}

type upstreamModel struct {
	ID     string `json:"id"`
	Status struct {
		Value *string `json:"value"`
	} `json:"status"`
}

func decodeModels(body []byte) ([]Model, bool) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	var envelope upstreamEnvelope
	if err := decoder.Decode(&envelope); err != nil {
		return nil, false
	}
	envelope.Data = bytes.TrimSpace(envelope.Data)
	if len(envelope.Data) == 0 || envelope.Data[0] != '[' {
		return nil, false
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, false
	}
	var upstream []upstreamModel
	if err := json.Unmarshal(envelope.Data, &upstream); err != nil {
		return nil, false
	}
	models := make([]Model, len(upstream))
	for index, model := range upstream {
		models[index] = Model{ID: model.ID, State: StateUnknown}
		if model.Status.Value != nil {
			switch *model.Status.Value {
			case "loaded":
				models[index].State = StateLoaded
			case "unloaded":
				models[index].State = StateUnloaded
			}
		}
	}
	return models, true
}
