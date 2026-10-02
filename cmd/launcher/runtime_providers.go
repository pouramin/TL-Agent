package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const providerRegistryVersion = 1

var providerProtocolPackages = map[string]string{
	"openai-compatible":      "@ai-sdk/openai-compatible",
	"openai-responses":       "@ai-sdk/openai",
	"anthropic-messages":     "@ai-sdk/anthropic",
	"gemini-generate-content": "@google/genai",
	"codex-chatgpt":            "@openai/codex",
}

type tlProviderModel struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Kind         string `json:"kind,omitempty"`
	ToolCall     bool   `json:"toolCall"`
	Reasoning       bool   `json:"reasoning"`
	ReasoningEffort string `json:"reasoningEffort,omitempty"`
	ContextLimit    int    `json:"contextLimit,omitempty"`
	OutputLimit  int    `json:"outputLimit,omitempty"`
}

type tlProviderDefinition struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Protocol  string            `json:"protocol"`
	BaseURL   string            `json:"baseURL"`
	ManagedBy string            `json:"managedBy,omitempty"`
	ProjectID string            `json:"projectId,omitempty"`
	Models    []tlProviderModel `json:"models"`
}

type providerRegistryFile struct {
	Version   int                    `json:"version"`
	Providers []tlProviderDefinition `json:"providers"`
}

type providerRegistryStore struct {
	mu        sync.Mutex
	loaded    bool
	existed   bool
	filePath  string
	providers map[string]tlProviderDefinition
}

func newProviderRegistryStore(filePath string) *providerRegistryStore {
	return &providerRegistryStore{filePath: filePath, providers: map[string]tlProviderDefinition{}}
}

func providerRegistryPath() string {
	if dir := strings.TrimSpace(os.Getenv("TL_STUDIO_STATE_DIR")); dir != "" {
		return filepath.Join(dir, "providers.json")
	}
	base, err := os.UserConfigDir()
	if err != nil || strings.TrimSpace(base) == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "TL Studio", "providers.json")
}

func cloneProviderDefinition(input tlProviderDefinition) tlProviderDefinition {
	result := input
	result.Models = append([]tlProviderModel(nil), input.Models...)
	return result
}

func (s *providerRegistryStore) loadLocked() error {
	if s.loaded { return nil }
	s.loaded = true
	s.providers = map[string]tlProviderDefinition{}
	data, err := os.ReadFile(s.filePath)
	if errors.Is(err, os.ErrNotExist) { return nil }
	if err != nil { return err }
	s.existed = true
	var stored providerRegistryFile
	if err := json.Unmarshal(data, &stored); err != nil { return fmt.Errorf("decode provider registry: %w", err) }
	if stored.Version != 0 && stored.Version != providerRegistryVersion {
		return fmt.Errorf("unsupported provider registry version %d", stored.Version)
	}
	for _, provider := range stored.Providers {
		normalized, err := normalizeProviderDefinition(provider)
		if err == nil { s.providers[normalized.ID] = normalized }
	}
	return nil
}

func (s *providerRegistryStore) snapshot() ([]tlProviderDefinition, bool, error) {
	s.mu.Lock(); defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil { return nil, false, err }
	result := make([]tlProviderDefinition, 0, len(s.providers))
	for _, provider := range s.providers { result = append(result, cloneProviderDefinition(provider)) }
	sort.Slice(result, func(i, j int) bool {
		return strings.ToLower(result[i].Name+"\x00"+result[i].ID) < strings.ToLower(result[j].Name+"\x00"+result[j].ID)
	})
	return result, s.existed, nil
}

func (s *providerRegistryStore) get(id string) (tlProviderDefinition, bool, error) {
	s.mu.Lock(); defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil { return tlProviderDefinition{}, false, err }
	provider, ok := s.providers[strings.TrimSpace(id)]
	return cloneProviderDefinition(provider), ok, nil
}

func (s *providerRegistryStore) replace(providers []tlProviderDefinition) error {
	s.mu.Lock(); defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil { return err }
	next := make(map[string]tlProviderDefinition, len(providers))
	for _, provider := range providers {
		normalized, err := normalizeProviderDefinition(provider)
		if err != nil { return err }
		next[normalized.ID] = normalized
	}
	s.providers = next
	s.existed = true
	return s.persistLocked()
}

