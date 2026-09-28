package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type fakeProviderAccountAdapter struct {
	status        providerAccountStatus
	authorizeBody json.RawMessage
	callbackBody  json.RawMessage
	disconnected  bool
	mu            sync.Mutex
}

func (a *fakeProviderAccountAdapter) ID() string { return a.status.ID }
func (a *fakeProviderAccountAdapter) Status(context.Context, string) (providerAccountStatus, error) {
	return a.status, nil
}
func (a *fakeProviderAccountAdapter) Authorize(context.Context, string) (json.RawMessage, error) {
	return a.authorizeBody, nil
}
func (a *fakeProviderAccountAdapter) Callback(context.Context, string) (json.RawMessage, error) {
	return a.callbackBody, nil
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
			ID:          "example",
			Name:        "Example Account",
			Description: "Example account-backed provider.",
			Available:   true,
			Connected:   false,
			AuthModes:   []string{"account"},
			Models:      []string{"example/model"},
		},
		authorizeBody: json.RawMessage(`{"url":"https://example.test/sign-in","instructions":"Use code TEST-CODE"}`),
		callbackBody:  json.RawMessage(`{"ok":true}`),
	}
	service := &providerAccountService{adapters: map[string]providerAccountAdapter{}}
	service.add(adapter)
	mux := http.NewServeMux()
	registerProviderAccountRoutes(mux, service)
	server := httptest.NewServer(mux)
	defer server.Close()

	res, err := http.Get(server.URL + "/local/provider-accounts?directory=/project")
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("list status=%d", res.StatusCode)
	}
	var list []providerAccountStatus
	decodeProviderAccountJSON(t, res, &list)
	if len(list) != 1 || list[0].ID != "example" || list[0].AuthModes[0] != "account" {
		t.Fatalf("unexpected provider account list %#v", list)
	}

	res, err = http.Get(server.URL + "/local/provider-accounts/example")
	if err != nil {
		t.Fatal(err)
	}
	var status providerAccountStatus
	decodeProviderAccountJSON(t, res, &status)
	if status.Name != "Example Account" || !status.Available {
		t.Fatalf("unexpected provider account status %#v", status)
	}

	res, err = http.Post(server.URL+"/local/provider-accounts/example/authorize", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	var auth map[string]any
	decodeProviderAccountJSON(t, res, &auth)
	if auth["url"] != "https://example.test/sign-in" {
		t.Fatalf("unexpected authorize response %#v", auth)
	}

	res, err = http.Post(server.URL+"/local/provider-accounts/example/callback", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	var callback map[string]any
	decodeProviderAccountJSON(t, res, &callback)
	if callback["ok"] != true {
		t.Fatalf("unexpected callback response %#v", callback)
	}

	req, _ := http.NewRequest(http.MethodDelete, server.URL+"/local/provider-accounts/example", nil)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var disconnected map[string]any
	decodeProviderAccountJSON(t, res, &disconnected)
	if disconnected["disconnected"] != true {
		t.Fatalf("unexpected disconnect response %#v", disconnected)
	}
	adapter.mu.Lock()
	wasDisconnected := adapter.disconnected
	adapter.mu.Unlock()
	if !wasDisconnected {
		t.Fatal("provider account adapter disconnect was not invoked")
	}
}

func TestKiloProviderAccountIsUnavailableWithoutCompatibilityRuntime(t *testing.T) {
	state := &appState{project: t.TempDir()}
	manager, err := newRuntimeProviderManager(state, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	adapter := &kiloProviderAccountAdapter{manager: manager}

	status, err := adapter.Status(context.Background(), state.project)
	if err != nil {
		t.Fatal(err)
	}
	if status.ID != "kilo" || status.Name != "Kilo" || status.Available || status.Connected {
		t.Fatalf("unexpected native-only Kilo account status %#v", status)
	}
	if !status.RequiresCompatibility || len(status.AuthModes) != 1 || status.AuthModes[0] != "account" {
		t.Fatalf("unexpected Kilo account metadata %#v", status)
	}
	if len(status.Models) != 0 {
		t.Fatalf("native-only Kilo account must not advertise models %#v", status.Models)
	}

	service := newProviderAccountService(manager)
	mux := http.NewServeMux()
	registerProviderAccountRoutes(mux, service)
	server := httptest.NewServer(mux)
	defer server.Close()

	res, err := http.Post(server.URL+"/local/provider-accounts/kilo/authorize", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusServiceUnavailable {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("expected unavailable Kilo account adapter, status=%d body=%s", res.StatusCode, body)
	}
}

func TestKiloProviderAccountUsesCompatibilityAdapterWhenAvailable(t *testing.T) {
	project := t.TempDir()
	var statusCalls, authorizeCalls, callbackCalls, disconnectCalls int
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "runtime" || pass != "secret" {
			t.Fatalf("unexpected compatibility auth %q %q %v", user, pass, ok)
		}
		if r.Method != http.MethodDelete && r.Header.Get("x-kilo-directory") == "" {
			t.Fatal("provider account request was not scoped to the selected project")
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/kilo/auth-status":
			statusCalls++
			_, _ = io.WriteString(w, `{"authenticated":true,"type":"oauth","organizationId":"org-1"}`)
		case r.Method == http.MethodPost && r.URL.Path == "/provider/kilo/oauth/authorize":
			authorizeCalls++
			_, _ = io.WriteString(w, `{"url":"https://app.kilo.ai/device","instructions":"Use code KILO-1234"}`)
		case r.Method == http.MethodPost && r.URL.Path == "/provider/kilo/oauth/callback":
			callbackCalls++
			_, _ = io.WriteString(w, `{"ok":true}`)
		case r.Method == http.MethodDelete && r.URL.Path == "/auth/kilo":
			disconnectCalls++
			_, _ = io.WriteString(w, `{"ok":true}`)
		default:
			t.Fatalf("unexpected compatibility request %s %s", r.Method, r.URL.String())
		}
	}))
	defer backend.Close()

	state := &appState{project: project, backendURL: backend.URL}
	manager, err := newRuntimeProviderManager(state, backend.URL, "runtime", "secret")
	if err != nil {
		t.Fatal(err)
	}
	service := newProviderAccountService(manager)
	adapter, ok := service.adapter("kilo")
	if !ok {
		t.Fatal("Kilo provider account adapter was not registered")
	}

	status, err := adapter.Status(context.Background(), project)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Available || !status.Connected || status.AccountType != "oauth" || status.OrganizationID != "org-1" {
		t.Fatalf("unexpected connected Kilo account status %#v", status)
	}
	if len(status.Models) != 1 || status.Models[0] != "kilo-auto/free" {
		t.Fatalf("unexpected Kilo preferred models %#v", status.Models)
	}

	if _, err := adapter.Authorize(context.Background(), project); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Callback(context.Background(), project); err != nil {
		t.Fatal(err)
	}
	if err := adapter.Disconnect(context.Background(), project); err != nil {
		t.Fatal(err)
	}
	if statusCalls != 1 || authorizeCalls != 1 || callbackCalls != 1 || disconnectCalls != 1 {
		t.Fatalf("unexpected Kilo account adapter calls status=%d authorize=%d callback=%d disconnect=%d",
			statusCalls, authorizeCalls, callbackCalls, disconnectCalls)
	}
}
