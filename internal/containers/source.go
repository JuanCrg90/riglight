package containers

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
)

const (
	proxyRequestTimeout = 2 * time.Second
	maxProxyBodyBytes   = 128 << 10
	maxProxyHeaderBytes = 8 << 10
)

type Source interface {
	Collect(context.Context) (Sample, error)
}
type HTTPSource struct {
	endpoint string
	client   *http.Client
}

func NewHTTPSource(rawURL string) (*HTTPSource, error) {
	endpoint, err := CanonicalURL(rawURL)
	if err != nil {
		return nil, errors.New("invalid Docker proxy URL")
	}
	transport := &http.Transport{Proxy: nil, MaxResponseHeaderBytes: maxProxyHeaderBytes}
	return &HTTPSource{endpoint: endpoint, client: &http.Client{Transport: transport, Timeout: proxyRequestTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func CanonicalURL(raw string) (string, error) {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil || parsed.Scheme == "" || parsed.Opaque != "" || parsed.User != nil || parsed.ForceQuery || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") || parsed.RawPath != "" {
		return "", errors.New("invalid Docker proxy URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("invalid Docker proxy URL")
	}
	ip := net.ParseIP(parsed.Hostname())
	if ip == nil || !ip.IsLoopback() || strings.Contains(parsed.Hostname(), "%") {
		return "", errors.New("invalid Docker proxy URL")
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil || port < 1 || port > 65535 {
		return "", errors.New("invalid Docker proxy URL")
	}
	return (&url.URL{Scheme: parsed.Scheme, Host: net.JoinHostPort(ip.String(), strconv.Itoa(port)), Path: "/api/v1/containers"}).String(), nil
}
func (s *HTTPSource) CloseIdleConnections() { s.client.CloseIdleConnections() }
func (s *HTTPSource) Collect(ctx context.Context) (Sample, error) {
	started := time.Now()
	requestContext, cancel := context.WithTimeout(ctx, proxyRequestTimeout)
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
		return sourceFailure(requestContext, ReasonConnectFailed), nil
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return UnavailableSample(time.Now(), ModeLive, ReasonAuthRequired), nil
	}
	if response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusNotImplemented || response.StatusCode == http.StatusMethodNotAllowed {
		return UnsupportedSample(time.Now(), ModeLive), nil
	}
	if response.StatusCode != http.StatusOK {
		return UnavailableSample(time.Now(), ModeLive, ReasonQueryFailed), nil
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxProxyBodyBytes+1))
	received := time.Now()
	if err != nil || len(body) > maxProxyBodyBytes || requestContext.Err() != nil {
		if ctx.Err() != nil {
			return Sample{}, ctx.Err()
		}
		return UnavailableSample(received, ModeLive, ReasonInvalidResponse), nil
	}
	sample, err := decodeProjected(body, received, received.Sub(started))
	if err != nil {
		return UnavailableSample(received, ModeLive, ReasonInvalidResponse), nil
	}
	return sample, nil
}
func sourceFailure(request context.Context, fallback ReasonCode) Sample {
	if errors.Is(request.Err(), context.DeadlineExceeded) {
		return UnavailableSample(time.Now(), ModeLive, ReasonTimeout)
	}
	return UnavailableSample(time.Now(), ModeLive, fallback)
}

type sourceWire struct {
	SchemaVersion int                   `json:"schemaVersion"`
	SampledAt     time.Time             `json:"sampledAt"`
	Mode          Mode                  `json:"mode"`
	Status        OverviewStatus        `json:"status"`
	ReasonCode    *ReasonCode           `json:"reasonCode"`
	AgeMS         *int64                `json:"ageMs"`
	Containers    []sourceWireContainer `json:"containers"`
}
type sourceWireContainer struct {
	Alias    string             `json:"alias"`
	Metadata sourceWireMetadata `json:"metadata"`
	CPU      sourceWireCPU      `json:"cpu"`
	Memory   sourceWireMemory   `json:"memory"`
}
type sourceWireMetadata struct {
	Status       MetadataStatus `json:"status"`
	SampledAt    *time.Time     `json:"sampledAt"`
	AgeMS        *int64         `json:"ageMs"`
	State        State          `json:"state"`
	Health       Health         `json:"health"`
	RestartCount *uint64        `json:"restartCount"`
}
type sourceWireCPU struct {
	Status    MetricStatus `json:"status"`
	SampledAt *time.Time   `json:"sampledAt"`
	AgeMS     *int64       `json:"ageMs"`
	Percent   *float64     `json:"percent"`
}
type sourceWireMemory struct {
	Status     MetricStatus `json:"status"`
	SampledAt  *time.Time   `json:"sampledAt"`
	AgeMS      *int64       `json:"ageMs"`
	UsedBytes  *uint64      `json:"usedBytes"`
	TotalBytes *uint64      `json:"totalBytes"`
}

func decodeProjected(body []byte, received time.Time, duration time.Duration) (Sample, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var wire sourceWire
	if err := decoder.Decode(&wire); err != nil {
		return Sample{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return Sample{}, errors.New("trailing response")
	}
	if wire.Mode != ModeLive || wire.Status != OverviewAvailable || wire.AgeMS == nil {
		return Sample{}, errors.New("not a live available projection")
	}
	completed, ok := completion(received, wire.AgeMS, duration)
	if !ok {
		return Sample{}, errors.New("invalid overview age")
	}
	rows := make([]Container, len(wire.Containers))
	for index, row := range wire.Containers {
		metadata, ok := importMetadata(row.Metadata, received, duration)
		if !ok {
			return Sample{}, errors.New("invalid metadata")
		}
		cpu, ok := importCPU(row.CPU, received, duration)
		if !ok {
			return Sample{}, errors.New("invalid cpu")
		}
		memory, ok := importMemory(row.Memory, received, duration)
		if !ok {
			return Sample{}, errors.New("invalid memory")
		}
		rows[index] = Container{Alias: row.Alias, Metadata: metadata, CPU: cpu, Memory: memory}
	}
	sample := Sample{SchemaVersion: wire.SchemaVersion, SampledAt: wire.SampledAt.UTC(), Mode: wire.Mode, Status: wire.Status, ReasonCode: wire.ReasonCode, Containers: rows, completedAt: completed}
	return sample, sample.Validate()
}
func completion(received time.Time, age *int64, duration time.Duration) (time.Time, bool) {
	if age == nil {
		return time.Time{}, false
	}
	if *age < 0 || *age > int64(^uint64(0)>>1)/int64(time.Millisecond) {
		return time.Time{}, false
	}
	return received.Add(-time.Duration(*age)*time.Millisecond - duration), true
}
func importMetadata(w sourceWireMetadata, received time.Time, duration time.Duration) (Metadata, bool) {
	value := Metadata{Status: w.Status, SampledAt: utcPointer(w.SampledAt), State: w.State, Health: w.Health, RestartCount: cloneUint64(w.RestartCount)}
	if w.Status != MetadataAvailable {
		return value, w.SampledAt == nil && w.AgeMS == nil
	}
	done, ok := completion(received, w.AgeMS, duration)
	value.completedAt = done
	return value, ok
}
func importCPU(w sourceWireCPU, received time.Time, duration time.Duration) (CPU, bool) {
	value := CPU{Status: w.Status, SampledAt: utcPointer(w.SampledAt), Percent: w.Percent}
	if w.Status != MetricAvailable {
		return value, w.SampledAt == nil && w.AgeMS == nil && w.Percent == nil
	}
	done, ok := completion(received, w.AgeMS, duration)
	value.completedAt = done
	return value, ok
}
func importMemory(w sourceWireMemory, received time.Time, duration time.Duration) (Memory, bool) {
	value := Memory{Status: w.Status, SampledAt: utcPointer(w.SampledAt), UsedBytes: cloneUint64(w.UsedBytes), TotalBytes: cloneUint64(w.TotalBytes)}
	if w.Status != MetricAvailable {
		return value, w.SampledAt == nil && w.AgeMS == nil && w.UsedBytes == nil && w.TotalBytes == nil
	}
	done, ok := completion(received, w.AgeMS, duration)
	value.completedAt = done
	return value, ok
}
