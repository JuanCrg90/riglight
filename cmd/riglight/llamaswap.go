package main

import (
	"context"
	"sync"
	"time"

	"riglight/internal/llamaswap"
)

const defaultLlamaSwapURL = "http://127.0.0.1:8080"

type llamaSwapSourceFactory func(string) (llamaswap.Source, error)

func defaultLlamaSwapSource(url string) (llamaswap.Source, error) {
	return llamaswap.NewHTTPSource(url)
}

// llamaSwapWorker collects independently from CPU publication and HTTP serving.
type llamaSwapWorker struct {
	hub    *llamaswap.Hub
	source llamaswap.Source
	done   chan struct{}
	once   sync.Once
}

func newLlamaSwapWorker(hub *llamaswap.Hub, source llamaswap.Source) *llamaSwapWorker {
	return &llamaSwapWorker{hub: hub, source: source, done: make(chan struct{})}
}

func (w *llamaSwapWorker) Start(ctx context.Context) {
	w.once.Do(func() {
		go func() {
			defer close(w.done)
			defer closeIdleConnections(w.source)
			w.collect(ctx)
			ticker := time.NewTicker(2 * time.Second)
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

func (w *llamaSwapWorker) collect(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	sample, err := w.source.Collect(ctx)
	if err == nil && ctx.Err() == nil {
		_ = w.hub.Publish(sample)
	}
}

func (w *llamaSwapWorker) Wait(ctx context.Context) {
	select {
	case <-w.done:
	case <-ctx.Done():
	}
}

func closeIdleConnections(source llamaswap.Source) {
	if closer, ok := source.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
}

type demoLlamaSwapSource struct{}

func (demoLlamaSwapSource) Collect(ctx context.Context) (llamaswap.Sample, error) {
	if err := ctx.Err(); err != nil {
		return llamaswap.Sample{}, err
	}
	return llamaswap.DemoSample(time.Now()), nil
}

func initialLlamaSwapSample(at time.Time, config config) llamaswap.Sample {
	if !config.llamaSwap {
		return llamaswap.DisabledSample(at)
	}
	mode := llamaswap.ModeLive
	if config.demo {
		mode = llamaswap.ModeDemo
	}
	return llamaswap.NotReadySample(at, mode)
}

func selectLlamaSwapWorker(hub *llamaswap.Hub, config config, newSource llamaSwapSourceFactory) (*llamaSwapWorker, error) {
	if !config.llamaSwap {
		return nil, nil
	}
	if config.demo {
		return newLlamaSwapWorker(hub, demoLlamaSwapSource{}), nil
	}
	source, err := newSource(config.llamaSwapURL)
	if err != nil {
		return nil, err
	}
	return newLlamaSwapWorker(hub, source), nil
}
