package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func nativeOnlyJSONRequest(t *testing.T, method, address string, body any) *http.Response {
	t.Helper()
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = strings.NewReader(string(data))
	}
	req, err := http.NewRequest(method, address, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func decodeNativeOnlyJSON(t *testing.T, res *http.Response, target any) {
	t.Helper()
	defer res.Body.Close()
	if err := json.NewDecoder(res.Body).Decode(target); err != nil {
		t.Fatal(err)
	}
}

func TestNativeOnlyServerRunsCustomProviderWithoutCompatibilityRuntime(t *testing.T) {
	stateDir := t.TempDir()
	project := t.TempDir()
	t.Setenv("TL_STUDIO_STATE_DIR", stateDir)

	var modelCalls atomic.Int32
	modelServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected model path %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer native-only-key" {
			t.Fatalf("unexpected provider auth header %q", got)
		}
		modelCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"choices":[{"message":{"content":"NATIVE_ONLY_OK"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":7,"completion_tokens":2}
		}`)
	}))
	defer modelServer.Close()

	state := &appState{
		project:              project,
		frontendURL:          "http://127.0.0.1",
		compatRuntimeEngine:  "kilo-code",
		compatRuntimeEnabled: false,
		ctx:                  context.Background(),
	}
	handler, err := newServerWithRuntime(state, "", runtimeCredentials{}, kiloRuntimeEngine{})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()

	healthRes := nativeOnlyJSONRequest(t, http.MethodGet, server.URL+"/local/health", nil)
	if healthRes.StatusCode != http.StatusOK {
		t.Fatalf("native health status=%d", healthRes.StatusCode)
	}
	var health map[string]any
	decodeNativeOnlyJSON(t, healthRes, &health)
	if health["healthy"] != true || health["mode"] != "native-only" || health["compatibilityAvailable"] != false {
		t.Fatalf("unexpected native health %#v", health)
	}

	runtimeRes := nativeOnlyJSONRequest(t, http.MethodGet, server.URL+"/runtime/global/health", nil)
	if runtimeRes.StatusCode != http.StatusServiceUnavailable {
		runtimeRes.Body.Close()
		t.Fatalf("expected unavailable compatibility proxy, got %d", runtimeRes.StatusCode)
	}
	runtimeRes.Body.Close()

	agentsRes := nativeOnlyJSONRequest(t, http.MethodGet, server.URL+"/local/agents", nil)
	var agents []map[string]any
	decodeNativeOnlyJSON(t, agentsRes, &agents)
	if len(agents) != 1 || agents[0]["id"] != "code" {
		t.Fatalf("native agents mismatch %#v", agents)
	}

	provider := map[string]any{
		"provider": map[string]any{
			"id":       "nativeproof",
			"name":     "Native Proof",
			"protocol": "openai-compatible",
			"baseURL":  modelServer.URL + "/v1",
			"models": []map[string]any{{
				"id":       "proof-model",
				"name":     "Proof Model",
				"toolCall": true,
			}},
		},
		"apiKey": "native-only-key",
	}
	providerRes := nativeOnlyJSONRequest(t, http.MethodPut, server.URL+"/runtime/providers/config/nativeproof", provider)
	if providerRes.StatusCode != http.StatusOK {
		var failure any
		decodeNativeOnlyJSON(t, providerRes, &failure)
		t.Fatalf("provider save failed status=%d body=%#v", providerRes.StatusCode, failure)
	}
	providerRes.Body.Close()

	catalogRes := nativeOnlyJSONRequest(t, http.MethodGet, server.URL+"/runtime/providers/catalog?directory="+url.QueryEscape(project), nil)
	if catalogRes.StatusCode != http.StatusOK {
		t.Fatalf("catalog status=%d", catalogRes.StatusCode)
	}
	var catalog providerCatalogResponse
	decodeNativeOnlyJSON(t, catalogRes, &catalog)
	if catalog.Hosted.Available || len(catalog.Hosted.PreferredModels) != 0 {
		t.Fatalf("hosted compatibility provider must be unavailable: %#v", catalog.Hosted)
	}
	foundNative := false
	for _, item := range catalog.All {
		if item.ID == "kilo" {
			t.Fatalf("hosted Kilo must not appear in native-only catalog: %#v", item)
		}
		if item.ID == "nativeproof" {
			foundNative = true
			model, ok := item.Models["proof-model"]
			if !ok || model.Enabled != nil && !*model.Enabled {
				t.Fatalf("native model unexpectedly disabled: %#v", item)
			}
		}
	}
	if !foundNative {
		t.Fatalf("custom native provider missing from catalog: %#v", catalog.All)
	}

	permissionsRes := nativeOnlyJSONRequest(t, http.MethodGet, server.URL+"/local/permissions", nil)
	var permissions []map[string]any
	decodeNativeOnlyJSON(t, permissionsRes, &permissions)
	if len(permissions) != 0 {
		t.Fatalf("unexpected native-only pending permissions %#v", permissions)
	}

	questionsRes := nativeOnlyJSONRequest(t, http.MethodGet, server.URL+"/local/questions", nil)
	var questions []questionRequestView
	decodeNativeOnlyJSON(t, questionsRes, &questions)
	if len(questions) != 0 {
		t.Fatalf("unexpected compatibility questions %#v", questions)
	}

	createRes := nativeOnlyJSONRequest(t, http.MethodPost, server.URL+"/local/sessions?directory="+url.QueryEscape(project), map[string]any{"title": "Native only"})
	if createRes.StatusCode != http.StatusCreated {
		var failure any
		decodeNativeOnlyJSON(t, createRes, &failure)
		t.Fatalf("native session create status=%d body=%#v", createRes.StatusCode, failure)
	}
	var session sessionView
	decodeNativeOnlyJSON(t, createRes, &session)
	if !strings.HasPrefix(session.ID, "tls_") || session.Title != "Native only" {
		t.Fatalf("unexpected native session %#v", session)
	}

	updateRes := nativeOnlyJSONRequest(t, http.MethodPatch, server.URL+"/local/sessions/"+url.PathEscape(session.ID)+"?directory="+url.QueryEscape(project), map[string]any{"title": "Native renamed"})
	if updateRes.StatusCode != http.StatusOK {
		t.Fatalf("native session update status=%d", updateRes.StatusCode)
	}
	var updated sessionView
	decodeNativeOnlyJSON(t, updateRes, &updated)
	if updated.Title != "Native renamed" {
		t.Fatalf("native rename failed %#v", updated)
	}

	runRes := nativeOnlyJSONRequest(t, http.MethodPost, server.URL+"/local/sessions/"+url.PathEscape(session.ID)+"/runs?directory="+url.QueryEscape(project), map[string]any{
		"text":  "Reply with proof",
		"agent": "code",
		"model": map[string]any{"providerID": "nativeproof", "id": "proof-model"},
	})
	if runRes.StatusCode != http.StatusAccepted {
		var failure any
		decodeNativeOnlyJSON(t, runRes, &failure)
		t.Fatalf("native run status=%d body=%#v", runRes.StatusCode, failure)
	}
	runRes.Body.Close()

	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		messagesRes := nativeOnlyJSONRequest(t, http.MethodGet, server.URL+"/local/sessions/"+url.PathEscape(session.ID)+"/messages?directory="+url.QueryEscape(project), nil)
		var messages []sessionMessageView
		decodeNativeOnlyJSON(t, messagesRes, &messages)
		for _, message := range messages {
			if message.Role == "assistant" && message.Text == "NATIVE_ONLY_OK" {
				if modelCalls.Load() != 1 {
					t.Fatalf("expected one native model request, got %d", modelCalls.Load())
				}
				deleteRes := nativeOnlyJSONRequest(t, http.MethodDelete, server.URL+"/local/sessions/"+url.PathEscape(session.ID)+"?directory="+url.QueryEscape(project), nil)
				if deleteRes.StatusCode != http.StatusOK {
					deleteRes.Body.Close()
					t.Fatalf("native delete status=%d", deleteRes.StatusCode)
				}
				deleteRes.Body.Close()
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("native-only semantic run did not complete")
}
