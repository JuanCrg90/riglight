package main

import (
	"riglight/internal/llamaswap"
	"riglight/internal/logs"
)

type logModel struct{ alias, reference string }

func selectLogSources(config config, hub *llamaswap.Hub) (logs.CatalogProvider, func(), error) {
	if !config.logs {
		return nil, func() {}, nil
	}
	if config.demo {
		entries := []logs.Entry{{Source: logs.Source{ID: "llama.proxy", Label: "llama-swap proxy", Kind: logs.KindLlamaProxy}, Provider: logs.DemoProvider{Lines: []string{"demo proxy log"}}}}
		if config.logLoadedModel {
			provider, err := newLoadedLogProvider(hub, logs.ModeDemo, func(string, *logs.InputBudget) (logs.Provider, error) {
				return logs.DemoFollowProvider{Lines: []string{"demo loaded model log"}}, nil
			})
			if err != nil {
				return nil, func() {}, err
			}
			entries = append(entries, logs.Entry{Source: logs.Source{ID: "model.loaded", Label: "Currently loaded model", Kind: logs.KindLoadedModel}, Provider: provider})
		} else {
			entries = append(entries, logs.Entry{Source: logs.Source{ID: "model.example-model", Label: "example-model", Kind: logs.KindModel}, Provider: logs.DemoProvider{Lines: []string{"demo model log"}}})
		}
		entries = append(entries, logs.Entry{Source: logs.Source{ID: "docker.example-api", Label: "example-api", Kind: logs.KindDocker}, Provider: logs.DemoProvider{Lines: []string{"demo container log"}}})
		registry, err := logs.NewRegistry(logs.ModeDemo, entries)
		return registry, func() {}, err
	}
	entries := make([]logs.Entry, 0, 1+len(config.logModels)+boolCount(config.logLoadedModel))
	closers := make([]interface{ CloseIdleConnections() }, 0, cap(entries)+1)
	if config.llamaLogProxy {
		provider, err := logs.NewLlamaProvider(config.llamaSwapURL, "", true)
		if err != nil {
			return nil, func() {}, err
		}
		entries = append(entries, logs.Entry{Source: logs.Source{ID: "llama.proxy", Label: "llama-swap proxy", Kind: logs.KindLlamaProxy}, Provider: provider})
		closers = append(closers, provider)
	}
	for _, model := range config.logModels {
		provider, err := logs.NewLlamaProvider(config.llamaSwapURL, model.reference, false)
		if err != nil {
			return nil, func() {}, err
		}
		entries = append(entries, logs.Entry{Source: logs.Source{ID: "model." + model.alias, Label: model.alias, Kind: logs.KindModel}, Provider: provider})
		closers = append(closers, provider)
	}
	if config.logLoadedModel {
		provider, err := newLoadedLogProvider(hub, logs.ModeLive, func(reference string, budget *logs.InputBudget) (logs.Provider, error) {
			return logs.NewLlamaProviderWithBudget(config.llamaSwapURL, reference, false, budget)
		})
		if err != nil {
			return nil, func() {}, err
		}
		entries = append(entries, logs.Entry{Source: logs.Source{ID: "model.loaded", Label: "Currently loaded model", Kind: logs.KindLoadedModel}, Provider: provider})
	}
	registry, err := logs.NewRegistry(logs.ModeLive, entries)
	if err != nil {
		return nil, func() {}, err
	}
	var helper *logs.HelperDockerCatalog
	if config.docker {
		helper, err = logs.NewHelperDockerCatalog(config.dockerProxyURL)
		if err != nil {
			return nil, func() {}, err
		}
		closers = append(closers, helper)
	}
	return logs.NewCompositeCatalog(registry, helper), func() {
		for _, closer := range closers {
			closer.CloseIdleConnections()
		}
	}, nil
}

func newLoadedLogProvider(hub *llamaswap.Hub, mode logs.Mode, reader func(string, *logs.InputBudget) (logs.Provider, error)) (*logs.FollowLoadedProvider, error) {
	return logs.NewFollowLoadedProvider(logs.FollowLoadedOptions{Hub: hub, Mode: mode, NewReader: reader})
}