func (s *providerRegistryStore) put(provider tlProviderDefinition) error {
	normalized, err := normalizeProviderDefinition(provider)
	if err != nil { return err }
	s.mu.Lock(); defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil { return err }
	s.providers[normalized.ID] = normalized
	s.existed = true
	return s.persistLocked()
}

func (s *providerRegistryStore) remove(id string) error {
	s.mu.Lock(); defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil { return err }
	delete(s.providers, strings.TrimSpace(id))
	s.existed = true
	return s.persistLocked()
}

func (s *providerRegistryStore) persistLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.filePath), 0o700); err != nil { return err }
	providers := make([]tlProviderDefinition, 0, len(s.providers))
	for _, provider := range s.providers { providers = append(providers, cloneProviderDefinition(provider)) }
	sort.Slice(providers, func(i, j int) bool { return providers[i].ID < providers[j].ID })
	data, err := json.MarshalIndent(providerRegistryFile{Version: providerRegistryVersion, Providers: providers}, "", "  ")
	if err != nil { return err }
	temp, err := os.CreateTemp(filepath.Dir(s.filePath), "providers-*.tmp")
	if err != nil { return err }
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(0o600); err != nil { _ = temp.Close(); return err }
	if _, err := temp.Write(data); err != nil { _ = temp.Close(); return err }
	if err := temp.Close(); err != nil { return err }
	if err := os.Rename(tempName, s.filePath); err != nil {
		if writeErr := os.WriteFile(s.filePath, data, 0o600); writeErr != nil { return err }
	}
	return nil
}

func validProviderID(id string) bool {
	if id == "" { return false }
	for index, r := range id {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || index > 0 && (r == '-' || r == '_') { continue }
		return false
	}
	return true
}

func normalizeProviderDefinition(input tlProviderDefinition) (tlProviderDefinition, error) {
	input.ID = strings.TrimSpace(input.ID)
	input.Name = strings.TrimSpace(input.Name)
	input.Protocol = strings.TrimSpace(input.Protocol)
	input.BaseURL = strings.TrimSpace(input.BaseURL)
	if !validProviderID(input.ID) {
		return tlProviderDefinition{}, errors.New("provider ID must use lowercase letters, numbers, dashes, or underscores")
	}
	if input.Name == "" { return tlProviderDefinition{}, errors.New("provider display name is required") }
	if _, ok := providerProtocolPackages[input.Protocol]; !ok {
		return tlProviderDefinition{}, errors.New("unsupported provider protocol")
	}
	parsed, err := url.Parse(input.BaseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return tlProviderDefinition{}, errors.New("provider base URL must be a valid http(s) URL")
	}
	input.BaseURL = strings.TrimRight(parsed.String(), "/")
	input.ManagedBy = strings.ToLower(strings.TrimSpace(input.ManagedBy))
	input.ProjectID = strings.TrimSpace(input.ProjectID)
	if input.ManagedBy != "" && input.ManagedBy != "jev" && input.ManagedBy != "account" {
		return tlProviderDefinition{}, fmt.Errorf("unsupported provider manager %q", input.ManagedBy)
	}
	if len(input.Models) == 0 { return tlProviderDefinition{}, errors.New("provider must define at least one model") }
	seen := map[string]bool{}
	models := make([]tlProviderModel, 0, len(input.Models))
	for _, model := range input.Models {
		model.ID = strings.TrimSpace(model.ID)
		model.Name = strings.TrimSpace(model.Name)
		model.Kind = strings.ToLower(strings.TrimSpace(model.Kind))
		model.ReasoningEffort = strings.ToLower(strings.TrimSpace(model.ReasoningEffort))
		if model.Kind != "" && model.Kind != "router" {
			return tlProviderDefinition{}, fmt.Errorf("unsupported model kind %q", model.Kind)
		}
		if model.ReasoningEffort != "" {
			switch model.ReasoningEffort {
			case "none", "minimal", "low", "medium", "high", "xhigh", "max":
			default:
				return tlProviderDefinition{}, fmt.Errorf("unsupported model reasoning effort %q", model.ReasoningEffort)
			}
		}
		if model.ID == "" { return tlProviderDefinition{}, errors.New("model ID is required") }
		if seen[model.ID] { return tlProviderDefinition{}, fmt.Errorf("duplicate model ID %q", model.ID) }
		if model.Name == "" { model.Name = model.ID }
		if model.ContextLimit < 0 || model.OutputLimit < 0 { return tlProviderDefinition{}, errors.New("model limits cannot be negative") }
		seen[model.ID] = true
		models = append(models, model)
	}
	sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
	input.Models = models
	return input, nil
}

