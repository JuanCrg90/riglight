package main

import (
	"context"
	"net"
	"net/http"
	"sync/atomic"
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
	if got := poller.closes.Load(); got != 1 {
		t.Errorf("closes = %d, want 1", got)
	}
}

func TestGPUWorkerSkipsPollForCanceledContext(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	hub, err := gpu.NewHub(gpu.NotReadySample(at, gpu.ModeLive))
	if err != nil {
		t.Fatal(err)
	}
	defer hub.Close()
	poller := &countingGPUPoller{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	worker := newGPUWorker(hub, poller)
	worker.Start(ctx)
	worker.Wait(context.Background())
	if got := poller.polls.Load(); got != 0 {
		t.Errorf("polls = %d, want 0", got)
	}
	if got := poller.closes.Load(); got != 1 {
		t.Errorf("closes = %d, want 1", got)
	}
}

func TestCLIBlockedGPUKeepsCPUAndHTTPResponsiveThenBoundsShutdown(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	poller := &blockedGPUPoller{started: make(chan struct{}), unblock: make(chan struct{}), closed: make(chan struct{})}
	cpu := &countingCPUPublisher{}
	done := make(chan error, 1)
	go func() {
		done <- runWithFactories(ctx, []string{"--gpu", "--listen", address}, func(hub *metrics.Hub) (publisher, error) {
			cpu.hub = hub
			return cpu, nil
		}, func() gpuPoller { return poller }, "linux")
	}()
	select {
	case <-poller.started:
	case <-time.After(time.Second):
		t.Fatal("GPU poll did not start")
	}
	waitForCPUCalls(t, &cpu.calls, 2, 3*time.Second)
	request, err := http.NewRequest(http.MethodGet, "http://"+address+"/healthz", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Host = address
	response, err := (&http.Client{Transport: &http.Transport{Proxy: nil}}).Do(request)
	if err != nil {
		t.Fatalf("guarded HTTP request: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d", response.StatusCode)
	}

	shutdownStarted := time.Now()
	cancel()
	select {
	case err := <-done:
		if elapsed := time.Since(shutdownStarted); elapsed > 5500*time.Millisecond {
			t.Fatalf("CLI shutdown took %s, exceeds shared five-second budget", elapsed)
		}
		if err != nil {
			t.Fatalf("run: %v", err)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("CLI exceeded shared shutdown budget")
	}
	if got := poller.closes.Load(); got != 0 {
		t.Fatalf("close calls while Poll blocked = %d", got)
	}
	close(poller.unblock)
	select {
	case <-poller.closed:
	case <-time.After(time.Second):
		t.Fatal("GPU poller did not close after release")
	}
	if got := poller.closes.Load(); got != 1 {
		t.Errorf("close calls = %d, want 1", got)
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
			nativeCalls := 0
			worker := selectGPUWorkerWithFactory(hub, test.demo, test.enabled, func() gpuPoller {
				nativeCalls++
				return &countingGPUPoller{}
			})
			if nativeCalls != 0 {
				t.Fatal("disabled or demo mode constructed a native poller")
			}
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
	closed  chan struct{}
	closes  atomic.Int32
}

func (p *blockedGPUPoller) Poll(at time.Time) gpu.Sample {
	close(p.started)
	<-p.unblock
	return gpu.UnavailableSample(at, gpu.ModeLive)
}
func (p *blockedGPUPoller) Close() {
	p.closes.Add(1)
	if p.closed != nil {
		close(p.closed)
	}
}

type countingGPUPoller struct{ polls, closes atomic.Int32 }

func (p *countingGPUPoller) Poll(at time.Time) gpu.Sample {
	p.polls.Add(1)
	return gpu.UnavailableSample(at, gpu.ModeLive)
}
func (p *countingGPUPoller) Close() { p.closes.Add(1) }

type countingCPUPublisher struct {
	hub   *metrics.Hub
	calls atomic.Int32
}

func (p *countingCPUPublisher) Publish(ctx context.Context, at time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p.calls.Add(1)
	return p.hub.Publish(metrics.Sample{SchemaVersion: metrics.SchemaVersion, SampledAt: at, Mode: metrics.ModeLive, CPU: metrics.AvailableCPU(at, at, 42), Memory: metrics.AvailableMemory(at, at, 8, 16)})
}
func waitForCPUCalls(t *testing.T, calls *atomic.Int32, want int32, timeout time.Duration) {
	t.Helper()
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for calls.Load() < want {
		select {
		case <-deadline.C:
			t.Fatalf("CPU calls = %d, want at least %d", calls.Load(), want)
		case <-time.After(time.Millisecond):
		}
	}
}
