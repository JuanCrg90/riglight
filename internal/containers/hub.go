package containers

import (
	"context"

	"riglight/internal/telemetry"
)

var ErrHubClosed = telemetry.ErrHubClosed

type Hub struct{ inner *telemetry.Hub[Sample] }

func NewHub(initial Sample) (*Hub, error) {
	inner, err := telemetry.New(initial, cloneSample, func(s Sample) error { return s.Validate() })
	if err != nil {
		return nil, err
	}
	return &Hub{inner: inner}, nil
}
func (h *Hub) Publish(s Sample) error { return h.inner.Publish(s) }
func (h *Hub) Latest() Sample         { return h.inner.Latest() }
func (h *Hub) Close()                 { h.inner.Close() }
func (h *Hub) SubscriberCount() int   { return h.inner.SubscriberCount() }
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
func cloneSample(s Sample) Sample {
	copy := s
	if s.ReasonCode != nil {
		copy.ReasonCode = reasonPointer(*s.ReasonCode)
	}
	copy.Containers = make([]Container, len(s.Containers))
	for i, row := range s.Containers {
		copy.Containers[i] = cloneContainer(row)
	}
	return copy
}
func cloneContainer(c Container) Container {
	copy := c
	copy.Metadata.SampledAt = utcPointer(c.Metadata.SampledAt)
	copy.Metadata.RestartCount = cloneUint64(c.Metadata.RestartCount)
	copy.CPU.SampledAt = utcPointer(c.CPU.SampledAt)
	if c.CPU.Percent != nil {
		copy.CPU.Percent = floatPointer(*c.CPU.Percent)
	}
	copy.Memory.SampledAt = utcPointer(c.Memory.SampledAt)
	copy.Memory.UsedBytes = cloneUint64(c.Memory.UsedBytes)
	copy.Memory.TotalBytes = cloneUint64(c.Memory.TotalBytes)
	return copy
}
