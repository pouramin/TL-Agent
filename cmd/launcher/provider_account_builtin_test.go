package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestProviderManagerRuntimeCredentialSupportsAccountTokens(t *testing.T) {
	t.Setenv("TL_STUDIO_STATE_DIR", t.TempDir())
	manager := newProviderManager(&appState{})
	account := providerAccountStoredCredential{
		AccessToken: "account-access-secret",
		RefreshToken: "account-refresh-secret",
		ExpiresAt: time.Now().UTC().Add(time.Hour).Format(time.RFC3339),
	}
	encoded, err := encodeProviderAccountCredential(account)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.credentials.Put(providerAccountCredentialID("huggingface"), encoded); err != nil {
		t.Fatal(err)
	}

	got, err := manager.runtimeCredential("huggingface")
	if err != nil {
		t.Fatal(err)
	}
	if got != account.AccessToken {
		t.Fatalf("unexpected runtime credential %q", got)
	}

	registryData, err := os.ReadFile(providerRegistryPath())
	if err == nil && (strings.Contains(string(registryData), account.AccessToken) || strings.Contains(string(registryData), account.RefreshToken)) {
		t.Fatal("provider registry leaked account credentials")
	}
}

func TestProviderManagerRuntimeCredentialPrefersExplicitAPIKey(t *testing.T) {
	t.Setenv("TL_STUDIO_STATE_DIR", t.TempDir())
	manager := newProviderManager(&appState{})
	if err := manager.credentials.Put("huggingface", "explicit-api-key"); err != nil {
		t.Fatal(err)
	}
	account, _ := encodeProviderAccountCredential(providerAccountStoredCredential{
		AccessToken: "account-token",
		ExpiresAt: time.Now().UTC().Add(time.Hour).Format(time.RFC3339),
	})
	if err := manager.credentials.Put(providerAccountCredentialID("huggingface"), account); err != nil {
		t.Fatal(err)
	}

	got, err := manager.runtimeCredential("huggingface")
	if err != nil {
		t.Fatal(err)
	}
	if got != "explicit-api-key" {
		t.Fatalf("explicit API key must retain precedence, got %q", got)
	}
}



