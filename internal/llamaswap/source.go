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
	if err != nil || parsed.Scheme == "" || parsed.Opaque != "" || parsed.User != nil || parsed.ForceQuery || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") || parsed.RawPath != "" {
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

// CloseIdleConnections releases this source's private idle HTTP connections.
func (s *HTTPSource) CloseIdleConnections() { s.client.CloseIdleConnections() }

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
	if err != nil {
		if ctx.Err() != nil {
			return Sample{}, ctx.Err()
		}
		if errors.Is(requestContext.Err(), context.DeadlineExceeded) {
			return UnavailableSample(time.Now(), ModeLive, ReasonTimeout), nil
		}
		return UnavailableSample(time.Now(), ModeLive, ReasonInvalidResponse), nil
	}
	if ctx.Err() != nil {
		return Sample{}, ctx.Err()
	}
	if errors.Is(requestContext.Err(), context.DeadlineExceeded) {
		return UnavailableSample(time.Now(), ModeLive, ReasonTimeout), nil
	}
	if len(body) > maxResponseBodyBytes || !utf8.Valid(body) {
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
	ID     json.RawMessage `json:"id"`
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
	var rows []json.RawMessage
	if err := json.Unmarshal(envelope.Data, &rows); err != nil {
		return nil, false
	}
	models := make([]Model, len(rows))
	for index, row := range rows {
		var upstream upstreamModel
		if err := json.Unmarshal(row, &upstream); err != nil || !validJSONStringUnicode(upstream.ID) {
			return nil, false
		}
		var id string
		if err := json.Unmarshal(upstream.ID, &id); err != nil {
			return nil, false
		}
		models[index] = Model{ID: id, State: StateUnknown}
		if upstream.Status.Value != nil {
			switch *upstream.Status.Value {
			case "loaded":
				models[index].State = StateLoaded
			case "unloaded":
				models[index].State = StateUnloaded
			}
		}
	}
	return models, true
}

// validJSONStringUnicode rejects unpaired UTF-16 surrogate escapes before
// encoding/json can replace them with U+FFFD. It deliberately accepts literal
// U+FFFD and escaped backslashes, which are handled separately by ID policy.
func validJSONStringUnicode(raw json.RawMessage) bool {
	if len(raw) < 2 || raw[0] != '"' || raw[len(raw)-1] != '"' {
		return false
	}
	end := len(raw) - 1
	for index := 1; index < end; {
		if raw[index] != '\\' {
			index++
			continue
		}
		if index+1 >= end {
			return false
		}
		if raw[index+1] != 'u' {
			index += 2
			continue
		}
		if index+6 > end {
			return false
		}
		value, ok := unicodeEscape(raw[index+2 : index+6])
		if !ok {
			return false
		}
		if value >= 0xD800 && value <= 0xDBFF {
			if index+12 > end || raw[index+6] != '\\' || raw[index+7] != 'u' {
				return false
			}
			low, ok := unicodeEscape(raw[index+8 : index+12])
			if !ok || low < 0xDC00 || low > 0xDFFF {
				return false
			}
			index += 12
			continue
		}
		if value >= 0xDC00 && value <= 0xDFFF {
			return false
		}
		index += 6
	}
	return true
}

func unicodeEscape(raw []byte) (uint16, bool) {
	if len(raw) != 4 {
		return 0, false
	}
	var value uint16
	for _, character := range raw {
		value <<= 4
		switch {
		case character >= '0' && character <= '9':
			value += uint16(character - '0')
		case character >= 'a' && character <= 'f':
			value += uint16(character-'a') + 10
		case character >= 'A' && character <= 'F':
			value += uint16(character-'A') + 10
		default:
			return 0, false
		}
	}
	return value, true
}
