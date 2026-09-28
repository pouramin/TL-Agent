package main

import (
	"context"
	"errors"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type memoryProviderCredentialStore struct {
	mu      sync.Mutex
	secrets map[string]string
}

func newMemoryProviderCredentialStore() *memoryProviderCredentialStore {
	return &memoryProviderCredentialStore{secrets: map[string]string{}}
}

func (s *memoryProviderCredentialStore) Backend() string { return "memory-test" }

func (s *memoryProviderCredentialStore) Put(providerID, secret string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.secrets[providerID] = secret
	return nil
}

func (s *memoryProviderCredentialStore) Get(providerID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.secrets[providerID]
	if !ok {
		return "", errCredentialNotFound
	}
	return value, nil
}

func (s *memoryProviderCredentialStore) Delete(providerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.secrets, providerID)
	return nil
}

func testProviderDefinition() tlProviderDefinition {
	return tlProviderDefinition{
		ID:       "example-provider",
		Name:     "Example Provider",
		Protocol: "openai-compatible",
		BaseURL:  "https://api.example.com/v1",
		Models: []tlProviderModel{{
			ID:           "example-model",
			Name:         "Example Model",
			ToolCall:     true,
			Reasoning:    true,
			ContextLimit: 128000,
			OutputLimit:  16384,
		}},
	}
}

func TestProviderRegistryPersistsTLStudioSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "providers.json")
	store := newProviderRegistryStore(path)
	if err := store.put(testProviderDefinition()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, forbidden := range []string{"@ai-sdk/", "apiKey", "kilo-auto/free"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("TL Studio provider registry leaked runtime detail %q: %s", forbidden, text)
		}
	}
	if !strings.Contains(text, `"protocol": "openai-compatible"`) {
		t.Fatalf("registry missing TL Studio protocol: %s", text)
	}

	reloaded := newProviderRegistryStore(path)
	providers, existed, err := reloaded.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !existed || len(providers) != 1 || providers[0].ID != "example-provider" {
		t.Fatalf("unexpected reloaded registry: existed=%v providers=%#v", existed, providers)
	}
}

func TestLegacyJevOnlyProviderMigratesToManagedIntegration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "providers.json")
	store := newProviderRegistryStore(path)
	if err := store.put(tlProviderDefinition{
		ID: "openrouter", Name: "OpenRouter", Protocol: "openai-compatible", BaseURL: openRouterBaseURL,
		Models: []tlProviderModel{{
			ID: jevRouterModelID, Name: jevRouterDisplayName, Kind: "router", ToolCall: false,
		}},
	}); err != nil {
		t.Fatal(err)
	}

	manager := &runtimeProviderManager{store: store}
	providers, err := manager.ensureRegistryInitialized(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(providers) != 1 || providers[0].ManagedBy != "jev" {
		t.Fatalf("legacy Jev-only provider was not migrated to managed integration metadata: %#v", providers)
	}

	reloaded := newProviderRegistryStore(path)
	persisted, existed, err := reloaded.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !existed || len(persisted) != 1 || persisted[0].ManagedBy != "jev" {
		t.Fatalf("migrated managedBy metadata was not persisted: %#v", persisted)
	}
}

func TestNativeProviderSaveSkipsKiloCompatibilitySync(t *testing.T) {
	project := t.TempDir()
	stateDir := t.TempDir()
	t.Setenv("TL_STUDIO_STATE_DIR", stateDir)

	var runtimeCalls int
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		runtimeCalls++
		http.Error(w, "native provider should not touch compatibility runtime", http.StatusBadRequest)
	}))
	defer backend.Close()

	state := &appState{project: project}
	manager, err := newRuntimeProviderManager(state, backend.URL, "runtime", "secret")
	if err != nil {
		t.Fatal(err)
	}
	manager.credentials = newMemoryProviderCredentialStore()
	if err := manager.store.replace([]tlProviderDefinition{}); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	registerRuntimeProviderRoutes(mux, manager)
	server := httptest.NewServer(mux)
	defer server.Close()

	definition := testProviderDefinition()
	body, _ := json.Marshal(providerWriteRequest{Provider: definition, APIKey: "native-secret"})
	req, _ := http.NewRequest(http.MethodPut, server.URL+"/runtime/providers/config/"+definition.ID, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(res.Body)
		t.Fatalf("native provider save failed: status=%d body=%s", res.StatusCode, data)
	}
	if runtimeCalls != 0 {
		t.Fatalf("native provider save unexpectedly touched Kilo compatibility runtime %d times", runtimeCalls)
	}
	if stored, ok, err := manager.store.get(definition.ID); err != nil || !ok || stored.ID != definition.ID {
		t.Fatalf("native provider was not persisted: stored=%#v ok=%v err=%v", stored, ok, err)
	}
	if key, err := manager.credentials.Get(definition.ID); err != nil || key != "native-secret" {
		t.Fatalf("native provider credential was not persisted: key=%q err=%v", key, err)
	}
}

