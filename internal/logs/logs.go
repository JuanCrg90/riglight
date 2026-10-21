// Package logs contains the bounded, typed protocol used by optional log viewers.
package logs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	SchemaVersion      = 1
	MaxCatalogBytes    = 16 << 10
	MaxSources         = 33
	MaxSourceIDBytes   = 80
	MaxLabelBytes      = 128
	MaxTextBytes       = 4096
	MaxWireBytes       = 32 << 10
	MaxInputBytes      = 2 << 20
	MaxEmittedLines    = 4096
	SessionLifetime    = 5 * time.Minute
	MaxConfiguredItems = 16
)

var (
	ErrByteLimit    = errors.New("log input byte limit")
	ErrLineLimit    = errors.New("log line limit")
	ErrSessionLimit = errors.New("log session limit")
	ErrInvalid      = errors.New("invalid log response")
	ErrUnavailable  = errors.New("log upstream unavailable")
	ErrUnsupported  = errors.New("log upstream unsupported")
	ErrRead         = errors.New("log read failed")
)

type Mode string

const (
	ModeDisabled Mode = "DISABLED"
	ModeDemo     Mode = "DEMO"
	ModeLive     Mode = "LIVE"
)

type Kind string

const (
	KindLlamaProxy Kind = "LLAMA_PROXY"
	KindModel      Kind = "MODEL"
	KindDocker     Kind = "DOCKER"
)

type Stream string

const (
	StreamOutput Stream = "OUTPUT"
	StreamStdout Stream = "STDOUT"
	StreamStderr Stream = "STDERR"
)

type State string

const (
	StateOpening      State = "OPENING"
	StateStreaming    State = "STREAMING"
	StateEnded        State = "ENDED"
	StateUnavailable  State = "UNAVAILABLE"
	StateLimitReached State = "LIMIT_REACHED"
)

type Reason string

const (
	ReasonDisabled            Reason = "DISABLED"
	ReasonUnsupported         Reason = "UNSUPPORTED"
	ReasonUpstreamUnavailable Reason = "UPSTREAM_UNAVAILABLE"
	ReasonInvalidResponse     Reason = "INVALID_RESPONSE"
	ReasonReadFailed          Reason = "READ_FAILED"
	ReasonEOF                 Reason = "EOF"
	ReasonSessionLimit        Reason = "SESSION_LIMIT"
	ReasonByteLimit           Reason = "BYTE_LIMIT"
	ReasonLineLimit           Reason = "LINE_LIMIT"
)

type Source struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Kind  Kind   `json:"kind"`
}

type Catalog struct {
	SchemaVersion int      `json:"schemaVersion"`
	Mode          Mode     `json:"mode"`
	Sources       []Source `json:"sources"`
}

type Event struct {
	SchemaVersion int    `json:"schemaVersion"`
	Mode          Mode   `json:"mode"`
	SourceID      string `json:"sourceId"`
	Seq           int64  `json:"seq"`
	Text          string `json:"text"`
	Stream        Stream `json:"stream"`
	Truncated     bool   `json:"truncated"`
}

type Status struct {
	SchemaVersion int     `json:"schemaVersion"`
	Mode          Mode    `json:"mode"`
	SourceID      string  `json:"sourceId"`
	State         State   `json:"state"`
	ReasonCode    *Reason `json:"reasonCode"`
}

// Emit receives one normalized display line. Returning an error stops the read.
type Emit func(Stream, string, bool) error

// Provider opens only after a stream route acquired its slot. ready must be
// called after the provider has received valid response headers and before text.
type Provider interface {
	Stream(context.Context, func() error, Emit) error
}

type CatalogProvider interface {
	Catalog(context.Context) (Catalog, error)
	Provider(context.Context, string) (Provider, error)
}

type Entry struct {
	Source
	Provider Provider
}

type Registry struct {
	mode      Mode
	sources   []Source
	providers map[string]Provider
}

func NewRegistry(mode Mode, entries []Entry) (*Registry, error) {
	sources := make([]Source, len(entries))
	providers := make(map[string]Provider, len(entries))
	for i, entry := range entries {
		if entry.Provider == nil {
			return nil, ErrInvalid
		}
		sources[i] = entry.Source
		providers[entry.ID] = entry.Provider
	}
	catalog := Catalog{SchemaVersion: SchemaVersion, Mode: mode, Sources: sources}
	if err := ValidateCatalog(catalog); err != nil {
		return nil, err
	}
	return &Registry{mode: mode, sources: sources, providers: providers}, nil
}

