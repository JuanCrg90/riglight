// Package gpu defines the privacy-safe optional GPU telemetry contract.
package gpu

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"
)

const (
	SchemaVersion      = 1
	MaxDevices         = 16
	maxSafeJSONInteger = uint64(1<<53 - 1)
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

type Status string

const (
	StatusNotReady    Status = "NOT_READY"
	StatusAvailable   Status = "AVAILABLE"
	StatusUnavailable Status = "UNAVAILABLE"
	StatusUnsupported Status = "UNSUPPORTED"
)

type ReasonCode string

const (
	ReasonDisabled            ReasonCode = "DISABLED"
	ReasonUnsupportedPlatform ReasonCode = "UNSUPPORTED_PLATFORM"
	ReasonBackendUnavailable  ReasonCode = "BACKEND_UNAVAILABLE"
	ReasonDriverUnavailable   ReasonCode = "DRIVER_UNAVAILABLE"
	ReasonNoDevices           ReasonCode = "NO_DEVICES"
	ReasonQueryFailed         ReasonCode = "QUERY_FAILED"
	ReasonDeviceLimit         ReasonCode = "DEVICE_LIMIT"
)

// Sample contains only the numeric GPU contract permitted by the API.
type Sample struct {
	SchemaVersion int            `json:"schemaVersion"`
	SampledAt     time.Time      `json:"sampledAt"`
	Mode          Mode           `json:"mode"`
	Status        OverviewStatus `json:"status"`
	ReasonCode    *ReasonCode    `json:"reasonCode"`
	IsTruncated   bool           `json:"isTruncated"`
	Devices       []Device       `json:"devices"`
}

type Device struct {
	Index       int         `json:"index"`
	Utilization Utilization `json:"utilization"`
	Memory      Memory      `json:"memory"`
	Temperature Temperature `json:"temperature"`
	Power       Power       `json:"power"`
}

type Utilization struct {
	Status      Status     `json:"status"`
	SampledAt   *time.Time `json:"sampledAt"`
	Percent     *float64   `json:"percent"`
	collectedAt time.Time
}

type Memory struct {
	Status      Status     `json:"status"`
	SampledAt   *time.Time `json:"sampledAt"`
	UsedBytes   *uint64    `json:"usedBytes"`
	TotalBytes  *uint64    `json:"totalBytes"`
	collectedAt time.Time
}

type Temperature struct {
	Status      Status     `json:"status"`
	SampledAt   *time.Time `json:"sampledAt"`
	Celsius     *float64   `json:"celsius"`
	collectedAt time.Time
}

type Power struct {
	Status      Status     `json:"status"`
	SampledAt   *time.Time `json:"sampledAt"`
	Watts       *float64   `json:"watts"`
	collectedAt time.Time
}

func AvailableUtilization(at, collectedAt time.Time, percent float64) Utilization {
	return Utilization{Status: StatusAvailable, SampledAt: timePointer(at), Percent: float64Pointer(percent), collectedAt: collectedAt}
}
func AvailableMemory(at, collectedAt time.Time, used, total uint64) Memory {
	return Memory{Status: StatusAvailable, SampledAt: timePointer(at), UsedBytes: uint64Pointer(used), TotalBytes: uint64Pointer(total), collectedAt: collectedAt}
}
func AvailableTemperature(at, collectedAt time.Time, celsius float64) Temperature {
	return Temperature{Status: StatusAvailable, SampledAt: timePointer(at), Celsius: float64Pointer(celsius), collectedAt: collectedAt}
}
func AvailablePower(at, collectedAt time.Time, watts float64) Power {
	return Power{Status: StatusAvailable, SampledAt: timePointer(at), Watts: float64Pointer(watts), collectedAt: collectedAt}
}

func DisabledSample(at time.Time) Sample {
	return Sample{SchemaVersion: SchemaVersion, SampledAt: at, Mode: ModeDisabled, Status: OverviewDisabled, ReasonCode: reasonPointer(ReasonDisabled), Devices: []Device{}}
}

func NotReadySample(at time.Time, mode Mode) Sample {
	return Sample{SchemaVersion: SchemaVersion, SampledAt: at, Mode: mode, Status: OverviewNotReady, Devices: []Device{}}
}

func UnsupportedSample(at time.Time, mode Mode, reason ReasonCode) Sample {
	return Sample{SchemaVersion: SchemaVersion, SampledAt: at, Mode: mode, Status: OverviewUnsupported, ReasonCode: reasonPointer(reason), Devices: []Device{}}
}

func UnavailableSample(at time.Time, mode Mode) Sample {
	return Sample{SchemaVersion: SchemaVersion, SampledAt: at, Mode: mode, Status: OverviewUnavailable, ReasonCode: reasonPointer(ReasonQueryFailed), Devices: []Device{}}
}

func (s Sample) Validate() error {
	if s.SchemaVersion != SchemaVersion || s.SampledAt.IsZero() || !validMode(s.Mode) {
		return errors.New("invalid GPU sample header")
	}
	if len(s.Devices) > MaxDevices {
		return errors.New("too many GPU devices")
	}
	if err := s.validateOverview(); err != nil {
		return err
	}
	previousIndex := -1
	for _, device := range s.Devices {
		if device.Index <= previousIndex || device.Index < 0 || device.Index >= MaxDevices {
			return errors.New("invalid GPU device index")
		}
		if err := device.validate(); err != nil {
			return fmt.Errorf("device %d: %w", device.Index, err)
		}
		previousIndex = device.Index
	}
	return nil
}

func (s Sample) validateOverview() error {
	empty := len(s.Devices) == 0 && !s.IsTruncated
	switch s.Status {
	case OverviewDisabled:
		if s.Mode != ModeDisabled || !sameReason(s.ReasonCode, ReasonDisabled) || !empty {
			return errors.New("invalid disabled overview")
		}
	case OverviewNotReady:
		if s.Mode == ModeDisabled || s.ReasonCode != nil || !empty {
			return errors.New("invalid not-ready overview")
		}
	case OverviewAvailable:
		if s.Mode == ModeDisabled || len(s.Devices) == 0 ||
			(s.IsTruncated && (!sameReason(s.ReasonCode, ReasonDeviceLimit) || len(s.Devices) != MaxDevices)) ||
			(!s.IsTruncated && s.ReasonCode != nil) {
			return errors.New("invalid available overview")
		}
	case OverviewUnavailable:
		if s.Mode == ModeDisabled || !sameReason(s.ReasonCode, ReasonQueryFailed) || !empty {
			return errors.New("invalid unavailable overview")
		}
	case OverviewUnsupported:
		if s.Mode == ModeDisabled || !unsupportedReason(s.ReasonCode) || !empty {
			return errors.New("invalid unsupported overview")
		}
	default:
		return errors.New("invalid overview status")
	}
	return nil
}

func (d Device) validate() error {
	if err := d.Utilization.validate(); err != nil {
		return fmt.Errorf("utilization: %w", err)
	}
	if err := d.Memory.validate(); err != nil {
		return fmt.Errorf("memory: %w", err)
	}
	if err := d.Temperature.validate(); err != nil {
		return fmt.Errorf("temperature: %w", err)
	}
	return d.Power.validate()
}

func (m Utilization) validate() error {
	if !validMetricStatus(m.Status) || !validMetricValues(m.Status, m.SampledAt, m.collectedAt, m.Percent != nil) {
		return errors.New("invalid utilization")
	}
	if m.Percent != nil && !finiteBetween(*m.Percent, 0, 100) {
		return errors.New("invalid utilization percent")
	}
	return nil
}
func (m Memory) validate() error {
	if !validMetricStatus(m.Status) || (m.UsedBytes == nil) != (m.TotalBytes == nil) {
		return errors.New("invalid memory")
	}
	hasValues := m.UsedBytes != nil
	if !validMetricValues(m.Status, m.SampledAt, m.collectedAt, hasValues) {
		return errors.New("invalid memory")
	}
	if !hasValues {
		return nil
	}
	if *m.TotalBytes == 0 || *m.TotalBytes > maxSafeJSONInteger || *m.UsedBytes > *m.TotalBytes {
		return errors.New("invalid memory values")
	}
	return nil
}
func (m Temperature) validate() error {
	if !validMetricStatus(m.Status) || !validMetricValues(m.Status, m.SampledAt, m.collectedAt, m.Celsius != nil) {
		return errors.New("invalid temperature")
	}
	if m.Celsius != nil && !finiteBetween(*m.Celsius, -273.15, 200) {
		return errors.New("invalid temperature celsius")
	}
	return nil
}
func (m Power) validate() error {
	if !validMetricStatus(m.Status) || !validMetricValues(m.Status, m.SampledAt, m.collectedAt, m.Watts != nil) {
		return errors.New("invalid power")
	}
	if m.Watts != nil && !finiteBetween(*m.Watts, 0, 10000) {
		return errors.New("invalid power watts")
	}
	return nil
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
		IsTruncated   bool           `json:"isTruncated"`
		Devices       []wireDevice   `json:"devices"`
	}
	devices := make([]wireDevice, len(s.Devices))
	for index, device := range s.Devices {
		devices[index] = device.wire(now)
	}
	return json.Marshal(wireSample{SchemaVersion: s.SchemaVersion, SampledAt: s.SampledAt.UTC(), Mode: s.Mode, Status: s.Status, ReasonCode: s.ReasonCode, IsTruncated: s.IsTruncated, Devices: devices})
}