func TestJevRouterNeverNeedsKiloCompatibilitySync(t *testing.T) {
	provider := tlProviderDefinition{
		ID: "openrouter", Name: "OpenRouter", Protocol: "openai-compatible", BaseURL: openRouterBaseURL,
		ManagedBy: "jev",
		Models: []tlProviderModel{{
			ID: jevRouterModelID, Name: jevRouterDisplayName, Kind: "router", ToolCall: false,
		}},
	}
	if providerNeedsRuntimeCompatibility(provider) {
		t.Fatal("official OpenRouter Jev Router must stay entirely on TL Studio native execution")
	}
}

func TestRuntimeProviderRoutesTranslateTLStudioConfig(t *testing.T) {
	project := t.TempDir()
	stateDir := t.TempDir()
	t.Setenv("TL_STUDIO_STATE_DIR", stateDir)

	var mu sync.Mutex
	var lastPatch map[string]any
	var credentialBody map[string]any
	disposeCalls := 0
	authDeleteCalls := 0

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "runtime" || pass != "secret" {
			t.Fatalf("unexpected runtime auth: %q %q %v", user, pass, ok)
		}
		if got := r.Header.Get("x-kilo-directory"); got == "" {
			t.Fatal("runtime provider bridge did not scope request to the selected project")
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/config/overlay":
			_, _ = io.WriteString(w, `{"effective":{"provider":{},"disabled_providers":[]}}`)
		case r.Method == http.MethodPatch && r.URL.Path == "/config/overlay":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			mu.Lock()
			lastPatch = body
			mu.Unlock()
			_, _ = io.WriteString(w, `{"ok":true}`)
		case r.Method == http.MethodPut && r.URL.Path == "/auth/example-provider":
			if err := json.NewDecoder(r.Body).Decode(&credentialBody); err != nil {
				t.Fatal(err)
			}
			_, _ = io.WriteString(w, `{"ok":true}`)
		case r.Method == http.MethodDelete && r.URL.Path == "/auth/example-provider":
			authDeleteCalls++
			_, _ = io.WriteString(w, `{"ok":true}`)
		case r.Method == http.MethodPost && r.URL.Path == "/global/dispose":
			disposeCalls++
			_, _ = io.WriteString(w, `{"ok":true}`)
		case r.Method == http.MethodGet && r.URL.Path == "/provider":
			_, _ = io.WriteString(w, `{"all":[{"id":"kilo","name":"Hosted","models":{"kilo-auto/free":{"name":"Auto Free"},"kilo-paid/model":{"name":"Paid Kilo Model"}}},{"id":"openrouter","name":"OpenRouter Runtime Catalog","models":{"openai/gpt-paid":{"name":"Runtime Only Paid Model"}}},{"id":"example-provider","name":"Example Provider","models":{"example-model":{"name":"Example Model"},"unselected-model":{"name":"Unselected Runtime Model"}}}],"connected":["kilo","openrouter","example-provider"],"default":{"kilo":"kilo-auto/free"},"failed":[]}`)
		default:
			t.Fatalf("unexpected runtime request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer backend.Close()

	state := &appState{project: project, backendURL: backend.URL, frontendURL: "http://127.0.0.1"}
	manager, err := newRuntimeProviderManager(state, backend.URL, "runtime", "secret")
	if err != nil {
		t.Fatal(err)
	}
	credentialStore := newMemoryProviderCredentialStore()
	manager.credentials = credentialStore
	mux := http.NewServeMux()
	registerRuntimeProviderRoutes(mux, manager)
	server := httptest.NewServer(mux)
	defer server.Close()

	get, err := http.Get(server.URL + "/runtime/providers/config")
	if err != nil {
		t.Fatal(err)
	}
	_ = get.Body.Close()
	if get.StatusCode != http.StatusOK {
		t.Fatalf("initial provider config status=%d", get.StatusCode)
	}

	definition := testProviderDefinition()
	definition.Models[0].ToolCall = false // exercise the compatibility-only runtime bridge
	body, _ := json.Marshal(providerWriteRequest{Provider: definition, APIKey: "top-secret"})
	req, _ := http.NewRequest(http.MethodPut, server.URL+"/runtime/providers/config/example-provider", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("provider save status=%d", res.StatusCode)
	}

	mu.Lock()
	patch := lastPatch
	mu.Unlock()
	set, _ := patch["set"].(map[string]any)
	runtimeProviders, _ := set["provider"].(map[string]any)
	runtimeProvider, _ := runtimeProviders["example-provider"].(map[string]any)
	if runtimeProvider["npm"] != "@ai-sdk/openai-compatible" {
		t.Fatalf("runtime translation missing package: %#v", runtimeProvider)
	}
	if credentialBody["key"] != "top-secret" {
		t.Fatalf("credential was not synchronized to the runtime execution store: %#v", credentialBody)
	}
	if stored, err := credentialStore.Get("example-provider"); err != nil || stored != "top-secret" {
		t.Fatalf("TL Studio credential store did not become source of truth: value=%q err=%v", stored, err)
	}

	registryData, err := os.ReadFile(filepath.Join(stateDir, "providers.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(registryData), "top-secret") || strings.Contains(string(registryData), "@ai-sdk") {
		t.Fatalf("TL Studio registry must not persist credential/runtime package details: %s", registryData)
	}

	catalogRes, err := http.Get(server.URL + "/runtime/providers/catalog?directory=" + url.QueryEscape(project))
	if err != nil {
		t.Fatal(err)
	}
	defer catalogRes.Body.Close()
	if catalogRes.StatusCode != http.StatusOK {
		t.Fatalf("catalog status=%d", catalogRes.StatusCode)
	}
	var catalog providerCatalogResponse
	if err := json.NewDecoder(catalogRes.Body).Decode(&catalog); err != nil {
		t.Fatal(err)
	}
	if catalog.Hosted.ProviderID != "kilo" || len(catalog.Hosted.PreferredModels) != 1 || catalog.Hosted.PreferredModels[0] != "kilo-auto/free" {
		t.Fatalf("hosted metadata missing from TL Studio catalog: %#v", catalog.Hosted)
	}
	foundCustom := false
	for _, provider := range catalog.All {
		if provider.ID == "example-provider" {
			foundCustom = provider.Source == "custom"
		}
	}
	if !foundCustom {
		t.Fatalf("custom provider was not marked as TL Studio-managed: %#v", catalog.All)
	}
	if len(catalog.All) != 2 {
		t.Fatalf("runtime-only providers must not leak into the TL Studio selector catalog: %#v", catalog.All)
	}
	for _, provider := range catalog.All {
		switch provider.ID {
		case "kilo":
			if len(provider.Models) != 1 {
				t.Fatalf("hosted Kilo must expose only preferred free models: %#v", provider.Models)
			}
			if _, ok := provider.Models["kilo-auto/free"]; !ok {
				t.Fatalf("Kilo Auto Free missing from hosted catalog: %#v", provider.Models)
			}
			if _, ok := provider.Models["kilo-paid/model"]; ok {
				t.Fatalf("paid Kilo model leaked into normal selector: %#v", provider.Models)
			}
		case "example-provider":
			if len(provider.Models) != 1 {
				t.Fatalf("managed provider must expose only user-selected models: %#v", provider.Models)
			}
			if _, ok := provider.Models["example-model"]; !ok {
				t.Fatalf("selected custom model missing: %#v", provider.Models)
			}
			if _, ok := provider.Models["unselected-model"]; ok {
				t.Fatalf("unselected runtime model leaked into managed provider: %#v", provider.Models)
			}
		default:
			t.Fatalf("unexpected runtime-only provider leaked into selector catalog: %#v", provider)
		}
	}

	deleteReq, _ := http.NewRequest(http.MethodDelete, server.URL+"/runtime/providers/config/example-provider", nil)
	deleteRes, err := http.DefaultClient.Do(deleteReq)
	if err != nil {
		t.Fatal(err)
	}
	_ = deleteRes.Body.Close()
	if deleteRes.StatusCode != http.StatusOK {
		t.Fatalf("provider delete status=%d", deleteRes.StatusCode)
	}
	if authDeleteCalls != 1 || disposeCalls < 2 {
		t.Fatalf("delete/dispose calls unexpected: authDelete=%d dispose=%d", authDeleteCalls, disposeCalls)
	}
	if _, err := credentialStore.Get("example-provider"); !errors.Is(err, errCredentialNotFound) {
		t.Fatalf("TL Studio credential was not removed with provider: %v", err)
	}
}

func TestDisabledJevRouterIsHiddenFromSelectableCatalog(t *testing.T) {
	project := t.TempDir()
	stateDir := t.TempDir()
	t.Setenv("TL_STUDIO_STATE_DIR", stateDir)
	if _, err := saveJevRouterConfig(jevRouterConfig{Enabled: false}); err != nil {
		t.Fatal(err)
	}

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/provider":
			_, _ = io.WriteString(w, `{"all":[{"id":"kilo","name":"Hosted","models":{"kilo-auto/free":{"name":"Auto Free"}}}],"connected":[],"default":{},"failed":[]}`)
		default:
			t.Fatalf("unexpected runtime request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer backend.Close()

	state := &appState{project: project}
	manager, err := newRuntimeProviderManager(state, backend.URL, "runtime", "secret")
	if err != nil {
		t.Fatal(err)
	}
	manager.credentials = newMemoryProviderCredentialStore()
	if err := manager.store.replace([]tlProviderDefinition{{
		ID: "openrouter", Name: "OpenRouter", Protocol: "openai-compatible", BaseURL: openRouterBaseURL, ManagedBy: "jev",
		Models: []tlProviderModel{{ID: jevRouterModelID, Name: jevRouterDisplayName, Kind: "router", ToolCall: false}},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := manager.credentials.Put("openrouter", "stored-key"); err != nil {
		t.Fatal(err)
	}

	catalog, err := manager.catalog(context.Background(), project)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, provider := range catalog.All {
		if provider.ID != "openrouter" {
			continue
		}
		model, ok := provider.Models[jevRouterModelID]
		if !ok {
			t.Fatalf("configured JEV metadata disappeared from backend catalog: %#v", provider.Models)
		}
		if model.Enabled == nil || *model.Enabled {
			t.Fatalf("disabled JEV must be marked non-selectable in catalog: %#v", model)
		}
		found = true
	}
	if !found {
		t.Fatalf("configured JEV provider missing from catalog: %#v", catalog.All)
	}
}

func TestProviderRegistryAndCatalogSurviveRuntimeCompatibilityFailure(t *testing.T) {
	project := t.TempDir()
	stateDir := t.TempDir()
	t.Setenv("TL_STUDIO_STATE_DIR", stateDir)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/config/overlay":
			_, _ = io.WriteString(w, `{"effective":{"provider":{},"disabled_providers":[]}}`)
		case r.Method == http.MethodPatch && r.URL.Path == "/config/overlay":
			http.Error(w, "runtime rejected provider overlay", http.StatusBadRequest)
		case r.Method == http.MethodGet && r.URL.Path == "/provider":
			http.Error(w, "runtime provider catalog unavailable", http.StatusBadRequest)
		case r.Method == http.MethodPost && r.URL.Path == "/global/dispose":
			http.Error(w, "runtime reload unavailable", http.StatusBadRequest)
		default:
			http.Error(w, "runtime compatibility unavailable", http.StatusBadRequest)
		}
	}))
	defer backend.Close()

	state := &appState{project: project}
	manager, err := newRuntimeProviderManager(state, backend.URL, "runtime", "secret")
	if err != nil {
		t.Fatal(err)
	}
	manager.credentials = newMemoryProviderCredentialStore()

	existing := testProviderDefinition()
	if err := manager.store.put(existing); err != nil {
		t.Fatal(err)
	}
	if err := manager.credentials.Put(existing.ID, "existing-secret"); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	registerRuntimeProviderRoutes(mux, manager)
	server := httptest.NewServer(mux)
	defer server.Close()

	configRes, err := http.Get(server.URL + "/runtime/providers/config")
	if err != nil {
		t.Fatal(err)
	}
	defer configRes.Body.Close()
	if configRes.StatusCode != http.StatusOK {
		t.Fatalf("provider registry must remain readable when runtime sync fails: status=%d", configRes.StatusCode)
	}
	var config providerConfigResponse
	if err := json.NewDecoder(configRes.Body).Decode(&config); err != nil {
		t.Fatal(err)
	}
	if len(config.Providers) != 1 || config.Providers[0].ID != existing.ID {
		t.Fatalf("saved providers disappeared after runtime compatibility failure: %#v", config.Providers)
	}

	catalogRes, err := http.Get(server.URL + "/runtime/providers/catalog?directory=" + url.QueryEscape(project))
	if err != nil {
		t.Fatal(err)
	}
	defer catalogRes.Body.Close()
	if catalogRes.StatusCode != http.StatusOK {
		t.Fatalf("catalog fallback must survive runtime failure: status=%d", catalogRes.StatusCode)
	}
	var catalog providerCatalogResponse
	if err := json.NewDecoder(catalogRes.Body).Decode(&catalog); err != nil {
		t.Fatal(err)
	}
	foundHosted := false
	foundExisting := false
	for _, provider := range catalog.All {
		switch provider.ID {
		case runtimeHostedProviderID:
			foundHosted = true
			if len(provider.Models) != 1 {
				t.Fatalf("fallback Kilo catalog should expose only Auto Free: %#v", provider.Models)
			}
			if model, ok := provider.Models["kilo-auto/free"]; !ok || model.Name != "Auto Free" {
				t.Fatalf("fallback Kilo Auto Free missing: %#v", provider.Models)
			}
		case existing.ID:
			foundExisting = true
			if len(provider.Models) != 1 {
				t.Fatalf("managed provider must retain only saved models: %#v", provider.Models)
			}
		}
	}
	if !foundHosted || !foundExisting {
		t.Fatalf("catalog fallback missing hosted/custom providers: %#v", catalog.All)
	}
	connected := false
	for _, id := range catalog.Connected {
		if id == existing.ID {
			connected = true
		}
	}
	if !connected {
		t.Fatalf("TL Studio vault credential should mark saved provider connected: %#v", catalog.Connected)
	}

	next := tlProviderDefinition{
		ID: "second-provider", Name: "Second Provider", Protocol: "openai-compatible",
		BaseURL: "https://api.second.example/v1",
		Models: []tlProviderModel{{ID: "second-model", Name: "Second Model", ToolCall: true}},
	}
	body, _ := json.Marshal(providerWriteRequest{Provider: next, APIKey: "second-secret"})
	req, _ := http.NewRequest(http.MethodPut, server.URL+"/runtime/providers/config/"+next.ID, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	saveRes, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer saveRes.Body.Close()
	if saveRes.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(saveRes.Body)
		t.Fatalf("TL Studio provider save must not fail because runtime compatibility rejected it: status=%d body=%s", saveRes.StatusCode, data)
	}
	if stored, ok, err := manager.store.get(next.ID); err != nil || !ok || stored.ID != next.ID {
		t.Fatalf("new provider was not persisted after runtime compatibility failure: stored=%#v ok=%v err=%v", stored, ok, err)
	}
	if key, err := manager.credentials.Get(next.ID); err != nil || key != "second-secret" {
		t.Fatalf("new provider credential was not persisted in TL Studio vault: key=%q err=%v", key, err)
	}
}

func TestProviderManagerRestoresTLStudioCredentialIntoFreshRuntime(t *testing.T) {
	project := t.TempDir()
	stateDir := t.TempDir()
	t.Setenv("TL_STUDIO_STATE_DIR", stateDir)

	registry := newProviderRegistryStore(filepath.Join(stateDir, "providers.json"))
	compatibilityProvider := testProviderDefinition()
	compatibilityProvider.Models[0].ToolCall = false
	if err := registry.put(compatibilityProvider); err != nil {
		t.Fatal(err)
	}
	credentials := newMemoryProviderCredentialStore()
	if err := credentials.Put("example-provider", "restored-secret"); err != nil {
		t.Fatal(err)
	}

	credentialWrites := 0
	disposeCalls := 0
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/config/overlay":
			_, _ = io.WriteString(w, `{"effective":{"provider":{},"disabled_providers":[]}}`)
		case r.Method == http.MethodPatch && r.URL.Path == "/config/overlay":
			_, _ = io.WriteString(w, `{"ok":true}`)
		case r.Method == http.MethodPut && r.URL.Path == "/auth/example-provider":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["key"] != "restored-secret" {
				t.Fatalf("unexpected restored credential: %#v", body)
			}
			credentialWrites++
			_, _ = io.WriteString(w, `{"ok":true}`)
		case r.Method == http.MethodPost && r.URL.Path == "/global/dispose":
			disposeCalls++
			_, _ = io.WriteString(w, `{"ok":true}`)
		default:
			t.Fatalf("unexpected runtime request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer backend.Close()

	state := &appState{project: project}
	manager, err := newRuntimeProviderManager(state, backend.URL, "runtime", "secret")
	if err != nil {
		t.Fatal(err)
	}
	manager.store = registry
	manager.credentials = credentials

	if err := manager.ensureBootstrapped(context.Background()); err != nil {
		t.Fatal(err)
	}
	if credentialWrites != 1 {
		t.Fatalf("expected one credential restore into runtime, got %d", credentialWrites)
	}
	if disposeCalls < 2 {
		t.Fatalf("expected provider sync and post-credential runtime reload, got %d dispose calls", disposeCalls)
	}
}

func urlQueryEscape(value string) string {
	request := httptest.NewRequest(http.MethodGet, "/?directory="+value, nil)
	return request.URL.Query().Get("directory")
}
