package llamaswap

import (
	"context"

	"riglight/internal/telemetry"
)

var ErrHubClosed = telemetry.ErrHubClosed

// Hub retains immutable inventory samples for HTTP and SSE consumers.
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
	if sample.ReasonCode != nil {
		copy.ReasonCode = reasonPointer(*sample.ReasonCode)
	}
	copy.Models = append([]Model(nil), sample.Models...)
	return copy
}
