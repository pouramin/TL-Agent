package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	jevDirectPluginID = "jev-direct"
	jevDirectAPIKeyEnv = "TYPESAFE_API_KEY"
	jevDirectModelsEndpoint = "https://api.typesafe.ai/v1/models"
	jevDirectCredentialTestTimeout = 10 * time.Second
	jevDirectCredentialTestMaxResponse = 1 << 20
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
			"profile": jevDirectRoutingProfile(config),
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

func (jevDirectPluginIntegration) Test(ctx context.Context, manager *pluginManager, config pluginConfig, project string) error {
	if manager == nil || manager.credentials == nil {
		return errors.New("TL Studio credential vault is unavailable")
	}
	key, err := manager.credentials.Get(pluginCredentialID(config, jevDirectAPIKeyEnv))
	if err != nil || strings.TrimSpace(key) == "" {
		return errors.New("JEV Direct requires a TypeSafe API key")
	}

	endpoint := firstSessionString(config.Metadata["modelsEndpoint"], jevDirectModelsEndpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(key))
	req.Header.Set("Accept", "application/json")

	response, err := (&http.Client{Timeout: jevDirectCredentialTestTimeout}).Do(req)
	if err != nil {
		return fmt.Errorf("TypeSafe connection test failed: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, jevDirectCredentialTestMaxResponse+1))
	if err != nil {
		return fmt.Errorf("read TypeSafe model catalog: %w", err)
	}
	if len(body) > jevDirectCredentialTestMaxResponse {
		return errors.New("TypeSafe model catalog response is too large")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = http.StatusText(response.StatusCode)
		}
		return fmt.Errorf("TypeSafe credential validation returned status %d: %s", response.StatusCode, message)
	}

	var payload struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("decode TypeSafe model catalog: %w", err)
	}
	target := firstSessionString(config.Metadata["model"], jevDirectModel)
	for _, model := range payload.Models {
		if strings.TrimSpace(model.Name) == target {
			return nil
		}
	}
	return fmt.Errorf("TypeSafe account does not expose configured JEV model %q", target)
}

func (jevDirectPluginIntegration) RunAction(ctx context.Context, manager *pluginManager, config pluginConfig, project, actionID string) (any, error) {
	return nil, errors.New("JEV Direct has no server-side plugin action")
}
