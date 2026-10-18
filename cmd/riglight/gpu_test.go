package main

import (
	"context"
	"testing"
	"time"

	"riglight/internal/gpu"
	"riglight/internal/metrics"
)

func TestGPUWorkerBlockedPollDoesNotBlockCPUHub(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	gpuHub, err := gpu.NewHub(gpu.NotReadySample(at, gpu.ModeLive))
	if err != nil {
		t.Fatal(err)
	}
	defer gpuHub.Close()
	started := make(chan struct{})
	poller := &blockedGPUPoller{started: started, unblock: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	worker := newGPUWorker(gpuHub, poller)
	worker.Start(ctx)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("GPU poll did not begin")
	}
	cpuHub, err := metrics.NewHub(initialSample(at, true))
	if err != nil {
		t.Fatal(err)
	}
	defer cpuHub.Close()
	done := make(chan error, 1)
	go func() { done <- cpuHub.Publish(initialSample(at.Add(time.Second), true)) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("blocked GPU stalled CPU publication")
	}
	cancel()
	select {
	case <-worker.done:
		t.Fatal("blocked GPU worker stopped before call returned")
	case <-time.After(10 * time.Millisecond):
	}
	close(poller.unblock)
	worker.Wait(context.Background())
	if poller.closes != 1 {
		t.Errorf("closes = %d, want 1", poller.closes)
	}
}

func TestGPUSelectionKeepsDisabledAndDemoNativeFree(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	for _, test := range []struct {
		name          string
		demo, enabled bool
		status        gpu.OverviewStatus
		mode          gpu.Mode
	}{
		{"disabled", false, false, gpu.OverviewDisabled, gpu.ModeDisabled},
		{"demo", true, true, gpu.OverviewNotReady, gpu.ModeDemo},
	} {
		t.Run(test.name, func(t *testing.T) {
			sample := initialGPUSample(at, test.demo, test.enabled)
			if sample.Status != test.status || sample.Mode != test.mode {
				t.Fatalf("sample = %#v", sample)
			}
			hub, err := gpu.NewHub(sample)
			if err != nil {
				t.Fatal(err)
			}
			defer hub.Close()
			worker := selectGPUWorker(hub, test.demo, test.enabled)
			if test.enabled && worker == nil {
				t.Fatal("enabled demo has no worker")
			}
			if !test.enabled && worker != nil {
				t.Fatal("disabled GPU constructed a worker")
			}
		})
	}
}

type blockedGPUPoller struct {
	started chan struct{}
	unblock chan struct{}
	closes  int
}

func (p *blockedGPUPoller) Poll(at time.Time) gpu.Sample {
	close(p.started)
	<-p.unblock
	return gpu.UnavailableSample(at, gpu.ModeLive)
}
func (p *blockedGPUPoller) Close() { p.closes++ }
