package main

import (
	"errors"
	"testing"
	"time"

	"riglight/internal/llamaswap"
	"riglight/internal/logs"
)

func TestParseConfigGatesAndValidatesLogSources(t *testing.T) {
	valid, err := parseConfig([]string{"--logs", "--llama-swap", "--llama-log-proxy", "--log-model", "small=model-1"})
	if err != nil || !valid.logs || !valid.llamaLogProxy || len(valid.logModels) != 1 || valid.logModels[0].reference != "model-1" {
		t.Fatalf("config = %#v, %v", valid, err)
	}
	loaded, err := parseConfig([]string{"--logs", "--llama-swap", "--log-loaded-model"})
	if err != nil || !loaded.logLoadedModel {
		t.Fatalf("loaded config = %#v, %v", loaded, err)
	}
	for _, arguments := range [][]string{
		{"--log-loaded-model"}, {"--logs", "--log-loaded-model"}, {"--logs", "--llama-swap", "--log-loaded-model", "--log-model", "small=model-1"}, {"--logs", "--llama-swap", "--log-loaded-model", "--log-loaded-model"},
	} {
		if _, err := parseConfig(arguments); !errors.Is(err, errInvalidArguments) {
			t.Errorf("parseConfig(%q) = %v", arguments, err)
		}
	}
	if _, err := parseConfig([]string{"--logs", "--llama-swap", "--log-loaded-model=false"}); !errors.Is(err, errInvalidArguments) {
		t.Fatal("explicit disabled loaded flag was accepted")
	}
	for _, arguments := range [][]string{
		{"--llama-log-proxy"}, {"--logs", "--llama-log-proxy"}, {"--logs", "--llama-swap", "--log-model", "x=proxy"}, {"--logs", "--llama-swap", "--log-model", "x=."}, {"--logs", "--llama-swap", "--log-model", "x=.."}, {"--logs", "--llama-swap", "--log-model", "x=author/model"}, {"--logs", "--llama-swap", "--log-model", "x=model", "--log-model", "x=other"},
	} {
		if _, err := parseConfig(arguments); !errors.Is(err, errInvalidArguments) {
			t.Errorf("parseConfig(%q) = %v", arguments, err)
		}
	}
}

func TestDemoLoadedLogSourceUsesOnlySyntheticInventory(t *testing.T) {
	at := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	hub, err := llamaswap.NewHub(llamaswap.DemoSample(at))
	if err != nil {
		t.Fatal(err)
	}
	defer hub.Close()
	sources, closeSources, err := selectLogSources(config{demo: true, logs: true, llamaSwap: true, logLoadedModel: true}, hub)
	if err != nil {
		t.Fatal(err)
	}
	defer closeSources()
	catalog, err := sources.Catalog(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Sources) != 3 || catalog.Sources[1].ID != "model.loaded" || catalog.Sources[1].Kind != logs.KindLoadedModel {
		t.Fatalf("catalog = %#v", catalog)
	}
}

func TestLogSourceSelectionNeverConstructsLiveClientsWhenDisabledOrDemo(t *testing.T) {
	for _, config := range []config{{}, {demo: true, logs: true}} {
		sources, closeSources, err := selectLogSources(config, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer closeSources()
		if config.logs && sources == nil {
			t.Fatal("demo logs missing")
		}
	}
	demo, closeSources, err := selectLogSources(config{demo: true, logs: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer closeSources()
	catalog, err := demo.Catalog(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if catalog.Mode != logs.ModeDemo || len(catalog.Sources) != 3 {
		t.Fatalf("catalog = %#v", catalog)
	}
}
