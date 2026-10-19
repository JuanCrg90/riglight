// Package containers defines the privacy-safe selected-container contract.
package containers

import (
	"encoding/json"
	"errors"
	"math"
	"sort"
	"time"
)

const (
	SchemaVersion      = 1
	MaxContainers      = 16
	maxSafeJSONInteger = uint64(1<<53 - 1)
	MaxCPUPercent      = 409600.0
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
	ReasonConnectFailed   ReasonCode = "CONNECT_FAILED"
	ReasonTimeout         ReasonCode = "TIMEOUT"
	ReasonAuthRequired    ReasonCode = "AUTH_REQUIRED"
	ReasonQueryFailed     ReasonCode = "QUERY_FAILED"
	ReasonInvalidResponse ReasonCode = "INVALID_RESPONSE"
	ReasonAPIUnsupported  ReasonCode = "API_UNSUPPORTED"
)

type MetadataStatus string

const (
	MetadataNotReady    MetadataStatus = "NOT_READY"
	MetadataAvailable   MetadataStatus = "AVAILABLE"
	MetadataUnavailable MetadataStatus = "UNAVAILABLE"
	MetadataNotFound    MetadataStatus = "NOT_FOUND"
	MetadataUnsupported MetadataStatus = "UNSUPPORTED"
)

type MetricStatus string

const (
	MetricNotReady    MetricStatus = "NOT_READY"
	MetricAvailable   MetricStatus = "AVAILABLE"
	MetricUnavailable MetricStatus = "UNAVAILABLE"
	MetricUnsupported MetricStatus = "UNSUPPORTED"
)

type State string

const (
	StateCreated    State = "CREATED"
	StateRunning    State = "RUNNING"
	StatePaused     State = "PAUSED"
	StateRestarting State = "RESTARTING"
	StateRemoving   State = "REMOVING"
	StateExited     State = "EXITED"
	StateDead       State = "DEAD"
	StateUnknown    State = "UNKNOWN"
)

type Health string

const (
	HealthStarting      Health = "STARTING"
	HealthHealthy       Health = "HEALTHY"
	HealthUnhealthy     Health = "UNHEALTHY"
	HealthNotConfigured Health = "NOT_CONFIGURED"
	HealthUnknown       Health = "UNKNOWN"
)

type Sample struct {
	SchemaVersion int            `json:"schemaVersion"`
	SampledAt     time.Time      `json:"sampledAt"`
	Mode          Mode           `json:"mode"`
	Status        OverviewStatus `json:"status"`
	ReasonCode    *ReasonCode    `json:"reasonCode"`
	Containers    []Container    `json:"containers"`
	completedAt   time.Time
}
type Container struct {
	Alias    string   `json:"alias"`
	Metadata Metadata `json:"metadata"`
	CPU      CPU      `json:"cpu"`
	Memory   Memory   `json:"memory"`
}
type Metadata struct {
	Status       MetadataStatus `json:"status"`
	SampledAt    *time.Time     `json:"sampledAt"`
	State        State          `json:"state"`
	Health       Health         `json:"health"`
	RestartCount *uint64        `json:"restartCount"`
	completedAt  time.Time
}
type CPU struct {
	Status      MetricStatus `json:"status"`
	SampledAt   *time.Time   `json:"sampledAt"`
	Percent     *float64     `json:"percent"`
	completedAt time.Time
}
type Memory struct {
	Status      MetricStatus `json:"status"`
	SampledAt   *time.Time   `json:"sampledAt"`
	UsedBytes   *uint64      `json:"usedBytes"`
	TotalBytes  *uint64      `json:"totalBytes"`
	completedAt time.Time
}

