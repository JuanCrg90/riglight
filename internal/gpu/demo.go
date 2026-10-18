package gpu

import (
	"context"
	"sync"
	"time"
)

// Demo publishes deterministic synthetic GPU values and never constructs a
// native driver source.
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
	return d.hub.Publish(DemoSample(at, step))
}
func DemoSample(at time.Time, step uint64) Sample {
	return Sample{SchemaVersion: SchemaVersion, SampledAt: at, Mode: ModeDemo, Status: OverviewAvailable, Devices: []Device{{Index: 0, Utilization: AvailableUtilization(at, at, 40+float64(step%50)), Memory: AvailableMemory(at, at, 8_000_000_000+step, 16_000_000_000), Temperature: AvailableTemperature(at, at, 50+float64(step%20)), Power: AvailablePower(at, at, 120+float64(step%30))}}}
}
