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
	if config.demo || config.dev || config.gpu {
		t.Errorf("unexpected defaults: %#v", config)
	}
	gpuConfig, err := parseConfig([]string{"--gpu"})
	if err != nil || !gpuConfig.gpu {
		t.Fatalf("--gpu config = %#v, %v", gpuConfig, err)
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

	for input, want := range map[string]string{
		"[0:0:0:0:0:0:0:1]:9090": "[::1]:9090",
		"127.0.0.1:09090":        "127.0.0.1:9090",
	} {
		config, err := parseConfig([]string{"--listen", input})
		if err != nil {
			t.Errorf("parse %q: %v", input, err)
			continue
		}
		if config.listen != want {
			t.Errorf("listen %q normalized to %q, want %q", input, config.listen, want)
		}
	}
}

func TestParseConfigRequiresCompleteTailnetMode(t *testing.T) {
	t.Parallel()

	valid, err := parseConfig([]string{
		"--tailnet",
		"--listen", "100.64.0.10:9090",
		"--host", "workstation.example.ts.net:9090",
		"--allow-client", "100.64.0.20",
		"--allow-client", "fd7a:115c:a1e0::20",
	})
	if err != nil {
		t.Fatalf("parse valid tailnet config: %v", err)
	}
	if !valid.tailnet || valid.listen != "100.64.0.10:9090" || valid.host != "workstation.example.ts.net:9090" {
		t.Errorf("tailnet config = %#v", valid)
	}
	if len(valid.allowedClients) != 2 {
		t.Errorf("allowed clients = %#v, want two", valid.allowedClients)
	}

	for _, arguments := range [][]string{
		{"--tailnet"},
		{"--tailnet", "--host", "workstation.example.ts.net:9090", "--allow-client", "100.64.0.20"},
		{"--tailnet", "--listen", "100.64.0.10:9090", "--allow-client", "100.64.0.20"},
		{"--tailnet", "--listen", "100.64.0.10:9090", "--host", "workstation.example.ts.net:9090"},
		{"--host", "workstation.example.ts.net:9090"},
		{"--allow-client", "100.64.0.20"},
		{"--tailnet", "--dev", "--listen", "100.64.0.10:9090", "--host", "workstation.example.ts.net:9090", "--allow-client", "100.64.0.20"},
	} {
		if _, err := parseConfig(arguments); !errors.Is(err, errInvalidArguments) {
			t.Errorf("parseConfig(%q) error = %v, want invalid arguments", arguments, err)
		}
	}
}

func TestParseConfigRejectsInvalidTailnetValues(t *testing.T) {
	t.Parallel()

	base := []string{"--tailnet", "--listen", "100.64.0.10:9090", "--host", "workstation.example.ts.net:9090", "--allow-client", "100.64.0.20"}
	for _, replacement := range []struct {
		name  string
		flag  string
		value string
	}{
		{name: "wildcard listen", flag: "--listen", value: "0.0.0.0:9090"},
		{name: "lan listen", flag: "--listen", value: "192.168.1.2:9090"},
		{name: "public listen", flag: "--listen", value: "192.0.2.1:9090"},
		{name: "dns listen", flag: "--listen", value: "workstation.example.ts.net:9090"},
		{name: "cidr listen", flag: "--listen", value: "100.64.0.10/32:9090"},
		{name: "bad listen port", flag: "--listen", value: "100.64.0.10:0"},
		{name: "uppercase host", flag: "--host", value: "Workstation.example.ts.net:9090"},
		{name: "wrong host suffix", flag: "--host", value: "workstation.example.invalid:9090"},
		{name: "host port mismatch", flag: "--host", value: "workstation.example.ts.net:9091"},
		{name: "host URI", flag: "--host", value: "http://workstation.example.ts.net:9090"},
		{name: "client cidr", flag: "--allow-client", value: "100.64.0.20/32"},
		{name: "client lan", flag: "--allow-client", value: "192.168.1.2"},
		{name: "client public", flag: "--allow-client", value: "192.0.2.2"},
	} {
		t.Run(replacement.name, func(t *testing.T) {
			arguments := append([]string(nil), base...)
			for index, argument := range arguments[:len(arguments)-1] {
				if argument == replacement.flag {
					arguments[index+1] = replacement.value
				}
			}
			if _, err := parseConfig(arguments); !errors.Is(err, errInvalidArguments) {
				t.Errorf("parseConfig(%q) error = %v, want invalid arguments", arguments, err)
			}
		})
	}
}

func TestRunRejectsNonLinuxLiveModeBeforeFactory(t *testing.T) {
	t.Parallel()

	calledLive := false
	err := runForOS(context.Background(), []string{"--listen", "127.0.0.1:9090"}, func(*metrics.Hub) (publisher, error) {
		calledLive = true
		return nil, errors.New("live factory should not be called")
	}, "darwin")
	if !errors.Is(err, errUnableToStart) {
		t.Fatalf("run non-Linux live mode error = %v, want generic startup error", err)
	}
	if calledLive {
		t.Fatal("non-Linux live mode constructed a live publisher")
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

func TestLiveSelectionConstructsOnlyTheLivePublisher(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	hub, err := metrics.NewHub(initialSample(at, false))
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	defer hub.Close()
	calledLive := false
	publisher, err := selectPublisher(false, hub, func(hub *metrics.Hub) (publisher, error) {
		calledLive = true
		return metrics.NewDemo(hub), nil
	})
	if err != nil {
		t.Fatalf("select live publisher: %v", err)
	}
	if !calledLive || publisher == nil {
		t.Fatal("live selection did not construct its publisher")
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
	if err := publisher.Publish(context.Background(), at); err != nil {
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
