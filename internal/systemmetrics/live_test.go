package systemmetrics

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"
)

type fakeSource struct {
	swap                     rawSwap
	disk                     rawDisk
	rows                     []rawCounter
	swapErr, diskErr, netErr error
}

func (f *fakeSource) Swap(context.Context) (rawSwap, error)         { return f.swap, f.swapErr }
func (f *fakeSource) Disk(context.Context) (rawDisk, error)         { return f.disk, f.diskErr }
func (f *fakeSource) Network(context.Context) ([]rawCounter, error) { return f.rows, f.netErr }

func TestRawNativeSwapUnitZeroIsUnavailable(t *testing.T) {
	t.Parallel()
	if _, _, ok := projectSwap(rawSwapFromNative(4, 3, 0)); ok {
		t.Fatal("zero raw native unit projected as bytes")
	}
}

type cancelAfterSwapSource struct {
	cancel                             context.CancelFunc
	swapCalls, diskCalls, networkCalls int
}

func (f *cancelAfterSwapSource) Swap(context.Context) (rawSwap, error) {
	f.swapCalls++
	f.cancel()
	return rawSwap{totalUnits: 1, unitBytes: 1}, nil
}
func (f *cancelAfterSwapSource) Disk(context.Context) (rawDisk, error) {
	f.diskCalls++
	return rawDisk{}, nil
}
func (f *cancelAfterSwapSource) Network(context.Context) ([]rawCounter, error) {
	f.networkCalls++
	return nil, nil
}

func TestLivePreCanceledDoesNotStartProvidersOrPublish(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	hub, err := NewHub(NotReadySample(at, ModeLive))
	if err != nil {
		t.Fatal(err)
	}
	defer hub.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	source := &cancelAfterSwapSource{cancel: func() {}}
	live := newLiveWithSource(hub, []string{"uplink1"}, source, func() time.Time { return at })
	if err := live.Publish(ctx, at); !errors.Is(err, context.Canceled) {
		t.Fatalf("publish error = %v", err)
	}
	if source.swapCalls != 0 || source.diskCalls != 0 || source.networkCalls != 0 {
		t.Fatalf("provider calls = %#v", source)
	}
	if got := hub.Latest().Swap.Status; got != StatusNotReady {
		t.Fatalf("published canceled sample status = %s", got)
	}
}

func TestLiveCancellationStopsRemainingProvidersAndPublication(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	hub, err := NewHub(NotReadySample(at, ModeLive))
	if err != nil {
		t.Fatal(err)
	}
	defer hub.Close()
	ctx, cancel := context.WithCancel(context.Background())
	source := &cancelAfterSwapSource{cancel: cancel}
	live := newLiveWithSource(hub, []string{"uplink1"}, source, func() time.Time { return at })
	if err := live.Publish(ctx, at); !errors.Is(err, context.Canceled) {
		t.Fatalf("publish error = %v", err)
	}
	if source.swapCalls != 1 || source.diskCalls != 0 || source.networkCalls != 0 {
		t.Fatalf("provider calls = swap %d disk %d network %d", source.swapCalls, source.diskCalls, source.networkCalls)
	}
	if got := hub.Latest(); got.SampledAt != at || got.Swap.Status != StatusNotReady {
		t.Fatalf("published canceled sample = %#v", got)
	}
}

func TestValidateInterfacesRejectsPrivateUnsafeSelection(t *testing.T) {
	t.Parallel()
	got, err := ValidateInterfaces([]string{"uplink2", "uplink1"})
	if err != nil || len(got) != 2 || got[0] != "uplink1" {
		t.Fatalf("valid names = %#v, %v", got, err)
	}
	for _, names := range [][]string{{}, {"lo"}, {"two", "two"}, {"has space"}, {"/path"}, {"é"}, {"0123456789abcdef"}, {"a", "b", "c", "d", "e", "f", "g", "h", "i"}} {
		if _, err := ValidateInterfaces(names); err == nil {
			t.Errorf("ValidateInterfaces(%q) succeeded", names)
		}
	}
}