func TestProviderAccountCredentialPersistsAcrossManagerRestart(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("TL_STUDIO_STATE_DIR", stateDir)

	state := &appState{frontendURL: "http://127.0.0.1:32126"}
	first := newProviderManager(state)
	credential := providerAccountStoredCredential{
		AccessToken:  "restart-access",
		RefreshToken: "restart-refresh",
		ExpiresAt:    time.Now().UTC().Add(time.Hour).Format(time.RFC3339),
		AccountLabel: "restart@example.test",
	}
	encoded, err := encodeProviderAccountCredential(credential)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.credentials.Put(providerAccountCredentialID("huggingface"), encoded); err != nil {
		t.Fatal(err)
	}
	if err := first.store.put(tlProviderDefinition{
		ID:       "huggingface",
		Name:     "Hugging Face",
		Protocol: "openai-compatible",
		BaseURL:  "https://router.huggingface.co/v1",
		ManagedBy: "account",
		Models: []tlProviderModel{{ID: "test/model", Name: "Test Model", ToolCall: true}},
	}); err != nil {
		t.Fatal(err)
	}

	second := newProviderManager(state)
	adapter := newHuggingFaceAccountAdapter(state, second)
	adapter.clientID = "public-test-client"
	status, err := adapter.Status(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if !status.Connected || status.AccountLabel != "restart@example.test" {
		t.Fatalf("account credential did not survive restart: %#v", status)
	}
	provider, found, err := second.store.get("huggingface")
	if err != nil || !found || len(provider.Models) != 1 || provider.Models[0].ID != "test/model" {
		t.Fatalf("account-managed provider did not survive restart: found=%v provider=%#v err=%v", found, provider, err)
	}
}

func TestHuggingFaceAccountLifecycleUsesPKCEVaultAndModelDiscovery(t *testing.T) {
	t.Setenv("TL_STUDIO_STATE_DIR", t.TempDir())

	var mu sync.Mutex
	tokenRequests := 0
	var tokenBodies []string
	modelAuthorization := ""
	var tokenServer *httptest.Server
	tokenServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/token":
			body, _ := io.ReadAll(r.Body)
			mu.Lock()
			tokenRequests++
			tokenBodies = append(tokenBodies, string(body))
			requestNumber := tokenRequests
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			if strings.Contains(string(body), "grant_type=refresh_token") {
				_, _ = w.Write([]byte(`{"access_token":"hf-access-refreshed","refresh_token":"hf-refresh-2","token_type":"Bearer","scope":"openid profile email inference-api","expires_in":3600}`))
				return
			}
			if requestNumber != 1 {
				t.Errorf("unexpected authorization token exchange count %d", requestNumber)
			}
			_, _ = w.Write([]byte(`{"access_token":"hf-access-secret","refresh_token":"hf-refresh-secret","token_type":"Bearer","scope":"openid profile email inference-api","expires_in":3600}`))
		case "/oauth/userinfo":
			if got := r.Header.Get("Authorization"); got != "Bearer hf-access-secret" {
				t.Errorf("unexpected userinfo authorization %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"preferred_username":"amin-test","email":"amin@example.test","isPro":true}`))
		case "/v1/models":
			modelAuthorization = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"test/model-a","name":"Model A"},{"id":"test/model-b","name":"Model B"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer tokenServer.Close()

	state := &appState{frontendURL: "http://127.0.0.1:32123"}
	manager := newProviderManager(state)
	adapter := newHuggingFaceAccountAdapter(state, manager)
	adapter.clientID = "public-test-client"
	adapter.tokenURL = tokenServer.URL + "/oauth/token"
	adapter.userInfoURL = tokenServer.URL + "/oauth/userinfo"
	adapter.inferenceBaseURL = tokenServer.URL + "/v1"

	challenge, err := adapter.BeginLogin(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if challenge.Flow != "authorization_code_pkce" || challenge.LoginID == "" {
		t.Fatalf("unexpected challenge %#v", challenge)
	}
	authorize, err := url.Parse(challenge.AuthorizationURL)
	if err != nil {
		t.Fatal(err)
	}
	query := authorize.Query()
	if query.Get("state") != challenge.LoginID {
		t.Fatal("authorization state does not match the login transaction")
	}
	if query.Get("code_challenge_method") != "S256" || query.Get("code_challenge") == "" {
		t.Fatal("Hugging Face login must use PKCE S256")
	}
	if !strings.Contains(query.Get("scope"), "inference-api") {
		t.Fatal("Hugging Face login must request inference-api")
	}
	if strings.Contains(challenge.AuthorizationURL, "secret") {
		t.Fatal("authorization URL must not expose a client secret")
	}

	badValues := url.Values{"state": {"wrong-state"}, "code": {"bad"}}
	if err := adapter.HandleCallback(context.Background(), "", challenge.LoginID, badValues); err == nil {
		t.Fatal("invalid OAuth state must be rejected")
	}

	challenge, err = adapter.BeginLogin(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	values := url.Values{"state": {challenge.LoginID}, "code": {"authorization-code"}}
	if err := adapter.HandleCallback(context.Background(), "", challenge.LoginID, values); err != nil {
		t.Fatal(err)
	}
	status, err := adapter.CompleteLogin(context.Background(), "", challenge.LoginID)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Connected || status.AccountLabel != "amin@example.test" || status.AccountType != "PRO" {
		t.Fatalf("unexpected connected account status %#v", status)
	}
	if modelAuthorization != "Bearer hf-access-secret" {
		t.Fatalf("model discovery did not use account token: %q", modelAuthorization)
	}

	rawCredential, err := manager.credentials.Get(providerAccountCredentialID("huggingface"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rawCredential, "hf-access-secret") || !strings.Contains(rawCredential, "hf-refresh-secret") {
		t.Fatal("account credential was not persisted in the credential vault")
	}
	if _, err := manager.credentials.Get("huggingface"); !errors.Is(err, errCredentialNotFound) {
		t.Fatalf("account login must not overwrite the API-key credential slot: %v", err)
	}

	registry, _, err := manager.store.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(registry) != 1 || registry[0].ID != "huggingface" || registry[0].ManagedBy != "account" {
		t.Fatalf("unexpected account-managed provider registry %#v", registry)
	}
	encodedRegistry, _ := json.Marshal(registry)
	if strings.Contains(string(encodedRegistry), "hf-access-secret") || strings.Contains(string(encodedRegistry), "hf-refresh-secret") {
		t.Fatal("provider registry leaked OAuth credentials")
	}

	models, err := adapter.DiscoverModels(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 {
		t.Fatalf("unexpected discovered models %#v", models)
	}

	credential, err := adapter.loadCredential()
	if err != nil {
		t.Fatal(err)
	}
	credential.ExpiresAt = time.Now().UTC().Add(-time.Minute).Format(time.RFC3339)
	if err := adapter.saveCredential(credential); err != nil {
		t.Fatal(err)
	}
	refreshed, err := adapter.Refresh(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if !refreshed.Connected {
		t.Fatalf("expected refreshed account to remain connected: %#v", refreshed)
	}
	mu.Lock()
	bodies := append([]string(nil), tokenBodies...)
	mu.Unlock()
	if len(bodies) < 2 || !strings.Contains(bodies[len(bodies)-1], "grant_type=refresh_token") {
		t.Fatalf("refresh grant was not used: %#v", bodies)
	}

	if err := adapter.Disconnect(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.credentials.Get(providerAccountCredentialID("huggingface")); !errors.Is(err, errCredentialNotFound) {
		t.Fatalf("account credential remained after sign out: %v", err)
	}
	if _, found, err := manager.store.get("huggingface"); err != nil || found {
		t.Fatalf("account-managed provider remained after sign out: found=%v err=%v", found, err)
	}
}
