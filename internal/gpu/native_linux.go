//go:build linux && cgo

package gpu

import "github.com/NVIDIA/go-nvml/pkg/nvml"

/*
 * Copyright 1993-2026 NVIDIA Corporation.  All rights reserved.
 *
 * NOTICE TO USER:
 *
 * This source code is subject to NVIDIA ownership rights under U.S. and
 * international Copyright laws.  Users and possessors of this source code
 * are hereby granted a nonexclusive, royalty-free license to use this code
 * in individual and commercial software.
 *
 * NVIDIA MAKES NO REPRESENTATION ABOUT THE SUITABILITY OF THIS SOURCE
 * CODE FOR ANY PURPOSE.  IT IS PROVIDED "AS IS" WITHOUT EXPRESS OR
 * IMPLIED WARRANTY OF ANY KIND.  NVIDIA DISCLAIMS ALL WARRANTIES WITH
 * REGARD TO THIS SOURCE CODE, INCLUDING ALL IMPLIED WARRANTIES OF
 * MERCHANTABILITY, NONINFRINGEMENT, AND FITNESS FOR A PARTICULAR PURPOSE.
 * IN NO EVENT SHALL NVIDIA BE LIABLE FOR ANY SPECIAL, INDIRECT, INCIDENTAL,
 * OR CONSEQUENTIAL DAMAGES, OR ANY DAMAGES WHATSOEVER RESULTING FROM LOSS
 * OF USE, DATA OR PROFITS,  WHETHER IN AN ACTION OF CONTRACT, NEGLIGENCE
 * OR OTHER TORTIOUS ACTION,  ARISING OUT OF OR IN CONNECTION WITH THE USE
 * OR PERFORMANCE OF THIS SOURCE CODE.
 *
 * U.S. Government End Users.   This source code is a "commercial item" as
 * that term is defined at  48 C.F.R. 2.101 (OCT 1995), consisting  of
 * "commercial computer  software"  and "commercial computer software
 * documentation" as such terms are  used in 48 C.F.R. 12.212 (SEPT 1995)
 * and is provided to the U.S. Government only as a commercial end item.
 * Consistent with 48 C.F.R.12.212 and 48 C.F.R. 227.7202-1 through
 * 227.7202-4 (JUNE 1995), all U.S. Government End Users acquire the
 * source code with only those rights set forth herein.
 *
 * Any use of this source code in individual and commercial software must
 * include, in the user documentation and internal comments to the code,
 * the above Disclaimer and U.S. Government End Users Notice.
 */

// NewNativeProvider dynamically loads NVML only when its owner first polls.
func NewNativeProvider() *Provider { return NewProvider(nvmlDriver{}, ModeLive) }

type nvmlDriver struct{}
type nvmlDevice struct{ device nvml.Device }

func (nvmlDriver) Init() result     { return mapResult(nvml.Init()) }
func (nvmlDriver) Shutdown() result { return mapResult(nvml.Shutdown()) }
func (nvmlDriver) Count() (int, result) {
	count, returned := nvml.DeviceGetCount()
	return count, mapResult(returned)
}
func (nvmlDriver) Device(index int) (device, result) {
	value, returned := nvml.DeviceGetHandleByIndex(index)
	if returned != nvml.SUCCESS {
		return nil, mapResult(returned)
	}
	return nvmlDevice{device: value}, resultSuccess
}
func (d nvmlDevice) Utilization() (float64, result) {
	value, returned := d.device.GetUtilizationRates()
	return float64(value.Gpu), mapResult(returned)
}
func (d nvmlDevice) MemoryV2() (memoryReading, result) {
	value, returned := d.device.GetMemoryInfo_v2()
	return memoryReading{used: value.Used, total: value.Total}, mapResult(returned)
}
func (d nvmlDevice) MemoryV1() (memoryReading, result) {
	value, returned := d.device.GetMemoryInfo()
	return memoryReading{used: value.Used, total: value.Total}, mapResult(returned)
}
func (d nvmlDevice) TemperatureV1() (float64, result) {
	value, returned := d.device.GetTemperatureV().V1()
	return float64(value.Temperature), mapResult(returned)
}
func (d nvmlDevice) TemperatureLegacy() (float64, result) {
	value, returned := d.device.GetTemperature(nvml.TEMPERATURE_GPU)
	return float64(value), mapResult(returned)
}
func (d nvmlDevice) PowerMilliwatts() (uint32, result) {
	value, returned := d.device.GetPowerUsage()
	return value, mapResult(returned)
}
func mapResult(value nvml.Return) result {
	switch value {
	case nvml.SUCCESS:
		return resultSuccess
	case nvml.ERROR_NOT_SUPPORTED:
		return resultUnsupported
	case nvml.ERROR_FUNCTION_NOT_FOUND:
		return resultFunctionNotFound
	case nvml.ERROR_ARGUMENT_VERSION_MISMATCH:
		return resultArgumentVersionMismatch
	case nvml.ERROR_LIBRARY_NOT_FOUND:
		return resultLibraryNotFound
	case nvml.ERROR_DRIVER_NOT_LOADED:
		return resultDriverNotLoaded
	case nvml.ERROR_NO_PERMISSION:
		return resultPermission
	default:
		return resultUnknown
	}
}
