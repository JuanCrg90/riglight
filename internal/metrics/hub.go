package metrics

import (
	"context"
	"errors"
	"sync"
)

var ErrHubClosed = errors.New("metrics hub is closed")

// Hub owns one immutable latest sample and fans it out without waiting for
// subscribers. It is safe for concurrent use.
type Hub struct {
	mu          sync.Mutex
	latest      Sample
	subscribers map[*Subscription]struct{}
	closed      bool
}

// NewHub starts a hub with an initial sample, normally NOT_READY during startup.
func NewHub(initial Sample) (*Hub, error) {
	if err := initial.Validate(); err != nil {
		return nil, err
	}
	return &Hub{
		latest:      cloneSample(initial),
		subscribers: make(map[*Subscription]struct{}),
	}, nil
}

// Publish replaces the latest sample and queues it for each subscriber. A full
// queue loses its older sample, never blocking collection on a network client.
func (h *Hub) Publish(sample Sample) error {
	if err := sample.Validate(); err != nil {
		return err
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return ErrHubClosed
	}
	h.latest = cloneSample(sample)
	for subscriber := range h.subscribers {
		queueLatest(subscriber.samples, h.latest)
	}
	return nil
}

// Latest returns a private copy of the most recent sample.
func (h *Hub) Latest() Sample {
	h.mu.Lock()
	defer h.mu.Unlock()
	return cloneSample(h.latest)
}

// Subscribe atomically registers a subscriber and enqueues the current sample.
func (h *Hub) Subscribe() (*Subscription, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil, ErrHubClosed
	}

	subscription := &Subscription{
		hub:     h,
		samples: make(chan Sample, 1),
	}
	h.subscribers[subscription] = struct{}{}
	queueLatest(subscription.samples, h.latest)
	return subscription, nil
}

// SubscriberCount reports the active subscription count for lifecycle checks.
func (h *Hub) SubscriberCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subscribers)
}

// Close removes all subscriptions. It does not close subscriber channels, so a
// publisher can never race a channel close; callers cancel their request context.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.closed = true
	clear(h.subscribers)
}

// Subscription is a capacity-one stream of immutable latest samples.
type Subscription struct {
	hub     *Hub
	samples chan Sample
	once    sync.Once
}

// Next waits for the newest sample or request cancellation.
func (s *Subscription) Next(ctx context.Context) (Sample, error) {
	select {
	case sample := <-s.samples:
		return cloneSample(sample), nil
	case <-ctx.Done():
		return Sample{}, ctx.Err()
	}
}

// Unsubscribe is safe to call more than once.
func (s *Subscription) Unsubscribe() {
	s.once.Do(func() {
		s.hub.mu.Lock()
		delete(s.hub.subscribers, s)
		s.hub.mu.Unlock()
	})
}

func queueLatest(queue chan Sample, sample Sample) {
	select {
	case queue <- cloneSample(sample):
		return
	default:
	}
	select {
	case <-queue:
	default:
	}
	queue <- cloneSample(sample)
}

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
