package metrics

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"
)

func TestLiveCollectorUsesCPUBaselineThenGuestCorrectedAggregateDelta(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	hub, err := NewHub(liveNotReadySample(at))
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	defer hub.Close()
	source := &fakeLiveSource{
		cpu: []cpuCounters{
			{user: 10, nice: 2, system: 10, idle: 50, iowait: 5, guest: 3, guestNice: 1},
			{user: 20, nice: 4, system: 20, idle: 70, iowait: 7, guest: 6, guestNice: 2},
		},
		memory: []memoryReading{{usedBytes: 8, totalBytes: 16}, {usedBytes: 9, totalBytes: 16}},
	}
	collector := newLiveWithSource(hub, source, sequenceClock(at, at.Add(2*time.Second), at.Add(4*time.Second), at.Add(6*time.Second)))

	if err := collector.Publish(context.Background(), at); err != nil {
		t.Fatalf("collect baseline: %v", err)
	}
	baseline := hub.Latest()
	if baseline.CPU.Status != StatusNotReady {
		t.Errorf("baseline CPU status = %s, want NOT_READY", baseline.CPU.Status)
	}
	if baseline.Memory.Status != StatusAvailable {
		t.Errorf("baseline memory status = %s, want AVAILABLE", baseline.Memory.Status)
	}

	if err := collector.Publish(context.Background(), at.Add(2*time.Second)); err != nil {
		t.Fatalf("collect delta: %v", err)
	}
	got := hub.Latest()
	if got.CPU.Status != StatusAvailable || got.CPU.UtilizationPercent == nil {
		t.Fatalf("CPU = %#v, want available measurement", got.CPU)
	}
	if want := 45.0; math.Abs(*got.CPU.UtilizationPercent-want) > 0.0001 {
		t.Errorf("CPU utilization = %f, want %f", *got.CPU.UtilizationPercent, want)
	}
	if got.Memory.Status != StatusAvailable || got.Memory.UsedBytes == nil || *got.Memory.UsedBytes != 9 {
		t.Errorf("memory = %#v, want current independent measurement", got.Memory)
	}
}

func TestLiveCollectorKeepsMetricFailuresIndependent(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	hub, err := NewHub(liveNotReadySample(at))
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	defer hub.Close()
	source := &fakeLiveSource{
		cpu:    []cpuCounters{{user: 1, idle: 9}, {user: 2, idle: 18}},
		cpuErr: []error{nil, errors.New("synthetic CPU failure")},
		memory: []memoryReading{{}, {usedBytes: 4, totalBytes: 8}},
		memErr: []error{errors.New("synthetic memory failure"), nil},
	}
	collector := newLiveWithSource(hub, source, sequenceClock(at, at.Add(time.Second), at.Add(2*time.Second), at.Add(3*time.Second)))

	if err := collector.Publish(context.Background(), at); err != nil {
		t.Fatalf("collect baseline: %v", err)
	}
	first := hub.Latest()
	if first.CPU.Status != StatusNotReady || first.Memory.Status != StatusUnavailable {
		t.Errorf("first statuses = %s/%s, want NOT_READY/UNAVAILABLE", first.CPU.Status, first.Memory.Status)
	}
	if err := collector.Publish(context.Background(), at.Add(2*time.Second)); err != nil {
		t.Fatalf("collect with CPU failure: %v", err)
	}
	second := hub.Latest()
	if second.CPU.Status != StatusUnavailable || second.Memory.Status != StatusAvailable {
		t.Errorf("second statuses = %s/%s, want UNAVAILABLE/AVAILABLE", second.CPU.Status, second.Memory.Status)
	}
}

func TestLiveCollectorRejectsInvalidCountersAndUnsafeMemory(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	hub, err := NewHub(liveNotReadySample(at))
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	defer hub.Close()
	source := &fakeLiveSource{
		cpu:    []cpuCounters{{user: math.NaN(), idle: 1}},
		memory: []memoryReading{{usedBytes: maxSafeJSONInteger + 1, totalBytes: maxSafeJSONInteger + 1}},
	}
	collector := newLiveWithSource(hub, source, sequenceClock(at, at.Add(time.Second)))

	if err := collector.Publish(context.Background(), at); err != nil {
		t.Fatalf("collect invalid readings: %v", err)
	}
	got := hub.Latest()
	if got.CPU.Status != StatusUnavailable || got.Memory.Status != StatusUnavailable {
		t.Errorf("statuses = %s/%s, want UNAVAILABLE/UNAVAILABLE", got.CPU.Status, got.Memory.Status)
	}
}

func TestLiveCollectorPassesContextToBothSources(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	hub, err := NewHub(liveNotReadySample(at))
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	defer hub.Close()
	source := &fakeLiveSource{cpu: []cpuCounters{{user: 1, idle: 9}}, memory: []memoryReading{{usedBytes: 1, totalBytes: 2}}}
	collector := newLiveWithSource(hub, source, sequenceClock(at, at.Add(time.Second)))
	ctx := context.WithValue(context.Background(), contextKey{}, "synthetic")

	if err := collector.Publish(ctx, at); err != nil {
		t.Fatalf("collect: %v", err)
	}
	if source.cpuContext != ctx || source.memoryContext != ctx {
		t.Fatal("collector did not forward collection context")
	}
}

type contextKey struct{}

type fakeLiveSource struct {
	cpu           []cpuCounters
	cpuErr        []error
	memory        []memoryReading
	memErr        []error
	cpuCalls      int
	memoryCalls   int
	cpuContext    context.Context
	memoryContext context.Context
}

func (source *fakeLiveSource) cpuTimes(ctx context.Context) (cpuCounters, error) {
	source.cpuContext = ctx
	index := source.cpuCalls
	source.cpuCalls++
	if index < len(source.cpuErr) && source.cpuErr[index] != nil {
		return cpuCounters{}, source.cpuErr[index]
	}
	return source.cpu[index], nil
}

func (source *fakeLiveSource) virtualMemory(ctx context.Context) (memoryReading, error) {
	source.memoryContext = ctx
	index := source.memoryCalls
	source.memoryCalls++
	if index < len(source.memErr) && source.memErr[index] != nil {
		return memoryReading{}, source.memErr[index]
	}
	return source.memory[index], nil
}

func liveNotReadySample(at time.Time) Sample {
	return Sample{
		SchemaVersion: SchemaVersion,
		SampledAt:     at,
		Mode:          ModeLive,
		CPU:           CPU{Status: StatusNotReady},
		Memory:        Memory{Status: StatusNotReady},
	}
}

func sequenceClock(values ...time.Time) func() time.Time {
	index := 0
	return func() time.Time {
		value := values[index]
		if index < len(values)-1 {
			index++
		}
		return value
	}
}