func DisabledSample(at time.Time) Sample {
	return Sample{SchemaVersion: SchemaVersion, SampledAt: at.UTC(), Mode: ModeDisabled, Status: OverviewDisabled, ReasonCode: reasonPointer(ReasonDisabled), Containers: []Container{}}
}
func NotReadySample(at time.Time, mode Mode) Sample {
	return Sample{SchemaVersion: SchemaVersion, SampledAt: at.UTC(), Mode: mode, Status: OverviewNotReady, Containers: []Container{}}
}
func UnavailableSample(at time.Time, mode Mode, reason ReasonCode) Sample {
	return Sample{SchemaVersion: SchemaVersion, SampledAt: at.UTC(), Mode: mode, Status: OverviewUnavailable, ReasonCode: reasonPointer(reason), Containers: []Container{}}
}
func UnsupportedSample(at time.Time, mode Mode) Sample {
	return Sample{SchemaVersion: SchemaVersion, SampledAt: at.UTC(), Mode: mode, Status: OverviewUnsupported, ReasonCode: reasonPointer(ReasonAPIUnsupported), Containers: []Container{}}
}

func NewAvailableSample(at, completedAt time.Time, mode Mode, rows []Container) (Sample, error) {
	copyRows := append([]Container(nil), rows...)
	sort.Slice(copyRows, func(i, j int) bool { return copyRows[i].Alias < copyRows[j].Alias })
	sample := Sample{SchemaVersion: SchemaVersion, SampledAt: at.UTC(), Mode: mode, Status: OverviewAvailable, Containers: copyRows, completedAt: completedAt}
	return sample, sample.Validate()
}
func AvailableMetadata(at, completedAt time.Time, state State, health Health, restarts *uint64) Metadata {
	return Metadata{Status: MetadataAvailable, SampledAt: timePointer(at.UTC()), State: state, Health: health, RestartCount: cloneUint64(restarts), completedAt: completedAt}
}
func UnavailableMetadata(status MetadataStatus) Metadata {
	return Metadata{Status: status, State: StateUnknown, Health: HealthUnknown}
}
func AvailableCPU(at, completedAt time.Time, percent float64) CPU {
	return CPU{Status: MetricAvailable, SampledAt: timePointer(at.UTC()), Percent: floatPointer(percent), completedAt: completedAt}
}
func NotReadyCPU() CPU    { return CPU{Status: MetricNotReady} }
func UnavailableCPU() CPU { return CPU{Status: MetricUnavailable} }
func UnsupportedCPU() CPU { return CPU{Status: MetricUnsupported} }
func AvailableMemory(at, completedAt time.Time, used, total uint64) Memory {
	return Memory{Status: MetricAvailable, SampledAt: timePointer(at.UTC()), UsedBytes: uint64Pointer(used), TotalBytes: uint64Pointer(total), completedAt: completedAt}
}
func NotReadyMemory() Memory    { return Memory{Status: MetricNotReady} }
func UnavailableMemory() Memory { return Memory{Status: MetricUnavailable} }
func UnsupportedMemory() Memory { return Memory{Status: MetricUnsupported} }

