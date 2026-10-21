package main

import (
	"errors"
	"testing"

	"riglight/internal/logs"
)

func TestParseConfigGatesAndValidatesLogSources(t *testing.T) {
	valid, err := parseConfig([]string{"--logs", "--llama-swap", "--llama-log-proxy", "--log-model", "small=model-1"})
	if err != nil || !valid.logs || !valid.llamaLogProxy || len(valid.logModels) != 1 || valid.logModels[0].reference != "model-1" {
		t.Fatalf("config = %#v, %v", valid, err)
	}
	for _, arguments := range [][]string{
		{"--llama-log-proxy"}, {"--logs", "--llama-log-proxy"}, {"--logs", "--llama-swap", "--log-model", "x=proxy"}, {"--logs", "--llama-swap", "--log-model", "x=author/model"}, {"--logs", "--llama-swap", "--log-model", "x=model", "--log-model", "x=other"},
	} {
		if _, err := parseConfig(arguments); !errors.Is(err, errInvalidArguments) {
			t.Errorf("parseConfig(%q) = %v", arguments, err)
		}
	}
}

func TestLogSourceSelectionNeverConstructsLiveClientsWhenDisabledOrDemo(t *testing.T) {
	for _, config := range []config{{}, {demo: true, logs: true}} {
		sources, closeSources, err := selectLogSources(config)
		if err != nil {
			t.Fatal(err)
		}
		defer closeSources()
		if config.logs && sources == nil {
			t.Fatal("demo logs missing")
		}
	}
	demo, closeSources, err := selectLogSources(config{demo: true, logs: true})
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
