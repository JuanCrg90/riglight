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
	"time"
	"unicode/utf8"
)

var catalogTimeout = 2 * time.Second

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
	dialer := &net.Dialer{Timeout: openingTimeout}
	return &HelperDockerCatalog{base: base, client: newHTTPClient(dialer.DialContext)}, nil
}
func (h *HelperDockerCatalog) CloseIdleConnections() { h.client.CloseIdleConnections() }
func (h *HelperDockerCatalog) catalog(ctx context.Context) ([]string, error) {
	catalogContext, cancel := context.WithTimeout(ctx, catalogTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(catalogContext, http.MethodGet, h.base+"/api/v1/log-sources", nil)
	if err != nil {
		return nil, ErrInvalid
	}
	response, finish, err := OpenHTTP(catalogContext, h.client, request)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, ErrUnavailable
	}
	defer finish()
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
	enabled, aliases, ok := parseHelperCatalog(body)
	if !ok {
		return nil, ErrInvalid
	}
	if !enabled {
		if len(aliases) != 0 {
			return nil, ErrInvalid
		}
		return []string{}, nil
	}
	if len(aliases) > MaxConfiguredItems {
		return nil, ErrInvalid
	}
	seen := map[string]struct{}{}
	for _, alias := range aliases {
		if !ValidAlias(alias) {
			return nil, ErrInvalid
		}
		if _, ok := seen[alias]; ok {
			return nil, ErrInvalid
		}
		seen[alias] = struct{}{}
	}
	return aliases, nil
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
	response, finish, err := OpenHTTP(ctx, p.helper.client, request)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return ErrUnavailable
	}
	defer finish()
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
	return readHelperSSE(ctx, response.Body, p.sourceID, emit)
}
func isEventStream(value string) bool {
	media, _, err := mime.ParseMediaType(value)
	return err == nil && media == "text/event-stream"
}

