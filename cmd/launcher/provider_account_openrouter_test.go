package main

import (\n\t"errors"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func newOpenRouterAdapterTestFixture(t *testing.T) (*openRouterAccountAdapter, *providerManager, *appState, *httptest.Server) {
	t.Helper()
	t.Setenv("TL_STUDIO_STATE_DIR", t.TempDir())

	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/keys":
			if r.Method != http.MethodPost {
				t.Fatalf("unexpected method %s", r.Method)
			}
			var input map[string]string
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				t.Fatal(err)
			}
			if input["code"] != "oauth-code" {
				t.Fatalf("unexpected authorization code %q", input["code"])
			}
			if strings.TrimSpace(input["code_verifier"]) == "" {
				t.Fatal("PKCE verifier was not sent")
			}
			if input["code_challenge_method"] != "S256" {
				t.Fatalf("unexpected PKCE method %q", input["code_challenge_method"])
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"key":"account-secret"}`))
		case "/models":
			if got := r.Header.Get("Authorization"); got != "Bearer account-secret" {
				t.Fatalf("model discovery credential leaked or missing: %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"example/model","name":"Example Model","supported_parameters":["tools"]}]}`))
		default:
			http.NotFound(w, r)
		}
	}))

	state := &appState{frontendURL: "http://127.0.0.1:43123", project: t.TempDir()}
	manager := newProviderManager(state)
	manager.credentials = privateFileCredentialStore{}
	adapter := newOpenRouterAccountAdapter(state, manager)
	adapter.authBaseURL = providerServer.URL
	adapter.apiBaseURL = providerServer.URL
	adapter.client = providerServer.Client()
	return adapter, manager, state, providerServer
}

func TestOpenRouterAccountLoginPersistsCredentialAndModels(t *testing.T) {
	adapter, manager, _, providerServer := newOpenRouterAdapterTestFixture(t)
	defer providerServer.Close()

	login, err := adapter.BeginLogin(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if login.Flow != "authorization_code_pkce" || login.LoginID == "" {
		t.Fatalf("unexpected login challenge %#v", login)
	}
	authorizeURL, err := url.Parse(login.AuthorizationURL)
	if err != nil {
		t.Fatal(err)
	}
	state := authorizeURL.Query().Get("state")
	if state == "" || authorizeURL.Query().Get("code_challenge") == "" {
		t.Fatalf("OAuth state or PKCE challenge missing from %s", login.AuthorizationURL)
	}
	if authorizeURL.Query().Get("code_challenge_method") != "S256" {
		t.Fatalf("unexpected PKCE method in %s", login.AuthorizationURL)
	}
	if callback := authorizeURL.Query().Get("callback_url"); callback != "http://127.0.0.1:43123/local/provider-accounts/openrouter/oauth/callback" {
		t.Fatalf("unexpected callback %q", callback)
	}

	if err := adapter.CompleteLogin(context.Background(), "", providerAccountCallback{Code: "oauth-code", State: state}); err != nil {
		t.Fatal(err)
	}

	status, err := adapter.PollLogin(context.Background(), "", login.LoginID)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Connected || status.State != providerAccountConnected {
		t.Fatalf("unexpected connected status %#v", status)
	}
	if len(status.Models) != 1 || status.Models[0] != "example/model" {
		t.Fatalf("unexpected models %#v", status.Models)
	}

	secret, err := getProviderCredentialSlot(manager.credentials, openRouterAccountProviderID, providerCredentialSlotAccount)
	if err != nil {
		t.Fatal(err)
	}
	if secret != "account-secret" {
		t.Fatalf("unexpected stored account credential %q", secret)
	}
	provider, ok, err := manager.store.get(openRouterAccountProviderID)
	if err != nil || !ok {
		t.Fatalf("provider not persisted: ok=%v err=%v", ok, err)
	}
	if provider.Protocol != "openai-compatible" || len(provider.Models) != 1 {
		t.Fatalf("unexpected persisted provider %#v", provider)
	}

	encoded, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "account-secret") || strings.Contains(string(encoded), "oauth-code") {
		t.Fatalf("secret leaked into browser-visible account status: %s", encoded)
	}
}

func TestOpenRouterAccountRejectsInvalidState(t *testing.T) {
	adapter, _, _, providerServer := newOpenRouterAdapterTestFixture(t)
	defer providerServer.Close()

	if _, err := adapter.BeginLogin(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	if err := adapter.CompleteLogin(context.Background(), "", providerAccountCallback{Code: "oauth-code", State: "wrong-state"}); err == nil {
		t.Fatal("expected invalid OAuth state to fail")
	}
}

func TestOpenRouterAccountLogoutKeepsManualAPIKey(t *testing.T) {
	adapter, manager, _, providerServer := newOpenRouterAdapterTestFixture(t)
	defer providerServer.Close()

	if err := putProviderCredentialSlot(manager.credentials, openRouterAccountProviderID, providerCredentialSlotAPI, "manual-secret"); err != nil {
		t.Fatal(err)
	}
	if err := putProviderCredentialSlot(manager.credentials, openRouterAccountProviderID, providerCredentialSlotAccount, "account-secret"); err != nil {
		t.Fatal(err)
	}
	if err := adapter.Disconnect(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	if _, err := getProviderCredentialSlot(manager.credentials, openRouterAccountProviderID, providerCredentialSlotAccount); !errors.Is(err, errCredentialNotFound) {
		t.Fatalf("expected account credential deletion, got %v", err)
	}
	manual, err := getProviderCredentialSlot(manager.credentials, openRouterAccountProviderID, providerCredentialSlotAPI)
	if err != nil || manual != "manual-secret" {
		t.Fatalf("manual API credential must survive logout, value=%q err=%v", manual, err)
	}
}

func TestOpenRouterAccountStatusSurvivesAdapterRestart(t *testing.T) {
	adapter, manager, state, providerServer := newOpenRouterAdapterTestFixture(t)
	defer providerServer.Close()

	if err := putProviderCredentialSlot(manager.credentials, openRouterAccountProviderID, providerCredentialSlotAccount, "account-secret"); err != nil {
		t.Fatal(err)
	}
	restarted := newOpenRouterAccountAdapter(state, manager)
	status, err := restarted.Status(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if !status.Connected || status.State != providerAccountConnected {
		t.Fatalf("expected persisted account after restart, got %#v", status)
	}
}
