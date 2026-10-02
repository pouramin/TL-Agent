package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeClaudeWebTransport struct {
	availableErr error
	probeErr     error
	probe        claudeWebProbe
	completion   string
	opened       bool
	closed       bool
	paired       bool
}

func (f *fakeClaudeWebTransport) Available() error {
	return f.availableErr
}

func (f *fakeClaudeWebTransport) OpenLogin(context.Context) error {
	if f.availableErr != nil {
		return f.availableErr
	}
	f.opened = true
	f.paired = true
	return nil
}

func (f *fakeClaudeWebTransport) Probe(context.Context) (claudeWebProbe, error) {
	if f.availableErr != nil {
		return claudeWebProbe{}, f.availableErr
	}
	if f.probeErr != nil {
		return claudeWebProbe{}, f.probeErr
	}
	return f.probe, nil
}

func (f *fakeClaudeWebTransport) Complete(context.Context, string) (string, error) {
	if f.completion == "" {
		return "", errors.New("fake Claude Web completion is empty")
	}
	return f.completion, nil
}

func (f *fakeClaudeWebTransport) Close(context.Context) error {
	f.closed = true
	return nil
}

func (f *fakeClaudeWebTransport) PairingToken() string { return "test-pair-token-abcdefghijklmnopqrstuvwxyz" }
func (f *fakeClaudeWebTransport) PairingOrigin() string { return "http://127.0.0.1:32123" }
func (f *fakeClaudeWebTransport) Paired() bool { return f.paired }

func newClaudeWebTestManager(t *testing.T, transport *fakeClaudeWebTransport) (*providerManager, *claudeWebAccountAdapter) {
	t.Helper()
	t.Setenv("TL_STUDIO_STATE_DIR", t.TempDir())
	manager := newProviderManager(&appState{})
	adapter := newClaudeWebAccountAdapterWithTransport(&appState{}, manager, transport)
	manager.registerAccountAdapter(adapter)
	return manager, adapter
}