func TestLiveNetworkMissingOrDuplicateRowsClearBaselineThenRecover(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	for _, malformed := range [][]rawCounter{
		{{name: "uplink1", rx: 11, tx: 11}},
		{{name: "uplink1", rx: 11, tx: 11}, {name: "uplink1", rx: 12, tx: 12}, {name: "uplink2", rx: 11, tx: 11}},
	} {
		hub, err := NewHub(NotReadySample(at, ModeLive))
		if err != nil {
			t.Fatal(err)
		}
		source := &fakeSource{swap: rawSwap{totalUnits: 1, unitBytes: 1}, disk: rawDisk{blocks: 1, blockBytes: 1}, rows: []rawCounter{{name: "uplink1", rx: 10, tx: 10}, {name: "uplink2", rx: 10, tx: 10}}}
		now := at
		live := newLiveWithSource(hub, []string{"uplink1", "uplink2"}, source, func() time.Time { now = now.Add(time.Second); return now })
		_ = live.Publish(context.Background(), at)
		source.rows = malformed
		_ = live.Publish(context.Background(), at)
		if got := hub.Latest().Network.Status; got != StatusUnavailable {
			t.Errorf("malformed rows status = %s", got)
		}
		source.rows = []rawCounter{{name: "uplink1", rx: 20, tx: 20}, {name: "uplink2", rx: 20, tx: 20}}
		_ = live.Publish(context.Background(), at)
		if got := hub.Latest().Network.Status; got != StatusNotReady {
			t.Errorf("recovery baseline status = %s", got)
		}
		source.rows = []rawCounter{{name: "uplink1", rx: 22, tx: 22}, {name: "uplink2", rx: 22, tx: 22}}
		_ = live.Publish(context.Background(), at)
		if got := hub.Latest().Network.Status; got != StatusAvailable {
			t.Errorf("recovered interval status = %s", got)
		}
		hub.Close()
	}
}

func TestLiveNetworkRejectsDeltaOverflowAndReportsUnchangedZero(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	for _, test := range []struct {
		name   string
		second []rawCounter
		want   Status
		zero   bool
	}{
		{name: "overflow", second: []rawCounter{{name: "uplink1", rx: math.MaxUint64, tx: 0}, {name: "uplink2", rx: 1, tx: 0}}, want: StatusUnavailable},
		{name: "unchanged", second: []rawCounter{{name: "uplink1", rx: 0, tx: 0}, {name: "uplink2", rx: 0, tx: 0}}, want: StatusAvailable, zero: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			hub, err := NewHub(NotReadySample(at, ModeLive))
			if err != nil {
				t.Fatal(err)
			}
			defer hub.Close()
			source := &fakeSource{swap: rawSwap{totalUnits: 1, unitBytes: 1}, disk: rawDisk{blocks: 1, blockBytes: 1}, rows: []rawCounter{{name: "uplink1", rx: 0, tx: 0}, {name: "uplink2", rx: 0, tx: 0}}}
			now := at
			live := newLiveWithSource(hub, []string{"uplink1", "uplink2"}, source, func() time.Time { now = now.Add(time.Second); return now })
			_ = live.Publish(context.Background(), at)
			source.rows = test.second
			_ = live.Publish(context.Background(), at)
			sample := hub.Latest()
			if sample.Network.Status != test.want {
				t.Fatalf("status = %s, want %s", sample.Network.Status, test.want)
			}
			if test.zero && (*sample.Network.RXBytesPerSecond != 0 || *sample.Network.TXBytesPerSecond != 0) {
				t.Fatalf("rates = %v, %v", *sample.Network.RXBytesPerSecond, *sample.Network.TXBytesPerSecond)
			}
		})
	}
}

