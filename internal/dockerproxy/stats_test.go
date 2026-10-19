package dockerproxy

import (
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"riglight/internal/containers"
)

func TestCPUUsesOwnBaselineAndTreatsValidZeroAsAvailable(t *testing.T) {
	t.Parallel()
	collector := &Collector{baselines: make(map[string]cpuBaseline)}
	at := time.Now()
	first := statsWithCounters(10, 100, 2)
	if got := collector.projectCPU("app", testID, first, at); got.Status != containers.MetricNotReady {
		t.Fatalf("first status = %s", got.Status)
	}
	zero := statsWithCounters(10, 110, 2)
	got := collector.projectCPU("app", testID, zero, at)
	if got.Status != containers.MetricAvailable || got.Percent == nil || *got.Percent != 0 {
		t.Fatalf("zero delta = %#v", got)
	}
	if got := collector.projectCPU("app", testID, statsWithCounters(20, 120, 3), at); got.Status != containers.MetricNotReady {
		t.Fatalf("count change = %#v", got)
	}
	if got := collector.projectCPU("app", testID, statsWithCounters(19, 130, 3), at); got.Status != containers.MetricNotReady {
		t.Fatalf("counter reset = %#v", got)
	}
	if got := collector.projectCPU("app", "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789", statsWithCounters(20, 140, 3), at); got.Status != containers.MetricNotReady {
		t.Fatalf("identity change = %#v", got)
	}
}

func TestMemoryRequiresCorrectCacheCounter(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name  string
		stats map[string]json.RawMessage
		want  containers.MetricStatus
	}{
		{"missing", map[string]json.RawMessage{}, containers.MetricUnsupported},
		{"cache greater than usage", map[string]json.RawMessage{"total_inactive_file": json.RawMessage(`9`)}, containers.MetricUnavailable},
		{"v1 invalid wins over v2", map[string]json.RawMessage{"total_inactive_file": json.RawMessage(`"bad"`), "inactive_file": json.RawMessage(`2`)}, containers.MetricUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := statsResponse{MemoryStats: memoryStats{Usage: json.RawMessage(`8`), Limit: json.RawMessage(`16`), Stats: test.stats}}
			if got := projectMemory(value, time.Now()); got.Status != test.want {
				t.Fatalf("memory = %#v", got)
			}
		})
	}
}
func statsWithCounters(total, system uint64, cpus uint64) statsResponse {
	var value statsResponse
	value.CPUStats.CPUUsage.TotalUsage = json.RawMessage(strconv.FormatUint(total, 10))
	value.CPUStats.SystemUsage = json.RawMessage(strconv.FormatUint(system, 10))
	value.CPUStats.OnlineCPUs = json.RawMessage(strconv.FormatUint(cpus, 10))
	return value
}