func TestClaudeWebBrowserLoginSyncsIndependentRuntimeProvider(t *testing.T) {
	transport := &fakeClaudeWebTransport{}
	manager, adapter := newClaudeWebTestManager(t, transport)

	status, err := adapter.Status(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if !status.Available || status.Connected {
		t.Fatalf("unexpected initial Claude Web status: %#v", status)
	}

	login, err := adapter.BeginLogin(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if login.Flow != "claude_web_extension" || login.LoginID == "" || !transport.opened {
		t.Fatalf("unexpected Claude Web login challenge: %#v opened=%v", login, transport.opened)
	}

	transport.probe = claudeWebProbe{
		Connected:        true,
		Status:           200,
		OrganizationID:   "org_test",
		OrganizationName: "Personal",
	}
	status, err = adapter.PollLogin(context.Background(), "", login.LoginID)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Connected || status.AccountType != "Claude Web" || status.OrganizationID != "org_test" {
		t.Fatalf("unexpected connected Claude Web status: %#v", status)
	}
	if len(status.Models) != 1 || status.Models[0] != claudeWebModelID {
		t.Fatalf("unexpected Claude Web models: %#v", status.Models)
	}

	provider, found, err := manager.store.get(claudeWebRuntimeProviderID)
	if err != nil || !found {
		t.Fatalf("Claude Web runtime provider missing: found=%v err=%v", found, err)
	}
	if provider.Protocol != claudeWebProviderProtocol || provider.ManagedBy != "account" {
		t.Fatalf("unexpected Claude Web runtime provider: %#v", provider)
	}
	credential, err := manager.effectiveCredential(context.Background(), claudeWebRuntimeProviderID, "")
	if err != nil || credential != claudeWebRuntimeCredentialSentinel {
		t.Fatalf("unexpected Claude Web runtime credential: %q err=%v", credential, err)
	}
	if got := manager.accountAdapter(claudeWebRuntimeProviderID); got != adapter {
		t.Fatalf("Claude Web runtime provider alias did not resolve account adapter: %#v", got)
	}
}

func TestClaudeWebBridgeKeepsTLStudioToolLoop(t *testing.T) {
	transport := &fakeClaudeWebTransport{
		probe: claudeWebProbe{Connected: true, Status: 200, OrganizationID: "org_test"},
		completion: `{"text":"","toolCalls":[{"name":"files.read","arguments":"{\"path\":\"README.md\"}"}]}`,
	}
	_, adapter := newClaudeWebTestManager(t, transport)
	config := claudeWebConfig{Connected: true, OrganizationID: "org_test"}
	if err := saveClaudeWebConfig(config); err != nil {
		t.Fatal(err)
	}

	response, err := adapter.CompleteModelTurn(context.Background(), nativeModelRequest{
		Provider: tlProviderDefinition{
			ID:        claudeWebRuntimeProviderID,
			Name:      "Claude Web / Account",
			Protocol:  claudeWebProviderProtocol,
			BaseURL:   claudeWebRuntimeBaseURL,
			ManagedBy: "account",
		},
		Model:  tlProviderModel{ID: claudeWebModelID, Name: "Claude Sonnet 5.5 (Web)", ToolCall: true},
		APIKey: claudeWebRuntimeCredentialSentinel,
		Tools: []nativeModelToolDefinition{{
			ID:          "files.read",
			Name:        "Read file",
			Description: "Read a project file",
			InputSchema: map[string]any{"type": "object"},
		}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(response.ToolCalls) != 1 || response.ToolCalls[0].Name != "files.read" {
		t.Fatalf("unexpected Claude Web tool response: %#v", response)
	}
	if !strings.Contains(string(response.ToolCalls[0].Arguments), "README.md") {
		t.Fatalf("unexpected Claude Web tool arguments: %s", response.ToolCalls[0].Arguments)
	}
}

func TestClaudeWebBridgeAcceptsFencedJSONAndPlainTextFallback(t *testing.T) {
	output := parseClaudeWebBridgeOutput("~~~not-used~~~")
	if output.Text != "~~~not-used~~~" || len(output.ToolCalls) != 0 {
		t.Fatalf("plain text fallback changed: %#v", output)
	}
	fenced := parseClaudeWebBridgeOutput("```json\n{\"text\":\"hello\",\"toolCalls\":[]}\n```")
	if fenced.Text != "hello" || len(fenced.ToolCalls) != 0 {
		t.Fatalf("fenced JSON parsing failed: %#v", fenced)
	}
}

func TestClaudeWebDisconnectPreservesManualAPIAndUnpairsBridge(t *testing.T) {
	transport := &fakeClaudeWebTransport{probe: claudeWebProbe{Connected: true}}
	manager, adapter := newClaudeWebTestManager(t, transport)

	manualProvider := tlProviderDefinition{
		ID:       claudeAccountProviderID,
		Name:     "Claude",
		Protocol: "anthropic-messages",
		BaseURL:  "https://api.anthropic.com/v1",
		Models:   []tlProviderModel{{ID: "manual", Name: "Manual Claude", ToolCall: true}},
	}
	if err := manager.store.put(manualProvider); err != nil { t.Fatal(err) }
	if err := putProviderCredentialSlot(manager.credentials, claudeAccountProviderID, providerCredentialSlotAPI, "manual-secret"); err != nil { t.Fatal(err) }
	if err := saveClaudeWebConfig(claudeWebConfig{Connected: true, OrganizationID: "org_test"}); err != nil { t.Fatal(err) }
	if _, err := adapter.syncProvider(claudeWebConfig{Connected: true, OrganizationID: "org_test"}); err != nil { t.Fatal(err) }

	if err := adapter.Disconnect(context.Background(), ""); err != nil { t.Fatal(err) }
	if !transport.closed { t.Fatal("Claude Web bridge was not closed") }
	if _, found, err := manager.store.get(claudeWebRuntimeProviderID); err != nil || found {
		t.Fatalf("Claude Web runtime provider remained after disconnect: found=%v err=%v", found, err)
	}
	provider, found, err := manager.store.get(claudeAccountProviderID)
	if err != nil || !found || provider.Protocol != "anthropic-messages" {
		t.Fatalf("manual Claude API provider was changed: found=%v err=%v provider=%#v", found, err, provider)
	}
	secret, err := getProviderCredentialSlot(manager.credentials, claudeAccountProviderID, providerCredentialSlotAPI)
	if err != nil || secret != "manual-secret" {
		t.Fatalf("manual Claude API credential was changed: %q err=%v", secret, err)
	}
}

func TestClaudeWebUnavailableWithoutBrowserTransport(t *testing.T) {
	transport := &fakeClaudeWebTransport{availableErr: errors.New("no browser")}
	_, adapter := newClaudeWebTestManager(t, transport)
	status, err := adapter.Status(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if status.Available || !strings.Contains(status.Error, "no browser") {
		t.Fatalf("unexpected unavailable Claude Web status: %#v", status)
	}
}


func TestClaudeWebLoginRouteNeverFallsThroughToClaudeCode(t *testing.T) {
	t.Setenv("TL_STUDIO_STATE_DIR", t.TempDir())
	manager := newProviderManager(&appState{})
	transport := &fakeClaudeWebTransport{}
	webAdapter := newClaudeWebAccountAdapterWithTransport(&appState{}, manager, transport)
	codeAdapter := newClaudeAccountAdapter(&appState{}, manager)
	service := newProviderAccountService(codeAdapter, webAdapter)

	mux := http.NewServeMux()
	registerProviderAccountRoutes(mux, service)

	request := httptest.NewRequest(http.MethodPost, "/local/provider-accounts/claude-web/login", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("Claude Web login route returned HTTP %d: %s", response.Code, response.Body.String())
	}
	if !transport.opened {
		t.Fatal("Claude Web login route did not open the normal Chrome bridge")
	}
	if adapter, ok := service.adapter("claude-web"); !ok || adapter != webAdapter {
		t.Fatalf("claude-web route resolved the wrong adapter: %#v ok=%v", adapter, ok)
	}
	if adapter, ok := service.adapter("claude"); !ok || adapter != codeAdapter {
		t.Fatalf("claude route no longer resolves the Claude Code adapter: %#v ok=%v", adapter, ok)
	}
}


func TestClaudeWebPersistedConnectionRequiresLiveBridgePairing(t *testing.T) {
	transport := &fakeClaudeWebTransport{paired: false}
	manager, adapter := newClaudeWebTestManager(t, transport)
	if err := saveClaudeWebConfig(claudeWebConfig{
		Connected: true,
		OrganizationID: "org_test",
		OrganizationName: "Personal",
	}); err != nil {
		t.Fatal(err)
	}

	status, err := adapter.Status(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if status.Connected || status.State != providerAccountNeedsReauthentication {
		t.Fatalf("stale Claude Web connection must require bridge reconnect: %#v", status)
	}
	if status.OrganizationID != "org_test" || status.AccountLabel != "Personal" {
		t.Fatalf("persisted account identity should survive bridge restart: %#v", status)
	}
	if _, found, err := manager.store.get(claudeWebRuntimeProviderID); err != nil || found {
		t.Fatalf("stale Claude Web runtime provider must not remain selectable: found=%v err=%v", found, err)
	}
}

func TestClaudeWebModelIdentityMatchesRequestedWebModel(t *testing.T) {
	models := claudeWebModels()
	if len(models) != 1 {
		t.Fatalf("unexpected Claude Web model count: %#v", models)
	}
	if models[0].ID != "claude-sonnet-5-5" || models[0].Name != "Claude Sonnet 5.5 (Web)" {
		t.Fatalf("Claude Web model identity is misleading: %#v", models[0])
	}
}
