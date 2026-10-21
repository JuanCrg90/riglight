package logs

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"strings"
	"unicode/utf8"
)

type helperCatalogWire struct {
	SchemaVersion int      `json:"schemaVersion"`
	Enabled       bool     `json:"enabled"`
	Aliases       []string `json:"aliases"`
}

// HelperDockerCatalog is the unprivileged dashboard-side view of the helper.
type HelperDockerCatalog struct {
	base   string
	client *http.Client
}

func NewHelperDockerCatalog(raw string) (*HelperDockerCatalog, error) {
	base, err := CanonicalBaseURL(raw)
	if err != nil {
		return nil, err
	}
	return &HelperDockerCatalog{base: base, client: newHTTPClient((&net.Dialer{}).DialContext)}, nil
}
func (h *HelperDockerCatalog) CloseIdleConnections() { h.client.CloseIdleConnections() }
func (h *HelperDockerCatalog) catalog(ctx context.Context) ([]string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, h.base+"/api/v1/log-sources", nil)
	if err != nil {
		return nil, ErrInvalid
	}
	response, err := h.client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, ErrUnavailable
	}
	media, _, contentTypeErr := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if contentTypeErr != nil || media != "application/json" {
		return nil, ErrInvalid
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, MaxCatalogBytes+1))
	if err != nil || len(body) > MaxCatalogBytes {
		return nil, ErrInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var wire helperCatalogWire
	if err := decoder.Decode(&wire); err != nil {
		return nil, ErrInvalid
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) || wire.SchemaVersion != SchemaVersion || wire.Aliases == nil {
		return nil, ErrInvalid
	}
	if !wire.Enabled {
		if len(wire.Aliases) != 0 {
			return nil, ErrInvalid
		}
		return []string{}, nil
	}
	if len(wire.Aliases) > MaxConfiguredItems {
		return nil, ErrInvalid
	}
	seen := map[string]struct{}{}
	for _, alias := range wire.Aliases {
		if !ValidAlias(alias) {
			return nil, ErrInvalid
		}
		if _, ok := seen[alias]; ok {
			return nil, ErrInvalid
		}
		seen[alias] = struct{}{}
	}
	return wire.Aliases, nil
}

// CompositeCatalog keeps optional helper failures from hiding independent llama
// sources. A helper alias is rebound immediately before its stream opens.
type CompositeCatalog struct {
	static *Registry
	helper *HelperDockerCatalog
}

func NewCompositeCatalog(static *Registry, helper *HelperDockerCatalog) *CompositeCatalog {
	return &CompositeCatalog{static: static, helper: helper}
}
func (c *CompositeCatalog) Catalog(ctx context.Context) (Catalog, error) {
	base, err := c.static.Catalog(ctx)
	if err != nil {
		return Catalog{}, err
	}
	if c.helper == nil {
		return base, nil
	}
	aliases, err := c.helper.catalog(ctx)
	if err != nil {
		return base, nil
	}
	for _, alias := range aliases {
		base.Sources = append(base.Sources, Source{ID: "docker." + alias, Label: alias, Kind: KindDocker})
	}
	if err := ValidateCatalog(base); err != nil {
		return Catalog{}, err
	}
	return base, nil
}
func (c *CompositeCatalog) Provider(ctx context.Context, id string) (Provider, error) {
	if provider, err := c.static.Provider(ctx, id); err == nil {
		return provider, nil
	}
	if c.helper == nil || !strings.HasPrefix(id, "docker.") {
		return nil, ErrInvalid
	}
	alias := strings.TrimPrefix(id, "docker.")
	if alias == "" {
		return nil, ErrInvalid
	}
	aliases, err := c.helper.catalog(ctx)
	if err != nil {
		return nil, err
	}
	for _, candidate := range aliases {
		if candidate == alias {
			return &helperDockerProvider{helper: c.helper, alias: candidate, sourceID: id}, nil
		}
	}
	return nil, ErrInvalid
}

type helperDockerProvider struct {
	helper          *HelperDockerCatalog
	alias, sourceID string
}

func (p *helperDockerProvider) Stream(ctx context.Context, ready func() error, emit Emit) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, p.helper.base+"/api/v1/logs/"+p.alias, nil)
	if err != nil {
		return ErrInvalid
	}
	response, err := p.helper.client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return ErrInvalid
	}
	if response.StatusCode == http.StatusTooManyRequests {
		return ErrUnavailable
	}
	if response.StatusCode != http.StatusOK || !isEventStream(response.Header.Get("Content-Type")) {
		return ErrInvalid
	}
	if err := ready(); err != nil {
		return err
	}
	return readHelperSSE(ctx, response.Body, p.alias, emit)
}
func isEventStream(value string) bool {
	media, _, err := mime.ParseMediaType(value)
	return err == nil && media == "text/event-stream"
}

