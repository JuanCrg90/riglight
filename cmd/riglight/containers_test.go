package main

import (
	"errors"
	"testing"
	"time"

	"riglight/internal/containers"
)

func TestContainerSelectionNeverConstructsHTTPForDisabledOrDemo(t *testing.T) {
	t.Parallel()
	at := time.Now()
	for _, config := range []config{{}, {demo: true, docker: true}} {
		hub, err := containers.NewHub(initialContainerSample(at, config))
		if err != nil {
			t.Fatal(err)
		}
		defer hub.Close()
		calls := 0
		worker, err := selectContainerWorker(hub, config, func(string) (containers.Source, error) { calls++; return nil, errors.New("must not construct") })
		if err != nil || calls != 0 || (config.docker && worker == nil) || (!config.docker && worker != nil) {
			t.Fatalf("worker=%#v calls=%d err=%v", worker, calls, err)
		}
	}
}

func TestDockerCLIRequiresExplicitLiveProxyAndKeepsDemoSynthetic(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"--docker"}, {"--docker-proxy-url", "http://127.0.0.1:9123"}, {"--docker", "--docker-proxy-url", "http://localhost:9123"}} {
		if _, err := parseConfig(args); !errors.Is(err, errInvalidArguments) {
			t.Errorf("parseConfig(%q) = %v", args, err)
		}
	}
	live, err := parseConfig([]string{"--docker", "--docker-proxy-url", "http://127.0.0.1:9123"})
	if err != nil || !live.docker || live.dockerProxyURL == "" {
		t.Fatalf("live = %#v, %v", live, err)
	}
	demo, err := parseConfig([]string{"--demo", "--docker"})
	if err != nil || !demo.demo || !demo.docker || demo.dockerProxyURL != "" {
		t.Fatalf("demo = %#v, %v", demo, err)
	}
}
