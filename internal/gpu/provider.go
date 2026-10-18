package gpu

import (
	"sync"
	"time"
)

// result deliberately maps only outcomes relevant to the public status contract.
type result uint8

const (
	resultSuccess result = iota
	resultUnsupported
	resultFunctionNotFound
	resultArgumentVersionMismatch
	resultLibraryNotFound
	resultDriverNotLoaded
	resultPermission
	resultBackendUnavailable
	resultUnknown
)

type memoryReading struct{ used, total uint64 }

type driver interface {
	Init() result
	Shutdown() result
	Count() (int, result)
	Device(int) (device, result)
}

type device interface {
	Utilization() (float64, result)
	MemoryV2() (memoryReading, result)
	MemoryV1() (memoryReading, result)
	TemperatureV1() (float64, result)
	TemperatureLegacy() (float64, result)
	PowerMilliwatts() (uint32, result)
}

// Provider owns all native driver calls. Poll and Close are serialized so Close
// can never unload NVML while a non-cancellable native call remains active.
//
// go-nvml loads the shared library before calling nvmlInit. A failed Init after
// loading can retain a library reference that Shutdown cannot safely balance,
// so the first initialization result is retained until process restart.
type Provider struct {
	mu                             sync.Mutex
	driver                         driver
	mode                           Mode
	now                            func() time.Time
	initialized, closed, attempted bool
	initResult                     result
}

func NewProvider(driver driver, mode Mode) *Provider { return newProvider(driver, mode, time.Now) }
func newProvider(driver driver, mode Mode, now func() time.Time) *Provider {
	if now == nil {
		now = time.Now
	}
	return &Provider{driver: driver, mode: mode, now: now}
}
func (p *Provider) Poll(at time.Time) Sample {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed || p.driver == nil {
		return UnsupportedSample(at, p.mode, ReasonBackendUnavailable)
	}
	if !p.attempted {
		p.attempted, p.initResult = true, p.driver.Init()
		p.initialized = p.initResult == resultSuccess
	}
	if !p.initialized {
		return p.initFailure(at)
	}
	count, returned := p.driver.Count()
	if returned != resultSuccess || count < 0 {
		return UnavailableSample(at, p.mode)
	}
	if count == 0 {
		return UnsupportedSample(at, p.mode, ReasonNoDevices)
	}
	truncated := count > MaxDevices
	if truncated {
		count = MaxDevices
	}
	sample := Sample{SchemaVersion: SchemaVersion, SampledAt: at, Mode: p.mode, Status: OverviewAvailable, IsTruncated: truncated, Devices: make([]Device, count)}
	if truncated {
		sample.ReasonCode = reasonPointer(ReasonDeviceLimit)
	}
	for index := range sample.Devices {
		sample.Devices[index] = p.readDevice(index)
	}
	return sample
}
func (p *Provider) initFailure(at time.Time) Sample {
	switch p.initResult {
	case resultLibraryNotFound, resultDriverNotLoaded:
		return UnsupportedSample(at, p.mode, ReasonDriverUnavailable)
	case resultUnsupported, resultFunctionNotFound, resultArgumentVersionMismatch, resultBackendUnavailable:
		return UnsupportedSample(at, p.mode, ReasonBackendUnavailable)
	default:
		return UnavailableSample(at, p.mode)
	}
}
func (p *Provider) readDevice(index int) Device {
	out := Device{Index: index, Utilization: Utilization{Status: StatusUnavailable}, Memory: Memory{Status: StatusUnavailable}, Temperature: Temperature{Status: StatusUnavailable}, Power: Power{Status: StatusUnavailable}}
	value, returned := p.driver.Device(index)
	if returned != resultSuccess {
		return out
	}
	if percent, result := value.Utilization(); result == resultSuccess {
		collectedAt := p.now()
		if finiteBetween(percent, 0, 100) {
			out.Utilization = AvailableUtilization(collectedAt, collectedAt, percent)
		}
	} else if isUnsupported(result) {
		out.Utilization.Status = StatusUnsupported
	}
	memory, result := value.MemoryV2()
	if isOlderDriver(result) {
		memory, result = value.MemoryV1()
	}
	if result == resultSuccess {
		collectedAt := p.now()
		if validMemory(memory) {
			out.Memory = AvailableMemory(collectedAt, collectedAt, memory.used, memory.total)
		}
	} else if isUnsupported(result) {
		out.Memory.Status = StatusUnsupported
	}
	temperature, result := value.TemperatureV1()
	if isOlderDriver(result) {
		temperature, result = value.TemperatureLegacy()
	}
	if result == resultSuccess {
		collectedAt := p.now()
		if finiteBetween(temperature, -273.15, 200) {
			out.Temperature = AvailableTemperature(collectedAt, collectedAt, temperature)
		}
	} else if isUnsupported(result) {
		out.Temperature.Status = StatusUnsupported
	}
	if milliwatts, result := value.PowerMilliwatts(); result == resultSuccess {
		collectedAt := p.now()
		watts := float64(milliwatts) / 1000
		if finiteBetween(watts, 0, 10000) {
			out.Power = AvailablePower(collectedAt, collectedAt, watts)
		}
	} else if isUnsupported(result) {
		out.Power.Status = StatusUnsupported
	}
	return out
}
func (p *Provider) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return
	}
	p.closed = true
	if p.initialized {
		_ = p.driver.Shutdown()
		p.initialized = false
	}
}
func validMemory(value memoryReading) bool {
	return value.total > 0 && value.total <= maxSafeJSONInteger && value.used <= value.total
}
func isUnsupported(value result) bool {
	return value == resultUnsupported || value == resultFunctionNotFound || value == resultArgumentVersionMismatch
}
func isOlderDriver(value result) bool { return isUnsupported(value) }
