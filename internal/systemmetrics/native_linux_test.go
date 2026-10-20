//go:build linux

package systemmetrics

import (
	"errors"
	"syscall"
	"testing"
)

func TestNativeCapabilityErrorsMapToUnsupported(t *testing.T) {
	t.Parallel()
	if !errors.Is(nativeError(syscall.ENOSYS), ErrUnsupported) || !errors.Is(nativeError(syscall.EOPNOTSUPP), ErrUnsupported) {
		t.Fatal("native capability errors did not map to unsupported")
	}
	if errors.Is(nativeError(errors.New("ordinary")), ErrUnsupported) {
		t.Fatal("ordinary native error mapped to unsupported")
	}
}
