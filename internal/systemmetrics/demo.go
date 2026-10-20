package systemmetrics

import (
	"context"
	"sync"
	"time"
)

// Demo publishes deterministic synthetic system data and never accesses host state.
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
	return d.hub.Publish(demoSample(at, step))
}
func demoSample(at time.Time, step uint64) Sample {
	return Sample{
		SchemaVersion: SchemaVersion, SampledAt: at, Mode: ModeDemo,
		Swap:    AvailableSwap(at, at, 1_073_741_824, 4_294_967_296),
		Disk:    AvailableDisk(at, at, 214_748_364_800, 268_435_456_000, 536_870_912_000),
		Network: AvailableNetwork(at, at, 1_048_576+float64(step%10), 262_144),
	}
}
