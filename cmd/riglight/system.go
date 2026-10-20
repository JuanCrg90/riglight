package main

import (
	"context"
	"sync"
	"time"

	"riglight/internal/systemmetrics"
)

type systemPublisher interface {
	Publish(context.Context, time.Time) error
}
type systemPublisherFactory func(*systemmetrics.Hub, []string) (systemPublisher, error)

type systemWorker struct {
	hub       *systemmetrics.Hub
	publisher systemPublisher
	done      chan struct{}
	now       func() time.Time
	once      sync.Once
}

func newSystemWorker(hub *systemmetrics.Hub, publisher systemPublisher) *systemWorker {
	return &systemWorker{hub: hub, publisher: publisher, done: make(chan struct{}), now: time.Now}
}

// Start keeps system calls on their own serial worker. A blocked native syscall
// can delay only this worker; it neither overlaps nor blocks other collectors.
func (w *systemWorker) Start(ctx context.Context) {
	w.once.Do(func() {
		go func() {
			defer close(w.done)
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			w.run(ctx, ticker.C)
		}()
	})
}

func (w *systemWorker) run(ctx context.Context, ticks <-chan time.Time) {
	w.collect(ctx, w.now())
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticks:
			// A blocked source can leave ticker values buffered; attempt time is
			// the actual collection start, never that stale scheduled tick.
			w.collect(ctx, w.now())
		}
	}
}
func (w *systemWorker) collect(ctx context.Context, at time.Time) {
	if ctx.Err() == nil {
		_ = w.publisher.Publish(ctx, at)
	}
}
func (w *systemWorker) Wait(ctx context.Context) {
	select {
	case <-w.done:
	case <-ctx.Done():
	}
}
func initialSystemSample(at time.Time, config config) systemmetrics.Sample {
	if !config.systemDetails {
		return systemmetrics.DisabledSample(at)
	}
	mode := systemmetrics.ModeLive
	if config.demo {
		mode = systemmetrics.ModeDemo
	}
	return systemmetrics.NotReadySample(at, mode)
}
func defaultSystemPublisher(hub *systemmetrics.Hub, names []string) (systemPublisher, error) {
	return systemmetrics.NewLive(hub, names)
}
func selectSystemWorker(hub *systemmetrics.Hub, config config, factory systemPublisherFactory) (*systemWorker, error) {
	if !config.systemDetails {
		return nil, nil
	}
	if config.demo {
		return newSystemWorker(hub, systemmetrics.NewDemo(hub)), nil
	}
	publisher, err := factory(hub, config.networkInterfaces)
	if err != nil {
		return nil, err
	}
	return newSystemWorker(hub, publisher), nil
}
