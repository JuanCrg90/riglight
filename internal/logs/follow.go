package logs

import (
	"context"
	"time"

	"riglight/internal/llamaswap"
)

const loadedSourceID = "model.loaded"
const loadedFreshness = 6 * time.Second
const maxSafeRevision int64 = 1<<53 - 1

type TargetState string

const (
	TargetFollowing   TargetState = "FOLLOWING"
	TargetWaiting     TargetState = "WAITING"
	TargetUnavailable TargetState = "UNAVAILABLE"
	TargetAmbiguous   TargetState = "AMBIGUOUS"
	TargetUnsupported TargetState = "UNSUPPORTED"
)

// Target is the exact target-transition envelope for an automatic source.
type Target struct {
	SchemaVersion int         `json:"schemaVersion"`
	Mode          Mode        `json:"mode"`
	SourceID      string      `json:"sourceId"`
	Revision      int64       `json:"revision"`
	State         TargetState `json:"state"`
	Model         *string     `json:"model"`
}

func (t Target) Validate() error {
	if t.SchemaVersion != SchemaVersion || (t.Mode != ModeDemo && t.Mode != ModeLive) || t.SourceID != loadedSourceID || t.Revision < 1 || t.Revision > maxSafeRevision {
		return ErrInvalid
	}
	switch t.State {
	case TargetFollowing:
		if t.Model == nil || !ValidModelReference(*t.Model) {
			return ErrInvalid
		}
	case TargetWaiting, TargetUnavailable, TargetAmbiguous, TargetUnsupported:
		if t.Model != nil {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	return nil
}

type TargetEmit func(Target) error

// TargetProvider is optional. The server uses it when a catalog source needs
// target transitions serialized with its log lines on one physical SSE writer.
type TargetProvider interface {
	StreamWithTarget(context.Context, func() error, TargetEmit, Emit) error
}

// FollowLoadedTimer permits deterministic expiry tests without changing the
// production monotonic timer behavior.
type FollowLoadedTimer interface {
	C() <-chan time.Time
	Stop() bool
}

type realFollowLoadedTimer struct{ *time.Timer }

func (t realFollowLoadedTimer) C() <-chan time.Time { return t.Timer.C }

type FollowLoadedOptions struct {
	Hub       *llamaswap.Hub
	Mode      Mode
	Now       func() time.Time
	NewTimer  func(time.Duration) FollowLoadedTimer
	NewReader func(string, *InputBudget) (Provider, error)
	Freshness time.Duration
}

// FollowLoadedProvider follows only the fresh unique LOADED model in the
// already-populated inventory hub. It never fetches inventory itself.
type FollowLoadedProvider struct {
	hub       *llamaswap.Hub
	mode      Mode
	now       func() time.Time
	newTimer  func(time.Duration) FollowLoadedTimer
	newReader func(string, *InputBudget) (Provider, error)
	freshness time.Duration
}

func NewFollowLoadedProvider(options FollowLoadedOptions) (*FollowLoadedProvider, error) {
	if options.Hub == nil || (options.Mode != ModeDemo && options.Mode != ModeLive) || options.NewReader == nil {
		return nil, ErrInvalid
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.NewTimer == nil {
		options.NewTimer = func(duration time.Duration) FollowLoadedTimer { return realFollowLoadedTimer{time.NewTimer(duration)} }
	}
	if options.Freshness == 0 {
		options.Freshness = loadedFreshness
	}
	if options.Freshness <= 0 {
		return nil, ErrInvalid
	}
	return &FollowLoadedProvider{hub: options.Hub, mode: options.Mode, now: options.Now, newTimer: options.NewTimer, newReader: options.NewReader, freshness: options.Freshness}, nil
}

// Stream preserves Provider compatibility for callers that do not expose the
// additive target protocol.
func (p *FollowLoadedProvider) Stream(ctx context.Context, ready func() error, emit Emit) error {
	return p.StreamWithTarget(ctx, ready, func(Target) error { return nil }, emit)
}

func (p *FollowLoadedProvider) StreamWithTarget(ctx context.Context, ready func() error, targetEmit TargetEmit, emit Emit) error {
	if ready == nil || targetEmit == nil || emit == nil {
		return ErrInvalid
	}
	subscription, err := p.hub.Subscribe()
	if err != nil {
		return ErrUnavailable
	}
	defer subscription.Unsubscribe()
	if err := ready(); err != nil {
		return err
	}

	budget := NewInputBudget()
	var timer FollowLoadedTimer
	var timerUpdates <-chan time.Time
	var latest llamaswap.Sample
	var active *followLoadedReader
	var previous Target
	hasPrevious := false

	stopTimer := func() {
		if timer != nil {
			timer.Stop()
			timer = nil
			timerUpdates = nil
		}
	}
	stopReader := func() error {
		if active == nil {
			return nil
		}
		active.cancel()
		if !active.finished {
			active.result = <-active.done
			active.finished = true
		}
		for {
			select {
			case <-active.lines:
			default:
				err := active.result
				active = nil
				return err
			}
		}
	}
	defer func() {
		stopTimer()
		_ = stopReader()
	}()

	apply := func(sample llamaswap.Sample) error {
		latest = sample
		selection, remaining := p.selectTarget(sample)
		stopTimer()
		if remaining > 0 {
			timer = p.newTimer(remaining)
			if timer == nil {
				return ErrInvalid
			}
			timerUpdates = timer.C()
		}
		if hasPrevious && sameTarget(previous, selection) {
			return nil
		}
		// A cancellation caused by a target transition is expected. Its reader
		// result is intentionally discarded before the new target is announced.
		_ = stopReader()
		selection.Revision = 1
		if hasPrevious {
			selection.Revision = previous.Revision + 1
		}
		if selection.Revision > maxSafeRevision {
			return ErrInvalid
		}
		if selection.State == TargetFollowing && budget.Exhausted() {
			return ErrByteLimit
		}
		if err := targetEmit(selection); err != nil {
			return err
		}
		previous, hasPrevious = selection, true
		if selection.State != TargetFollowing {
			return nil
		}
		reader, err := p.newReader(*selection.Model, budget)
		if err != nil {
			return err
		}
		active = startFollowLoadedReader(ctx, reader)
		return nil
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		// A pending inventory update always wins over a queued line, so a line
		// from the replaced model cannot pass its target transition.
		select {
		case sample := <-subscription.Updates():
			if err := apply(sample); err != nil {
				return err
			}
			continue
		default:
		}
		select {
		case <-timerUpdates:
			if err := apply(latest); err != nil {
				return err
			}
			continue
		default:
		}
		// The clock is checked at the line boundary as well as by the timer.
		// This prevents a concurrently ready timer from letting stale output win
		// a select race.
		if active != nil && hasPrevious {
			selection, _ := p.selectTarget(latest)
			if !sameTarget(previous, selection) {
				if err := apply(latest); err != nil {
					return err
				}
				continue
			}
		}
		if active != nil && active.finished {
			select {
			case line := <-active.lines:
				if err := emit(line.stream, line.text, line.truncated); err != nil {
					return err
				}
				continue
			default:
				err := active.result
				active = nil
				return err
			}
		}
		var lines <-chan followLoadedLine
		var done <-chan error
		if active != nil {
			lines, done = active.lines, active.done
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case sample := <-subscription.Updates():
			if err := apply(sample); err != nil {
				return err
			}
		case <-timerUpdates:
			if err := apply(latest); err != nil {
				return err
			}
		case line := <-lines:
			if err := emit(line.stream, line.text, line.truncated); err != nil {
				return err
			}
		case err := <-done:
			active.finished, active.result = true, err
		}
	}
}

func (p *FollowLoadedProvider) selectTarget(sample llamaswap.Sample) (Target, time.Duration) {
	result := Target{SchemaVersion: SchemaVersion, Mode: p.mode, SourceID: loadedSourceID, State: TargetUnavailable}
	if sample.Mode != llamaswap.Mode(p.mode) || sample.Status == llamaswap.OverviewUnsupported {
		if sample.Status == llamaswap.OverviewUnsupported && sample.Mode == llamaswap.Mode(p.mode) {
			result.State = TargetUnsupported
		}
		return result, 0
	}
	if sample.Status != llamaswap.OverviewAvailable || sample.IsTruncated {
		return result, 0
	}
	remaining, fresh := sample.Freshness(p.now(), p.freshness)
	if !fresh {
		return result, 0
	}
	loaded := ""
	count := 0
	for _, model := range sample.Models {
		if model.State == llamaswap.StateLoaded {
			loaded, count = model.ID, count+1
		}
	}
	switch {
	case count == 0:
		result.State = TargetWaiting
	case count > 1:
		result.State = TargetAmbiguous
	case !ValidModelReference(loaded):
		result.State = TargetUnsupported
	default:
		result.State = TargetFollowing
		result.Model = &loaded
	}
	return result, remaining
}

func sameTarget(left, right Target) bool {
	if left.State != right.State {
		return false
	}
	if left.Model == nil || right.Model == nil {
		return left.Model == nil && right.Model == nil
	}
	return *left.Model == *right.Model
}

type followLoadedLine struct {
	stream    Stream
	text      string
	truncated bool
}

type followLoadedReader struct {
	cancel   context.CancelFunc
	lines    chan followLoadedLine
	done     chan error
	finished bool
	result   error
}

func startFollowLoadedReader(parent context.Context, provider Provider) *followLoadedReader {
	ctx, cancel := context.WithCancel(parent)
	reader := &followLoadedReader{cancel: cancel, lines: make(chan followLoadedLine, 1), done: make(chan error, 1)}
	go func() {
		defer func() {
			if closer, ok := provider.(interface{ CloseIdleConnections() }); ok {
				closer.CloseIdleConnections()
			}
		}()
		err := provider.Stream(ctx, func() error { return nil }, func(stream Stream, text string, truncated bool) error {
			line := followLoadedLine{stream: stream, text: text, truncated: truncated}
			select {
			case reader.lines <- line:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
		reader.done <- err
	}()
	return reader
}

var _ Provider = (*FollowLoadedProvider)(nil)
var _ TargetProvider = (*FollowLoadedProvider)(nil)
