package main

import (
	"bytes"
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
	jevDirectRouterProviderID = "tl-jev-direct-router"
	jevDirectRouterModelID    = "auto"
	jevDirectRouterDisplayName = "JEV Direct Router"
	jevDirectEndpoint = "https://api.typesafe.ai/v1/systemone"
	jevDirectModel = "jev-latest"
	jevDirectMaxResponseSize = 2 << 20
)

type jevDirectRouterStatus struct {
	Available     bool                  `json:"available"`
	PluginEnabled bool                  `json:"pluginEnabled"`
	KeyConfigured bool                  `json:"keyConfigured"`
	ProviderID    string                `json:"providerID"`
	ModelID       string                `json:"modelID"`
	DisplayName   string                `json:"displayName"`
	Models        []layaRouterCandidate `json:"models"`
	Message       string                `json:"message,omitempty"`
}

type jevDirectAnswer struct {
	Type          string             `json:"type"`
	Choice        string             `json:"choice,omitempty"`
	Score         *float64           `json:"score,omitempty"`
	Noul          *float64           `json:"noul,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
	Confidence    *float64           `json:"confidence,omitempty"`
}

type jevDirectUsage struct {
	InputTokens  int64   `json:"input_tokens,omitempty"`
	OutputTokens int64   `json:"output_tokens,omitempty"`
	Cost         float64 `json:"cost,omitempty"`
}

type jevDirectResponse struct {
	Model   string                     `json:"model,omitempty"`
	Answers map[string]jevDirectAnswer `json:"answers"`
	Usage   jevDirectUsage             `json:"usage,omitempty"`
}

type jevDirectRouterService struct {
	providers *providerManager
	plugins   *pluginManager
	health    *layaRouterService
	client    *http.Client
	endpoint  string
	model     string
}

func newJevDirectRouterService(providers *providerManager, plugins *pluginManager, health *layaRouterService) *jevDirectRouterService {
	return &jevDirectRouterService{
		providers: providers,
		plugins: plugins,
		health: health,
		client: &http.Client{Timeout: 20 * time.Second},
		endpoint: jevDirectEndpoint,
		model: jevDirectModel,
	}
}

func (s *jevDirectRouterService) pluginConfig(project string) (pluginConfig, bool, error) {
	if s == nil || s.plugins == nil {
		return pluginConfig{}, false, errors.New("plugin manager is unavailable")
	}
	config, found, err := s.plugins.store.find(project, jevDirectPluginID)
	if err != nil || !found {
		return config, found, err
	}
	return config, true, nil
}

func (s *jevDirectRouterService) apiKey(project string) (pluginConfig, string, error) {
	config, found, err := s.pluginConfig(project)
	if err != nil {
		return pluginConfig{}, "", err
	}
	if !found {
		return pluginConfig{}, "", errors.New("JEV Direct plugin is not installed")
	}
	if !config.Enabled {
		return config, "", errors.New("JEV Direct plugin is disabled")
	}
	if s.plugins.credentials == nil {
		return config, "", errors.New("TL Studio credential vault is unavailable")
	}
	value, err := s.plugins.credentials.Get(pluginCredentialID(config, jevDirectAPIKeyEnv))
	if err != nil || strings.TrimSpace(value) == "" {
		return config, "", errors.New("JEV Direct requires a TypeSafe API key")
	}
	return config, strings.TrimSpace(value), nil
}

func (s *jevDirectRouterService) candidates(ctx context.Context, project string) ([]layaRouterCandidate, error) {
	if s == nil || s.health == nil {
		return nil, errors.New("routing health service is unavailable")
	}
	candidates, err := s.health.candidates(ctx, project)
	if err != nil {
		return nil, err
	}
	result := make([]layaRouterCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		candidate.Enabled = true
		if layaCandidateReady(candidate) {
			result = append(result, candidate)
		}
	}
	return result, nil
}

func (s *jevDirectRouterService) Status(ctx context.Context, project string) (jevDirectRouterStatus, error) {
	status := jevDirectRouterStatus{
		ProviderID: jevDirectRouterProviderID,
		ModelID: jevDirectRouterModelID,
		DisplayName: jevDirectRouterDisplayName,
		Models: []layaRouterCandidate{},
	}
	config, found, err := s.pluginConfig(project)
	if err != nil {
		return status, err
	}
	if !found {
		status.Message = "Add JEV Direct from Plugins to use the direct TypeSafe router."
		return status, nil
	}
	status.PluginEnabled = config.Enabled
	if s.plugins.credentials != nil {
		if key, keyErr := s.plugins.credentials.Get(pluginCredentialID(config, jevDirectAPIKeyEnv)); keyErr == nil && strings.TrimSpace(key) != "" {
			status.KeyConfigured = true
		}
	}
	if !config.Enabled {
		if status.KeyConfigured {
			status.Message = "Enable JEV Direct to add its router to the model selector."
		} else {
			status.Message = "Configure a TypeSafe API key and enable JEV Direct."
		}
		return status, nil
	}
	if !status.KeyConfigured {
		status.Message = "JEV Direct requires a TypeSafe API key."
		return status, nil
	}
	models, err := s.candidates(ctx, project)
	if err != nil {
		return status, err
	}
	status.Models = models
	status.Available = len(models) > 0
	if !status.Available {
		status.Message = "No connected tool-capable TL Studio models are available for JEV Direct."
	}
	return status, nil
}

func (s *jevDirectRouterService) Handles(model *sessionModelRef) bool {
	return model != nil &&
		strings.TrimSpace(model.ProviderID) == jevDirectRouterProviderID &&
		strings.TrimSpace(model.ID) == jevDirectRouterModelID
}

func jevDirectRouteCriteria(candidates []layaRouterCandidate) (map[string]any, map[string]layaRouterCandidate) {
	criteria := make(map[string]any, len(candidates))
	lookup := make(map[string]layaRouterCandidate, len(candidates))
	for index, candidate := range candidates {
		key := fmt.Sprintf("m%03d", index+1)
		description := fmt.Sprintf(
			"%s / %s — cost group %s; quality %d/5; speed %d/5; tool-capable coding Agent model.",
			firstSessionString(candidate.ProviderName, candidate.ProviderID),
			firstSessionString(candidate.ModelName, candidate.ModelID),
			normalizeLayaModelGroup(candidate.Group),
			clampLayaScore(candidate.Quality),
			clampLayaScore(candidate.Speed),
		)
		criteria[key] = description
		lookup[key] = candidate
	}
	return criteria, lookup
}

func (s *jevDirectRouterService) evaluate(ctx context.Context, apiKey, prompt string, candidates []layaRouterCandidate) (nativeRouteSelection, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return nativeRouteSelection{}, errors.New("prompt is required for JEV Direct routing")
	}
	if len(candidates) == 0 {
		return nativeRouteSelection{}, errors.New("JEV Direct has no connected tool-capable models")
	}
	if len(candidates) > 255 {
		candidates = candidates[:255]
	}
	routeCriteria, routeLookup := jevDirectRouteCriteria(candidates)
	payload := map[string]any{
		"model": firstSessionString(s.model, jevDirectModel),
		"state": map[string]any{"request": prompt},
		"questions": map[string]any{
			"route": map[string]any{
				"type": "choice",
				"instructions": "Choose the best available TL Studio model to execute this coding Agent request. Balance task difficulty, model quality, speed, and cost information in the criteria. Return exactly one candidate.",
				"criteria": routeCriteria,
			},
			"difficulty": map[string]any{
				"type": "score",
				"instructions": "Rate the reasoning difficulty of this request.",
				"criteria": []string{
					"Simple or routine",
					"Moderate",
					"Hard",
					"Very hard or multi-step",
				},
			},
			"domain": map[string]any{
				"type": "choice",
				"instructions": "Classify the primary domain of the request.",
				"criteria": map[string]any{
					"code": "Programming, debugging, repository work, or software architecture",
					"math_or_logic": "Mathematics, formal logic, or symbolic reasoning",
					"data_analysis": "Data analysis, statistics, tables, or quantitative investigation",
					"general": "General reasoning or another domain",
				},
			},
			"needs_tools": map[string]any{
				"type": "noul",
				"instructions": "Does completing this request materially benefit from using coding Agent tools such as files, search, or terminal?",
			},
			"is_sensitive": map[string]any{
				"type": "noul",
				"instructions": "Does this request require unusually careful handling because errors could have high impact or irreversible consequences?",
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nativeRouteSelection{}, err
	}
	endpoint := firstSessionString(s.endpoint, jevDirectEndpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nativeRouteSelection{}, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if idempotency, randomErr := randomSecret(12); randomErr == nil {
		req.Header.Set("Idempotency-Key", "tlstudio-"+idempotency)
	}

	started := time.Now()
	client := s.client
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	response, err := client.Do(req)
	if err != nil {
		return nativeRouteSelection{}, fmt.Errorf("JEV Direct request failed: %w", err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, jevDirectMaxResponseSize+1))
	if err != nil {
		return nativeRouteSelection{}, err
	}
	if len(raw) > jevDirectMaxResponseSize {
		return nativeRouteSelection{}, errors.New("JEV Direct response is too large")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message := strings.TrimSpace(string(raw))
		if message == "" {
			message = http.StatusText(response.StatusCode)
		}
		return nativeRouteSelection{}, fmt.Errorf("JEV Direct returned status %d: %s", response.StatusCode, message)
	}
	var result jevDirectResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return nativeRouteSelection{}, fmt.Errorf("decode JEV Direct response: %w", err)
	}
	routeAnswer, ok := result.Answers["route"]
	if !ok || strings.TrimSpace(routeAnswer.Choice) == "" {
		return nativeRouteSelection{}, errors.New("JEV Direct response did not select a route")
	}
	selected, ok := routeLookup[strings.TrimSpace(routeAnswer.Choice)]
	if !ok {
		return nativeRouteSelection{}, fmt.Errorf("JEV Direct selected unknown route %q", routeAnswer.Choice)
	}
	analysis := layaRouteAnalysis{
		Domain: "general",
		Checkpoint: firstSessionString(result.Model, firstSessionString(s.model, jevDirectModel)),
		LatencyMS: float64(time.Since(started).Milliseconds()),
	}
	if answer, ok := result.Answers["difficulty"]; ok && answer.Score != nil {
		analysis.Difficulty = *answer.Score
	}
	if answer, ok := result.Answers["domain"]; ok && strings.TrimSpace(answer.Choice) != "" {
		analysis.Domain = strings.TrimSpace(answer.Choice)
	}
	if answer, ok := result.Answers["needs_tools"]; ok && answer.Noul != nil {
		analysis.NeedsTools = *answer.Noul
	}
	if answer, ok := result.Answers["is_sensitive"]; ok && answer.Noul != nil {
		analysis.Sensitive = *answer.Noul
	}
	confidence := ""
	if routeAnswer.Confidence != nil {
		confidence = fmt.Sprintf(" · confidence %.2f", *routeAnswer.Confidence)
	}
	analysis.LayaReason = "Direct TypeSafe JEV route " + strings.TrimSpace(routeAnswer.Choice) + confidence
	reason := fmt.Sprintf(
		"JEV Direct selected %s / %s%s",
		firstSessionString(selected.ProviderName, selected.ProviderID),
		firstSessionString(selected.ModelName, selected.ModelID),
		confidence,
	)
	return nativeRouteSelection{
		ProviderID: selected.ProviderID,
		ProviderName: selected.ProviderName,
		ModelID: selected.ModelID,
		ModelName: selected.ModelName,
		Profile: "jev-direct",
		Group: selected.Group,
		Quality: selected.Quality,
		Speed: selected.Speed,
		Reason: reason,
		Analysis: analysis,
		RouterProviderID: jevDirectRouterProviderID,
		RouterModelID: jevDirectRouterModelID,
		RouterName: "JEV Direct",
		RouterSource: "typesafe-system-one",
	}, nil
}

func (s *jevDirectRouterService) Route(ctx context.Context, project, prompt string, _ ...*sessionModelRef) (nativeRouteSelection, error) {
	_, key, err := s.apiKey(project)
	if err != nil {
		return nativeRouteSelection{}, err
	}
	candidates, err := s.candidates(ctx, project)
	if err != nil {
		return nativeRouteSelection{}, err
	}
	return s.evaluate(ctx, key, prompt, candidates)
}

func (s *jevDirectRouterService) Fallback(
	ctx context.Context,
	project string,
	prompt string,
	previous nativeRouteSelection,
	failure error,
) (nativeRouteSelection, bool, error) {
	if s == nil || s.health == nil || !s.health.markRouteFailure(previous, failure) {
		return nativeRouteSelection{}, false, nil
	}
	candidates, err := s.candidates(ctx, project)
	if err != nil {
		return nativeRouteSelection{}, false, err
	}
	for index := range candidates {
		candidates[index].Enabled = true
	}
	selected, reason, err := chooseLayaCandidate("balanced", previous.Analysis, candidates)
	if err != nil {
		return nativeRouteSelection{}, false, nil
	}
	if selected.ProviderID == previous.ProviderID && selected.ModelID == previous.ModelID {
		return nativeRouteSelection{}, false, nil
	}
	return nativeRouteSelection{
		ProviderID: selected.ProviderID,
		ProviderName: selected.ProviderName,
		ModelID: selected.ModelID,
		ModelName: selected.ModelName,
		Profile: "jev-direct",
		Group: selected.Group,
		Quality: selected.Quality,
		Speed: selected.Speed,
		Reason: "Automatic fallback after " + strings.TrimSpace(failure.Error()) + " · " + reason,
		Analysis: previous.Analysis,
		RouterProviderID: jevDirectRouterProviderID,
		RouterModelID: jevDirectRouterModelID,
		RouterName: "JEV Direct",
		RouterSource: "typesafe-system-one",
	}, true, nil
}

func (s *jevDirectRouterService) decorateCatalog(ctx context.Context, directory string, result *providerCatalogResponse) error {
	if result == nil {
		return nil
	}
	status, err := s.Status(ctx, directory)
	if err != nil || !status.Available {
		return err
	}
	enabled := true
	result.All = append(result.All, catalogProvider{
		ID: jevDirectRouterProviderID,
		Name: jevDirectRouterDisplayName,
		Source: "local-router",
		Models: map[string]catalogModel{
			jevDirectRouterModelID: {Name: jevDirectRouterDisplayName, Kind: "router", Enabled: &enabled},
		},
	})
	result.Connected = appendUniqueString(result.Connected, jevDirectRouterProviderID)
	result.Default[jevDirectRouterProviderID] = jevDirectRouterModelID
	return nil
}
