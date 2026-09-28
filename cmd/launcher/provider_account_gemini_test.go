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

func TestGoogleGeminiAccountLifecycleUsesPKCERefreshAndRevocation(t *testing.T) {
	t.Setenv("TL_STUDIO_STATE_DIR", t.TempDir())

	tokenGrantTypes := []string{}
	modelAuthorization := ""
	modelQuotaProject := ""
	revokeSeen := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			body, _ := io.ReadAll(r.Body)
			values, _ := url.ParseQuery(string(body))
			tokenGrantTypes = append(tokenGrantTypes, values.Get("grant_type"))
			w.Header().Set("Content-Type", "application/json")
			if values.Get("grant_type") == "refresh_token" {
				_, _ = w.Write([]byte(`{"access_token":"access-b","refresh_token":"refresh-b","token_type":"Bearer","scope":"scope-a","expires_in":3600}`))
				return
			}
			_, _ = w.Write([]byte(`{"access_token":"access-a","refresh_token":"refresh-a","token_type":"Bearer","scope":"scope-a","expires_in":3600}`))
		case "/userinfo":
			if got := r.Header.Get("Authorization"); got != "Bearer access-a" {
				t.Errorf("unexpected userinfo authorization %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"email":"amin@example.test","name":"Amin Test"}`))
		case "/models":
			modelAuthorization = r.Header.Get("Authorization")
			modelQuotaProject = r.Header.Get("x-goog-user-project")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"models":[{"name":"models/gemini-test","displayName":"Gemini Test","inputTokenLimit":100000,"outputTokenLimit":8192,"supportedGenerationMethods":["generateContent"]},{"name":"models/embed-test","displayName":"Embed Test","supportedGenerationMethods":["embedContent"]}]}`))
		case "/revoke":
			revokeSeen = true
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

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

	challenge, err := adapter.BeginLogin(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if challenge.Flow != "authorization_code_pkce" || challenge.LoginID == "" {
		t.Fatalf("unexpected challenge %#v", challenge)
	}
	authorizeURL, err := url.Parse(challenge.AuthorizationURL)
	if err != nil {
		t.Fatal(err)
	}
	query := authorizeURL.Query()
	if query.Get("state") != challenge.LoginID {
		t.Fatal("OAuth state must match the login transaction")
	}
	if query.Get("code_challenge_method") != "S256" || query.Get("code_challenge") == "" {
		t.Fatal("Google login must use PKCE S256")
	}
	if query.Get("access_type") != "offline" {
		t.Fatal("Google login must request offline access for refresh tokens")
	}
	if !strings.Contains(query.Get("scope"), "generative-language.retriever") {
		t.Fatal("Google login must request the documented Gemini OAuth scope")
	}

	badChallenge, err := adapter.BeginLogin(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if err := adapter.HandleCallback(context.Background(), "", badChallenge.LoginID, url.Values{
		"state": {"invalid-state"},
		"code":  {"bad-code"},
	}); err == nil {
		t.Fatal("invalid OAuth state must be rejected")
	}

	if err := adapter.HandleCallback(context.Background(), "", challenge.LoginID, url.Values{
		"state": {challenge.LoginID},
		"code":  {"authorization-code"},
	}); err != nil {
		t.Fatal(err)
	}
	status, err := adapter.CompleteLogin(context.Background(), "", challenge.LoginID)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Connected || status.AccountLabel != "amin@example.test" {
		t.Fatalf("unexpected Gemini account status %#v", status)
	}
	if modelAuthorization != "Bearer access-a" {
		t.Fatalf("model discovery did not use the OAuth access token: %q", modelAuthorization)
	}
	if modelQuotaProject != "google-test-project" {
		t.Fatalf("model discovery did not use the configured quota project: %q", modelQuotaProject)
	}

	provider, found, err := manager.store.get(googleGeminiAccountProviderID)
	if err != nil {
		t.Fatal(err)
	}
	if !found || provider.Protocol != "gemini-generate-content" || provider.ProjectID != "google-test-project" || len(provider.Models) != 1 {
		t.Fatalf("unexpected Gemini provider %#v", provider)
	}
	if provider.Models[0].ID != "gemini-test" {
		t.Fatalf("unexpected discovered Gemini model %#v", provider.Models)
	}

	credential, err := adapter.loadCredential()
	if err != nil {
		t.Fatal(err)
	}
	credential.ExpiresAt = time.Now().UTC().Add(-time.Minute).Format(time.RFC3339)
	if err := adapter.saveCredential(credential); err != nil {
		t.Fatal(err)
	}
	token, err := adapter.RuntimeCredential(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if token != "access-b" {
		t.Fatalf("runtime credential did not refresh: %q", token)
	}
	if len(tokenGrantTypes) < 2 || tokenGrantTypes[len(tokenGrantTypes)-1] != "refresh_token" {
		t.Fatalf("refresh grant was not used: %#v", tokenGrantTypes)
	}

	rawRegistry, err := json.Marshal(provider)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"access-a", "refresh-a", "access-b", "refresh-b"} {
		if strings.Contains(string(rawRegistry), secret) {
			t.Fatalf("provider registry leaked credential material %q", secret)
		}
	}

	if err := adapter.Disconnect(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	if !revokeSeen {
		t.Fatal("Google token revocation endpoint was not called")
	}
	if _, err := manager.credentials.Get(providerAccountCredentialID(googleGeminiAccountProviderID)); !errors.Is(err, errCredentialNotFound) {
		t.Fatalf("Gemini account credential remained after disconnect: %v", err)
	}
	if _, found, err := manager.store.get(googleGeminiAccountProviderID); err != nil || found {
		t.Fatalf("Gemini provider remained after disconnect: found=%v err=%v", found, err)
	}
}

func TestNativeGeminiToolContinuationPreservesThoughtSignature(t *testing.T) {
	requestCount := 0
	sawQuotaProject := false
	sawThoughtSignature := false
	sawFunctionResponse := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.Header.Get("Authorization") != "Bearer runtime-token" {
			t.Errorf("unexpected model authorization %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("x-goog-user-project") == "quota-project" {
			sawQuotaProject = true
		}
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Errorf("invalid Gemini request body: %v", err)
		}
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
		ID:        "gemini",
		Name:      "Google / Gemini",
		Protocol:  "gemini-generate-content",
		BaseURL:   server.URL + "/v1beta",
		ProjectID: "quota-project",
	}
	model := tlProviderModel{ID: "gemini-test", Name: "Gemini Test", ToolCall: true}
	tools := []nativeModelToolDefinition{{
		ID:          "workspace.read",
		Name:        "Read file",
		Description: "Read a workspace file.",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}}},
	}}

	first, err := client.Complete(context.Background(), nativeModelRequest{
		System:   "Use tools when needed.",
		Provider: provider,
		Model:    model,
		APIKey:   "runtime-token",
		Messages: []nativeConversationMessage{{Role: "user", Text: "Read README.md"}},
		Tools:    tools,
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
		System:   "Use tools when needed.",
		Provider: provider,
		Model:    model,
		APIKey:   "runtime-token",
		Messages: []nativeConversationMessage{
			{Role: "user", Text: "Read README.md"},
			{Role: "assistant", ToolCalls: first.ToolCalls},
			{Role: "tool", ToolCallID: "call-a", ToolName: "workspace.read", Text: `{"ok":true,"output":"hello"}`},
		},
		Tools: tools,
	}, func(delta string) {
		streamed.WriteString(delta)
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.Text != "Done." || streamed.String() != "Done." {
		t.Fatalf("unexpected Gemini final response text=%q streamed=%q", second.Text, streamed.String())
	}
	if !sawQuotaProject {
		t.Fatal("Gemini transport did not send the quota project header")
	}
	if !sawThoughtSignature {
		t.Fatal("Gemini continuation did not return the provider thought signature")
	}
	if !sawFunctionResponse {
		t.Fatal("Gemini continuation did not send the matching function response")
	}
}
