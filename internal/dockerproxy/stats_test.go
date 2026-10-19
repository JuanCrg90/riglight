package dockerproxy

import (
	"encoding/json"
	"strconv"
	"strings"
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

func TestRawNumbersRejectNullAndInvalidOnlineCPUWithoutFallback(t *testing.T) {
	t.Parallel()
	for _, raw := range []json.RawMessage{json.RawMessage(`null`), json.RawMessage(`1.5`), json.RawMessage(`-1`), json.RawMessage(`"1"`)} {
		if _, ok := rawUint(raw); ok {
			t.Errorf("accepted %s", raw)
		}
	}
	for _, online := range []json.RawMessage{json.RawMessage(`4097`), json.RawMessage(`1.5`), json.RawMessage(`-1`)} {
		stats := statsWithCounters(1, 1, 1)
		stats.CPUStats.OnlineCPUs = online
		stats.CPUStats.CPUUsage.PercpuUsage = []json.RawMessage{json.RawMessage(`1`)}
		if got := cpuCount(stats.CPUStats); got != 0 {
			t.Errorf("online %s count = %d", online, got)
		}
	}
	stats := statsWithCounters(1, 1, 0)
	stats.CPUStats.CPUUsage.PercpuUsage = []json.RawMessage{json.RawMessage(`1`), json.RawMessage(`2`)}
	if got := cpuCount(stats.CPUStats); got != 2 {
		t.Fatalf("zero online fallback = %d", got)
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
func TestMemoryRejectsUnsafeOrOverLimitWorkingSet(t *testing.T) {
	t.Parallel()
	for _, memory := range []memoryStats{
		{Usage: json.RawMessage(`17`), Limit: json.RawMessage(`16`), Stats: map[string]json.RawMessage{"total_inactive_file": json.RawMessage(`0`)}},
		{Usage: json.RawMessage(`9007199254740992`), Limit: json.RawMessage(`9007199254740992`), Stats: map[string]json.RawMessage{"total_inactive_file": json.RawMessage(`0`)}},
	} {
		if got := projectMemory(statsResponse{MemoryStats: memory}, time.Now()); got.Status != containers.MetricUnavailable {
			t.Fatalf("memory = %#v", got)
		}
	}
}

func TestFullIDRequiresLowercaseHexButNameCanBe64Characters(t *testing.T) {
	t.Parallel()
	name := strings.Repeat("g", 64)
	if !validReference(name) || fullID(name) {
		t.Fatalf("reference classification failed")
	}
	if fullID(strings.Repeat("_", 64)) || fullID(strings.ToUpper(testID)) || !fullID(testID) {
		t.Fatal("full ID validation failed")
	}
}

func statsWithCounters(total, system uint64, cpus uint64) statsResponse {
	var value statsResponse
	value.CPUStats.CPUUsage.TotalUsage = json.RawMessage(strconv.FormatUint(total, 10))
	value.CPUStats.SystemUsage = json.RawMessage(strconv.FormatUint(system, 10))
	value.CPUStats.OnlineCPUs = json.RawMessage(strconv.FormatUint(cpus, 10))
	return value
}
