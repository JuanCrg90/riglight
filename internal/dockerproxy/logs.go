package dockerproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	"riglight/internal/logs"
)

// LogProvider has a separate client and no relationship to snapshot polling.
type LogProvider struct {
	targets map[string]Target
	client  *http.Client
}

func NewLogProvider(config Config) (*LogProvider, error) {
	if _, err := NewConfig(config.Socket, config.Listen, targetsToSelections(config.Containers)); err != nil {
		return nil, err
	}
	targets := make(map[string]Target, len(config.Containers))
	for _, target := range config.Containers {
		targets[target.Alias] = target
	}
	return &LogProvider{targets: targets, client: logs.NewUnixHTTPClient(config.Socket)}, nil
}
func (p *LogProvider) CloseIdleConnections() { p.client.CloseIdleConnections() }
func (p *LogProvider) Aliases() []string {
	aliases := make([]string, 0, len(p.targets))
	for _, target := range p.targets {
		aliases = append(aliases, target.Alias)
	}
	// Config order is not preserved in a map; catalog order is a presentation
	// detail, but deterministic output is useful and avoids accidental leakage.
	for i := range aliases {
		for j := i + 1; j < len(aliases); j++ {
			if aliases[j] < aliases[i] {
				aliases[i], aliases[j] = aliases[j], aliases[i]
			}
		}
	}
	return aliases
}
func (p *LogProvider) Stream(ctx context.Context, alias string, ready func() error, emit logs.Emit) error {
	target, ok := p.targets[alias]
	if !ok {
		return logs.ErrInvalid
	}
	inspect, err := p.inspect(ctx, target)
	if err != nil {
		return err
	}
	return p.stream(ctx, inspect, ready, emit)
}

type logInspect struct {
	ID     string `json:"Id"`
	Name   string `json:"Name"`
	Config *struct {
		TTY *bool `json:"Tty"`
	} `json:"Config"`
}

func (p *LogProvider) inspect(ctx context.Context, target Target) (logInspect, error) {
	var inspect logInspect
	response, finish, err := p.get(ctx, "/v1.44/containers/"+target.Reference+"/json")
	if err != nil {
		return inspect, err
	}
	defer finish()
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return inspect, logs.ErrUnavailable
	}
	if response.StatusCode != http.StatusOK {
		return inspect, logs.ErrUnavailable
	}
	body, err := readBoundedJSON(response.Body)
	if err != nil {
		return inspect, err
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&inspect); err != nil {
		return inspect, logs.ErrInvalid
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return inspect, logs.ErrInvalid
	}
	if !fullID(inspect.ID) || inspect.Config == nil || inspect.Config.TTY == nil || !inspectMatches(target.Reference, inspect) {
		return logInspect{}, logs.ErrInvalid
	}
	return inspect, nil
}
func inspectMatches(reference string, inspect logInspect) bool {
	if fullID(reference) {
		return inspect.ID == reference
	}
	return inspect.Name == "/"+reference
}
func (p *LogProvider) stream(ctx context.Context, inspect logInspect, ready func() error, emit logs.Emit) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker/v1.44/containers/"+inspect.ID+"/logs?stdout=1&stderr=1&follow=1&tail=200&timestamps=0", nil)
	if err != nil {
		return logs.ErrInvalid
	}
	response, finish, err := logs.OpenHTTP(ctx, p.client, request)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return logs.ErrUnavailable
	}
	defer finish()
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusMethodNotAllowed || response.StatusCode == http.StatusNotImplemented {
		return logs.ErrUnsupported
	}
	if response.StatusCode != http.StatusOK || !dockerContentType(response.Header.Get("Content-Type")) {
		return logs.ErrInvalid
	}
	if err := ready(); err != nil {
		return err
	}
	return logs.ConsumeDocker(ctx, response.Body, *inspect.Config.TTY, emit)
}
func dockerContentType(value string) bool {
	media, _, err := mime.ParseMediaType(value)
	return err == nil && (media == "application/vnd.docker.raw-stream" || media == "application/octet-stream")
}
func (p *LogProvider) get(ctx context.Context, path string) (*http.Response, func(), error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker"+path, nil)
	if err != nil {
		return nil, func() {}, logs.ErrInvalid
	}
	response, finish, err := logs.OpenHTTP(ctx, p.client, request)
	if err != nil {
		if ctx.Err() != nil {
			return nil, func() {}, ctx.Err()
		}
		return nil, func() {}, logs.ErrUnavailable
	}
	return response, finish, nil
}

const maxInspectBytes = 64 << 10

func readBoundedJSON(body io.Reader) ([]byte, error) {
	value, err := io.ReadAll(io.LimitReader(body, maxInspectBytes+1))
	if err != nil || len(value) > maxInspectBytes {
		return nil, logs.ErrInvalid
	}
	return value, nil
}
