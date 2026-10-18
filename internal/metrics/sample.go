// Package metrics defines the privacy-safe CPU and memory sample contract.
package metrics

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"
)

const SchemaVersion = 1

type Mode string

const (
	ModeDemo Mode = "DEMO"
	ModeLive Mode = "LIVE"
)

type Status string

const (
	StatusNotReady    Status = "NOT_READY"
	StatusAvailable   Status = "AVAILABLE"
	StatusUnavailable Status = "UNAVAILABLE"
)

// Sample contains only the metrics allowed by the milestone-one API contract.
type Sample struct {
	SchemaVersion int       `json:"schemaVersion"`
	SampledAt     time.Time `json:"sampledAt"`
	Mode          Mode      `json:"mode"`
	CPU           CPU       `json:"cpu"`
	Memory        Memory    `json:"memory"`
}

type CPU struct {
	Status             Status     `json:"status"`
	SampledAt          *time.Time `json:"sampledAt"`
	UtilizationPercent *float64   `json:"utilizationPercent"`
	collectedAt        time.Time
}

type Memory struct {
	Status      Status     `json:"status"`
	SampledAt   *time.Time `json:"sampledAt"`
	UsedBytes   *uint64    `json:"usedBytes"`
	TotalBytes  *uint64    `json:"totalBytes"`
	collectedAt time.Time
}

// AvailableCPU constructs an available CPU measurement. collectedAt is retained
// privately so each API frame can calculate its age without relying on wall time.
func AvailableCPU(sampledAt, collectedAt time.Time, utilizationPercent float64) CPU {
	return CPU{
		Status:             StatusAvailable,
		SampledAt:          timePointer(sampledAt),
		UtilizationPercent: float64Value(utilizationPercent),
		collectedAt:        collectedAt,
	}
}

// AvailableMemory constructs an available memory measurement.
func AvailableMemory(sampledAt, collectedAt time.Time, usedBytes, totalBytes uint64) Memory {
	return Memory{
		Status:      StatusAvailable,
		SampledAt:   timePointer(sampledAt),
		UsedBytes:   uint64Value(usedBytes),
		TotalBytes:  uint64Value(totalBytes),
		collectedAt: collectedAt,
	}
}

// Validate rejects samples which would misrepresent unavailable data.
func (s Sample) Validate() error {
	if s.SchemaVersion != SchemaVersion {
		return errors.New("unsupported schema version")
	}
	if s.SampledAt.IsZero() {
		return errors.New("missing sample timestamp")
	}
	if s.Mode != ModeDemo && s.Mode != ModeLive {
		return errors.New("invalid sample mode")
	}
	if err := s.CPU.validate(); err != nil {
		return fmt.Errorf("cpu: %w", err)
	}
	if err := s.Memory.validate(); err != nil {
		return fmt.Errorf("memory: %w", err)
	}
	return nil
}

func (s Sample) MarshalJSON() ([]byte, error) {
	return s.MarshalAt(time.Now())
}

// MarshalAt serializes a sample with measurement age calculated at frame time.
// Production callers use MarshalJSON; the explicit time supports deterministic
// transport tests without parsing display timestamps.
func (s Sample) MarshalAt(now time.Time) ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}

	type wireSample struct {
		SchemaVersion int        `json:"schemaVersion"`
		SampledAt     time.Time  `json:"sampledAt"`
		Mode          Mode       `json:"mode"`
		CPU           wireCPU    `json:"cpu"`
		Memory        wireMemory `json:"memory"`
	}
	return json.Marshal(wireSample{
		SchemaVersion: s.SchemaVersion,
		SampledAt:     s.SampledAt.UTC(),
		Mode:          s.Mode,
		CPU:           s.CPU.wire(now),
		Memory:        s.Memory.wire(now),
	})
}

type wireCPU struct {
	Status             Status     `json:"status"`
	SampledAt          *time.Time `json:"sampledAt"`
	AgeMS              *int64     `json:"ageMs"`
	UtilizationPercent *float64   `json:"utilizationPercent"`
}

func (c CPU) validate() error {
	if !validStatus(c.Status) {
		return errors.New("invalid status")
	}
	if c.Status != StatusAvailable {
		if c.SampledAt != nil || c.UtilizationPercent != nil || !c.collectedAt.IsZero() {
			return errors.New("unavailable measurements must be nil")
		}
		return nil
	}
	if c.SampledAt == nil || c.SampledAt.IsZero() || c.collectedAt.IsZero() || c.UtilizationPercent == nil {
		return errors.New("missing available measurement")
	}
	if math.IsNaN(*c.UtilizationPercent) || math.IsInf(*c.UtilizationPercent, 0) || *c.UtilizationPercent < 0 || *c.UtilizationPercent > 100 {
		return errors.New("invalid utilization percentage")
	}
	return nil
}

func (c CPU) wire(now time.Time) wireCPU {
	var sampledAt *time.Time
	if c.SampledAt != nil {
		sampledAt = timePointer(c.SampledAt.UTC())
	}
	return wireCPU{
		Status:             c.Status,
		SampledAt:          sampledAt,
		AgeMS:              ageMS(now, c.collectedAt, c.Status == StatusAvailable),
		UtilizationPercent: c.UtilizationPercent,
	}
}

type wireMemory struct {
	Status     Status     `json:"status"`
	SampledAt  *time.Time `json:"sampledAt"`
	AgeMS      *int64     `json:"ageMs"`
	UsedBytes  *uint64    `json:"usedBytes"`
	TotalBytes *uint64    `json:"totalBytes"`
}

func (m Memory) validate() error {
	if !validStatus(m.Status) {
		return errors.New("invalid status")
	}
	if m.Status != StatusAvailable {
		if m.SampledAt != nil || m.UsedBytes != nil || m.TotalBytes != nil || !m.collectedAt.IsZero() {
			return errors.New("unavailable measurements must be nil")
		}
		return nil
	}
	if m.SampledAt == nil || m.SampledAt.IsZero() || m.collectedAt.IsZero() || m.UsedBytes == nil || m.TotalBytes == nil {
		return errors.New("missing available measurement")
	}
	if *m.TotalBytes == 0 || *m.TotalBytes > maxSafeJSONInteger || *m.UsedBytes > *m.TotalBytes {
		return errors.New("invalid memory measurements")
	}
	return nil
}

func (m Memory) wire(now time.Time) wireMemory {
	var sampledAt *time.Time
	if m.SampledAt != nil {
		sampledAt = timePointer(m.SampledAt.UTC())
	}
	return wireMemory{
		Status:     m.Status,
		SampledAt:  sampledAt,
		AgeMS:      ageMS(now, m.collectedAt, m.Status == StatusAvailable),
		UsedBytes:  m.UsedBytes,
		TotalBytes: m.TotalBytes,
	}
}

func ageMS(now, collectedAt time.Time, available bool) *int64 {
	if !available {
		return nil
	}
	age := now.Sub(collectedAt).Milliseconds()
	if age < 0 {
		age = 0
	}
	return &age
}

func timePointer(value time.Time) *time.Time { return &value }

func float64Value(value float64) *float64 { return &value }

func uint64Value(value uint64) *uint64 { return &value }

func validStatus(status Status) bool {
	return status == StatusNotReady || status == StatusAvailable || status == StatusUnavailable
}