func providerModelUsesNativeAgent(provider tlProviderDefinition, model tlProviderModel) bool {
	if _, ok := providerProtocolPackages[provider.Protocol]; !ok { return false }
	if model.ToolCall { return true }
	return model.ID == jevRouterModelID && model.Kind == "router" && isOpenRouterBaseURL(provider.BaseURL)
}

func migrateManagedProviderMetadata(providers []tlProviderDefinition) ([]tlProviderDefinition, bool) {
	result := make([]tlProviderDefinition, len(providers))
	copy(result, providers)
	changed := false
	for index := range result {
		provider := &result[index]
		if provider.ManagedBy != "" || !isOpenRouterBaseURL(provider.BaseURL) || len(provider.Models) != 1 { continue }
		model := provider.Models[0]
		if model.ID == jevRouterModelID && model.Kind == "router" {
			provider.ManagedBy = "jev"
			changed = true
		}
	}
	return result, changed
}

type providerManager struct {
	state       *appState
	store       *providerRegistryStore
	credentials providerCredentialStore
	registryMu  sync.Mutex
	accountMu   sync.RWMutex
	accounts    map[string]providerAccountAdapter
}

func newProviderManager(state *appState) *providerManager {
	return &providerManager{
		state: state,
		store: newProviderRegistryStore(providerRegistryPath()),
		credentials: newProviderCredentialStore(),
		accounts: map[string]providerAccountAdapter{},
	}
}

func (m *providerManager) ensureRegistryInitialized(_ context.Context) ([]tlProviderDefinition, error) {
	m.registryMu.Lock(); defer m.registryMu.Unlock()
	providers, _, err := m.store.snapshot()
	if err != nil { return nil, err }
	if migrated, changed := migrateManagedProviderMetadata(providers); changed {
		if err := m.store.replace(migrated); err != nil { return nil, err }
		return migrated, nil
	}
	return providers, nil
}

func (m *providerManager) ensureBootstrapped(ctx context.Context) error {
	_, err := m.ensureRegistryInitialized(ctx)
	return err
}

func (m *providerManager) registerAccountAdapter(adapter providerAccountAdapter) {
	if m == nil || adapter == nil {
		return
	}
	id := strings.TrimSpace(adapter.ID())
	if id == "" {
		return
	}
	m.accountMu.Lock()
	defer m.accountMu.Unlock()
	if m.accounts == nil {
		m.accounts = map[string]providerAccountAdapter{}
	}
	m.accounts[id] = adapter
}

func (m *providerManager) accountAdapter(providerID string) providerAccountAdapter {
	if m == nil {
		return nil
	}
	m.accountMu.RLock()
	defer m.accountMu.RUnlock()
	return m.accounts[strings.TrimSpace(providerID)]
}

func (m *providerManager) effectiveCredential(ctx context.Context, providerID, directory string) (string, error) {
	if m == nil || m.credentials == nil {
		return "", errCredentialNotFound
	}
	if adapter := m.accountAdapter(providerID); adapter != nil {
		value, err := adapter.ResolveCredential(ctx, directory)
		if err == nil && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value), nil
		}
		if err != nil && !errors.Is(err, errCredentialNotFound) {
			return "", err
		}
	} else if value, err := getProviderCredentialSlot(m.credentials, providerID, providerCredentialSlotAccount); err == nil {
		if credential, structured, decodeErr := decodeProviderOAuthCredential(value); decodeErr != nil {
			return "", decodeErr
		} else if structured {
			if !credential.needsRefresh(time.Now()) {
				return strings.TrimSpace(credential.AccessToken), nil
			}
		} else if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value), nil
		}
	} else if !errors.Is(err, errCredentialNotFound) {
		return "", err
	}
	return getProviderCredentialSlot(m.credentials, providerID, providerCredentialSlotAPI)
}


