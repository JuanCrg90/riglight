package main

import (
	"context"
	"sync"
	"time"

	"riglight/internal/containers"
)

type containerSourceFactory func(string) (containers.Source, error)

func defaultContainerSource(url string) (containers.Source, error) {
	return containers.NewHTTPSource(url)
}

type containerWorker struct {
	hub    *containers.Hub
	source containers.Source
	done   chan struct{}
	once   sync.Once
}

func newContainerWorker(hub *containers.Hub, source containers.Source) *containerWorker {
	return &containerWorker{hub: hub, source: source, done: make(chan struct{})}
}
func (w *containerWorker) Start(ctx context.Context) {
	w.once.Do(func() {
		go func() {
			defer close(w.done)
			defer func() {
				if closer, ok := w.source.(interface{ CloseIdleConnections() }); ok {
					closer.CloseIdleConnections()
				}
			}()
			w.collect(ctx)
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					w.collect(ctx)
				}
			}
		}()
	})
}
func (w *containerWorker) collect(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	sample, err := w.source.Collect(ctx)
	if err == nil && ctx.Err() == nil {
		_ = w.hub.Publish(sample)
	}
}
func (w *containerWorker) Wait(ctx context.Context) {
	select {
	case <-w.done:
	case <-ctx.Done():
	}
}

type demoContainerSource struct{}

func (demoContainerSource) Collect(ctx context.Context) (containers.Sample, error) {
	if err := ctx.Err(); err != nil {
		return containers.Sample{}, err
	}
	at := time.Now()
	sample, err := containers.NewAvailableSample(at, at, containers.ModeDemo, []containers.Container{{Alias: "demo-service", Metadata: containers.AvailableMetadata(at, at, containers.StateRunning, containers.HealthHealthy, nil), CPU: containers.AvailableCPU(at, at, 75), Memory: containers.AvailableMemory(at, at, 268435456, 1073741824)}})
	return sample, err
}
func initialContainerSample(at time.Time, config config) containers.Sample {
	if !config.docker {
		return containers.DisabledSample(at)
	}
	mode := containers.ModeLive
	if config.demo {
		mode = containers.ModeDemo
	}
	return containers.NotReadySample(at, mode)
}
func selectContainerWorker(hub *containers.Hub, config config, factory containerSourceFactory) (*containerWorker, error) {
	if !config.docker {
		return nil, nil
	}
	if config.demo {
		return newContainerWorker(hub, demoContainerSource{}), nil
	}
	source, err := factory(config.dockerProxyURL)
	if err != nil {
		return nil, err
	}
	return newContainerWorker(hub, source), nil
}
