package systemmetrics

import (
	"context"

	"riglight/internal/telemetry"
)

var ErrHubClosed = telemetry.ErrHubClosed

// Hub retains immutable system samples for HTTP and SSE consumers.
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
	copy.Swap.SampledAt = utcPointer(sample.Swap.SampledAt)
	if sample.Swap.UsedBytes != nil {
		copy.Swap.UsedBytes = uint64Pointer(*sample.Swap.UsedBytes)
	}
	if sample.Swap.TotalBytes != nil {
		copy.Swap.TotalBytes = uint64Pointer(*sample.Swap.TotalBytes)
	}
	copy.Disk.SampledAt = utcPointer(sample.Disk.SampledAt)
	if sample.Disk.UsedBytes != nil {
		copy.Disk.UsedBytes = uint64Pointer(*sample.Disk.UsedBytes)
	}
	if sample.Disk.AvailableBytes != nil {
		copy.Disk.AvailableBytes = uint64Pointer(*sample.Disk.AvailableBytes)
	}
	if sample.Disk.TotalBytes != nil {
		copy.Disk.TotalBytes = uint64Pointer(*sample.Disk.TotalBytes)
	}
	copy.Network.SampledAt = utcPointer(sample.Network.SampledAt)
	if sample.Network.RXBytesPerSecond != nil {
		copy.Network.RXBytesPerSecond = float64Pointer(*sample.Network.RXBytesPerSecond)
	}
	if sample.Network.TXBytesPerSecond != nil {
		copy.Network.TXBytesPerSecond = float64Pointer(*sample.Network.TXBytesPerSecond)
	}
	return copy
}
