package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"

	"riglight/internal/metrics"
)

func TestParseConfigDefaultsToLoopbackDemoPort(t *testing.T) {
	t.Parallel()

	config, err := parseConfig(nil)
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	if config.listen != "127.0.0.1:9090" {
		t.Errorf("listen = %q, want 127.0.0.1:9090", config.listen)
	}
	if config.demo || config.dev {
		t.Errorf("unexpected defaults: %#v", config)
	}
}

func TestParseConfigOnlyPermitsLoopbackIPAddresses(t *testing.T) {
	t.Parallel()

	for _, arguments := range [][]string{
		{"--listen", "0.0.0.0:9090"},
		{"--listen", "localhost:9090"},
		{"--listen", "192.0.2.1:9090"},
		{"--listen", "127.0.0.1"},
		{"--listen", "[::]:9090"},
	} {
		if _, err := parseConfig(arguments); !errors.Is(err, errInvalidArguments) {
			t.Errorf("parseConfig(%q) error = %v, want invalid arguments", arguments, err)
		}
	}
	config, err := parseConfig([]string{"--listen", "[::1]:9090"})
	if err != nil {
		t.Fatalf("parse IPv6 loopback: %v", err)
	}
	if config.listen != "[::1]:9090" {
		t.Errorf("IPv6 listen = %q", config.listen)
	}
}

func TestRunDemoServesEmbeddedAssetsWithoutLivePublisher(t *testing.T) {
	t.Parallel()

	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve loopback port: %v", err)
	}
	address := probe.Addr().String()
	if err := probe.Close(); err != nil {
		t.Fatalf("release loopback port: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calledLive := false
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, []string{"--demo", "--listen", address}, func(*metrics.Hub) (publisher, error) {
			calledLive = true
			return nil, errors.New("live collector should not be constructed")
		})
	}()

	waitForHTTP(t, address)
	if calledLive {
		t.Fatal("demo mode constructed a live publisher")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run demo: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("demo shutdown did not complete")
	}
}

func TestRunDevAllowsOnlyTheViteOrigin(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		arguments []string
		want      int
	}{
		{name: "production", arguments: []string{"--demo"}, want: http.StatusForbidden},
		{name: "development", arguments: []string{"--demo", "--dev"}, want: http.StatusOK},
	} {
		t.Run(test.name, func(t *testing.T) {
			probe, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatalf("reserve loopback port: %v", err)
			}
			address := probe.Addr().String()
			if err := probe.Close(); err != nil {
				t.Fatalf("release loopback port: %v", err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan error, 1)
			go func() {
				done <- run(ctx, append(test.arguments, "--listen", address), func(*metrics.Hub) (publisher, error) {
					return nil, errors.New("live collector should not be constructed")
				})
			}()
			waitForHTTPStatus(t, address, devOrigin, test.want)
			cancel()
			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("run demo: %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("demo shutdown did not complete")
			}
		})
	}
}

func TestDemoSelectionNeverConstructsLivePublisher(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	hub, err := metrics.NewHub(metrics.Sample{
		SchemaVersion: metrics.SchemaVersion,
		SampledAt:     at,
		Mode:          metrics.ModeDemo,
		CPU:           metrics.CPU{Status: metrics.StatusNotReady},
		Memory:        metrics.Memory{Status: metrics.StatusNotReady},
	})
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	defer hub.Close()

	calledLive := false
	publisher, err := selectPublisher(true, hub, func(*metrics.Hub) (publisher, error) {
		calledLive = true
		return nil, errors.New("live collector should not be constructed")
	})
	if err != nil {
		t.Fatalf("select demo publisher: %v", err)
	}
	if calledLive {
		t.Fatal("demo selection constructed live publisher")
	}
	if err := publisher.Publish(at); err != nil {
		t.Fatalf("publish demo sample: %v", err)
	}
}

func waitForHTTP(t *testing.T, address string) {
	waitForHTTPStatus(t, address, "", http.StatusOK)
}

func waitForHTTPStatus(t *testing.T, address, origin string, want int) {
	t.Helper()
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	client := &http.Client{Transport: &http.Transport{Proxy: nil}}
	defer client.CloseIdleConnections()
	for {
		request, err := http.NewRequest(http.MethodGet, "http://"+address+"/", nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		request.Host = address
		if origin != "" {
			request.Header.Set("Origin", origin)
		}
		response, err := client.Do(request)
		if err == nil {
			response.Body.Close()
			if response.StatusCode == want {
				return
			}
		}
		select {
		case <-deadline.C:
			t.Fatal("demo server did not serve embedded assets")
		case <-time.After(time.Millisecond):
		}
	}
}
