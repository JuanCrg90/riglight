package systemmetrics

import (
	"context"
	"errors"
	"math"
	"sync"
	"time"
)

// ErrUnsupported is returned only for an explicitly absent sensor capability.
var ErrUnsupported = errors.New("system metric unsupported")

type rawSwap struct{ totalUnits, freeUnits, unitBytes uint64 }
type rawDisk struct{ blocks, freeBlocks, availableBlocks, blockBytes uint64 }
type rawCounter struct {
	name   string
	rx, tx uint64
}
type liveSource interface {
	Swap(context.Context) (rawSwap, error)
	Disk(context.Context) (rawDisk, error)
	Network(context.Context) ([]rawCounter, error)
}
type networkBaseline struct {
	counters map[string]rawCounter
	at       time.Time
}

// Live projects private raw readings into the frozen aggregate contract.
type Live struct {
	hub      *Hub
	source   liveSource
	names    []string
	now      func() time.Time
	mu       sync.Mutex
	baseline *networkBaseline
}

func NewLive(hub *Hub, names []string) (*Live, error) {
	selection, err := ValidateInterfaces(names)
	if err != nil {
		return nil, err
	}
	return newLiveWithSource(hub, selection, newNativeSource(), time.Now), nil
}
func newLiveWithSource(hub *Hub, names []string, source liveSource, now func() time.Time) *Live {
	return &Live{hub: hub, names: append([]string(nil), names...), source: source, now: now}
}
func (l *Live) Publish(ctx context.Context, attemptAt time.Time) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	sample := Sample{SchemaVersion: SchemaVersion, SampledAt: attemptAt, Mode: ModeLive, Swap: Swap{Status: StatusUnavailable}, Disk: Disk{Status: StatusUnavailable}, Network: Network{Status: StatusUnavailable}}
	if raw, err := l.source.Swap(ctx); err != nil {
		sample.Swap.Status = statusForError(err)
	} else if used, total, ok := projectSwap(raw); ok {
		at := l.now()
		sample.Swap = AvailableSwap(at, at, used, total)
	}
	if raw, err := l.source.Disk(ctx); err != nil {
		sample.Disk.Status = statusForError(err)
	} else if used, available, total, ok := projectDisk(raw); ok {
		at := l.now()
		sample.Disk = AvailableDisk(at, at, used, available, total)
	}
	if rows, err := l.source.Network(ctx); err != nil {
		l.baseline = nil
		sample.Network.Status = statusForError(err)
	} else {
		at := l.now()
		sample.Network = l.network(rows, at)
	}
	return l.hub.Publish(sample)
}
func statusForError(err error) Status {
	if errors.Is(err, ErrUnsupported) {
		return StatusUnsupported
	}
	return StatusUnavailable
}
func projectSwap(raw rawSwap) (uint64, uint64, bool) {
	if raw.unitBytes == 0 || raw.freeUnits > raw.totalUnits {
		return 0, 0, false
	}
	total, ok := checkedBytes(raw.totalUnits, raw.unitBytes)
	if !ok {
		return 0, 0, false
	}
	used, ok := checkedBytes(raw.totalUnits-raw.freeUnits, raw.unitBytes)
	if !ok || used > total {
		return 0, 0, false
	}
	return used, total, true
}
func projectDisk(raw rawDisk) (uint64, uint64, uint64, bool) {
	if raw.blockBytes == 0 || raw.blocks == 0 || raw.freeBlocks > raw.blocks || raw.availableBlocks > raw.freeBlocks {
		return 0, 0, 0, false
	}
	total, ok := checkedBytes(raw.blocks, raw.blockBytes)
	if !ok {
		return 0, 0, 0, false
	}
	used, ok := checkedBytes(raw.blocks-raw.freeBlocks, raw.blockBytes)
	if !ok {
		return 0, 0, 0, false
	}
	available, ok := checkedBytes(raw.availableBlocks, raw.blockBytes)
	if !ok || used > total || available > total-used {
		return 0, 0, 0, false
	}
	return used, available, total, true
}
func checkedBytes(units, bytes uint64) (uint64, bool) {
	if units != 0 && bytes > maxSafeJSONInteger/units {
		return 0, false
	}
	return units * bytes, true
}
func (l *Live) network(rows []rawCounter, at time.Time) Network {
	current, ok := selectedCounters(l.names, rows)
	if !ok {
		l.baseline = nil
		return Network{Status: StatusUnavailable}
	}
	previous := l.baseline
	l.baseline = &networkBaseline{counters: current, at: at}
	if previous == nil || !at.After(previous.at) {
		return Network{Status: StatusNotReady}
	}
	seconds := at.Sub(previous.at).Seconds()
	var rx, tx uint64
	for _, name := range l.names {
		before := previous.counters[name]
		after := current[name]
		if after.rx < before.rx || after.tx < before.tx {
			return Network{Status: StatusNotReady}
		}
		var added bool
		rx, added = addChecked(rx, after.rx-before.rx)
		if !added {
			l.baseline = nil
			return Network{Status: StatusUnavailable}
		}
		tx, added = addChecked(tx, after.tx-before.tx)
		if !added {
			l.baseline = nil
			return Network{Status: StatusUnavailable}
		}
	}
	rxRate, txRate := float64(rx)/seconds, float64(tx)/seconds
	if !validRate(rxRate) || !validRate(txRate) || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		l.baseline = nil
		return Network{Status: StatusUnavailable}
	}
	return AvailableNetwork(at, at, rxRate, txRate)
}
func selectedCounters(names []string, rows []rawCounter) (map[string]rawCounter, bool) {
	selected := make(map[string]rawCounter, len(names))
	wanted := make(map[string]struct{}, len(names))
	for _, name := range names {
		wanted[name] = struct{}{}
	}
	for _, row := range rows {
		if _, ok := wanted[row.name]; ok {
			if _, duplicate := selected[row.name]; duplicate {
				return nil, false
			}
			selected[row.name] = row
		}
	}
	if len(selected) != len(names) {
		return nil, false
	}
	return selected, true
}
func addChecked(left, right uint64) (uint64, bool) {
	if right > math.MaxUint64-left {
		return 0, false
	}
	return left + right, true
}