func (r *Registry) Catalog(context.Context) (Catalog, error) {
	sources := make([]Source, len(r.sources))
	copy(sources, r.sources)
	return Catalog{SchemaVersion: SchemaVersion, Mode: r.mode, Sources: sources}, nil
}
func (r *Registry) Provider(_ context.Context, id string) (Provider, error) {
	provider, ok := r.providers[id]
	if !ok {
		return nil, ErrInvalid
	}
	return provider, nil
}

func ValidateCatalog(c Catalog) error {
	if c.SchemaVersion != SchemaVersion || (c.Mode != ModeDisabled && c.Mode != ModeDemo && c.Mode != ModeLive) || c.Sources == nil || len(c.Sources) > MaxSources || (c.Mode == ModeDisabled && len(c.Sources) != 0) {
		return ErrInvalid
	}
	seen := make(map[string]struct{}, len(c.Sources))
	proxyCount, modelCount, dockerCount, total := 0, 0, 0, 0
	for _, source := range c.Sources {
		if !validID(source.ID) || !safeLabel(source.Label) {
			return ErrInvalid
		}
		if _, exists := seen[source.ID]; exists {
			return ErrInvalid
		}
		seen[source.ID] = struct{}{}
		switch source.Kind {
		case KindLlamaProxy:
			if source.ID != "llama.proxy" {
				return ErrInvalid
			}
			proxyCount++
		case KindModel:
			if !validSourceAlias(source.ID, "model.") {
				return ErrInvalid
			}
			modelCount++
		case KindDocker:
			if !validSourceAlias(source.ID, "docker.") {
				return ErrInvalid
			}
			dockerCount++
		default:
			return ErrInvalid
		}
		total += len(source.ID) + len(source.Label) + len(source.Kind) + 32
	}
	if proxyCount > 1 || modelCount > MaxConfiguredItems || dockerCount > MaxConfiguredItems || total > MaxCatalogBytes || (c.Mode == ModeDemo && !validDemoCatalog(c.Sources)) {
		return ErrInvalid
	}
	return nil
}

func safeLabel(label string) bool {
	if len(label) == 0 || len(label) > MaxLabelBytes || !utf8.ValidString(label) {
		return false
	}
	for _, value := range label {
		if (value >= 0 && value < 0x20) || (value >= 0x7f && value <= 0x9f) {
			return false
		}
	}
	return true
}
func validSourceAlias(id, prefix string) bool {
	return strings.HasPrefix(id, prefix) && ValidAlias(strings.TrimPrefix(id, prefix))
}
func validDemoCatalog(sources []Source) bool {
	if len(sources) != 3 {
		return false
	}
	kinds := map[string]Kind{}
	for _, source := range sources {
		kinds[source.ID] = source.Kind
	}
	return kinds["llama.proxy"] == KindLlamaProxy && kinds["model.example-model"] == KindModel && kinds["docker.example-api"] == KindDocker
}

func validID(id string) bool {
	if len(id) == 0 || len(id) > MaxSourceIDBytes {
		return false
	}
	for i := range id {
		c := id[i]
		if c > 0x7f || !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '.' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

func ValidAlias(value string) bool {
	if len(value) == 0 || len(value) > 64 {
		return false
	}
	for i := range value {
		c := value[i]
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || (i > 0 && (c == '_' || c == '.' || c == '-'))) {
			return false
		}
	}
	return true
}

func ValidModelReference(value string) bool {
	if len(value) == 0 || len(value) > 128 || strings.Contains(value, "/") || strings.Contains(value, "\\") || strings.Contains(value, "%") {
		return false
	}
	if value == "proxy" || value == "upstream" || value == "http" {
		return false
	}
	for i := range value {
		c := value[i]
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' || c == '.' || c == '-') {
			return false
		}
	}
	return true
}

func ReasonFor(err error) (State, *Reason) {
	switch {
	case err == nil:
		r := ReasonEOF
		return StateEnded, &r
	case errors.Is(err, ErrSessionLimit):
		r := ReasonSessionLimit
		return StateLimitReached, &r
	case errors.Is(err, ErrByteLimit):
		r := ReasonByteLimit
		return StateLimitReached, &r
	case errors.Is(err, ErrLineLimit):
		r := ReasonLineLimit
		return StateLimitReached, &r
	case errors.Is(err, ErrUnsupported):
		r := ReasonUnsupported
		return StateUnavailable, &r
	case errors.Is(err, ErrInvalid):
		r := ReasonInvalidResponse
		return StateUnavailable, &r
	case errors.Is(err, ErrRead):
		r := ReasonReadFailed
		return StateUnavailable, &r
	default:
		r := ReasonUpstreamUnavailable
		return StateUnavailable, &r
	}
}

