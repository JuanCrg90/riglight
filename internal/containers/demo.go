package containers

import (
	"context"
	"sync"
	"time"
)

// Demo publishes deterministic synthetic selected-container values only.
type Demo struct {
	hub  *Hub
	mu   sync.Mutex
	step uint64
}

func NewDemo(hub *Hub) *Demo { return &Demo{hub: hub} }
func (d *Demo) Publish(ctx context.Context, at time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	d.mu.Lock()
	d.step++
	step := d.step
	d.mu.Unlock()
	sample, err := NewAvailableSample(at, at, ModeDemo, []Container{{Alias: "demo-service", Metadata: AvailableMetadata(at, at, StateRunning, HealthHealthy, uint64Pointer(step%3)), CPU: AvailableCPU(at, at, 75+float64(step%50)), Memory: AvailableMemory(at, at, 268435456+step, 1073741824)}})
	if err != nil {
		return err
	}
	return d.hub.Publish(sample)
}