func TestLiveProjectsRawValuesAndReservedDiskSpace(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	hub, _ := NewHub(NotReadySample(at, ModeLive))
	defer hub.Close()
	source := &fakeSource{swap: rawSwap{totalUnits: 4, freeUnits: 3, unitBytes: 2}, disk: rawDisk{blocks: 10, freeBlocks: 4, availableBlocks: 3, blockBytes: 2}, rows: []rawCounter{{name: "uplink1", rx: 8, tx: 4}}}
	live := newLiveWithSource(hub, []string{"uplink1"}, source, func() time.Time { return at })
	if err := live.Publish(context.Background(), at); err != nil {
		t.Fatal(err)
	}
	sample := hub.Latest()
	if *sample.Swap.UsedBytes != 2 || *sample.Disk.UsedBytes != 12 || *sample.Disk.AvailableBytes != 6 || sample.Network.Status != StatusNotReady {
		t.Fatalf("sample = %#v", sample)
	}
}

func TestLiveNetworkUsesIndividualBaselinesAndActualElapsed(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	times := []time.Time{at, at, at, at.Add(3 * time.Second), at.Add(3 * time.Second), at.Add(3 * time.Second), at.Add(4 * time.Second), at.Add(4 * time.Second), at.Add(4 * time.Second)}
	index := 0
	hub, _ := NewHub(NotReadySample(at, ModeLive))
	defer hub.Close()
	source := &fakeSource{swap: rawSwap{totalUnits: 1, unitBytes: 1}, disk: rawDisk{blocks: 1, blockBytes: 1}, rows: []rawCounter{{name: "uplink1", rx: 10, tx: 20}, {name: "uplink2", rx: 10, tx: 20}}}
	live := newLiveWithSource(hub, []string{"uplink1", "uplink2"}, source, func() time.Time { value := times[index]; index++; return value })
	_ = live.Publish(context.Background(), at)
	source.rows = []rawCounter{{name: "uplink1", rx: 16, tx: 23}, {name: "uplink2", rx: 13, tx: 25}}
	_ = live.Publish(context.Background(), at)
	sample := hub.Latest()
	if sample.Network.Status != StatusAvailable || *sample.Network.RXBytesPerSecond != 3 || *sample.Network.TXBytesPerSecond != 8.0/3 {
		t.Fatalf("network = %#v", sample.Network)
	}
	source.rows = []rawCounter{{name: "uplink1", rx: 1, tx: 24}, {name: "uplink2", rx: 99, tx: 30}}
	_ = live.Publish(context.Background(), at)
	if got := hub.Latest().Network.Status; got != StatusNotReady {
		t.Fatalf("reset status = %s", got)
	}
}

func TestLiveMakesFailuresIndependentAndRejectsMalformedRawValues(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.January, 1, 0, 0, 2, 0, time.UTC)
	hub, _ := NewHub(NotReadySample(at, ModeLive))
	defer hub.Close()
	source := &fakeSource{swapErr: ErrUnsupported, disk: rawDisk{blocks: 1, freeBlocks: 2, blockBytes: 1}, rows: []rawCounter{{name: "uplink1", rx: math.MaxUint64, tx: 1}}}
	live := newLiveWithSource(hub, []string{"uplink1"}, source, func() time.Time { return at })
	_ = live.Publish(context.Background(), at)
	sample := hub.Latest()
	if sample.Swap.Status != StatusUnsupported || sample.Disk.Status != StatusUnavailable || sample.Network.Status != StatusNotReady {
		t.Fatalf("statuses = %s %s %s", sample.Swap.Status, sample.Disk.Status, sample.Network.Status)
	}
	source.swap = rawSwap{totalUnits: math.MaxUint64, unitBytes: 2}
	source.swapErr = nil
	source.diskErr = errors.New("ordinary")
	source.rows = []rawCounter{{name: "uplink1", rx: math.MaxUint64, tx: 1}}
	_ = live.Publish(context.Background(), at)
	sample = hub.Latest()
	if sample.Swap.Status != StatusUnavailable || sample.Disk.Status != StatusUnavailable {
		t.Fatalf("statuses = %s %s", sample.Swap.Status, sample.Disk.Status)
	}
}
