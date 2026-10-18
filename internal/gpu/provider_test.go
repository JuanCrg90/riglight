package gpu

import (
	"strings"
	"testing"
	"time"
)

func TestProviderUsesVersionedReadsAndConvertsMilliwatts(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	driver := &fakeDriver{devices: []fakeDevice{{utilization: 42, memory: memoryReading{used: 8, total: 16}, temperature: 55, powerMW: 125500}}}
	provider := NewProvider(driver, ModeLive)
	sample := provider.Poll(at)
	if sample.Status != OverviewAvailable || len(sample.Devices) != 1 {
		t.Fatalf("sample = %#v", sample)
	}
	if got := *sample.Devices[0].Power.Watts; got != 125.5 {
		t.Errorf("watts = %v, want 125.5", got)
	}
	if driver.devices[0].legacyMemory || driver.devices[0].legacyTemperature {
		t.Fatal("used legacy reads despite versioned success")
	}
}

func TestProviderFallsBackOnlyForOlderDriverResultAndKeepsSensorFailuresLocal(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	driver := &fakeDriver{devices: []fakeDevice{{utilizationResult: resultUnsupported, memoryV2Result: resultFunctionNotFound, memory: memoryReading{used: 8, total: 16}, temperatureVResult: resultArgumentVersionMismatch, temperature: 55, powerMW: 1000}}}
	sample := NewProvider(driver, ModeLive).Poll(at)
	device := sample.Devices[0]
	if device.Utilization.Status != StatusUnsupported || device.Memory.Status != StatusAvailable || device.Temperature.Status != StatusAvailable || device.Power.Status != StatusAvailable {
		t.Fatalf("statuses = %s/%s/%s/%s", device.Utilization.Status, device.Memory.Status, device.Temperature.Status, device.Power.Status)
	}
	if !driver.devices[0].legacyMemory || !driver.devices[0].legacyTemperature {
		t.Fatal("did not use documented legacy fallback")
	}
}

func TestProviderCapturesEachMetricTimeAfterNativeReads(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	clock := &fakeClock{now: base}
	driver := &fakeDriver{devices: []fakeDevice{{
		utilization:      42,
		memory:           memoryReading{used: 8, total: 16},
		temperature:      55,
		powerMW:          1_000,
		afterUtilization: func() { clock.advance(10 * time.Second) },
		afterMemory:      func() { clock.advance(10 * time.Second) },
	}}}
	driver.afterInit = func() { clock.advance(10 * time.Second) }
	provider := newProvider(driver, ModeLive, clock.Now)
	sample := provider.Poll(base)
	device := sample.Devices[0]
	if got, want := *device.Utilization.SampledAt, base.Add(20*time.Second); !got.Equal(want) {
		t.Errorf("utilization sampled at = %s, want %s", got, want)
	}
	if got, want := *device.Memory.SampledAt, base.Add(30*time.Second); !got.Equal(want) {
		t.Errorf("memory sampled at = %s, want %s", got, want)
	}
	clock.advance(5 * time.Second)
	body, err := sample.MarshalAt(clock.Now())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(body), `"utilization":{"status":"AVAILABLE","sampledAt":"2026-01-01T00:00:22Z","ageMs":15`) ||
		!strings.Contains(string(body), `"memory":{"status":"AVAILABLE","sampledAt":"2026-01-01T00:00:32Z","ageMs":5`) {
		t.Fatalf("metric ages = %s", body)
	}
}

func TestProviderCachesFailedInitializationWithoutShutdown(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	for _, test := range []struct {
		name, reason string
		result       result
		status       OverviewStatus
		reasonCode   ReasonCode
	}{
		{name: "library missing", result: resultLibraryNotFound, status: OverviewUnsupported, reasonCode: ReasonDriverUnavailable},
		{name: "driver missing", result: resultDriverNotLoaded, status: OverviewUnsupported, reasonCode: ReasonDriverUnavailable},
		{name: "unsupported backend", result: resultUnsupported, status: OverviewUnsupported, reasonCode: ReasonBackendUnavailable},
		{name: "permission failure", result: resultPermission, status: OverviewUnavailable, reasonCode: ReasonQueryFailed},
		{name: "unknown failure", result: resultUnknown, status: OverviewUnavailable, reasonCode: ReasonQueryFailed},
	} {
		t.Run(test.name, func(t *testing.T) {
			driver := &fakeDriver{initResult: test.result}
			provider := NewProvider(driver, ModeLive)
			for range 2 {
				sample := provider.Poll(at)
				if sample.Status != test.status || sample.ReasonCode == nil || *sample.ReasonCode != test.reasonCode {
					t.Fatalf("sample = %#v", sample)
				}
			}
			provider.Close()
			if driver.inits != 1 || driver.shutdowns != 0 {
				t.Fatalf("init/shutdown = %d/%d, want 1/0", driver.inits, driver.shutdowns)
			}
		})
	}
}

