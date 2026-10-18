package gpu

import (
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
	devices     []fakeDevice
	shutdowns   int
}

func (d *fakeDriver) Init() result         { return d.initResult }
func (d *fakeDriver) Shutdown() result     { d.shutdowns++; return resultSuccess }
func (d *fakeDriver) Count() (int, result) { return len(d.devices), d.countResult }
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
}

func (d *fakeDevice) Utilization() (float64, result)    { return d.utilization, d.utilizationResult }
func (d *fakeDevice) MemoryV2() (memoryReading, result) { return d.memory, d.memoryV2Result }
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
