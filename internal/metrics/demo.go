package metrics

import (
	"context"
	"sync"
	"time"
)

// Demo publishes deterministic synthetic data and never accesses host metrics.
type Demo struct {
	hub  *Hub
	mu   sync.Mutex
	step uint64
}

func NewDemo(hub *Hub) *Demo {
	return &Demo{hub: hub}
}

// Publish emits the next synthetic sample at the supplied collection time.
func (d *Demo) Publish(ctx context.Context, at time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	d.mu.Lock()
	d.step++
	step := d.step
	d.mu.Unlock()

	return d.hub.Publish(demoSample(at, step))
}

func demoSample(at time.Time, step uint64) Sample {
	utilization := 40 + float64(step%50)
	usedBytes := uint64(8_000_000_000) + step
	return Sample{
		SchemaVersion: SchemaVersion,
		SampledAt:     at,
		Mode:          ModeDemo,
		CPU:           AvailableCPU(at, at, utilization),
		Memory:        AvailableMemory(at, at, usedBytes, 16_000_000_000),
	}
}
