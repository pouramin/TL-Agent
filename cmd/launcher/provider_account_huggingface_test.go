package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

func newHuggingFaceAdapterFixture(t *testing.T) (*huggingFaceAccountAdapter, *providerManager, *httptest.Server, *sync.Mutex, *[]string) {
	t.Helper()
	t.Setenv("TL_STUDIO_STATE_DIR", t.TempDir())

	var mu sync.Mutex
	tokenBodies := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/token":
			body, _ := io.ReadAll(r.Body)
			mu.Lock()
			tokenBodies = append(tokenBodies, string(body))
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			if strings.Contains(string(body), "grant_type=refresh_token") {
				_, _ = w.Write([]byte(`{"access_token":"hf-access-refreshed","refresh_token":"hf-refresh-2","token_type":"Bearer","scope":"openid profile email inference-api","expires_in":3600}`))
				return
			}
			_, _ = w.Write([]byte(`{"access_token":"hf-access-secret","refresh_token":"hf-refresh-secret","token_type":"Bearer","scope":"openid profile email inference-api","expires_in":3600}`))
		case "/oauth/userinfo":
			if got := r.Header.Get("Authorization"); got != "Bearer hf-access-secret" {
				t.Fatalf("unexpected userinfo authorization %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"preferred_username":"amin-test","email":"amin@example.test","isPro":true}`))
		case "/v1/models":
			if got := r.Header.Get("Authorization"); got != "Bearer hf-access-secret" && got != "Bearer hf-access-refreshed" {
				t.Fatalf("unexpected model authorization %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"test/model-a","name":"Model A","providers":[{"provider":"test","status":"live","context_length":131072,"supports_tools":true}]},{"id":"test/model-b","name":"Model B","providers":[{"provider":"test","status":"live","context_length":65536,"supports_tools":false}]}]}`))
		default:
			http.NotFound(w, r)
		}
	}))

	state := &appState{frontendURL: "http://127.0.0.1:32123"}
	manager := newProviderManager(state)
	adapter := newHuggingFaceAccountAdapter(state, manager)
	adapter.clientID = "public-test-client"
	adapter.authorizeURL = server.URL + "/oauth/authorize"
	adapter.tokenURL = server.URL + "/oauth/token"
	adapter.userInfoURL = server.URL + "/oauth/userinfo"
	adapter.inferenceBaseURL = server.URL + "/v1"
	adapter.client = server.Client()
	manager.registerAccountAdapter(adapter)
	return adapter, manager, server, &mu, &tokenBodies
}

