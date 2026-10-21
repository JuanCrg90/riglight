package logs

import (
	"context"
	"net/url"
)

// LlamaProvider performs only the two admitted llama-swap log operations.
type LlamaProvider struct{ plain *PlainHTTPProvider }

func NewLlamaProvider(base, reference string, proxy bool) (*LlamaProvider, error) {
	return NewLlamaProviderWithBudget(base, reference, proxy, nil)
}

func NewLlamaProviderWithBudget(base, reference string, proxy bool, budget *InputBudget) (*LlamaProvider, error) {
	canonical, err := CanonicalBaseURL(base)
	if err != nil {
		return nil, err
	}
	path := "/logs/stream/"
	if proxy {
		path += "proxy"
	} else {
		if !ValidModelReference(reference) {
			return nil, ErrInvalid
		}
		path += url.PathEscape(reference)
	}
	return &LlamaProvider{plain: NewPlainHTTPProviderWithBudget(canonical+path, budget)}, nil
}
func (p *LlamaProvider) Stream(ctx context.Context, ready func() error, emit Emit) error {
	return p.plain.Stream(ctx, ready, emit)
}
func (p *LlamaProvider) CloseIdleConnections() { p.plain.CloseIdleConnections() }

type DemoProvider struct{ Lines []string }

func (p DemoProvider) Stream(ctx context.Context, ready func() error, emit Emit) error {
	if err := ready(); err != nil {
		return err
	}
	for _, line := range p.Lines {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := emit(StreamOutput, line, false); err != nil {
			return err
		}
	}
	return nil
}

// DemoFollowProvider emits only synthetic lines and then remains quiet until
// cancellation, matching a live follow session without any HTTP client.
type DemoFollowProvider struct{ Lines []string }

func (p DemoFollowProvider) Stream(ctx context.Context, ready func() error, emit Emit) error {
	if err := (DemoProvider{Lines: p.Lines}).Stream(ctx, ready, emit); err != nil {
		return err
	}
	<-ctx.Done()
	return ctx.Err()
}