type catalogModel struct {
	Name    string `json:"name"`
	Kind    string `json:"kind,omitempty"`
	Enabled *bool  `json:"enabled,omitempty"`
	Variant any    `json:"variant,omitempty"`
}

type catalogProvider struct {
	ID     string                  `json:"id"`
	Name   string                  `json:"name"`
	Source string                  `json:"source"`
	Models map[string]catalogModel `json:"models"`
}

type providerCatalogResponse struct {
	All       []catalogProvider `json:"all"`
	Connected []string          `json:"connected"`
	Default   map[string]string `json:"default"`
	Failed    []json.RawMessage `json:"failed,omitempty"`
}

func managedCatalogProvider(definition tlProviderDefinition) catalogProvider {
	result := catalogProvider{
		ID: definition.ID, Name: definition.Name, Source: "custom",
		Models: map[string]catalogModel{},
	}
	for _, configured := range definition.Models {
		enabled := providerModelUsesNativeAgent(definition, configured)
		model := catalogModel{Name: configured.Name, Kind: configured.Kind, Enabled: &enabled}
		if model.Name == "" { model.Name = configured.ID }
		result.Models[configured.ID] = model
	}
	return result
}

func appendUniqueString(values []string, value string) []string {
	for _, existing := range values { if existing == value { return values } }
	return append(values, value)
}

func (m *providerManager) catalog(ctx context.Context, _ string) (providerCatalogResponse, error) {
	definitions, err := m.ensureRegistryInitialized(ctx)
	if err != nil { return providerCatalogResponse{}, err }
	jevConfig, err := loadJevRouterConfig()
	if err != nil { return providerCatalogResponse{}, err }
	result := providerCatalogResponse{
		All: []catalogProvider{}, Connected: []string{}, Default: map[string]string{}, Failed: []json.RawMessage{},
	}
	for _, definition := range definitions {
		provider := managedCatalogProvider(definition)
		if !jevConfig.Enabled && providerHasJevRouter(definition) {
			if model, ok := provider.Models[jevRouterModelID]; ok {
				disabled := false
				model.Enabled = &disabled
				provider.Models[jevRouterModelID] = model
			}
		}
		result.All = append(result.All, provider)
		for _, model := range definition.Models {
			if entry, ok := provider.Models[model.ID]; ok && entry.Enabled != nil && *entry.Enabled {
				if _, exists := result.Default[definition.ID]; !exists { result.Default[definition.ID] = model.ID }
			}
		}
		if m.credentials != nil {
			if key, credentialErr := m.effectiveCredential(ctx, definition.ID, ""); credentialErr == nil && strings.TrimSpace(key) != "" {
				result.Connected = appendUniqueString(result.Connected, definition.ID)
			} else if credentialErr != nil && !errors.Is(credentialErr, errCredentialNotFound) {
				return providerCatalogResponse{}, credentialErr
			}
		}
	}
	sort.Slice(result.All, func(i, j int) bool {
		return strings.ToLower(result.All[i].Name+"\x00"+result.All[i].ID) < strings.ToLower(result.All[j].Name+"\x00"+result.All[j].ID)
	})
	return result, nil
}

type providerWriteRequest struct {
	Provider tlProviderDefinition `json:"provider"`
	APIKey   string               `json:"apiKey,omitempty"`
}

type providerConfigResponse struct {
	Providers []tlProviderDefinition `json:"providers"`
}

func writeProviderManagerError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if strings.Contains(err.Error(), "unsupported provider") || strings.Contains(err.Error(), "must ") ||
		strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "valid http") {
		status = http.StatusBadRequest
	}
	writeJSON(w, status, jsonError{Error: err.Error()})
}

