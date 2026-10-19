package dockerproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"riglight/internal/containers"
)

const (
	requestTimeout = time.Second
	pollBudget     = 4 * time.Second
	maxBodyBytes   = 1 << 20
	maxHeaderBytes = 8 << 10
	pollWorkers    = 4
)

type Collector struct {
	targets   []Target
	client    *http.Client
	mu        sync.Mutex
	baselines map[string]cpuBaseline
	next      int
}
type cpuBaseline struct {
	id            string
	total, system uint64
	cpus          uint32
}

func NewCollector(config Config) (*Collector, error) {
	if _, err := NewConfig(config.Socket, config.Listen, targetsToSelections(config.Containers)); err != nil {
		return nil, err
	}
	dialer := &net.Dialer{}
	transport := &http.Transport{Proxy: nil, DisableCompression: true, MaxResponseHeaderBytes: maxHeaderBytes, DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return dialer.DialContext(ctx, "unix", config.Socket)
	}}
	return &Collector{targets: append([]Target(nil), config.Containers...), client: &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, baselines: make(map[string]cpuBaseline)}, nil
}
func targetsToSelections(targets []Target) []string {
	values := make([]string, len(targets))
	for i, target := range targets {
		values[i] = target.Alias + "=" + target.Reference
	}
	return values
}
func (c *Collector) CloseIdleConnections() { c.client.CloseIdleConnections() }

// Collect runs exactly one bounded, background-safe snapshot poll.
func (c *Collector) Collect(ctx context.Context) (containers.Sample, error) {
	poll, cancel := context.WithTimeout(ctx, pollBudget)
	defer cancel()
	at := time.Now()
	rows := make([]containers.Container, len(c.targets))
	c.mu.Lock()
	start := c.next % len(c.targets)
	c.next = (start + 1) % len(c.targets)
	c.mu.Unlock()
	jobs := make(chan int)
	var wait sync.WaitGroup
	for range min(pollWorkers, len(c.targets)) {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for offset := range jobs {
				index := (start + offset) % len(c.targets)
				rows[index] = c.collectTarget(poll, c.targets[index])
			}
		}()
	}
	for offset := range len(c.targets) {
		select {
		case jobs <- offset:
		case <-poll.Done():
			break
		}
		if poll.Err() != nil {
			break
		}
	}
	close(jobs)
	wait.Wait()
	for index, row := range rows {
		if row.Alias == "" {
			rows[index] = unavailableRow(c.targets[index].Alias)
		}
	}
	completed := time.Now()
	sample, err := containers.NewAvailableSample(at, completed, containers.ModeLive, rows)
	if err != nil {
		return containers.Sample{}, err
	}
	return sample, nil
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func (c *Collector) collectTarget(poll context.Context, target Target) containers.Container {
	inspect, status, err := c.inspect(poll, target.Reference)
	if err != nil {
		c.clearBaseline(target.Alias)
		if status == http.StatusNotFound {
			return containers.Container{Alias: target.Alias, Metadata: containers.UnavailableMetadata(containers.MetadataNotFound), CPU: containers.UnavailableCPU(), Memory: containers.UnavailableMemory()}
		}
		return unavailableRow(target.Alias)
	}
	if !inspect.validFor(target.Reference) {
		c.clearBaseline(target.Alias)
		return unavailableRow(target.Alias)
	}
	state, health := projectState(inspect.State)
	metadata := containers.AvailableMetadata(time.Now(), time.Now(), state, health, inspect.restartCount())
	row := containers.Container{Alias: target.Alias, Metadata: metadata, CPU: containers.UnavailableCPU(), Memory: containers.UnavailableMemory()}
	if state != containers.StateRunning {
		c.clearBaseline(target.Alias)
		return row
	}
	stats, _, err := c.stats(poll, inspect.ID)
	if err != nil || stats.ID != inspect.ID {
		c.clearBaseline(target.Alias)
		return row
	}
	completed := time.Now()
	row.CPU = c.projectCPU(target.Alias, inspect.ID, stats, completed)
	row.Memory = projectMemory(stats, completed)
	return row
}
func unavailableRow(alias string) containers.Container {
	return containers.Container{Alias: alias, Metadata: containers.UnavailableMetadata(containers.MetadataUnavailable), CPU: containers.UnavailableCPU(), Memory: containers.UnavailableMemory()}
}
func (c *Collector) clearBaseline(alias string) {
	c.mu.Lock()
	delete(c.baselines, alias)
	c.mu.Unlock()
}

func (c *Collector) inspect(ctx context.Context, ref string) (inspectResponse, int, error) {
	var out inspectResponse
	status, err := c.get(ctx, "/v1.44/containers/"+ref+"/json", &out)
	return out, status, err
}
func (c *Collector) stats(ctx context.Context, id string) (statsResponse, int, error) {
	var out statsResponse
	status, err := c.get(ctx, "/v1.44/containers/"+id+"/stats?stream=false&one-shot=true", &out)
	return out, status, err
}
func (c *Collector) get(parent context.Context, path string, out any) (int, error) {
	requestContext, cancel := context.WithTimeout(parent, requestTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestContext, http.MethodGet, "http://docker"+path, nil)
	if err != nil {
		return 0, err
	}
	response, err := c.client.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return response.StatusCode, errors.New("upstream response")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBodyBytes+1))
	if err != nil || len(body) > maxBodyBytes || requestContext.Err() != nil {
		return response.StatusCode, errors.New("invalid upstream response")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(out); err != nil {
		return response.StatusCode, errors.New("invalid upstream response")
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return response.StatusCode, errors.New("invalid upstream response")
	}
	return response.StatusCode, nil
}

