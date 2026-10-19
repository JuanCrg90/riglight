package dockerproxy

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"riglight/internal/containers"
)

func TestCollectorPublishesCompletedRowsWhenAnotherRowIsInvalid(t *testing.T) {
	t.Parallel()
	socket, stop := fakeDaemon(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1.44/containers/bad/json":
			_, _ = io.WriteString(w, `{"Id":"`+testID+`","Name":"/bad","State":{"Status":"running"}}`)
		case "/v1.44/containers/good/json":
			_, _ = io.WriteString(w, `{"Id":"abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789","Name":"/good","State":{"Status":"running"}}`)
		case "/v1.44/containers/" + testID + "/stats":
			_, _ = io.WriteString(w, `{"id":"`+testID+`","cpu_stats":{"cpu_usage":{"total_usage":1,"percpu_usage":[1]},"system_cpu_usage":1,"online_cpus":1},"memory_stats":{"usage":17,"limit":16,"stats":{"total_inactive_file":0}}}`)
		default:
			_, _ = io.WriteString(w, `{"id":"abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789","cpu_stats":{"cpu_usage":{"total_usage":1,"percpu_usage":[1]},"system_cpu_usage":1,"online_cpus":1},"memory_stats":{"usage":8,"limit":16,"stats":{"total_inactive_file":2}}}`)
		}
	}))
	defer stop()
	config, _ := NewConfig(socket, "127.0.0.1:9123", []string{"bad=bad", "good=good"})
	collector, _ := NewCollector(config)
	sample, err := collector.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if sample.Status != containers.OverviewAvailable || len(sample.Containers) != 2 {
		t.Fatalf("sample = %#v", sample)
	}
	if sample.Containers[0].Memory.Status != containers.MetricUnavailable || sample.Containers[1].Memory.Status != containers.MetricAvailable {
		t.Fatalf("rows = %#v", sample.Containers)
	}
}

func TestCollectorHardBudgetReturnsUnstartedRows(t *testing.T) {
	t.Parallel()
	socket, stop := fakeDaemon(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer stop()
	selections := []string{"a=a", "b=b", "c=c", "d=d", "e=e"}
	config, _ := NewConfig(socket, "127.0.0.1:9123", selections)
	collector, _ := NewCollector(config)
	started := time.Now()
	sample, err := collector.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed > pollBudget+time.Second {
		t.Fatalf("poll exceeded hard budget: %s", elapsed)
	}
	for _, row := range sample.Containers {
		if row.Metadata.Status != containers.MetadataUnavailable {
			t.Fatalf("row = %#v", row)
		}
	}
}
