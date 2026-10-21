// Package llamaswap defines the bounded llama-swap inventory contract.
package llamaswap

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	SchemaVersion = 1
	MaxModels     = 128
	maxIDRunes    = 128
	maxIDBytes    = 512
)

type Mode string

const (
	ModeDisabled Mode = "DISABLED"
	ModeDemo     Mode = "DEMO"
	ModeLive     Mode = "LIVE"
)

type OverviewStatus string

const (
	OverviewDisabled    OverviewStatus = "DISABLED"
	OverviewNotReady    OverviewStatus = "NOT_READY"
	OverviewAvailable   OverviewStatus = "AVAILABLE"
	OverviewUnavailable OverviewStatus = "UNAVAILABLE"
	OverviewUnsupported OverviewStatus = "UNSUPPORTED"
)

type ReasonCode string

const (
	ReasonDisabled        ReasonCode = "DISABLED"
	ReasonAPIUnsupported  ReasonCode = "API_UNSUPPORTED"
	ReasonConnectFailed   ReasonCode = "CONNECT_FAILED"
	ReasonTimeout         ReasonCode = "TIMEOUT"
	ReasonAuthRequired    ReasonCode = "AUTH_REQUIRED"
	ReasonQueryFailed     ReasonCode = "QUERY_FAILED"
	ReasonInvalidResponse ReasonCode = "INVALID_RESPONSE"
)

type State string

const (
	StateLoaded   State = "LOADED"
	StateUnloaded State = "UNLOADED"
	StateUnknown  State = "UNKNOWN"
)

// Model is the complete, allowlisted model representation.
type Model struct {
	ID    string `json:"id"`
	State State  `json:"state"`
}

// Sample is a private inventory snapshot. completedAt retains the monotonic
// completion reading used to calculate an honest age at serialization time.
type Sample struct {
	SchemaVersion int            `json:"schemaVersion"`
	SampledAt     time.Time      `json:"sampledAt"`
	Mode          Mode           `json:"mode"`
	Status        OverviewStatus `json:"status"`
	ReasonCode    *ReasonCode    `json:"reasonCode"`
	IsTruncated   bool           `json:"isTruncated"`
	Models        []Model        `json:"models"`
	completedAt   time.Time
}

func DisabledSample(at time.Time) Sample {
	return Sample{SchemaVersion: SchemaVersion, SampledAt: at.UTC(), Mode: ModeDisabled, Status: OverviewDisabled, ReasonCode: reasonPointer(ReasonDisabled), Models: []Model{}}
}

func NotReadySample(at time.Time, mode Mode) Sample {
	return Sample{SchemaVersion: SchemaVersion, SampledAt: at.UTC(), Mode: mode, Status: OverviewNotReady, Models: []Model{}}
}

func UnsupportedSample(at time.Time, mode Mode) Sample {
	return Sample{SchemaVersion: SchemaVersion, SampledAt: at.UTC(), Mode: mode, Status: OverviewUnsupported, ReasonCode: reasonPointer(ReasonAPIUnsupported), Models: []Model{}}
}

func UnavailableSample(at time.Time, mode Mode, reason ReasonCode) Sample {
	return Sample{SchemaVersion: SchemaVersion, SampledAt: at.UTC(), Mode: mode, Status: OverviewUnavailable, ReasonCode: reasonPointer(reason), Models: []Model{}}
}

// NewAvailableSample validates every upstream-projected row before sorting and
// applying the public cap. completedAt must be captured after acquisition.
func NewAvailableSample(at, completedAt time.Time, mode Mode, models []Model) (Sample, error) {
	if at.IsZero() || completedAt.IsZero() || mode == ModeDisabled || !validMode(mode) {
		return Sample{}, errors.New("invalid available sample")
	}
	modelsCopy := make([]Model, len(models))
	copy(modelsCopy, models)
	ids := make(map[string]struct{}, len(modelsCopy))
	for _, model := range modelsCopy {
		if !validID(model.ID) || !validState(model.State) {
			return Sample{}, errors.New("invalid model")
		}
		if _, exists := ids[model.ID]; exists {
			return Sample{}, errors.New("duplicate model")
		}
		ids[model.ID] = struct{}{}
	}
	sort.Slice(modelsCopy, func(i, j int) bool { return modelsCopy[i].ID < modelsCopy[j].ID })
	truncated := len(modelsCopy) > MaxModels
	if truncated {
		modelsCopy = modelsCopy[:MaxModels]
	}
	sample := Sample{
		SchemaVersion: SchemaVersion,
		SampledAt:     at.UTC(),
		Mode:          mode,
		Status:        OverviewAvailable,
		Models:        modelsCopy,
		IsTruncated:   truncated,
		completedAt:   completedAt,
	}
	return sample, sample.Validate()
}

func (s Sample) Validate() error {
	if s.SchemaVersion != SchemaVersion || s.SampledAt.IsZero() || s.SampledAt.Location() != time.UTC || !validMode(s.Mode) || len(s.Models) > MaxModels {
		return errors.New("invalid model sample header")
	}
	if err := s.validateOverview(); err != nil {
		return err
	}
	previous := ""
	for index, model := range s.Models {
		if !validID(model.ID) || !validState(model.State) || (index > 0 && model.ID <= previous) {
			return errors.New("invalid model")
		}
		previous = model.ID
	}
	return nil
}

