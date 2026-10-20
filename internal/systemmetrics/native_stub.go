//go:build !linux

package systemmetrics

import "context"

type unsupportedSource struct{}

func newNativeSource() liveSource                                       { return unsupportedSource{} }
func (unsupportedSource) Swap(context.Context) (rawSwap, error)         { return rawSwap{}, ErrUnsupported }
func (unsupportedSource) Disk(context.Context) (rawDisk, error)         { return rawDisk{}, ErrUnsupported }
func (unsupportedSource) Network(context.Context) ([]rawCounter, error) { return nil, ErrUnsupported }