func registerProviderRoutes(mux *http.ServeMux, manager *providerManager) {
	mux.HandleFunc("GET /local/providers/catalog", func(w http.ResponseWriter, r *http.Request) {
		catalog, err := manager.catalog(r.Context(), r.URL.Query().Get("directory"))
		if err != nil { writeProviderManagerError(w, err); return }
		writeJSON(w, http.StatusOK, catalog)
	})
	mux.HandleFunc("GET /local/providers/config", func(w http.ResponseWriter, r *http.Request) {
		providers, err := manager.ensureRegistryInitialized(r.Context())
		if err != nil { writeProviderManagerError(w, err); return }
		writeJSON(w, http.StatusOK, providerConfigResponse{Providers: providers})
	})
	mux.HandleFunc("PUT /local/providers/config/{id}", func(w http.ResponseWriter, r *http.Request) {
		if _, err := manager.ensureRegistryInitialized(r.Context()); err != nil { writeProviderManagerError(w, err); return }
		var input providerWriteRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 2<<20)).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, jsonError{Error: "invalid provider JSON body"}); return
		}
		input.Provider.ID = strings.TrimSpace(input.Provider.ID)
		pathID := strings.TrimSpace(r.PathValue("id"))
		if input.Provider.ID == "" { input.Provider.ID = pathID }
		if input.Provider.ID != pathID {
			writeJSON(w, http.StatusBadRequest, jsonError{Error: "provider ID does not match request path"}); return
		}
		provider, err := normalizeProviderDefinition(input.Provider)
		if err != nil { writeJSON(w, http.StatusBadRequest, jsonError{Error: err.Error()}); return }
		if err := manager.store.put(provider); err != nil { writeProviderManagerError(w, err); return }
		if strings.TrimSpace(input.APIKey) != "" {
			if manager.credentials == nil { writeProviderManagerError(w, errors.New("TL Studio credential store is unavailable")); return }
			if err := putProviderCredentialSlot(manager.credentials, provider.ID, providerCredentialSlotAPI, input.APIKey); err != nil { writeProviderManagerError(w, err); return }
		}
		writeJSON(w, http.StatusOK, provider)
	})
	mux.HandleFunc("DELETE /local/providers/config/{id}/api-connection", func(w http.ResponseWriter, r *http.Request) {
		if _, err := manager.ensureRegistryInitialized(r.Context()); err != nil { writeProviderManagerError(w, err); return }
		id := strings.TrimSpace(r.PathValue("id"))
		if !validProviderID(id) { writeJSON(w, http.StatusBadRequest, jsonError{Error: "invalid provider ID"}); return }
		provider, found, err := manager.store.get(id)
		if err != nil { writeProviderManagerError(w, err); return }
		if manager.credentials != nil {
			if err := deleteProviderCredentialSlot(manager.credentials, id, providerCredentialSlotAPI); err != nil { writeProviderManagerError(w, err); return }
		}
		removedProvider := false
		if found && provider.ManagedBy != "account" && provider.ManagedBy != "jev" {
			if err := manager.store.remove(id); err != nil { writeProviderManagerError(w, err); return }
			if err := removeProviderDiscoveryCache(id); err != nil { writeProviderManagerError(w, err); return }
			removedProvider = true
		}
		writeJSON(w, http.StatusOK, map[string]any{"disconnected": id, "providerRemoved": removedProvider})
	})
	mux.HandleFunc("DELETE /local/providers/config/{id}", func(w http.ResponseWriter, r *http.Request) {
		if _, err := manager.ensureRegistryInitialized(r.Context()); err != nil { writeProviderManagerError(w, err); return }
		id := strings.TrimSpace(r.PathValue("id"))
		if !validProviderID(id) { writeJSON(w, http.StatusBadRequest, jsonError{Error: "invalid provider ID"}); return }
		if _, ok, err := manager.store.get(id); err != nil { writeProviderManagerError(w, err); return
		} else if !ok { writeJSON(w, http.StatusNotFound, jsonError{Error: "provider is not managed by TL Studio"}); return }
		if manager.credentials != nil {
			if err := deleteProviderCredentialSlot(manager.credentials, id, providerCredentialSlotAPI); err != nil { writeProviderManagerError(w, err); return }
			if err := deleteProviderCredentialSlot(manager.credentials, id, providerCredentialSlotAccount); err != nil { writeProviderManagerError(w, err); return }
		}
		if err := manager.store.remove(id); err != nil { writeProviderManagerError(w, err); return }
		if err := removeProviderDiscoveryCache(id); err != nil { writeProviderManagerError(w, err); return }
		writeJSON(w, http.StatusOK, map[string]any{"removed": id})
	})
}
