package metrics

import (
	"context"
	"errors"
	"math"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
)

const maxSafeJSONInteger = uint64(1<<53 - 1)

type cpuCounters struct {
	user      float64
	nice      float64
	system    float64
	idle      float64
	iowait    float64
	irq       float64
	softirq   float64
	steal     float64
	guest     float64
	guestNice float64
}

type memoryReading struct {
	usedBytes  uint64
	totalBytes uint64
}

type liveSource interface {
	cpuTimes(context.Context) (cpuCounters, error)
	virtualMemory(context.Context) (memoryReading, error)
}

// Live publishes allowlisted CPU and memory measurements from gopsutil.
type Live struct {
	hub      *Hub
	source   liveSource
	now      func() time.Time
	mu       sync.Mutex
	previous *cpuCounters
}

func NewLive(hub *Hub) *Live {
	return newLiveWithSource(hub, gopsutilSource{}, time.Now)
}

func newLiveWithSource(hub *Hub, source liveSource, now func() time.Time) *Live {
	return &Live{hub: hub, source: source, now: now}
}

// Publish collects CPU and memory independently. CPU requires an earlier
// counter baseline; all source failures become explicit unavailable metrics.
func (collector *Live) Publish(ctx context.Context, attemptAt time.Time) error {
	collector.mu.Lock()
	defer collector.mu.Unlock()

	sample := Sample{
		SchemaVersion: SchemaVersion,
		SampledAt:     attemptAt,
		Mode:          ModeLive,
		CPU:           CPU{Status: StatusUnavailable},
		Memory:        Memory{Status: StatusUnavailable},
	}

	if current, err := collector.source.cpuTimes(ctx); err == nil && validCPUCounters(current) {
		collectedAt := collector.now()
		if collector.previous == nil {
			sample.CPU = CPU{Status: StatusNotReady}
		} else if utilization, ok := aggregateUtilization(*collector.previous, current); ok {
			sample.CPU = AvailableCPU(collectedAt, collectedAt, utilization)
		}
		baseline := current
		collector.previous = &baseline
	}

	if reading, err := collector.source.virtualMemory(ctx); err == nil && validMemoryReading(reading) {
		collectedAt := collector.now()
		sample.Memory = AvailableMemory(collectedAt, collectedAt, reading.usedBytes, reading.totalBytes)
	}

	return collector.hub.Publish(sample)
}

type gopsutilSource struct{}

func (gopsutilSource) cpuTimes(ctx context.Context) (cpuCounters, error) {
	times, err := cpu.TimesWithContext(ctx, false)
	if err != nil || len(times) != 1 {
		return cpuCounters{}, errors.New("cpu counters unavailable")
	}
	value := times[0]
	return cpuCounters{
		user:      value.User,
		nice:      value.Nice,
		system:    value.System,
		idle:      value.Idle,
		iowait:    value.Iowait,
		irq:       value.Irq,
		softirq:   value.Softirq,
		steal:     value.Steal,
		guest:     value.Guest,
		guestNice: value.GuestNice,
	}, nil
}

func (gopsutilSource) virtualMemory(ctx context.Context) (memoryReading, error) {
	value, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return memoryReading{}, err
	}
	return memoryReading{usedBytes: value.Used, totalBytes: value.Total}, nil
}

func validCPUCounters(value cpuCounters) bool {
	for _, counter := range []float64{
		value.user, value.nice, value.system, value.idle, value.iowait,
		value.irq, value.softirq, value.steal, value.guest, value.guestNice,
	} {
		if math.IsNaN(counter) || math.IsInf(counter, 0) || counter < 0 {
			return false
		}
	}
	return true
}

func aggregateUtilization(previous, current cpuCounters) (float64, bool) {
	user := counterDelta(previous.user, current.user)
	nice := counterDelta(previous.nice, current.nice)
	system := counterDelta(previous.system, current.system)
	idle := counterDelta(previous.idle, current.idle)
	iowait := counterDelta(previous.iowait, current.iowait)
	irq := counterDelta(previous.irq, current.irq)
	softirq := counterDelta(previous.softirq, current.softirq)
	steal := counterDelta(previous.steal, current.steal)
	guest := counterDelta(previous.guest, current.guest)
	guestNice := counterDelta(previous.guestNice, current.guestNice)

	// Linux accounts guest time in user and guest-nice time in nice. Exclude it
	// from the aggregate total to avoid double counting, as gopsutil does.
	total := user + nice + system + idle + iowait + irq + softirq + steal - guest - guestNice
	busy := total - idle - iowait
	if total <= 0 || busy < 0 || busy > total {
		return 0, false
	}
	utilization := busy / total * 100
	if math.IsNaN(utilization) || math.IsInf(utilization, 0) || utilization < 0 || utilization > 100 {
		return 0, false
	}
	return utilization, true
}

func counterDelta(previous, current float64) float64 {
	if current <= previous {
		return 0
	}
	return current - previous
}

func validMemoryReading(reading memoryReading) bool {
	return reading.totalBytes > 0 && reading.usedBytes <= reading.totalBytes && reading.totalBytes <= maxSafeJSONInteger
}