func readHelperSSE(ctx context.Context, body io.Reader, source string, emit Emit) error {
	reader := bufio.NewReaderSize(NewBudgetReader(body), 4096)
	var event, data string
	var sequence int64
	opening, streaming := false, false
	for {
		line, err := readSSELine(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			if errors.Is(err, ErrByteLimit) {
				return ErrByteLimit
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return ErrInvalid
		}
		if line != "" {
			key, value, ok := strings.Cut(line, ":")
			if !ok || (key != "event" && key != "data") || (key == "event" && event != "") || (key == "data" && data != "") {
				return ErrInvalid
			}
			value = strings.TrimPrefix(value, " ")
			if key == "event" {
				event = value
			} else {
				data = value
			}
			continue
		}
		if event == "" || data == "" {
			return ErrInvalid
		}
		switch event {
		case "log":
			var value Event
			if !streaming || !strictJSON([]byte(data), &value) || value.SchemaVersion != SchemaVersion || value.Mode != ModeLive || value.SourceID != source || value.Seq != sequence+1 || value.Seq <= 0 || !utf8.ValidString(value.Text) || len(value.Text) > MaxTextBytes || (value.Stream != StreamOutput && value.Stream != StreamStdout && value.Stream != StreamStderr) {
				return ErrInvalid
			}
			sequence = value.Seq
			if err := emit(value.Stream, value.Text, value.Truncated); err != nil {
				return err
			}
		case "log-status":
			var value Status
			if !strictJSON([]byte(data), &value) || !validRemoteStatus(value, source) {
				return ErrInvalid
			}
			switch value.State {
			case StateOpening:
				if opening {
					return ErrInvalid
				}
				opening = true
			case StateStreaming:
				if !opening || streaming {
					return ErrInvalid
				}
				streaming = true
			case StateEnded, StateLimitReached:
				if !opening || !streaming {
					return ErrInvalid
				}
				return errorForReason(*value.ReasonCode)
			case StateUnavailable:
				if !opening {
					return ErrInvalid
				}
				return errorForReason(*value.ReasonCode)
			}
		default:
			return ErrInvalid
		}
		event, data = "", ""
	}
}
func readSSELine(reader *bufio.Reader) (string, error) {
	var line []byte
	for {
		part, err := reader.ReadSlice('\n')
		if len(line)+len(part) > MaxWireBytes {
			return "", ErrInvalid
		}
		line = append(line, part...)
		if err == nil {
			return strings.TrimSuffix(strings.TrimSuffix(string(line), "\n"), "\r"), nil
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		return "", err
	}
}
func strictJSON(data []byte, target any) bool {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(target) != nil {
		return false
	}
	var extra any
	return errors.Is(decoder.Decode(&extra), io.EOF)
}
func validRemoteStatus(value Status, source string) bool {
	if value.SchemaVersion != SchemaVersion || value.Mode != ModeLive || value.SourceID != source {
		return false
	}
	switch value.State {
	case StateOpening, StateStreaming:
		return value.ReasonCode == nil
	case StateEnded:
		return value.ReasonCode != nil && *value.ReasonCode == ReasonEOF
	case StateUnavailable:
		return value.ReasonCode != nil && (*value.ReasonCode == ReasonDisabled || *value.ReasonCode == ReasonUnsupported || *value.ReasonCode == ReasonUpstreamUnavailable || *value.ReasonCode == ReasonInvalidResponse || *value.ReasonCode == ReasonReadFailed)
	case StateLimitReached:
		return value.ReasonCode != nil && (*value.ReasonCode == ReasonSessionLimit || *value.ReasonCode == ReasonByteLimit || *value.ReasonCode == ReasonLineLimit)
	}
	return false
}
func errorForReason(reason Reason) error {
	switch reason {
	case ReasonEOF:
		return nil
	case ReasonUnsupported:
		return ErrUnsupported
	case ReasonInvalidResponse:
		return ErrInvalid
	case ReasonReadFailed:
		return ErrRead
	case ReasonSessionLimit:
		return ErrSessionLimit
	case ReasonByteLimit:
		return ErrByteLimit
	case ReasonLineLimit:
		return ErrLineLimit
	default:
		return ErrUnavailable
	}
}
