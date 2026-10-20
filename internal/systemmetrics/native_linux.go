//go:build linux

package systemmetrics

import (
	"context"
	"errors"
	"syscall"

	"github.com/shirou/gopsutil/v4/net"
)

type nativeSource struct{}

func newNativeSource() liveSource { return nativeSource{} }
func (nativeSource) Swap(context.Context) (rawSwap, error) {
	var info syscall.Sysinfo_t
	if err := syscall.Sysinfo(&info); err != nil {
		return rawSwap{}, nativeError(err)
	}
	unit := uint64(info.Unit)
	if unit == 0 {
		unit = 1
	}
	return rawSwap{totalUnits: info.Totalswap, freeUnits: info.Freeswap, unitBytes: unit}, nil
}
func (nativeSource) Disk(context.Context) (rawDisk, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err != nil {
		return rawDisk{}, nativeError(err)
	}
	if stat.Bsize < 0 {
		return rawDisk{}, errors.New("invalid block size")
	}
	return rawDisk{blocks: stat.Blocks, freeBlocks: stat.Bfree, availableBlocks: stat.Bavail, blockBytes: uint64(stat.Bsize)}, nil
}
func (nativeSource) Network(ctx context.Context) ([]rawCounter, error) {
	counters, err := net.IOCountersWithContext(ctx, true)
	if err != nil {
		return nil, err
	}
	out := make([]rawCounter, len(counters))
	for index, counter := range counters {
		out[index] = rawCounter{name: counter.Name, rx: counter.BytesRecv, tx: counter.BytesSent}
	}
	return out, nil
}
func nativeError(err error) error {
	if errors.Is(err, syscall.ENOSYS) || errors.Is(err, syscall.EOPNOTSUPP) {
		return ErrUnsupported
	}
	return err
}
