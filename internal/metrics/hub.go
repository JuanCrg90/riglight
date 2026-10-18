package metrics

import (
	"context"

	"riglight/internal/telemetry"
)

var ErrHubClosed = telemetry.ErrHubClosed

// Hub preserves the CPU sample API while delegating bounded fan-out to the
// canonical telemetry hub shared with other collectors.
type Hub struct{ inner *telemetry.Hub[Sample] }

// NewHub starts a hub with an initial sample, normally NOT_READY during startup.
func NewHub(initial Sample) (*Hub, error) {
	inner, err := telemetry.New(initial, cloneSample, func(sample Sample) error { return sample.Validate() })
	if err != nil {
		return nil, err
	}
	return &Hub{inner: inner}, nil
}

// Publish replaces the latest sample and queues it for each subscriber.
func (h *Hub) Publish(sample Sample) error { return h.inner.Publish(sample) }

// Latest returns a private copy of the most recent sample.
func (h *Hub) Latest() Sample { return h.inner.Latest() }

// Subscribe atomically registers a subscriber and queues the current sample.
func (h *Hub) Subscribe() (*Subscription, error) {
	inner, err := h.inner.Subscribe()
	if err != nil {
		return nil, err
	}
	return &Subscription{inner: inner}, nil
}

// SubscriberCount reports the active subscription count for lifecycle checks.
func (h *Hub) SubscriberCount() int { return h.inner.SubscriberCount() }

// Close removes all subscriptions without closing their channels.
func (h *Hub) Close() { h.inner.Close() }

// Subscription is a capacity-one stream of immutable latest samples.
type Subscription struct {
	inner *telemetry.Subscription[Sample]
}

// Next waits for the newest sample or request cancellation.
func (s *Subscription) Next(ctx context.Context) (Sample, error) { return s.inner.Next(ctx) }

// Updates exposes queued immutable samples for a select loop. It is retained
// for transport multiplexing; callers must not mutate the received sample.
func (s *Subscription) Updates() <-chan Sample { return s.inner.Updates() }

// Unsubscribe is safe to call more than once.
func (s *Subscription) Unsubscribe() { s.inner.Unsubscribe() }

func cloneSample(sample Sample) Sample {
	copy := sample
	copy.CPU = cloneCPU(sample.CPU)
	copy.Memory = cloneMemory(sample.Memory)
	return copy
}

func cloneCPU(cpu CPU) CPU {
	copy := cpu
	if cpu.SampledAt != nil {
		copy.SampledAt = timePointer(*cpu.SampledAt)
	}
	if cpu.UtilizationPercent != nil {
		copy.UtilizationPercent = float64Value(*cpu.UtilizationPercent)
	}
	return copy
}

func cloneMemory(memory Memory) Memory {
	copy := memory
	if memory.SampledAt != nil {
		copy.SampledAt = timePointer(*memory.SampledAt)
	}
	if memory.UsedBytes != nil {
		copy.UsedBytes = uint64Value(*memory.UsedBytes)
	}
	if memory.TotalBytes != nil {
		copy.TotalBytes = uint64Value(*memory.TotalBytes)
	}
	return copy
}
