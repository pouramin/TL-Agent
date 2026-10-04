package main

import (
	"context"
	"errors"
	"strings"
)

const (
	jevDirectPluginID = "jev-direct"
	jevDirectAPIKeyEnv = "TYPESAFE_API_KEY"
)

type jevDirectPluginIntegration struct{}

func (jevDirectPluginIntegration) Matches(config pluginConfig) bool {
	return config.ID == jevDirectPluginID || strings.EqualFold(config.Metadata["integration"], jevDirectPluginID)
}

func jevDirectKeyConfigured(config pluginConfig) bool {
	for _, item := range config.Environment {
		if item.Name == jevDirectAPIKeyEnv && item.Configured {
			return true
		}
	}
	return false
}

func (jevDirectPluginIntegration) Snapshot(config pluginConfig, project string) *pluginIntegrationView {
	configured := jevDirectKeyConfigured(config)
	view := &pluginIntegrationView{
		ID: jevDirectPluginID,
		Details: map[string]string{
			"endpoint": firstSessionString(config.Metadata["endpoint"], jevDirectEndpoint),
			"model": firstSessionString(config.Metadata["model"], jevDirectModel),
			"apiKeyConfigured": boolText(configured),
		},
	}
	if configured {
		view.Status = "Ready"
		view.Summary = "TypeSafe API key configured · direct router"
	} else {
		view.Status = "API Key Required"
		view.Summary = "Configure a TypeSafe API key to enable direct routing"
	}
	return view
}

func (jevDirectPluginIntegration) ValidateStart(config pluginConfig, project string) error {
	if config.Type != pluginTypeRouter {
		return errors.New("JEV Direct must use the router plugin type")
	}
	if !jevDirectKeyConfigured(config) {
		return errors.New("JEV Direct requires a TypeSafe API key")
	}
	return nil
}

func (jevDirectPluginIntegration) RunAction(ctx context.Context, manager *pluginManager, config pluginConfig, project, actionID string) (any, error) {
	return nil, errors.New("JEV Direct has no server-side plugin action")
}
