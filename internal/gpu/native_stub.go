//go:build !linux || !cgo

package gpu

// NewNativeProvider preserves CPU/demo builds where NVML cannot be linked.
func NewNativeProvider() *Provider { return NewProvider(nil, ModeLive) }
