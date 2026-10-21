package main

import "riglight/internal/logs"

type logModel struct{ alias, reference string }

func selectLogSources(config config) (logs.CatalogProvider, func(), error) {
	if !config.logs {
		return nil, func() {}, nil
	}
	if config.demo {
		entries := []logs.Entry{
			{Source: logs.Source{ID: "llama.proxy", Label: "llama-swap proxy", Kind: logs.KindLlamaProxy}, Provider: logs.DemoProvider{Lines: []string{"demo proxy log"}}},
			{Source: logs.Source{ID: "model.example-model", Label: "example-model", Kind: logs.KindModel}, Provider: logs.DemoProvider{Lines: []string{"demo model log"}}},
			{Source: logs.Source{ID: "docker.example-api", Label: "example-api", Kind: logs.KindDocker}, Provider: logs.DemoProvider{Lines: []string{"demo container log"}}},
		}
		registry, err := logs.NewRegistry(logs.ModeDemo, entries)
		return registry, func() {}, err
	}
	entries := make([]logs.Entry, 0, 1+len(config.logModels))
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