func TestProviderRejectsNegativeDeviceCountAsQueryFailure(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	driver := &fakeDriver{count: -1}
	provider := NewProvider(driver, ModeLive)
	sample := provider.Poll(at)
	if sample.Status != OverviewUnavailable || sample.ReasonCode == nil || *sample.ReasonCode != ReasonQueryFailed {
		t.Fatalf("sample = %#v", sample)
	}
	provider.Close()
	if driver.shutdowns != 1 {
		t.Errorf("shutdowns = %d, want 1", driver.shutdowns)
	}
}

func TestProviderCapsInventoryAndSerializesSafeClose(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	devices := make([]fakeDevice, MaxDevices+1)
	for index := range devices {
		devices[index] = fakeDevice{memory: memoryReading{used: 1, total: 2}}
	}
	driver := &fakeDriver{devices: devices}
	provider := NewProvider(driver, ModeLive)
	sample := provider.Poll(at)
	if !sample.IsTruncated || len(sample.Devices) != MaxDevices || sample.ReasonCode == nil || *sample.ReasonCode != ReasonDeviceLimit {
		t.Fatalf("cap sample = %#v", sample)
	}
	provider.Close()
	provider.Close()
	if driver.shutdowns != 1 {
		t.Errorf("shutdowns = %d, want 1", driver.shutdowns)
	}
}

type fakeDriver struct {
	initResult  result
	countResult result
	count       int
	devices     []fakeDevice
	inits       int
	shutdowns   int
	afterInit   func()
}

func (d *fakeDriver) Init() result {
	d.inits++
	if d.afterInit != nil {
		d.afterInit()
	}
	return d.initResult
}
func (d *fakeDriver) Shutdown() result { d.shutdowns++; return resultSuccess }
func (d *fakeDriver) Count() (int, result) {
	if d.count != 0 {
		return d.count, d.countResult
	}
	return len(d.devices), d.countResult
}
func (d *fakeDriver) Device(index int) (device, result) {
	if index < 0 || index >= len(d.devices) {
		return nil, resultUnknown
	}
	return &d.devices[index], resultSuccess
}

type fakeDevice struct {
	utilization        float64
	utilizationResult  result
	memory             memoryReading
	memoryV2Result     result
	temperature        float64
	temperatureVResult result
	powerMW            uint32
	powerResult        result
	legacyMemory       bool
	legacyTemperature  bool
	afterUtilization   func()
	afterMemory        func()
}

func (d *fakeDevice) Utilization() (float64, result) {
	if d.afterUtilization != nil {
		d.afterUtilization()
	}
	return d.utilization, d.utilizationResult
}
func (d *fakeDevice) MemoryV2() (memoryReading, result) {
	if d.afterMemory != nil {
		d.afterMemory()
	}
	return d.memory, d.memoryV2Result
}
func (d *fakeDevice) MemoryV1() (memoryReading, result) {
	d.legacyMemory = true
	return d.memory, resultSuccess
}
func (d *fakeDevice) TemperatureV1() (float64, result) { return d.temperature, d.temperatureVResult }
func (d *fakeDevice) TemperatureLegacy() (float64, result) {
	d.legacyTemperature = true
	return d.temperature, resultSuccess
}
func (d *fakeDevice) PowerMilliwatts() (uint32, result) { return d.powerMW, d.powerResult }

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time                 { return c.now }
func (c *fakeClock) advance(duration time.Duration) { c.now = c.now.Add(duration) }
