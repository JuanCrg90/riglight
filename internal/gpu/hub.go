package gpu

import (
	"context"

	"riglight/internal/telemetry"
)

var ErrHubClosed = telemetry.ErrHubClosed

// Hub retains immutable GPU samples for HTTP and SSE consumers.
type Hub struct{ inner *telemetry.Hub[Sample] }

func NewHub(initial Sample) (*Hub, error) {
	inner, err := telemetry.New(initial, cloneSample, func(sample Sample) error { return sample.Validate() })
	if err != nil {
		return nil, err
	}
	return &Hub{inner: inner}, nil
}
func (h *Hub) Publish(sample Sample) error { return h.inner.Publish(sample) }
func (h *Hub) Latest() Sample              { return h.inner.Latest() }
func (h *Hub) Close()                      { h.inner.Close() }
func (h *Hub) SubscriberCount() int        { return h.inner.SubscriberCount() }
func (h *Hub) Subscribe() (*Subscription, error) {
	inner, err := h.inner.Subscribe()
	if err != nil {
		return nil, err
	}
	return &Subscription{inner: inner}, nil
}

type Subscription struct {
	inner *telemetry.Subscription[Sample]
}

func (s *Subscription) Next(ctx context.Context) (Sample, error) { return s.inner.Next(ctx) }
func (s *Subscription) Updates() <-chan Sample                   { return s.inner.Updates() }
func (s *Subscription) Unsubscribe()                             { s.inner.Unsubscribe() }

func cloneSample(sample Sample) Sample {
	copy := sample
	copy.ReasonCode = nil
	if sample.ReasonCode != nil {
		copy.ReasonCode = reasonPointer(*sample.ReasonCode)
	}
	copy.Devices = make([]Device, len(sample.Devices))
	for index, device := range sample.Devices {
		copy.Devices[index] = cloneDevice(device)
	}
	return copy
}
func cloneDevice(device Device) Device {
	copy := device
	copy.Utilization.SampledAt = utcPointer(device.Utilization.SampledAt)
	if device.Utilization.Percent != nil {
		copy.Utilization.Percent = float64Pointer(*device.Utilization.Percent)
	}
	copy.Memory.SampledAt = utcPointer(device.Memory.SampledAt)
	if device.Memory.UsedBytes != nil {
		copy.Memory.UsedBytes = uint64Pointer(*device.Memory.UsedBytes)
	}
	if device.Memory.TotalBytes != nil {
		copy.Memory.TotalBytes = uint64Pointer(*device.Memory.TotalBytes)
	}
	copy.Temperature.SampledAt = utcPointer(device.Temperature.SampledAt)
	if device.Temperature.Celsius != nil {
		copy.Temperature.Celsius = float64Pointer(*device.Temperature.Celsius)
	}
	copy.Power.SampledAt = utcPointer(device.Power.SampledAt)
	if device.Power.Watts != nil {
		copy.Power.Watts = float64Pointer(*device.Power.Watts)
	}
	return copy
}
