package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"riglight/internal/gpu"
	"riglight/internal/llamaswap"
	"riglight/internal/metrics"
)

func TestParseConfigRequiresExplicitLlamaSwapOptIn(t *testing.T) {
	t.Parallel()

	defaults, err := parseConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if defaults.llamaSwap {
		t.Fatalf("default config = %#v", defaults)
	}
	if _, err := parseConfig([]string{"--llama-swap-url", "http://127.0.0.1:1234"}); !errors.Is(err, errInvalidArguments) {
		t.Fatalf("URL without opt-in error = %v", err)
	}
	enabled, err := parseConfig([]string{"--llama-swap"})
	if err != nil || !enabled.llamaSwap || enabled.llamaSwapURL != defaultLlamaSwapURL {
		t.Fatalf("enabled config = %#v, %v", enabled, err)
	}
	demo, err := parseConfig([]string{"--demo", "--llama-swap"})
	if err != nil || !demo.demo || !demo.llamaSwap {
		t.Fatalf("demo config = %#v, %v", demo, err)
	}
}

func TestLlamaSwapSelectionNeverConstructsHTTPForDisabledOrDemo(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	for _, test := range []struct {
		name    string
		config  config
		wantNil bool
	}{
		{"disabled", config{}, true},
		{"demo", config{demo: true, llamaSwap: true}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			hub, err := llamaswap.NewHub(initialLlamaSwapSample(at, test.config))
			if err != nil {
				t.Fatal(err)
			}
			defer hub.Close()
			factoryCalls := 0
			worker, err := selectLlamaSwapWorker(hub, test.config, func(string) (llamaswap.Source, error) {
				factoryCalls++
				return nil, errors.New("HTTP source should not be constructed")
			})
			if err != nil {
				t.Fatal(err)
			}
			if (worker == nil) != test.wantNil || factoryCalls != 0 {
				t.Fatalf("worker = %#v, factory calls = %d", worker, factoryCalls)
			}
		})
	}
}

func TestCLIBlockedLlamaSwapKeepsCPUAndGPUAndHTTPResponsive(t *testing.T) {
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
	blocked := &blockedLlamaSwapSource{started: make(chan struct{}), canceled: make(chan struct{})}
	cpu := &countingCPUPublisher{}
	poller := &countingGPUPoller{}
	done := make(chan error, 1)
	go func() {
		done <- runWithAllFactories(ctx, []string{"--llama-swap", "--gpu", "--listen", address}, func(hub *metrics.Hub) (publisher, error) {
			cpu.hub = hub
			return cpu, nil
		}, func() gpuPoller { return poller }, func(string) (llamaswap.Source, error) { return blocked, nil }, "linux")
	}()
	select {
	case <-blocked.started:
	case <-time.After(time.Second):
		t.Fatal("llama-swap collection did not start")
	}
	waitForCPUCalls(t, &cpu.calls, 1, time.Second)
	if poller.polls.Load() == 0 {
		t.Fatal("GPU polling did not start")
	}
	request, err := http.NewRequest(http.MethodGet, "http://"+address+"/healthz", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Host = address
	response, err := (&http.Client{Transport: &http.Transport{Proxy: nil}}).Do(request)
	if err != nil {
		t.Fatalf("health request: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d", response.StatusCode)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("CLI shutdown did not cancel llama-swap worker")
	}
	select {
	case <-blocked.canceled:
	case <-time.After(time.Second):
		t.Fatal("blocked source was not canceled")
	}
}

type blockedLlamaSwapSource struct {
	started  chan struct{}
	canceled chan struct{}
	calls    atomic.Int32
}

func (s *blockedLlamaSwapSource) Collect(ctx context.Context) (llamaswap.Sample, error) {
	s.calls.Add(1)
	close(s.started)
	<-ctx.Done()
	close(s.canceled)
	return llamaswap.Sample{}, ctx.Err()
}
