// Package systemmetrics defines the privacy-safe optional system detail contract.
package systemmetrics

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"
)

const (
	SchemaVersion      = 1
	maxSafeJSONInteger = uint64(1<<53 - 1)
)

type Mode string

const (
	ModeDisabled Mode = "DISABLED"
	ModeDemo     Mode = "DEMO"
	ModeLive     Mode = "LIVE"
)

type Status string

const (
	StatusDisabled    Status = "DISABLED"
	StatusNotReady    Status = "NOT_READY"
	StatusAvailable   Status = "AVAILABLE"
	StatusUnavailable Status = "UNAVAILABLE"
	StatusUnsupported Status = "UNSUPPORTED"
)

// Sample exposes only the approved aggregate system detail measurements.
type Sample struct {
	SchemaVersion int       `json:"schemaVersion"`
	SampledAt     time.Time `json:"sampledAt"`
	Mode          Mode      `json:"mode"`
	Swap          Swap      `json:"swap"`
	Disk          Disk      `json:"disk"`
	Network       Network   `json:"network"`
}

type Swap struct {
	Status      Status     `json:"status"`
	SampledAt   *time.Time `json:"sampledAt"`
	UsedBytes   *uint64    `json:"usedBytes"`
	TotalBytes  *uint64    `json:"totalBytes"`
	collectedAt time.Time
}

type Disk struct {
	Status         Status     `json:"status"`
	SampledAt      *time.Time `json:"sampledAt"`
	UsedBytes      *uint64    `json:"usedBytes"`
	AvailableBytes *uint64    `json:"availableBytes"`
	TotalBytes     *uint64    `json:"totalBytes"`
	collectedAt    time.Time
}

type Network struct {
	Status           Status     `json:"status"`
	SampledAt        *time.Time `json:"sampledAt"`
	RXBytesPerSecond *float64   `json:"rxBytesPerSecond"`
	TXBytesPerSecond *float64   `json:"txBytesPerSecond"`
	collectedAt      time.Time
}

func DisabledSample(at time.Time) Sample {
	return Sample{SchemaVersion: SchemaVersion, SampledAt: at, Mode: ModeDisabled, Swap: Swap{Status: StatusDisabled}, Disk: Disk{Status: StatusDisabled}, Network: Network{Status: StatusDisabled}}
}

func NotReadySample(at time.Time, mode Mode) Sample {
	return Sample{SchemaVersion: SchemaVersion, SampledAt: at, Mode: mode, Swap: Swap{Status: StatusNotReady}, Disk: Disk{Status: StatusNotReady}, Network: Network{Status: StatusNotReady}}
}

func AvailableSwap(at, collectedAt time.Time, used, total uint64) Swap {
	return Swap{Status: StatusAvailable, SampledAt: timePointer(at), UsedBytes: uint64Pointer(used), TotalBytes: uint64Pointer(total), collectedAt: collectedAt}
}
func AvailableDisk(at, collectedAt time.Time, used, available, total uint64) Disk {
	return Disk{Status: StatusAvailable, SampledAt: timePointer(at), UsedBytes: uint64Pointer(used), AvailableBytes: uint64Pointer(available), TotalBytes: uint64Pointer(total), collectedAt: collectedAt}
}
func AvailableNetwork(at, collectedAt time.Time, rx, tx float64) Network {
	return Network{Status: StatusAvailable, SampledAt: timePointer(at), RXBytesPerSecond: float64Pointer(rx), TXBytesPerSecond: float64Pointer(tx), collectedAt: collectedAt}
}

func (s Sample) Validate() error {
	if s.SchemaVersion != SchemaVersion || s.SampledAt.IsZero() || !validMode(s.Mode) {
		return errors.New("invalid system sample header")
	}
	if err := s.Swap.validate(); err != nil {
		return fmt.Errorf("swap: %w", err)
	}
	if err := s.Disk.validate(); err != nil {
		return fmt.Errorf("disk: %w", err)
	}
	if err := s.Network.validate(); err != nil {
		return fmt.Errorf("network: %w", err)
	}
	if s.Mode == ModeDisabled {
		if s.Swap.Status != StatusDisabled || s.Disk.Status != StatusDisabled || s.Network.Status != StatusDisabled {
			return errors.New("invalid disabled system sample")
		}
	} else if s.Swap.Status == StatusDisabled || s.Disk.Status == StatusDisabled || s.Network.Status == StatusDisabled {
		return errors.New("disabled sensor in enabled system sample")
	}
	return nil
}

func (s Sample) MarshalJSON() ([]byte, error) { return s.MarshalAt(time.Now()) }
func (s Sample) MarshalAt(now time.Time) ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	type wire struct {
		SchemaVersion int         `json:"schemaVersion"`
		SampledAt     time.Time   `json:"sampledAt"`
		Mode          Mode        `json:"mode"`
		Swap          wireSwap    `json:"swap"`
		Disk          wireDisk    `json:"disk"`
		Network       wireNetwork `json:"network"`
	}
	return json.Marshal(wire{SchemaVersion: s.SchemaVersion, SampledAt: s.SampledAt.UTC(), Mode: s.Mode, Swap: s.Swap.wire(now), Disk: s.Disk.wire(now), Network: s.Network.wire(now)})
}

