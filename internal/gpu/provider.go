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
type Provider struct {
	mu                  sync.Mutex
	driver              driver
	mode                Mode
	initialized, closed bool
}

func NewProvider(driver driver, mode Mode) *Provider { return &Provider{driver: driver, mode: mode} }
func (p *Provider) Poll(at time.Time) Sample {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed || p.driver == nil {
		return UnsupportedSample(at, p.mode, ReasonBackendUnavailable)
	}
	if !p.initialized {
		switch p.driver.Init() {
		case resultSuccess:
			p.initialized = true
		case resultLibraryNotFound:
			return UnsupportedSample(at, p.mode, ReasonDriverUnavailable)
		default:
			return UnsupportedSample(at, p.mode, ReasonBackendUnavailable)
		}
	}
	count, returned := p.driver.Count()
	if returned != resultSuccess {
		return UnavailableSample(at, p.mode)
	}
	if count <= 0 {
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
		sample.Devices[index] = p.readDevice(at, index)
	}
	return sample
}
func (p *Provider) readDevice(at time.Time, index int) Device {
	out := Device{Index: index, Utilization: Utilization{Status: StatusUnavailable}, Memory: Memory{Status: StatusUnavailable}, Temperature: Temperature{Status: StatusUnavailable}, Power: Power{Status: StatusUnavailable}}
	value, returned := p.driver.Device(index)
	if returned != resultSuccess {
		return out
	}
	if percent, result := value.Utilization(); result == resultSuccess && finiteBetween(percent, 0, 100) {
		out.Utilization = AvailableUtilization(at, at, percent)
	} else if isUnsupported(result) {
		out.Utilization.Status = StatusUnsupported
	}
	memory, result := value.MemoryV2()
	if isOlderDriver(result) {
		memory, result = value.MemoryV1()
	}
	if result == resultSuccess && validMemory(memory) {
		out.Memory = AvailableMemory(at, at, memory.used, memory.total)
	} else if isUnsupported(result) {
		out.Memory.Status = StatusUnsupported
	}
	temperature, result := value.TemperatureV1()
	if isOlderDriver(result) {
		temperature, result = value.TemperatureLegacy()
	}
	if result == resultSuccess && finiteBetween(temperature, -273.15, 200) {
		out.Temperature = AvailableTemperature(at, at, temperature)
	} else if isUnsupported(result) {
		out.Temperature.Status = StatusUnsupported
	}
	if milliwatts, result := value.PowerMilliwatts(); result == resultSuccess && float64(milliwatts)/1000 <= 10000 {
		out.Power = AvailablePower(at, at, float64(milliwatts)/1000)
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