func readHelperSSE(ctx context.Context, body io.Reader, source string, emit Emit) error {
	if !validSourceAlias(source, "docker.") {
		return ErrInvalid
	}
	reader := bufio.NewReaderSize(NewBudgetReader(body), 4096)
	var event, data string
	var sequence int64
	opening, streaming, frameBytes := false, false, 0
	for {
		line, bytesRead, err := readSSELine(reader)
		if err != nil {
			if errors.Is(err, ErrByteLimit) {
				return ErrByteLimit
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return ErrInvalid
		}
		frameBytes += bytesRead
		if frameBytes > MaxWireBytes {
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
			value, ok := parseHelperEvent([]byte(data))
			if !streaming || !ok || value.Mode != ModeLive || value.SourceID != source || value.Seq != sequence+1 || value.Seq <= 0 || !utf8.ValidString(value.Text) || len(value.Text) > MaxTextBytes || (value.Stream != StreamOutput && value.Stream != StreamStdout && value.Stream != StreamStderr) {
				return ErrInvalid
			}
			sequence = value.Seq
			if err := emit(value.Stream, value.Text, value.Truncated); err != nil {
				return err
			}
		case "log-status":
			value, ok := parseHelperStatus([]byte(data))
			if !ok || !validRemoteStatus(value, source) {
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
		event, data, frameBytes = "", "", 0
	}
}

func readSSELine(reader *bufio.Reader) (string, int, error) {
	var line []byte
	for {
		part, err := reader.ReadSlice('\n')
		if len(line)+len(part) > MaxWireBytes {
			return "", 0, ErrInvalid
		}
		line = append(line, part...)
		if err == nil {
			return strings.TrimSuffix(strings.TrimSuffix(string(line), "\n"), "\r"), len(line), nil
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		if errors.Is(err, io.EOF) && len(line) != 0 {
			return "", 0, ErrInvalid
		}
		return "", 0, err
	}
}

func parseHelperCatalog(data []byte) (bool, []string, bool) {
	fields, ok := exactObject(data, "schemaVersion", "enabled", "aliases")
	if !ok || !exactInt(fields["schemaVersion"], SchemaVersion) {
		return false, nil, false
	}
	enabled, ok := exactBool(fields["enabled"])
	if !ok {
		return false, nil, false
	}
	aliases, ok := exactStrings(fields["aliases"])
	if !ok {
		return false, nil, false
	}
	return enabled, aliases, true
}
func parseHelperEvent(data []byte) (Event, bool) {
	fields, ok := exactObject(data, "schemaVersion", "mode", "sourceId", "seq", "text", "stream", "truncated")
	if !ok || !exactInt(fields["schemaVersion"], SchemaVersion) {
		return Event{}, false
	}
	mode, ok := exactString(fields["mode"])
	if !ok {
		return Event{}, false
	}
	source, ok := exactString(fields["sourceId"])
	if !ok {
		return Event{}, false
	}
	seq, ok := exactPositiveInt(fields["seq"])
	if !ok {
		return Event{}, false
	}
	text, ok := exactString(fields["text"])
	if !ok {
		return Event{}, false
	}
	stream, ok := exactString(fields["stream"])
	if !ok {
		return Event{}, false
	}
	truncated, ok := exactBool(fields["truncated"])
	if !ok {
		return Event{}, false
	}
	return Event{SchemaVersion: SchemaVersion, Mode: Mode(mode), SourceID: source, Seq: seq, Text: text, Stream: Stream(stream), Truncated: truncated}, true
}
func parseHelperStatus(data []byte) (Status, bool) {
	fields, ok := exactObject(data, "schemaVersion", "mode", "sourceId", "state", "reasonCode")
	if !ok || !exactInt(fields["schemaVersion"], SchemaVersion) {
		return Status{}, false
	}
	mode, ok := exactString(fields["mode"])
	if !ok {
		return Status{}, false
	}
	source, ok := exactString(fields["sourceId"])
	if !ok {
		return Status{}, false
	}
	state, ok := exactString(fields["state"])
	if !ok {
		return Status{}, false
	}
	if bytes.Equal(bytes.TrimSpace(fields["reasonCode"]), []byte("null")) {
		return Status{SchemaVersion: SchemaVersion, Mode: Mode(mode), SourceID: source, State: State(state)}, true
	}
	reason, ok := exactString(fields["reasonCode"])
	if !ok {
		return Status{}, false
	}
	value := Reason(reason)
	return Status{SchemaVersion: SchemaVersion, Mode: Mode(mode), SourceID: source, State: State(state), ReasonCode: &value}, true
}
func exactObject(data []byte, required ...string) (map[string]json.RawMessage, bool) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil {
		return nil, false
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '{' {
		return nil, false
	}
	fields := make(map[string]json.RawMessage, len(required))
	for decoder.More() {
		token, err := decoder.Token()
		key, isKey := token.(string)
		if err != nil || !isKey {
			return nil, false
		}
		if _, exists := fields[key]; exists {
			return nil, false
		}
		var value json.RawMessage
		if decoder.Decode(&value) != nil {
			return nil, false
		}
		fields[key] = value
	}
	token, err = decoder.Token()
	if err != nil {
		return nil, false
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '}' || len(fields) != len(required) {
		return nil, false
	}
	for _, key := range required {
		if _, exists := fields[key]; !exists {
			return nil, false
		}
	}
	var extra any
	return fields, errors.Is(decoder.Decode(&extra), io.EOF)
}
func exactString(raw json.RawMessage) (string, bool) {
	raw = bytes.TrimSpace(raw)
	if len(raw) < 2 || raw[0] != '"' || raw[len(raw)-1] != '"' {
		return "", false
	}
	var value string
	return value, json.Unmarshal(raw, &value) == nil
}
func exactBool(raw json.RawMessage) (bool, bool) {
	raw = bytes.TrimSpace(raw)
	if bytes.Equal(raw, []byte("true")) {
		return true, true
	}
	if bytes.Equal(raw, []byte("false")) {
		return false, true
	}
	return false, false
}
func exactInt(raw json.RawMessage, want int) bool {
	raw = bytes.TrimSpace(raw)
	var value int
	return !bytes.Equal(raw, []byte("null")) && json.Unmarshal(raw, &value) == nil && value == want
}
func exactPositiveInt(raw json.RawMessage) (int64, bool) {
	raw = bytes.TrimSpace(raw)
	var value int64
	return value, !bytes.Equal(raw, []byte("null")) && json.Unmarshal(raw, &value) == nil && value > 0
}
func exactStrings(raw json.RawMessage) ([]string, bool) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '[' {
		return nil, false
	}
	var values []string
	return values, json.Unmarshal(raw, &values) == nil && values != nil
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