func (s Sample) validateOverview() error {
	empty := len(s.Models) == 0 && !s.IsTruncated
	switch s.Status {
	case OverviewDisabled:
		if s.Mode != ModeDisabled || !sameReason(s.ReasonCode, ReasonDisabled) || !empty || !s.completedAt.IsZero() {
			return errors.New("invalid disabled model overview")
		}
	case OverviewNotReady:
		if s.Mode == ModeDisabled || s.ReasonCode != nil || !empty || !s.completedAt.IsZero() {
			return errors.New("invalid not-ready model overview")
		}
	case OverviewAvailable:
		if s.Mode == ModeDisabled || s.ReasonCode != nil || s.completedAt.IsZero() || (s.IsTruncated && len(s.Models) != MaxModels) {
			return errors.New("invalid available model overview")
		}
	case OverviewUnavailable:
		if s.Mode == ModeDisabled || !unavailableReason(s.ReasonCode) || !empty || !s.completedAt.IsZero() {
			return errors.New("invalid unavailable model overview")
		}
	case OverviewUnsupported:
		if s.Mode == ModeDisabled || !sameReason(s.ReasonCode, ReasonAPIUnsupported) || !empty || !s.completedAt.IsZero() {
			return errors.New("invalid unsupported model overview")
		}
	default:
		return errors.New("invalid model overview status")
	}
	return nil
}

// Freshness reports the remaining age budget from the private monotonic
// completion reading. It never derives freshness from the public sampledAt.
func (s Sample) Freshness(now time.Time, limit time.Duration) (time.Duration, bool) {
	if s.Status != OverviewAvailable || s.completedAt.IsZero() || limit <= 0 {
		return 0, false
	}
	age := now.Sub(s.completedAt)
	if age < 0 || age >= limit {
		return 0, false
	}
	return limit - age, true
}

func (s Sample) MarshalJSON() ([]byte, error) { return s.MarshalAt(time.Now()) }

func (s Sample) MarshalAt(now time.Time) ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	type wireSample struct {
		SchemaVersion int            `json:"schemaVersion"`
		SampledAt     time.Time      `json:"sampledAt"`
		Mode          Mode           `json:"mode"`
		Status        OverviewStatus `json:"status"`
		ReasonCode    *ReasonCode    `json:"reasonCode"`
		AgeMS         *int64         `json:"ageMs"`
		IsTruncated   bool           `json:"isTruncated"`
		Models        []Model        `json:"models"`
	}
	models := s.Models
	if len(models) == 0 {
		models = []Model{}
	}
	return json.Marshal(wireSample{SchemaVersion: s.SchemaVersion, SampledAt: s.SampledAt.UTC(), Mode: s.Mode, Status: s.Status, ReasonCode: s.ReasonCode, AgeMS: ageMS(now, s.completedAt, s.Status == OverviewAvailable), IsTruncated: s.IsTruncated, Models: models})
}

func validMode(mode Mode) bool { return mode == ModeDisabled || mode == ModeDemo || mode == ModeLive }
func validState(state State) bool {
	return state == StateLoaded || state == StateUnloaded || state == StateUnknown
}
func unavailableReason(reason *ReasonCode) bool {
	return reason != nil && (*reason == ReasonConnectFailed || *reason == ReasonTimeout || *reason == ReasonAuthRequired || *reason == ReasonQueryFailed || *reason == ReasonInvalidResponse)
}
func sameReason(reason *ReasonCode, want ReasonCode) bool { return reason != nil && *reason == want }
func reasonPointer(reason ReasonCode) *ReasonCode         { return &reason }

func ageMS(now, completedAt time.Time, available bool) *int64 {
	if !available {
		return nil
	}
	age := now.Sub(completedAt).Milliseconds()
	if age < 0 {
		age = 0
	}
	return &age
}

func validID(id string) bool {
	if id == "" || !utf8.ValidString(id) || len(id) > maxIDBytes || utf8.RuneCountInString(id) > maxIDRunes || strings.TrimSpace(id) != id || strings.Contains(id, "\\") || strings.HasPrefix(id, "/") || strings.HasPrefix(id, "~") || hasDrivePath(id) || hasScheme(id) {
		return false
	}
	for _, segment := range strings.Split(id, "/") {
		if segment == ".." {
			return false
		}
	}
	for _, r := range id {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			return false
		}
	}
	return true
}

func hasDrivePath(id string) bool {
	return len(id) >= 2 && id[1] == ':' && ((id[0] >= 'A' && id[0] <= 'Z') || (id[0] >= 'a' && id[0] <= 'z'))
}

func hasScheme(id string) bool {
	if len(id) == 0 || !isASCIILetter(id[0]) {
		return false
	}
	for index := 1; index < len(id); index++ {
		character := id[index]
		if character == ':' {
			return true
		}
		if !isASCIILetter(character) && !(character >= '0' && character <= '9') && character != '+' && character != '-' && character != '.' {
			return false
		}
	}
	return false
}

func isASCIILetter(character byte) bool {
	return (character >= 'A' && character <= 'Z') || (character >= 'a' && character <= 'z')
}