type inspectResponse struct {
	ID           string          `json:"Id"`
	Name         string          `json:"Name"`
	State        *inspectState   `json:"State"`
	RestartCount json.RawMessage `json:"RestartCount"`
}
type inspectState struct {
	Status string         `json:"Status"`
	Health *inspectHealth `json:"Health"`
}
type inspectHealth struct {
	Status string `json:"Status"`
}

func (r inspectResponse) validFor(reference string) bool {
	if !fullID(r.ID) || r.State == nil {
		return false
	}
	if fullID(reference) {
		return r.ID == reference
	}
	return r.Name == "/"+reference
}
func (r inspectResponse) restartCount() *uint64 {
	value, ok := rawUint(r.RestartCount)
	if !ok || value > 1<<53-1 {
		return nil
	}
	return &value
}
func projectState(state *inspectState) (containers.State, containers.Health) {
	value := containers.StateUnknown
	switch state.Status {
	case "created":
		value = containers.StateCreated
	case "running":
		value = containers.StateRunning
	case "paused":
		value = containers.StatePaused
	case "restarting":
		value = containers.StateRestarting
	case "removing":
		value = containers.StateRemoving
	case "exited":
		value = containers.StateExited
	case "dead":
		value = containers.StateDead
	}
	health := containers.HealthNotConfigured
	if state.Health != nil {
		health = containers.HealthUnknown
		switch state.Health.Status {
		case "starting":
			health = containers.HealthStarting
		case "healthy":
			health = containers.HealthHealthy
		case "unhealthy":
			health = containers.HealthUnhealthy
		}
	}
	return value, health
}

type statsResponse struct {
	ID          string      `json:"id"`
	CPUStats    cpuStats    `json:"cpu_stats"`
	MemoryStats memoryStats `json:"memory_stats"`
}
type cpuStats struct {
	CPUUsage struct {
		TotalUsage  json.RawMessage   `json:"total_usage"`
		PercpuUsage []json.RawMessage `json:"percpu_usage"`
	} `json:"cpu_usage"`
	SystemUsage json.RawMessage `json:"system_cpu_usage"`
	OnlineCPUs  json.RawMessage `json:"online_cpus"`
}
type memoryStats struct {
	Usage json.RawMessage            `json:"usage"`
	Limit json.RawMessage            `json:"limit"`
	Stats map[string]json.RawMessage `json:"stats"`
}

func (c *Collector) projectCPU(alias, id string, stats statsResponse, at time.Time) containers.CPU {
	total, ok := rawUint(stats.CPUStats.CPUUsage.TotalUsage)
	if !ok {
		c.clearBaseline(alias)
		return containers.UnavailableCPU()
	}
	system, ok := rawUint(stats.CPUStats.SystemUsage)
	if !ok {
		c.clearBaseline(alias)
		return containers.UnavailableCPU()
	}
	cpus := cpuCount(stats.CPUStats)
	if cpus == 0 {
		c.clearBaseline(alias)
		return containers.UnavailableCPU()
	}
	c.mu.Lock()
	previous, exists := c.baselines[alias]
	c.baselines[alias] = cpuBaseline{id: id, total: total, system: system, cpus: cpus}
	c.mu.Unlock()
	if !exists || previous.id != id || previous.cpus != cpus || total < previous.total || system <= previous.system {
		return containers.NotReadyCPU()
	}
	percent := float64(total-previous.total) / float64(system-previous.system) * float64(cpus) * 100
	if math.IsNaN(percent) || math.IsInf(percent, 0) || percent < 0 || percent > containers.MaxCPUPercent {
		return containers.UnavailableCPU()
	}
	return containers.AvailableCPU(at, at, percent)
}
func cpuCount(stats cpuStats) uint32 {
	if online, ok := rawUint(stats.OnlineCPUs); ok && online > 0 && online <= 4096 {
		return uint32(online)
	}
	if len(stats.CPUUsage.PercpuUsage) > 0 && len(stats.CPUUsage.PercpuUsage) <= 4096 {
		for _, raw := range stats.CPUUsage.PercpuUsage {
			if _, ok := rawUint(raw); !ok {
				return 0
			}
		}
		return uint32(len(stats.CPUUsage.PercpuUsage))
	}
	return 0
}
func projectMemory(stats statsResponse, at time.Time) containers.Memory {
	usage, ok := rawUint(stats.MemoryStats.Usage)
	if !ok {
		return containers.UnsupportedMemory()
	}
	limit, ok := rawUint(stats.MemoryStats.Limit)
	if !ok || limit == 0 || limit > 1<<53-1 {
		return containers.UnavailableMemory()
	}
	cache, exists, valid := inactiveFile(stats.MemoryStats.Stats)
	if !exists {
		return containers.UnsupportedMemory()
	}
	if !valid || cache > usage {
		return containers.UnavailableMemory()
	}
	return containers.AvailableMemory(at, at, usage-cache, limit)
}
func inactiveFile(values map[string]json.RawMessage) (uint64, bool, bool) {
	if raw, exists := values["total_inactive_file"]; exists {
		value, valid := rawUint(raw)
		return value, true, valid
	}
	raw, exists := values["inactive_file"]
	if !exists {
		return 0, false, false
	}
	value, valid := rawUint(raw)
	return value, true, valid
}
func rawUint(raw json.RawMessage) (uint64, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	var value uint64
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&value); err != nil {
		return 0, false
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return 0, false
	}
	return value, true
}
func fullID(value string) bool {
	return len(value) == 64 && validReference(value) && strings.ToLower(value) == value
}