type wireDevice struct {
	Index       int             `json:"index"`
	Utilization wireUtilization `json:"utilization"`
	Memory      wireMemory      `json:"memory"`
	Temperature wireTemperature `json:"temperature"`
	Power       wirePower       `json:"power"`
}
type wireUtilization struct {
	Status    Status     `json:"status"`
	SampledAt *time.Time `json:"sampledAt"`
	AgeMS     *int64     `json:"ageMs"`
	Percent   *float64   `json:"percent"`
}
type wireMemory struct {
	Status     Status     `json:"status"`
	SampledAt  *time.Time `json:"sampledAt"`
	AgeMS      *int64     `json:"ageMs"`
	UsedBytes  *uint64    `json:"usedBytes"`
	TotalBytes *uint64    `json:"totalBytes"`
}
type wireTemperature struct {
	Status    Status     `json:"status"`
	SampledAt *time.Time `json:"sampledAt"`
	AgeMS     *int64     `json:"ageMs"`
	Celsius   *float64   `json:"celsius"`
}
type wirePower struct {
	Status    Status     `json:"status"`
	SampledAt *time.Time `json:"sampledAt"`
	AgeMS     *int64     `json:"ageMs"`
	Watts     *float64   `json:"watts"`
}

func (d Device) wire(now time.Time) wireDevice {
	return wireDevice{Index: d.Index, Utilization: wireUtilization{d.Utilization.Status, utcPointer(d.Utilization.SampledAt), ageMS(now, d.Utilization.collectedAt, d.Utilization.Status == StatusAvailable), d.Utilization.Percent}, Memory: wireMemory{d.Memory.Status, utcPointer(d.Memory.SampledAt), ageMS(now, d.Memory.collectedAt, d.Memory.Status == StatusAvailable), d.Memory.UsedBytes, d.Memory.TotalBytes}, Temperature: wireTemperature{d.Temperature.Status, utcPointer(d.Temperature.SampledAt), ageMS(now, d.Temperature.collectedAt, d.Temperature.Status == StatusAvailable), d.Temperature.Celsius}, Power: wirePower{d.Power.Status, utcPointer(d.Power.SampledAt), ageMS(now, d.Power.collectedAt, d.Power.Status == StatusAvailable), d.Power.Watts}}
}

func validMode(mode Mode) bool { return mode == ModeDisabled || mode == ModeDemo || mode == ModeLive }
func validMetricStatus(status Status) bool {
	return status == StatusNotReady || status == StatusAvailable || status == StatusUnavailable || status == StatusUnsupported
}
func validMetricValues(status Status, sampledAt *time.Time, collectedAt time.Time, values bool) bool {
	if status != StatusAvailable {
		return sampledAt == nil && collectedAt.IsZero() && !values
	}
	return sampledAt != nil && !sampledAt.IsZero() && !collectedAt.IsZero() && values
}
func finiteBetween(value, min, max float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= min && value <= max
}
func sameReason(reason *ReasonCode, want ReasonCode) bool { return reason != nil && *reason == want }
func unsupportedReason(reason *ReasonCode) bool {
	return reason != nil && (*reason == ReasonUnsupportedPlatform || *reason == ReasonBackendUnavailable || *reason == ReasonDriverUnavailable || *reason == ReasonNoDevices)
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
func utcPointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	utc := value.UTC()
	return &utc
}
func float64Pointer(value float64) *float64      { return &value }
func uint64Pointer(value uint64) *uint64         { return &value }
func reasonPointer(value ReasonCode) *ReasonCode { return &value }