// BudgetReader counts body bytes before any protocol framing or normalization.
type BudgetReader struct {
	reader    io.Reader
	left      int
	exhausted bool
}

func NewBudgetReader(reader io.Reader) *BudgetReader {
	return &BudgetReader{reader: reader, left: MaxInputBytes}
}
func (r *BudgetReader) Read(p []byte) (int, error) {
	if r.exhausted {
		return 0, ErrByteLimit
	}
	if len(p) == 0 {
		return 0, nil
	}
	if r.left == 0 {
		r.exhausted = true
		return 0, ErrByteLimit
	}
	if len(p) > r.left {
		p = p[:r.left]
	}
	n, err := r.reader.Read(p)
	r.left -= n
	if err != nil {
		return n, err
	}
	return n, nil
}

type lineSink struct {
	emit    Emit
	buffers map[Stream]*lineBuffer
}
type lineBuffer struct {
	data       []byte
	discarding bool
}

func NewLineSink(emit Emit) *lineSink {
	return &lineSink{emit: emit, buffers: map[Stream]*lineBuffer{}}
}
func (s *lineSink) Write(stream Stream, data []byte) error {
	if stream != StreamOutput && stream != StreamStdout && stream != StreamStderr {
		return ErrInvalid
	}
	buffer := s.buffers[stream]
	if buffer == nil {
		buffer = &lineBuffer{data: make([]byte, 0, MaxTextBytes)}
		s.buffers[stream] = buffer
	}
	for len(data) > 0 {
		index := -1
		for i, value := range data {
			if value == '\n' {
				index = i
				break
			}
		}
		if index < 0 {
			s.append(buffer, data)
			return nil
		}
		s.append(buffer, data[:index])
		if err := s.flush(stream, buffer); err != nil {
			return err
		}
		data = data[index+1:]
	}
	return nil
}
func (s *lineSink) Close() error {
	for stream, buffer := range s.buffers {
		if len(buffer.data) != 0 || buffer.discarding {
			if err := s.flush(stream, buffer); err != nil {
				return err
			}
		}
	}
	return nil
}
func (s *lineSink) append(buffer *lineBuffer, value []byte) {
	if buffer.discarding {
		return
	}
	room := MaxTextBytes - len(buffer.data)
	if room <= 0 {
		buffer.discarding = true
		return
	}
	if len(value) > room {
		buffer.data = append(buffer.data, value[:room]...)
		buffer.discarding = true
		return
	}
	buffer.data = append(buffer.data, value...)
}
func (s *lineSink) flush(stream Stream, buffer *lineBuffer) error {
	text, normalizedTruncation := normalize(buffer.data)
	truncated := buffer.discarding || normalizedTruncation
	buffer.data = buffer.data[:0]
	buffer.discarding = false
	return s.emit(stream, text, truncated)
}
func normalize(input []byte) (string, bool) {
	var output strings.Builder
	output.Grow(len(input))
	for len(input) > 0 {
		r, size := utf8.DecodeRune(input)
		if r == utf8.RuneError && size == 1 {
			output.WriteRune(utf8.RuneError)
			input = input[1:]
			continue
		}
		input = input[size:]
		if (r >= 0 && r < 0x20) || (r >= 0x7f && r <= 0x9f) {
			continue
		}
		output.WriteRune(r)
	}
	value := output.String()
	if len(value) <= MaxTextBytes {
		return value, false
	}
	for len(value) > MaxTextBytes {
		_, size := utf8.DecodeLastRuneInString(value)
		value = value[:len(value)-size]
	}
	return value, true
}

func ConsumePlain(ctx context.Context, body io.Reader, stream Stream, emit Emit) error {
	reader := NewBudgetReader(body)
	sink := NewLineSink(emit)
	buffer := make([]byte, 4096)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		count, err := reader.Read(buffer)
		if count > 0 {
			if writeErr := sink.Write(stream, buffer[:count]); writeErr != nil {
				return writeErr
			}
		}
		if err == nil {
			continue
		}
		if errors.Is(err, io.EOF) {
			return sink.Close()
		}
		if errors.Is(err, ErrByteLimit) {
			return ErrByteLimit
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("%w: %v", ErrRead, err)
	}
}