func (s Sample) Validate() error {
	if s.SchemaVersion != SchemaVersion || s.SampledAt.IsZero() || s.SampledAt.Location() != time.UTC || !validMode(s.Mode) || len(s.Containers) > MaxContainers {
		return errors.New("invalid container sample header")
	}
	switch s.Status {
	case OverviewDisabled:
		if s.Mode != ModeDisabled || !sameReason(s.ReasonCode, ReasonDisabled) || len(s.Containers) != 0 || !s.completedAt.IsZero() {
			return errors.New("invalid disabled overview")
		}
	case OverviewNotReady:
		if s.Mode == ModeDisabled || s.ReasonCode != nil || len(s.Containers) != 0 || !s.completedAt.IsZero() {
			return errors.New("invalid not-ready overview")
		}
	case OverviewAvailable:
		if s.Mode == ModeDisabled || s.ReasonCode != nil || len(s.Containers) == 0 || s.completedAt.IsZero() {
			return errors.New("invalid available overview")
		}
	case OverviewUnavailable:
		if s.Mode == ModeDisabled || !unavailableReason(s.ReasonCode) || len(s.Containers) != 0 || !s.completedAt.IsZero() {
			return errors.New("invalid unavailable overview")
		}
	case OverviewUnsupported:
		if s.Mode == ModeDisabled || !sameReason(s.ReasonCode, ReasonAPIUnsupported) || len(s.Containers) != 0 || !s.completedAt.IsZero() {
			return errors.New("invalid unsupported overview")
		}
	default:
		return errors.New("invalid overview status")
	}
	last := ""
	for i, row := range s.Containers {
		if !validAlias(row.Alias) || (i > 0 && row.Alias <= last) || row.validate() != nil {
			return errors.New("invalid container")
		}
		last = row.Alias
	}
	return nil
}
func (c Container) validate() error {
	if !validMetadataStatus(c.Metadata.Status) || !validState(c.Metadata.State) || !validHealth(c.Metadata.Health) {
		return errors.New("invalid metadata")
	}
	available := c.Metadata.Status == MetadataAvailable
	if available != (c.Metadata.SampledAt != nil && !c.Metadata.SampledAt.IsZero() && !c.Metadata.completedAt.IsZero()) {
		return errors.New("invalid metadata age")
	}
	if !available && (c.Metadata.RestartCount != nil || c.Metadata.State != StateUnknown || c.Metadata.Health != HealthUnknown) {
		return errors.New("invalid unavailable metadata values")
	}
	if c.Metadata.RestartCount != nil && *c.Metadata.RestartCount > maxSafeJSONInteger {
		return errors.New("unsafe restart count")
	}
	if err := c.CPU.validate(); err != nil {
		return err
	}
	if err := c.Memory.validate(); err != nil {
		return err
	}
	if (c.CPU.Status == MetricAvailable || c.Memory.Status == MetricAvailable) && c.Metadata.State != StateRunning {
		return errors.New("available sensor on non-running container")
	}
	return nil
}
func (c CPU) validate() error {
	available := c.Status == MetricAvailable
	if !validMetricStatus(c.Status) || available != (c.SampledAt != nil && !c.SampledAt.IsZero() && !c.completedAt.IsZero() && c.Percent != nil) || (!available && (c.SampledAt != nil || !c.completedAt.IsZero() || c.Percent != nil)) {
		return errors.New("invalid cpu")
	}
	if c.Percent != nil && (math.IsNaN(*c.Percent) || math.IsInf(*c.Percent, 0) || *c.Percent < 0 || *c.Percent > MaxCPUPercent) {
		return errors.New("invalid cpu value")
	}
	return nil
}
func (m Memory) validate() error {
	available := m.Status == MetricAvailable
	if !validMetricStatus(m.Status) || (m.UsedBytes == nil) != (m.TotalBytes == nil) || available != (m.SampledAt != nil && !m.SampledAt.IsZero() && !m.completedAt.IsZero() && m.UsedBytes != nil) || (!available && (m.SampledAt != nil || !m.completedAt.IsZero() || m.UsedBytes != nil || m.TotalBytes != nil)) {
		return errors.New("invalid memory")
	}
	if m.UsedBytes != nil && (*m.TotalBytes == 0 || *m.TotalBytes > maxSafeJSONInteger || *m.UsedBytes > *m.TotalBytes) {
		return errors.New("invalid memory values")
	}
	return nil
}

func (s Sample) MarshalJSON() ([]byte, error) { return s.MarshalAt(time.Now()) }
func (s Sample) MarshalAt(now time.Time) ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	type wireSample struct {
		SchemaVersion int             `json:"schemaVersion"`
		SampledAt     time.Time       `json:"sampledAt"`
		Mode          Mode            `json:"mode"`
		Status        OverviewStatus  `json:"status"`
		ReasonCode    *ReasonCode     `json:"reasonCode"`
		AgeMS         *int64          `json:"ageMs"`
		Containers    []wireContainer `json:"containers"`
	}
	rows := make([]wireContainer, len(s.Containers))
	for i, row := range s.Containers {
		rows[i] = row.wire(now)
	}
	return json.Marshal(wireSample{s.SchemaVersion, s.SampledAt.UTC(), s.Mode, s.Status, s.ReasonCode, ageMS(now, s.completedAt, s.Status == OverviewAvailable), rows})
}

