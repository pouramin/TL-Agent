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
	"testing"
	"time"
)

func newGeminiAccountFixture(t *testing.T) (*googleGeminiAccountAdapter, *providerManager, *httptest.Server, *[]string, *bool) {
	t.Helper()
	t.Setenv("TL_STUDIO_STATE_DIR", t.TempDir())

	tokenGrantTypes := []string{}
	revokeSeen := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			body, _ := io.ReadAll(r.Body)
			values, _ := url.ParseQuery(string(body))
			tokenGrantTypes = append(tokenGrantTypes, values.Get("grant_type"))
			if values.Get("client_secret") != "" {
				t.Fatal("desktop OAuth token exchange must not embed a client secret")
			}
			w.Header().Set("Content-Type", "application/json")
			if values.Get("grant_type") == "refresh_token" {
				_, _ = w.Write([]byte(`{"access_token":"access-b","refresh_token":"refresh-b","token_type":"Bearer","scope":"scope-a","expires_in":3600}`))
				return
			}
			if strings.TrimSpace(values.Get("code_verifier")) == "" {
				t.Fatal("authorization-code exchange did not send the PKCE verifier")
			}
			_, _ = w.Write([]byte(`{"access_token":"access-a","refresh_token":"refresh-a","token_type":"Bearer","scope":"scope-a","expires_in":3600}`))
		case "/userinfo":
			if got := r.Header.Get("Authorization"); got != "Bearer access-a" {
				t.Fatalf("unexpected userinfo authorization %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"email":"amin@example.test","name":"Amin Test"}`))
		case "/models":
			if got := r.Header.Get("Authorization"); got != "Bearer access-a" && got != "Bearer access-b" {
				t.Fatalf("unexpected model authorization %q", got)
			}
			if got := r.Header.Get("x-goog-user-project"); got != "google-test-project" {
				t.Fatalf("unexpected quota project %q", got)
			}
			if got := r.Header.Get("x-goog-api-client"); !strings.HasPrefix(got, "tl-studio/") {
				t.Fatalf("missing TL Studio client identification: %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"models":[{"name":"models/gemini-test","displayName":"Gemini Test","inputTokenLimit":100000,"outputTokenLimit":8192,"supportedGenerationMethods":["generateContent"]},{"name":"models/embed-test","displayName":"Embed Test","supportedGenerationMethods":["embedContent"]}]}`))
		case "/revoke":
			revokeSeen = true
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))

	state := &appState{frontendURL: "http://127.0.0.1:32124"}
	manager := newProviderManager(state)
	adapter := newGoogleGeminiAccountAdapter(state, manager)
	adapter.clientID = "google-test-client"
	adapter.projectID = "google-test-project"
	adapter.authorizeURL = server.URL + "/authorize"
	adapter.tokenURL = server.URL + "/token"
	adapter.userInfoURL = server.URL + "/userinfo"
	adapter.modelsURL = server.URL + "/models"
	adapter.revokeURL = server.URL + "/revoke"
	adapter.baseURL = server.URL + "/v1beta"
	adapter.client = server.Client()
	manager.registerAccountAdapter(adapter)
	return adapter, manager, server, &tokenGrantTypes, &revokeSeen
}

func TestGoogleGeminiAccountLifecycleUsesPKCERefreshQuotaAndRevocation(t *testing.T) {
	adapter, manager, server, tokenGrantTypes, revokeSeen := newGeminiAccountFixture(t)
	defer server.Close()

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
	query := authorizeURL.Query()
	state := query.Get("state")
	if state == "" || state == login.LoginID {
		t.Fatal("OAuth state must be independent from the browser-visible login id")
	}
	if query.Get("client_id") != "google-test-client" {
		t.Fatalf("unexpected Google client id %q", query.Get("client_id"))
	}
	if query.Get("code_challenge_method") != "S256" || query.Get("code_challenge") == "" {
		t.Fatal("Google login must use PKCE S256")
	}
	if query.Get("access_type") != "offline" || query.Get("prompt") != "consent" {
		t.Fatal("Google login must request offline access and consent for a refresh token")
	}
	if !strings.Contains(query.Get("scope"), "generative-language.retriever") {
		t.Fatal("Google login must request the documented Gemini OAuth scope")
	}
	if strings.Contains(login.AuthorizationURL, "client_secret") {
		t.Fatal("authorization URL must not expose a client secret")
	}

	if err := adapter.CompleteLogin(context.Background(), "", providerAccountCallback{State: "wrong-state", Code: "bad"}); err == nil {
		t.Fatal("invalid OAuth state must be rejected")
	}
	if err := adapter.CompleteLogin(context.Background(), "", providerAccountCallback{State: state, Code: "authorization-code"}); err != nil {
		t.Fatal(err)
	}

	status, err := adapter.PollLogin(context.Background(), "", login.LoginID)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Connected || status.AccountLabel != "amin@example.test" || status.AccountType != "Google account" {
		t.Fatalf("unexpected Gemini account status %#v", status)
	}
	if len(status.Models) != 1 || status.Models[0] != "gemini-test" {
		t.Fatalf("unexpected Gemini model catalog %#v", status.Models)
	}

	provider, found, err := manager.store.get(googleGeminiAccountProviderID)
	if err != nil || !found {
		t.Fatalf("Gemini provider not persisted: found=%v err=%v", found, err)
	}
	if provider.Protocol != "gemini-generate-content" || provider.ProjectID != "google-test-project" || provider.ManagedBy != "account" {
		t.Fatalf("unexpected Gemini provider %#v", provider)
	}
	if len(provider.Models) != 1 || provider.Models[0].ID != "gemini-test" {
		t.Fatalf("unexpected discovered Gemini models %#v", provider.Models)
	}

	raw, err := getProviderCredentialSlot(manager.credentials, googleGeminiAccountProviderID, providerCredentialSlotAccount)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(raw, providerOAuthCredentialPrefix) || !strings.Contains(raw, "access-a") || !strings.Contains(raw, "refresh-a") {
		t.Fatal("Gemini OAuth credential was not stored in the account vault slot")
	}
	registryJSON, _ := json.Marshal(provider)
	for _, secret := range []string{"access-a", "refresh-a"} {
		if strings.Contains(string(registryJSON), secret) {
			t.Fatalf("provider registry leaked Gemini credential %q", secret)
		}
	}

	credential, err := adapter.loadCredential()
	if err != nil {
		t.Fatal(err)
	}
	credential.ExpiresAt = time.Now().UTC().Add(-time.Minute).Format(time.RFC3339)
	if err := adapter.saveCredential(credential); err != nil {
		t.Fatal(err)
	}
	token, err := adapter.ResolveCredential(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if token != "access-b" {
		t.Fatalf("runtime credential did not refresh: %q", token)
	}
	if len(*tokenGrantTypes) < 2 || (*tokenGrantTypes)[len(*tokenGrantTypes)-1] != "refresh_token" {
		t.Fatalf("refresh grant was not used: %#v", *tokenGrantTypes)
	}

	if err := adapter.Disconnect(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	if !*revokeSeen {
		t.Fatal("Google token revocation endpoint was not called")
	}
	if _, err := getProviderCredentialSlot(manager.credentials, googleGeminiAccountProviderID, providerCredentialSlotAccount); !errors.Is(err, errCredentialNotFound) {
		t.Fatalf("Gemini account credential remained after disconnect: %v", err)
	}
	if _, found, err := manager.store.get(googleGeminiAccountProviderID); err != nil || found {
		t.Fatalf("Gemini account-managed provider remained after disconnect: found=%v err=%v", found, err)
	}
}

func TestGoogleGeminiStatusExplainsMissingProductConfiguration(t *testing.T) {
	t.Setenv("TL_STUDIO_STATE_DIR", t.TempDir())
	t.Setenv("TL_STUDIO_GOOGLE_CLIENT_ID", "")
	t.Setenv("TL_STUDIO_GOOGLE_PROJECT_ID", "")
	adapter := newGoogleGeminiAccountAdapter(&appState{frontendURL: "http://127.0.0.1:32124"}, newProviderManager(&appState{}))
	status, err := adapter.Status(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if status.Available {
		t.Fatal("Gemini account login must not pretend to be available without a registered TL Studio Google OAuth client")
	}
	if !strings.Contains(status.Description, "OAuth client ID") {
		t.Fatalf("missing clear Google setup reason: %#v", status)
	}
}

func TestGoogleGeminiSetupCanBeConfiguredAndPersistsAcrossRestart(t *testing.T) {
	t.Setenv("TL_STUDIO_STATE_DIR", t.TempDir())
	t.Setenv("TL_STUDIO_GOOGLE_CLIENT_ID", "")
	t.Setenv("TL_STUDIO_GOOGLE_PROJECT_ID", "")

	state := &appState{frontendURL: "http://127.0.0.1:32124"}
	manager := newProviderManager(state)
	adapter := newGoogleGeminiAccountAdapter(state, manager)

	before, err := adapter.Status(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if before.Available || before.Setup == nil || !before.Setup.Configurable || before.Setup.Configured {
		t.Fatalf("unexpected unconfigured Gemini setup status %#v", before)
	}

	configured, err := adapter.Configure(context.Background(), "", map[string]string{
		"clientId":  "1234567890-tl-studio.apps.googleusercontent.com",
		"projectId": "tl-studio-test-project",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !configured.Available || configured.Setup == nil || !configured.Setup.Configured {
		t.Fatalf("Gemini setup did not become available: %#v", configured)
	}

	saved, err := loadGoogleGeminiSetupConfig()
	if err != nil {
		t.Fatal(err)
	}
	if saved.ClientID != "1234567890-tl-studio.apps.googleusercontent.com" || saved.ProjectID != "tl-studio-test-project" {
		t.Fatalf("unexpected persisted Gemini setup %#v", saved)
	}

	restarted := newGoogleGeminiAccountAdapter(state, newProviderManager(state))
	clientID, projectID := restarted.setupValues()
	if clientID != saved.ClientID || projectID != saved.ProjectID {
		t.Fatalf("Gemini setup did not survive restart: client=%q project=%q", clientID, projectID)
	}

	setup, err := restarted.Setup(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(setup.Fields) != 2 || setup.Fields[0].Value != saved.ProjectID || setup.Fields[1].Value != saved.ClientID {
		t.Fatalf("unexpected Gemini setup fields %#v", setup.Fields)
	}
}

func TestGoogleGeminiSetupRoutesUseGenericAccountContract(t *testing.T) {
	t.Setenv("TL_STUDIO_STATE_DIR", t.TempDir())
	t.Setenv("TL_STUDIO_GOOGLE_CLIENT_ID", "")
	t.Setenv("TL_STUDIO_GOOGLE_PROJECT_ID", "")

	state := &appState{frontendURL: "http://127.0.0.1:32124"}
	manager := newProviderManager(state)
	adapter := newGoogleGeminiAccountAdapter(state, manager)
	service := newProviderAccountService(adapter)
	mux := http.NewServeMux()
	registerProviderAccountRoutes(mux, service)
	server := httptest.NewServer(mux)
	defer server.Close()

	response, err := http.Get(server.URL + "/local/provider-accounts/gemini/setup")
	if err != nil {
		t.Fatal(err)
	}
	var setup providerAccountSetup
	if err := json.NewDecoder(response.Body).Decode(&setup); err != nil {
		response.Body.Close()
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK || len(setup.Fields) != 2 {
		t.Fatalf("unexpected setup response status=%d setup=%#v", response.StatusCode, setup)
	}

	body := strings.NewReader(`{"values":{"clientId":"1234567890-alpha.apps.googleusercontent.com","projectId":"alpha-project"}}`)
	request, err := http.NewRequest(http.MethodPut, server.URL+"/local/provider-accounts/gemini/setup", body)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	var status providerAccountStatus
	if err := json.NewDecoder(response.Body).Decode(&status); err != nil {
		response.Body.Close()
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK || !status.Available || status.Setup == nil || !status.Setup.Configured {
		t.Fatalf("unexpected configured setup response status=%d status=%#v", response.StatusCode, status)
	}
}

func TestGoogleGeminiProjectIDPersistsInProviderRegistry(t *testing.T) {
	t.Setenv("TL_STUDIO_STATE_DIR", t.TempDir())
	state := &appState{frontendURL: "http://127.0.0.1:32124"}
	manager := newProviderManager(state)
	if err := manager.store.put(tlProviderDefinition{
		ID: googleGeminiAccountProviderID,
		Name: "Google / Gemini",
		Protocol: "gemini-generate-content",
		BaseURL: googleGeminiBaseURL,
		ManagedBy: "account",
		ProjectID: "persisted-project",
		Models: []tlProviderModel{{ID: "gemini-test", Name: "Gemini Test", ToolCall: true}},
	}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TL_STUDIO_GOOGLE_PROJECT_ID", "")
	adapter := newGoogleGeminiAccountAdapter(state, manager)
	if adapter.projectID != "persisted-project" {
		t.Fatalf("Gemini project id did not survive restart: %q", adapter.projectID)
	}
}

func TestNativeGeminiToolContinuationPreservesThoughtSignature(t *testing.T) {
	requestCount := 0
	sawQuotaProject := false
	sawThoughtSignature := false
	sawFunctionResponse := false
	sawClientID := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.Header.Get("Authorization") != "Bearer runtime-token" {
			t.Fatalf("unexpected model authorization %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("x-goog-user-project") == "quota-project" {
			sawQuotaProject = true
		}
		if strings.HasPrefix(r.Header.Get("x-goog-api-client"), "tl-studio/") {
			sawClientID = true
		}
		body, _ := io.ReadAll(r.Body)
		if requestCount == 2 {
			if strings.Contains(string(body), `"thoughtSignature":"signature-a"`) {
				sawThoughtSignature = true
			}
			if strings.Contains(string(body), `"functionResponse"`) && strings.Contains(string(body), `"id":"call-a"`) {
				sawFunctionResponse = true
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if requestCount == 1 {
			_, _ = w.Write([]byte(`{"candidates":[{"content":{"role":"model","parts":[{"functionCall":{"id":"call-a","name":"tl_workspace__read","args":{"path":"README.md"}},"thoughtSignature":"signature-a"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":11,"candidatesTokenCount":5,"thoughtsTokenCount":2},"modelVersion":"gemini-test"}`))
			return
		}
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"role":"model","parts":[{"text":"Done."}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":20,"candidatesTokenCount":3},"modelVersion":"gemini-test"}`))
	}))
	defer server.Close()

	client := &nativeHTTPModelClient{httpClient: server.Client()}
	provider := tlProviderDefinition{
		ID: googleGeminiAccountProviderID,
		Name: "Google / Gemini",
		Protocol: "gemini-generate-content",
		BaseURL: server.URL + "/v1beta",
		ProjectID: "quota-project",
	}
	model := tlProviderModel{ID: "gemini-test", Name: "Gemini Test", ToolCall: true}
	tools := []nativeModelToolDefinition{{
		ID: "workspace.read", Name: "Read file", Description: "Read a workspace file.",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}}},
	}}

	first, err := client.Complete(context.Background(), nativeModelRequest{
		System: "Use tools when needed.", Provider: provider, Model: model, APIKey: "runtime-token",
		Messages: []nativeConversationMessage{{Role: "user", Text: "Read README.md"}}, Tools: tools,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.ToolCalls) != 1 || first.ToolCalls[0].ID != "call-a" || first.ToolCalls[0].Name != "workspace.read" {
		t.Fatalf("unexpected Gemini tool call %#v", first.ToolCalls)
	}
	if geminiThoughtSignature(first.ToolCalls[0].ProviderState) != "signature-a" {
		t.Fatalf("Gemini thought signature was not preserved: %s", first.ToolCalls[0].ProviderState)
	}

	var streamed strings.Builder
	second, err := client.Complete(context.Background(), nativeModelRequest{
		System: "Use tools when needed.", Provider: provider, Model: model, APIKey: "runtime-token",
		Messages: []nativeConversationMessage{
			{Role: "user", Text: "Read README.md"},
			{Role: "assistant", ToolCalls: first.ToolCalls},
			{Role: "tool", ToolCallID: "call-a", ToolName: "workspace.read", Text: `{"ok":true,"output":"hello"}`},
		},
		Tools: tools,
	}, func(delta string) { streamed.WriteString(delta) })
	if err != nil {
		t.Fatal(err)
	}
	if second.Text != "Done." || streamed.String() != "Done." {
		t.Fatalf("unexpected Gemini final response text=%q streamed=%q", second.Text, streamed.String())
	}
	if !sawQuotaProject || !sawClientID || !sawThoughtSignature || !sawFunctionResponse {
		t.Fatalf("Gemini continuation contract missing quota=%v client=%v signature=%v functionResponse=%v", sawQuotaProject, sawClientID, sawThoughtSignature, sawFunctionResponse)
	}
}
