package dockerproxy

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"riglight/internal/containers"
)

const testID = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestCollectorUsesOnlyPinnedPathsAndProjectsNoInspectSecrets(t *testing.T) {
	t.Parallel()
	socket, stop := fakeDaemon(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1.44/containers/service/json":
			if r.Method != http.MethodGet || r.URL.RawQuery != "" {
				t.Errorf("inspect request = %s %s", r.Method, r.URL)
			}
			_, _ = io.WriteString(w, `{"Id":"`+testID+`","Name":"/service","State":{"Status":"running","Health":{"Status":"healthy","Log":[{"Output":"HEALTH_SECRET"}]}},"RestartCount":2,"Config":{"Env":["ENV_SECRET"]}}`)
		case "/v1.44/containers/" + testID + "/stats":
			if r.Method != http.MethodGet || r.URL.RawQuery != "stream=false&one-shot=true" {
				t.Errorf("stats request = %s %s", r.Method, r.URL)
			}
			_, _ = io.WriteString(w, `{"id":"`+testID+`","cpu_stats":{"cpu_usage":{"total_usage":10,"percpu_usage":[1,2]},"system_cpu_usage":100,"online_cpus":2},"memory_stats":{"usage":8,"limit":16,"stats":{"total_inactive_file":2}},"networks":{"private":"NETWORK_SECRET"}}`)
		default:
			t.Errorf("unexpected daemon path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer stop()
	config, err := NewConfig(socket, "127.0.0.1:9123", []string{"app=service"})
	if err != nil {
		t.Fatal(err)
	}
	collector, err := NewCollector(config)
	if err != nil {
		t.Fatal(err)
	}
	sample, err := collector.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if sample.Status != containers.OverviewAvailable || sample.Mode != containers.ModeLive || len(sample.Containers) != 1 {
		t.Fatalf("sample = %#v", sample)
	}
	row := sample.Containers[0]
	if row.Metadata.State != containers.StateRunning || row.Metadata.Health != containers.HealthHealthy || row.CPU.Status != containers.MetricNotReady || row.Memory.Status != containers.MetricAvailable || *row.Memory.UsedBytes != 6 {
		t.Fatalf("row = %#v", row)
	}
	body, err := sample.MarshalAt(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"HEALTH_SECRET", "ENV_SECRET", "NETWORK_SECRET", testID, "service"} {
		if strings.Contains(string(body), forbidden) && forbidden != "service" {
			t.Errorf("projected body leaked %q: %s", forbidden, body)
		}
	}
	if !strings.Contains(string(body), `"alias":"app"`) {
		t.Fatalf("missing alias: %s", body)
	}
}

func TestCollectorCPUAndMemoryRules(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, stats string
		wantCPU     containers.MetricStatus
		wantMemory  containers.MetricStatus
		used        uint64
	}{
		{"v1 present invalid never falls back", `{"id":"` + testID + `","cpu_stats":{"cpu_usage":{"total_usage":10,"percpu_usage":[1]},"system_cpu_usage":100,"online_cpus":1},"memory_stats":{"usage":8,"limit":16,"stats":{"total_inactive_file":"bad","inactive_file":2}}}`, containers.MetricNotReady, containers.MetricUnavailable, 0},
		{"v2 fallback and zero", `{"id":"` + testID + `","cpu_stats":{"cpu_usage":{"total_usage":10,"percpu_usage":[1]},"system_cpu_usage":100,"online_cpus":1},"memory_stats":{"usage":8,"limit":16,"stats":{"inactive_file":8}}}`, containers.MetricNotReady, containers.MetricAvailable, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			socket, stop := fakeDaemon(t, runningDaemon(test.stats))
			defer stop()
			config, _ := NewConfig(socket, "127.0.0.1:9123", []string{"app=service"})
			collector, _ := NewCollector(config)
			sample, err := collector.Collect(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			row := sample.Containers[0]
			if row.CPU.Status != test.wantCPU || row.Memory.Status != test.wantMemory {
				t.Fatalf("row = %#v", row)
			}
			if row.Memory.UsedBytes != nil && *row.Memory.UsedBytes != test.used {
				t.Fatalf("used = %d", *row.Memory.UsedBytes)
			}
		})
	}
}

func runningDaemon(stats string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/json") {
			_, _ = io.WriteString(w, `{"Id":"`+testID+`","Name":"/service","State":{"Status":"running"},"RestartCount":0}`)
			return
		}
		_, _ = io.WriteString(w, stats)
	}
}
func fakeDaemon(t *testing.T, handler http.Handler) (string, func()) {
	t.Helper()
	socket := t.TempDir() + "/daemon.sock"
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: handler}
	go func() { _ = server.Serve(listener) }()
	return socket, func() { _ = server.Close(); _ = listener.Close() }
}