type wireContainer struct {
	Alias    string       `json:"alias"`
	Metadata wireMetadata `json:"metadata"`
	CPU      wireCPU      `json:"cpu"`
	Memory   wireMemory   `json:"memory"`
}
type wireMetadata struct {
	Status       MetadataStatus `json:"status"`
	SampledAt    *time.Time     `json:"sampledAt"`
	AgeMS        *int64         `json:"ageMs"`
	State        State          `json:"state"`
	Health       Health         `json:"health"`
	RestartCount *uint64        `json:"restartCount"`
}
type wireCPU struct {
	Status    MetricStatus `json:"status"`
	SampledAt *time.Time   `json:"sampledAt"`
	AgeMS     *int64       `json:"ageMs"`
	Percent   *float64     `json:"percent"`
}
type wireMemory struct {
	Status     MetricStatus `json:"status"`
	SampledAt  *time.Time   `json:"sampledAt"`
	AgeMS      *int64       `json:"ageMs"`
	UsedBytes  *uint64      `json:"usedBytes"`
	TotalBytes *uint64      `json:"totalBytes"`
}

func (c Container) wire(now time.Time) wireContainer {
	return wireContainer{c.Alias, wireMetadata{c.Metadata.Status, utcPointer(c.Metadata.SampledAt), ageMS(now, c.Metadata.completedAt, c.Metadata.Status == MetadataAvailable), c.Metadata.State, c.Metadata.Health, c.Metadata.RestartCount}, wireCPU{c.CPU.Status, utcPointer(c.CPU.SampledAt), ageMS(now, c.CPU.completedAt, c.CPU.Status == MetricAvailable), c.CPU.Percent}, wireMemory{c.Memory.Status, utcPointer(c.Memory.SampledAt), ageMS(now, c.Memory.completedAt, c.Memory.Status == MetricAvailable), c.Memory.UsedBytes, c.Memory.TotalBytes}}
}
func validMode(v Mode) bool { return v == ModeDisabled || v == ModeDemo || v == ModeLive }
func validMetadataStatus(v MetadataStatus) bool {
	return v == MetadataNotReady || v == MetadataAvailable || v == MetadataUnavailable || v == MetadataNotFound || v == MetadataUnsupported
}
func validMetricStatus(v MetricStatus) bool {
	return v == MetricNotReady || v == MetricAvailable || v == MetricUnavailable || v == MetricUnsupported
}
func validState(v State) bool {
	return v == StateCreated || v == StateRunning || v == StatePaused || v == StateRestarting || v == StateRemoving || v == StateExited || v == StateDead || v == StateUnknown
}
func validHealth(v Health) bool {
	return v == HealthStarting || v == HealthHealthy || v == HealthUnhealthy || v == HealthNotConfigured || v == HealthUnknown
}
func validAlias(v string) bool {
	if len(v) == 0 || len(v) > 64 {
		return false
	}
	for i := range v {
		c := v[i]
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || (i > 0 && (c == '_' || c == '.' || c == '-'))) {
			return false
		}
	}
	return true
}
func unavailableReason(v *ReasonCode) bool {
	return v != nil && (*v == ReasonConnectFailed || *v == ReasonTimeout || *v == ReasonAuthRequired || *v == ReasonQueryFailed || *v == ReasonInvalidResponse)
}
func sameReason(a *ReasonCode, b ReasonCode) bool { return a != nil && *a == b }
func reasonPointer(v ReasonCode) *ReasonCode      { return &v }
func timePointer(v time.Time) *time.Time          { return &v }
func uint64Pointer(v uint64) *uint64              { return &v }
func floatPointer(v float64) *float64             { return &v }
func cloneUint64(v *uint64) *uint64 {
	if v == nil {
		return nil
	}
	return uint64Pointer(*v)
}
func utcPointer(v *time.Time) *time.Time {
	if v == nil {
		return nil
	}
	return timePointer(v.UTC())
}
func ageMS(now, done time.Time, available bool) *int64 {
	if !available {
		return nil
	}
	n := now.Sub(done).Milliseconds()
	if n < 0 {
		n = 0
	}
	return &n
}