func TestHuggingFaceAccountLifecycleUsesPKCEVaultRefreshAndDiscovery(t *testing.T) {
	adapter, manager, server, mu, tokenBodies := newHuggingFaceAdapterFixture(t)
	defer server.Close()

	login, err := adapter.BeginLogin(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if login.Flow != "authorization_code_pkce" || login.LoginID == "" {
		t.Fatalf("unexpected login challenge %#v", login)
	}
	authorize, err := url.Parse(login.AuthorizationURL)
	if err != nil {
		t.Fatal(err)
	}
	query := authorize.Query()
	state := query.Get("state")
	if state == "" || state == login.LoginID {
		t.Fatal("OAuth state must be present and independent from the browser-visible login id")
	}
	if query.Get("client_id") != "public-test-client" {
		t.Fatalf("unexpected OAuth client id %q", query.Get("client_id"))
	}
	if query.Get("code_challenge_method") != "S256" || query.Get("code_challenge") == "" {
		t.Fatal("Hugging Face login must use PKCE S256")
	}
	if !strings.Contains(query.Get("scope"), "inference-api") {
		t.Fatal("Hugging Face login must request inference-api")
	}
	if strings.Contains(login.AuthorizationURL, "client_secret") {
		t.Fatal("authorization URL must not contain a client secret")
	}

	if err := adapter.CompleteLogin(context.Background(), "", providerAccountCallback{Code: "authorization-code", State: "wrong-state"}); err == nil {
		t.Fatal("invalid OAuth state must be rejected")
	}
	if err := adapter.CompleteLogin(context.Background(), "", providerAccountCallback{Code: "authorization-code", State: state}); err != nil {
		t.Fatal(err)
	}

	status, err := adapter.PollLogin(context.Background(), "", login.LoginID)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Connected || status.State != providerAccountConnected {
		t.Fatalf("expected connected Hugging Face account, got %#v", status)
	}
	if status.AccountLabel != "amin@example.test" || status.AccountType != "PRO" {
		t.Fatalf("unexpected Hugging Face account metadata %#v", status)
	}
	if len(status.Models) != 2 {
		t.Fatalf("expected discovered Hugging Face models, got %#v", status.Models)
	}

	raw, err := getProviderCredentialSlot(manager.credentials, huggingFaceAccountProviderID, providerCredentialSlotAccount)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(raw, providerOAuthCredentialPrefix) || !strings.Contains(raw, "hf-access-secret") || !strings.Contains(raw, "hf-refresh-secret") {
		t.Fatal("structured OAuth credential was not stored in the account vault slot")
	}
	if _, err := getProviderCredentialSlot(manager.credentials, huggingFaceAccountProviderID, providerCredentialSlotAPI); !errors.Is(err, errCredentialNotFound) {
		t.Fatalf("account login must not overwrite the manual API credential slot: %v", err)
	}

	provider, found, err := manager.store.get(huggingFaceAccountProviderID)
	if err != nil || !found {
		t.Fatalf("Hugging Face provider was not persisted: found=%v err=%v", found, err)
	}
	if provider.ManagedBy != "account" || provider.Protocol != "openai-compatible" {
		t.Fatalf("unexpected account-managed provider %#v", provider)
	}
	if len(provider.Models) != 2 || !provider.Models[0].ToolCall && !provider.Models[1].ToolCall {
		t.Fatalf("nested supports_tools was not discovered: %#v", provider.Models)
	}
	foundToolModel := false
	for _, model := range provider.Models {
		if model.ID == "test/model-a" {
			foundToolModel = model.ToolCall && model.ContextLimit == 131072
		}
	}
	if !foundToolModel {
		t.Fatalf("Hugging Face nested provider capability metadata was not preserved: %#v", provider.Models)
	}

	credential, err := adapter.loadCredential()
	if err != nil {
		t.Fatal(err)
	}
	credential.ExpiresAt = time.Now().UTC().Add(-time.Minute).Format(time.RFC3339)
	if err := adapter.saveCredential(credential); err != nil {
		t.Fatal(err)
	}
	resolved, err := adapter.ResolveCredential(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if resolved != "hf-access-refreshed" {
		t.Fatalf("expected refreshed runtime credential, got %q", resolved)
	}
	mu.Lock()
	bodies := append([]string(nil), (*tokenBodies)...)
	mu.Unlock()
	if len(bodies) < 2 || !strings.Contains(bodies[len(bodies)-1], "grant_type=refresh_token") {
		t.Fatalf("refresh grant was not used: %#v", bodies)
	}

	if err := adapter.Disconnect(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	if _, err := getProviderCredentialSlot(manager.credentials, huggingFaceAccountProviderID, providerCredentialSlotAccount); !errors.Is(err, errCredentialNotFound) {
		t.Fatalf("account credential remained after sign out: %v", err)
	}
	if _, found, err := manager.store.get(huggingFaceAccountProviderID); err != nil || found {
		t.Fatalf("account-managed provider remained after sign out: found=%v err=%v", found, err)
	}
}

func TestHuggingFaceAccountCredentialSurvivesRestartAndDoesNotLeak(t *testing.T) {
	t.Setenv("TL_STUDIO_STATE_DIR", t.TempDir())
	state := &appState{frontendURL: "http://127.0.0.1:32125"}
	first := newProviderManager(state)
	firstAdapter := newHuggingFaceAccountAdapter(state, first)

	encoded, err := encodeProviderOAuthCredential(providerOAuthCredential{
		AccessToken: "leak-check-access",
		RefreshToken: "leak-check-refresh",
		ExpiresAt: time.Now().UTC().Add(time.Hour).Format(time.RFC3339),
		AccountLabel: "user@example.test",
		AccountType: "PRO",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := putProviderCredentialSlot(first.credentials, huggingFaceAccountProviderID, providerCredentialSlotAccount, encoded); err != nil {
		t.Fatal(err)
	}
	if err := first.store.put(tlProviderDefinition{
		ID: huggingFaceAccountProviderID,
		Name: "Hugging Face",
		Protocol: "openai-compatible",
		BaseURL: huggingFaceInferenceBaseURL,
		ManagedBy: "account",
		Models: []tlProviderModel{{ID: "test/model", Name: "Test Model", ToolCall: true}},
	}); err != nil {
		t.Fatal(err)
	}

	second := newProviderManager(state)
	secondAdapter := newHuggingFaceAccountAdapter(state, second)
	second.registerAccountAdapter(secondAdapter)
	status, err := secondAdapter.Status(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if !status.Connected || status.AccountLabel != "user@example.test" {
		t.Fatalf("account credential did not survive restart: %#v", status)
	}

	service := newProviderAccountService(secondAdapter)
	mux := http.NewServeMux()
	registerProviderAccountRoutes(mux, service)
	local := httptest.NewServer(mux)
	defer local.Close()

	for _, endpoint := range []string{"/local/provider-accounts", "/local/provider-accounts/huggingface"} {
		response, err := http.Get(local.URL + endpoint)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(body), "leak-check-access") || strings.Contains(string(body), "leak-check-refresh") {
			t.Fatalf("%s leaked stored OAuth tokens: %s", endpoint, body)
		}
	}

	registry, _, err := second.store.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	serialized, _ := json.Marshal(registry)
	if strings.Contains(string(serialized), "leak-check-access") || strings.Contains(string(serialized), "leak-check-refresh") {
		t.Fatal("providers.json contract leaked OAuth tokens")
	}

	resolved, err := second.effectiveCredential(context.Background(), huggingFaceAccountProviderID, "")
	if err != nil || resolved != "leak-check-access" {
		t.Fatalf("runtime credential resolution failed after restart: value=%q err=%v", resolved, err)
	}

	_ = firstAdapter
}

func TestHuggingFaceAccountTakesPrecedenceWithoutDeletingManualAPIKey(t *testing.T) {
	t.Setenv("TL_STUDIO_STATE_DIR", t.TempDir())
	state := &appState{frontendURL: "http://127.0.0.1:32127"}
	manager := newProviderManager(state)
	adapter := newHuggingFaceAccountAdapter(state, manager)
	manager.registerAccountAdapter(adapter)

	if err := putProviderCredentialSlot(manager.credentials, huggingFaceAccountProviderID, providerCredentialSlotAPI, "manual-api-key"); err != nil {
		t.Fatal(err)
	}
	encoded, err := encodeProviderOAuthCredential(providerOAuthCredential{
		AccessToken: "account-access-token",
		ExpiresAt: time.Now().UTC().Add(time.Hour).Format(time.RFC3339),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := putProviderCredentialSlot(manager.credentials, huggingFaceAccountProviderID, providerCredentialSlotAccount, encoded); err != nil {
		t.Fatal(err)
	}

	got, err := manager.effectiveCredential(context.Background(), huggingFaceAccountProviderID, "")
	if err != nil || got != "account-access-token" {
		t.Fatalf("connected account must take precedence, got %q err=%v", got, err)
	}
	if err := adapter.Disconnect(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	got, err = manager.effectiveCredential(context.Background(), huggingFaceAccountProviderID, "")
	if err != nil || got != "manual-api-key" {
		t.Fatalf("manual API key must survive account sign-out, got %q err=%v", got, err)
	}
}
