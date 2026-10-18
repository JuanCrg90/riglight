// Package telemetry provides a bounded latest-value fan-out hub.
package telemetry

import (
	"context"
	"errors"
	"sync"
)

var ErrHubClosed = errors.New("telemetry hub is closed")

// Hub owns one immutable latest value and fans it out without waiting for
// subscribers. Values are cloned on every ownership boundary.
type Hub[T any] struct {
	mu          sync.Mutex
	latest      T
	clone       func(T) T
	validate    func(T) error
	subscribers map[*Subscription[T]]struct{}
	closed      bool
}

// New starts a hub with an initial valid value.
func New[T any](initial T, clone func(T) T, validate func(T) error) (*Hub[T], error) {
	if clone == nil || validate == nil {
		return nil, errors.New("telemetry hub requires clone and validation")
	}
	if err := validate(initial); err != nil {
		return nil, err
	}
	return &Hub[T]{
		latest:      clone(initial),
		clone:       clone,
		validate:    validate,
		subscribers: make(map[*Subscription[T]]struct{}),
	}, nil
}

// Publish replaces the latest value and queues it for each subscriber. A full
// queue loses its older value, never blocking collection on a network client.
func (h *Hub[T]) Publish(value T) error {
	if err := h.validate(value); err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return ErrHubClosed
	}
	h.latest = h.clone(value)
	for subscriber := range h.subscribers {
		h.queueLatest(subscriber.values, h.latest)
	}
	return nil
}

// Latest returns a private copy of the most recent value.
func (h *Hub[T]) Latest() T {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.clone(h.latest)
}

// Subscribe atomically registers a subscriber and queues the current value.
func (h *Hub[T]) Subscribe() (*Subscription[T], error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil, ErrHubClosed
	}
	subscription := &Subscription[T]{hub: h, values: make(chan T, 1)}
	h.subscribers[subscription] = struct{}{}
	h.queueLatest(subscription.values, h.latest)
	return subscription, nil
}

// SubscriberCount reports active subscriptions for lifecycle checks.
func (h *Hub[T]) SubscriberCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subscribers)
}

// Close removes all subscriptions. It does not close subscriber channels, so a
// publisher can never race a channel close; callers cancel their contexts.
func (h *Hub[T]) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.closed = true
	clear(h.subscribers)
}

func (h *Hub[T]) queueLatest(queue chan T, value T) {
	select {
	case queue <- h.clone(value):
		return
	default:
	}
	select {
	case <-queue:
	default:
	}
	queue <- h.clone(value)
}

// Subscription is a capacity-one stream of immutable latest values.
type Subscription[T any] struct {
	hub    *Hub[T]
	values chan T
	once   sync.Once
}

// Next waits for the newest value or context cancellation.
func (s *Subscription[T]) Next(ctx context.Context) (T, error) {
	select {
	case value := <-s.values:
		return s.hub.clone(value), nil
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	}
}

// Updates exposes already-owned queued copies for select loops. Callers must
// not mutate received values and must still Unsubscribe when finished.
func (s *Subscription[T]) Updates() <-chan T { return s.values }

// Unsubscribe is safe to call more than once.
func (s *Subscription[T]) Unsubscribe() {
	s.once.Do(func() {
		s.hub.mu.Lock()
		delete(s.hub.subscribers, s)
		s.hub.mu.Unlock()
	})
}
