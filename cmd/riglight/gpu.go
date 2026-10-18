package main

import (
	"context"
	"sync"
	"time"

	"riglight/internal/gpu"
)

type gpuPoller interface {
	Poll(time.Time) gpu.Sample
	Close()
}

// gpuWorker is independent from CPU publication and HTTP handling. Native
// calls cannot be context-cancelled; a blocked Poll deliberately prevents its
// Close, avoiding an unsafe NVML unload while a call remains active.
type gpuWorker struct {
	hub    *gpu.Hub
	poller gpuPoller
	done   chan struct{}
	once   sync.Once
}

func newGPUWorker(hub *gpu.Hub, poller gpuPoller) *gpuWorker {
	return &gpuWorker{hub: hub, poller: poller, done: make(chan struct{})}
}

func (w *gpuWorker) Start(ctx context.Context) {
	w.once.Do(func() {
		go func() {
			defer close(w.done)
			defer w.poller.Close()
			w.publish(time.Now())
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case at := <-ticker.C:
					w.publish(at)
				}
			}
		}()
	})
}

func (w *gpuWorker) publish(at time.Time) { _ = w.hub.Publish(w.poller.Poll(at)) }
func (w *gpuWorker) Wait(ctx context.Context) {
	select {
	case <-w.done:
	case <-ctx.Done():
	}
}

type demoGPUPoller struct{ step uint64 }

func (p *demoGPUPoller) Poll(at time.Time) gpu.Sample { p.step++; return gpu.DemoSample(at, p.step) }
func (*demoGPUPoller) Close()                         {}

func initialGPUSample(at time.Time, demo, enabled bool) gpu.Sample {
	if !enabled {
		return gpu.DisabledSample(at)
	}
	if demo {
		return gpu.NotReadySample(at, gpu.ModeDemo)
	}
	return gpu.NotReadySample(at, gpu.ModeLive)
}

func selectGPUWorker(hub *gpu.Hub, demo, enabled bool) *gpuWorker {
	if !enabled {
		return nil
	}
	if demo {
		return newGPUWorker(hub, &demoGPUPoller{})
	}
	return newGPUWorker(hub, gpu.NewNativeProvider())
}
