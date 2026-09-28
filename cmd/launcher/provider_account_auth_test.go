package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

type fakeProviderAccountAdapter struct {
	status       providerAccountStatus
	challenge    providerAccountLoginChallenge
	disconnected bool
	cancelled    bool
	refreshed    bool
	models       []string
	mu           sync.Mutex
}

func (a *fakeProviderAccountAdapter) ID() string { return a.status.ID }
func (a *fakeProviderAccountAdapter) Status(context.Context, string) (providerAccountStatus, error) {
	return a.status, nil
}
func (a *fakeProviderAccountAdapter) BeginLogin(context.Context, string) (providerAccountLoginChallenge, error) {
	return a.challenge, nil
}
func (a *fakeProviderAccountAdapter) CompleteLogin(context.Context, string, string) (providerAccountStatus, error) {
	status := a.status
	status.Connected = true
	status.State = providerAccountConnected
	return status, nil
}
func (a *fakeProviderAccountAdapter) HandleCallback(context.Context, string, string, url.Values) error {
	return nil
}
func (a *fakeProviderAccountAdapter) CancelLogin(context.Context, string, string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cancelled = true
	return nil
}
func (a *fakeProviderAccountAdapter) Refresh(context.Context, string) (providerAccountStatus, error) {
	a.mu.Lock()
	a.refreshed = true
	a.mu.Unlock()
	return a.status, nil
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
			Available: true, AuthModes: []string{"account"},
			Models: []string{"example/model"},
		},
		challenge: providerAccountLoginChallenge{
			LoginID: "login-1", Flow: "device_code",
			VerificationURL: "https://example.test/sign-in",
			UserCode: "TEST-CODE",
			Instructions: "Use code TEST-CODE",
		},
		models: []string{"example/model"},
	}
	service := newProviderAccountService(adapter)
	mux := http.NewServeMux()
	registerProviderAccountRoutes(mux, service)
	server := httptest.NewServer(mux)
	defer server.Close()

	res, err := http.Get(server.URL + "/local/provider-accounts?directory=/project")
	if err != nil { t.Fatal(err) }
	var list []providerAccountStatus
	decodeProviderAccountJSON(t, res, &list)
	if len(list) != 1 || list[0].ID != "example" || list[0].AuthModes[0] != "account" {
		t.Fatalf("unexpected provider account list %#v", list)
	}

	res, err = http.Post(server.URL+"/local/provider-accounts/example/authorize", "application/json", strings.NewReader("{}"))
	if err != nil { t.Fatal(err) }
	var auth map[string]any
	decodeProviderAccountJSON(t, res, &auth)
	if auth["loginId"] != "login-1" || auth["verificationUrl"] != "https://example.test/sign-in" {
		t.Fatalf("unexpected authorize response %#v", auth)
	}

	res, err = http.Post(server.URL+"/local/provider-accounts/example/complete?login=login-1", "application/json", strings.NewReader("{}"))
	if err != nil { t.Fatal(err) }
	var completed providerAccountStatus
	decodeProviderAccountJSON(t, res, &completed)
	if !completed.Connected || completed.State != providerAccountConnected {
		t.Fatalf("unexpected completed status %#v", completed)
	}

	res, err = http.Get(server.URL + "/local/provider-accounts/example/models")
	if err != nil { t.Fatal(err) }
	var discovered struct{ Models []string `json:"models"` }
	decodeProviderAccountJSON(t, res, &discovered)
	if len(discovered.Models) != 1 || discovered.Models[0] != "example/model" {
		t.Fatalf("unexpected model list %#v", discovered.Models)
	}

	req, _ := http.NewRequest(http.MethodDelete, server.URL+"/local/provider-accounts/example", nil)
	res, err = http.DefaultClient.Do(req)
	if err != nil { t.Fatal(err) }
	res.Body.Close()
	if res.StatusCode != http.StatusOK { t.Fatalf("disconnect status=%d", res.StatusCode) }
}

func TestProviderAccountServiceCanBeEmpty(t *testing.T) {
	service := newProviderAccountService()
	items, err := service.list(context.Background(), t.TempDir())
	if err != nil { t.Fatal(err) }
	if len(items) != 0 { t.Fatalf("expected no account adapters, got %#v", items) }

	mux := http.NewServeMux()
	registerProviderAccountRoutes(mux, service)
	server := httptest.NewServer(mux)
	defer server.Close()
	res, err := http.Get(server.URL + "/local/provider-accounts")
	if err != nil { t.Fatal(err) }
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK { t.Fatalf("list status=%d", res.StatusCode) }
	var got []providerAccountStatus
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil { t.Fatal(err) }
	if len(got) != 0 { t.Fatalf("expected empty account list, got %#v", got) }
}
