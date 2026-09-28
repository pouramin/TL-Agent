package main

import (
	"encoding/json"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeProviderAccountAdapter struct {
	status       providerAccountStatus
	login        providerAccountLogin
	pollStatus   providerAccountStatus
	models       []string
	cancelled    bool
	refreshed    bool
	disconnected bool
	mu           sync.Mutex
}

func (a *fakeProviderAccountAdapter) ID() string { return a.status.ID }
func (a *fakeProviderAccountAdapter) Status(context.Context, string) (providerAccountStatus, error) {
	return a.status, nil
}
func (a *fakeProviderAccountAdapter) BeginLogin(context.Context, string) (providerAccountLogin, error) {
	return a.login, nil
}
func (a *fakeProviderAccountAdapter) CompleteLogin(context.Context, string, providerAccountCallback) error {
	return nil
}
func (a *fakeProviderAccountAdapter) PollLogin(context.Context, string, string) (providerAccountStatus, error) {
	return a.pollStatus, nil
}
func (a *fakeProviderAccountAdapter) CancelLogin(context.Context, string, string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cancelled = true
	return nil
}
func (a *fakeProviderAccountAdapter) Refresh(context.Context, string) (providerAccountStatus, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.refreshed = true
	return a.pollStatus, nil
}
func (a *fakeProviderAccountAdapter) DiscoverModels(context.Context, string) ([]string, error) {
	return append([]string(nil), a.models...), nil
}
func (a *fakeProviderAccountAdapter) Disconnect(context.Context, string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.disconnected = true
	return nil
}

func decodeProviderAccountJSON(t *testing.T, res *http.Response, target any) {
	t.Helper()
	defer res.Body.Close()
	if err := json.NewDecoder(res.Body).Decode(target); err != nil {
		t.Fatal(err)
	}
}

func TestProviderAccountRoutesAreProviderNeutral(t *testing.T) {
	adapter := &fakeProviderAccountAdapter{
		status: providerAccountStatus{
			ID: "example", Name: "Example Account",
			Description: "Example account-backed provider.",
			Available: true, AuthModes: []string{"authorization_code_pkce"},
		},
		login: providerAccountLogin{
			LoginID: "login-1", Flow: "authorization_code_pkce",
			AuthorizationURL: "https://example.test/sign-in",
			ExpiresAt: time.Now().Add(5 * time.Minute).UTC().Format(time.RFC3339),
		},
		pollStatus: providerAccountStatus{
			ID: "example", Name: "Example Account", Available: true,
			State: providerAccountConnected, AccountLabel: "user@example.test",
		},
		models: []string{"example/model"},
	}
	service := newProviderAccountService(adapter)
	mux := http.NewServeMux()
	registerProviderAccountRoutes(mux, service)
	server := httptest.NewServer(mux)
	defer server.Close()

	res, err := http.Get(server.URL + "/local/provider-accounts?directory=/project")
	if err != nil {
		t.Fatal(err)
	}
	var list []providerAccountStatus
	decodeProviderAccountJSON(t, res, &list)
	if len(list) != 1 || list[0].ID != "example" || list[0].State != providerAccountDisconnected {
		t.Fatalf("unexpected provider account list %#v", list)
	}

	res, err = http.Post(server.URL+"/local/provider-accounts/example/login", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	var login providerAccountLogin
	decodeProviderAccountJSON(t, res, &login)
	if login.LoginID != "login-1" || login.AuthorizationURL != "https://example.test/sign-in" {
		t.Fatalf("unexpected login response %#v", login)
	}

	res, err = http.Get(server.URL + "/local/provider-accounts/example/login/login-1")
	if err != nil {
		t.Fatal(err)
	}
	var connected providerAccountStatus
	decodeProviderAccountJSON(t, res, &connected)
	if connected.State != providerAccountConnected || !connected.Connected || connected.AccountLabel != "user@example.test" {
		t.Fatalf("unexpected connected status %#v", connected)
	}

	res, err = http.Get(server.URL + "/local/provider-accounts/example/models")
	if err != nil {
		t.Fatal(err)
	}
	var modelPayload struct {
		Models []string `json:"models"`
	}
	decodeProviderAccountJSON(t, res, &modelPayload)
	if len(modelPayload.Models) != 1 || modelPayload.Models[0] != "example/model" {
		t.Fatalf("unexpected models %#v", modelPayload.Models)
	}

	req, _ := http.NewRequest(http.MethodDelete, server.URL+"/local/provider-accounts/example", nil)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK || !adapter.disconnected {
		t.Fatalf("disconnect status=%d disconnected=%v", res.StatusCode, adapter.disconnected)
	}
}

func TestProviderAccountLoginRejectsExpiredChallenge(t *testing.T) {
	adapter := &fakeProviderAccountAdapter{
		status: providerAccountStatus{ID: "example", Name: "Example", Available: true},
		login: providerAccountLogin{
			LoginID: "expired", Flow: "device_code",
			ExpiresAt: time.Now().Add(-time.Minute).UTC().Format(time.RFC3339),
		},
	}
	mux := http.NewServeMux()
	registerProviderAccountRoutes(mux, newProviderAccountService(adapter))
	server := httptest.NewServer(mux)
	defer server.Close()

	res, err := http.Post(server.URL+"/local/provider-accounts/example/login", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected expired login to fail, got %d", res.StatusCode)
	}
}

func TestProviderAccountServiceCanBeEmpty(t *testing.T) {
	service := newProviderAccountService()
	items, err := service.list(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("expected no account adapters, got %#v", items)
	}

	mux := http.NewServeMux()
	registerProviderAccountRoutes(mux, service)
	server := httptest.NewServer(mux)
	defer server.Close()
	res, err := http.Get(server.URL + "/local/provider-accounts")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("list status=%d", res.StatusCode)
	}
	var got []providerAccountStatus
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty account list, got %#v", got)
	}
}


func TestProviderAccountBrowserContractHasNoSecretFields(t *testing.T) {
	statusJSON, err := json.Marshal(providerAccountStatus{
		ID: "example", Name: "Example", Available: true,
		State: providerAccountConnected, Connected: true,
		AccountLabel: "user@example.test",
	})
	if err != nil {
		t.Fatal(err)
	}
	loginJSON, err := json.Marshal(providerAccountLogin{
		LoginID: "login-id",
		Flow: "authorization_code_pkce",
		AuthorizationURL: "https://example.test/authorize",
		UserCode: "SAFE-CODE",
	})
	if err != nil {
		t.Fatal(err)
	}
	combined := strings.ToLower(string(statusJSON) + string(loginJSON))
	for _, forbidden := range []string{
		"access_token", "accesstoken",
		"refresh_token", "refreshtoken",
		"client_secret", "clientsecret",
		"code_verifier", "codeverifier",
		"authorization_code", "authorizationcode",
	} {
		if strings.Contains(combined, forbidden) {
			t.Fatalf("provider account browser contract exposes forbidden secret field %q: %s", forbidden, combined)
		}
	}
}