type wireSwap struct {
	Status     Status     `json:"status"`
	SampledAt  *time.Time `json:"sampledAt"`
	AgeMS      *int64     `json:"ageMs"`
	UsedBytes  *uint64    `json:"usedBytes"`
	TotalBytes *uint64    `json:"totalBytes"`
}
type wireDisk struct {
	Status         Status     `json:"status"`
	SampledAt      *time.Time `json:"sampledAt"`
	AgeMS          *int64     `json:"ageMs"`
	UsedBytes      *uint64    `json:"usedBytes"`
	AvailableBytes *uint64    `json:"availableBytes"`
	TotalBytes     *uint64    `json:"totalBytes"`
}
type wireNetwork struct {
	Status           Status     `json:"status"`
	SampledAt        *time.Time `json:"sampledAt"`
	AgeMS            *int64     `json:"ageMs"`
	RXBytesPerSecond *float64   `json:"rxBytesPerSecond"`
	TXBytesPerSecond *float64   `json:"txBytesPerSecond"`
}

func (s Swap) wire(now time.Time) wireSwap {
	return wireSwap{Status: s.Status, SampledAt: utcPointer(s.SampledAt), AgeMS: ageMS(now, s.collectedAt, s.Status == StatusAvailable), UsedBytes: s.UsedBytes, TotalBytes: s.TotalBytes}
}
func (d Disk) wire(now time.Time) wireDisk {
	return wireDisk{Status: d.Status, SampledAt: utcPointer(d.SampledAt), AgeMS: ageMS(now, d.collectedAt, d.Status == StatusAvailable), UsedBytes: d.UsedBytes, AvailableBytes: d.AvailableBytes, TotalBytes: d.TotalBytes}
}
func (n Network) wire(now time.Time) wireNetwork {
	return wireNetwork{Status: n.Status, SampledAt: utcPointer(n.SampledAt), AgeMS: ageMS(now, n.collectedAt, n.Status == StatusAvailable), RXBytesPerSecond: n.RXBytesPerSecond, TXBytesPerSecond: n.TXBytesPerSecond}
}

func (s Swap) validate() error {
	if !validStatus(s.Status) || !validValues(s.Status, s.SampledAt, s.collectedAt, s.UsedBytes != nil, s.TotalBytes != nil) {
		return errors.New("invalid swap")
	}
	if s.Status == StatusAvailable && (*s.TotalBytes > maxSafeJSONInteger || *s.UsedBytes > *s.TotalBytes) {
		return errors.New("invalid swap bytes")
	}
	return nil
}
func (d Disk) validate() error {
	if !validStatus(d.Status) || !validValues(d.Status, d.SampledAt, d.collectedAt, d.UsedBytes != nil, d.AvailableBytes != nil, d.TotalBytes != nil) {
		return errors.New("invalid disk")
	}
	if d.Status == StatusAvailable && (*d.TotalBytes == 0 || *d.TotalBytes > maxSafeJSONInteger || *d.UsedBytes > *d.TotalBytes || *d.AvailableBytes > *d.TotalBytes || *d.UsedBytes > *d.TotalBytes-*d.AvailableBytes) {
		return errors.New("invalid disk bytes")
	}
	return nil
}
func (n Network) validate() error {
	if !validStatus(n.Status) || !validValues(n.Status, n.SampledAt, n.collectedAt, n.RXBytesPerSecond != nil, n.TXBytesPerSecond != nil) {
		return errors.New("invalid network")
	}
	if n.Status == StatusAvailable && (!validRate(*n.RXBytesPerSecond) || !validRate(*n.TXBytesPerSecond)) {
		return errors.New("invalid network rates")
	}
	return nil
}
func validMode(mode Mode) bool { return mode == ModeDisabled || mode == ModeDemo || mode == ModeLive }
func validStatus(status Status) bool {
	return status == StatusDisabled || status == StatusNotReady || status == StatusAvailable || status == StatusUnavailable || status == StatusUnsupported
}
func validValues(status Status, sampledAt *time.Time, collectedAt time.Time, values ...bool) bool {
	for _, value := range values {
		if status != StatusAvailable && value {
			return false
		}
		if status == StatusAvailable && !value {
			return false
		}
	}
	if status != StatusAvailable {
		return sampledAt == nil && collectedAt.IsZero()
	}
	return sampledAt != nil && !sampledAt.IsZero() && !collectedAt.IsZero()
}
func validRate(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= float64(maxSafeJSONInteger)
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
func uint64Pointer(value uint64) *uint64     { return &value }
func float64Pointer(value float64) *float64  { return &value }
func utcPointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	utc := value.UTC()
	return &utc
}
