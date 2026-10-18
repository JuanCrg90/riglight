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
}

type Memory struct {
	Status     Status     `json:"status"`
	SampledAt  *time.Time `json:"sampledAt"`
	UsedBytes  *uint64    `json:"usedBytes"`
	TotalBytes *uint64    `json:"totalBytes"`
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
		CPU:           s.CPU.wire(),
		Memory:        s.Memory.wire(),
	})
}

type wireCPU struct {
	Status             Status     `json:"status"`
	SampledAt          *time.Time `json:"sampledAt"`
	UtilizationPercent *float64   `json:"utilizationPercent"`
}

func (c CPU) validate() error {
	if !validStatus(c.Status) {
		return errors.New("invalid status")
	}
	if c.Status != StatusAvailable {
		if c.SampledAt != nil || c.UtilizationPercent != nil {
			return errors.New("unavailable measurements must be nil")
		}
		return nil
	}
	if c.SampledAt == nil || c.SampledAt.IsZero() || c.UtilizationPercent == nil {
		return errors.New("missing available measurement")
	}
	if math.IsNaN(*c.UtilizationPercent) || math.IsInf(*c.UtilizationPercent, 0) || *c.UtilizationPercent < 0 || *c.UtilizationPercent > 100 {
		return errors.New("invalid utilization percentage")
	}
	return nil
}

func (c CPU) wire() wireCPU {
	if c.SampledAt != nil {
		at := c.SampledAt.UTC()
		c.SampledAt = &at
	}
	return wireCPU(c)
}

type wireMemory struct {
	Status     Status     `json:"status"`
	SampledAt  *time.Time `json:"sampledAt"`
	UsedBytes  *uint64    `json:"usedBytes"`
	TotalBytes *uint64    `json:"totalBytes"`
}

func (m Memory) validate() error {
	if !validStatus(m.Status) {
		return errors.New("invalid status")
	}
	if m.Status != StatusAvailable {
		if m.SampledAt != nil || m.UsedBytes != nil || m.TotalBytes != nil {
			return errors.New("unavailable measurements must be nil")
		}
		return nil
	}
	if m.SampledAt == nil || m.SampledAt.IsZero() || m.UsedBytes == nil || m.TotalBytes == nil {
		return errors.New("missing available measurement")
	}
	if *m.TotalBytes == 0 || *m.UsedBytes > *m.TotalBytes {
		return errors.New("invalid memory measurements")
	}
	return nil
}

func (m Memory) wire() wireMemory {
	if m.SampledAt != nil {
		at := m.SampledAt.UTC()
		m.SampledAt = &at
	}
	return wireMemory(m)
}

func validStatus(status Status) bool {
	return status == StatusNotReady || status == StatusAvailable || status == StatusUnavailable
}
