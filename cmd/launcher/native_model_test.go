package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func nativeTestToolDefinition() nativeModelToolDefinition {
	return nativeModelToolDefinition{
		ID:          "files.write",
		Name:        "Write file",
		Description: "Write a project file.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string"},
			},
			"required": []string{"path"},
		},
	}
}

func TestNativeOpenAICompatibleRequestAndToolNormalization(t *testing.T) {
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("missing bearer auth")
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &received); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"choices":[{"message":{"content":"","tool_calls":[{"id":"call-1","type":"function","function":{"name":"tl_files__write","arguments":"{\"path\":\"x.txt\"}"}}]},"finish_reason":"tool_calls"}],
			"usage":{"prompt_tokens":9,"completion_tokens":4}
		}`)
	}))
	defer server.Close()

	client := newNativeModelClient()
	response, err := client.Complete(context.Background(), nativeModelRequest{
		Provider: tlProviderDefinition{Protocol: "openai-compatible", BaseURL: server.URL + "/v1"},
		Model:    tlProviderModel{ID: "test-model", ToolCall: true},
		APIKey:   "secret",
		Messages: []nativeConversationMessage{{Role: "user", Text: "write it"}},
		Tools:    []nativeModelToolDefinition{nativeTestToolDefinition()},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if received["model"] != "test-model" || received["stream"] != true {
		t.Fatalf("unexpected request payload %#v", received)
	}
	tools, _ := received["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("expected one tool, got %#v", received["tools"])
	}
	if len(response.ToolCalls) != 1 || response.ToolCalls[0].Name != "files.write" {
		t.Fatalf("tool name was not normalized: %#v", response.ToolCalls)
	}
	var arguments map[string]any
	if err := json.Unmarshal(response.ToolCalls[0].Arguments, &arguments); err != nil {
		t.Fatalf("tool arguments are not a JSON object: %s (%v)", response.ToolCalls[0].Arguments, err)
	}
	if arguments["path"] != "x.txt" {
		t.Fatalf("unexpected normalized tool arguments %#v", arguments)
	}
	if response.Usage.Input != 9 || response.Usage.Output != 4 {
		t.Fatalf("unexpected usage %#v", response.Usage)
	}
}

func TestNativeOpenAIResponsesTranslation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["model"] != "responses-model" {
			t.Fatalf("unexpected model %#v", payload["model"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"output":[
				{"type":"function_call","call_id":"r1","name":"tl_files__write","arguments":"{\"path\":\"r.txt\"}"},
				{"type":"message","content":[{"type":"output_text","text":"done"}]}
			],
			"usage":{"input_tokens":5,"output_tokens":2}
		}`)
	}))
	defer server.Close()

	response, err := newNativeModelClient().Complete(context.Background(), nativeModelRequest{
		Provider: tlProviderDefinition{Protocol: "openai-responses", BaseURL: server.URL + "/v1"},
		Model:    tlProviderModel{ID: "responses-model", ToolCall: true},
		APIKey:   "secret",
		Messages: []nativeConversationMessage{{Role: "user", Text: "task"}},
		Tools:    []nativeModelToolDefinition{nativeTestToolDefinition()},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if response.Text != "done" || len(response.ToolCalls) != 1 || response.ToolCalls[0].Name != "files.write" {
		t.Fatalf("unexpected normalized response %#v", response)
	}
}

func TestNativeAnthropicTranslation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "anthropic-secret" {
			t.Fatalf("missing Anthropic API key")
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Fatalf("missing Anthropic version")
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "tl_files__write") {
			t.Fatalf("TL tool wire name missing from request: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"content":[
				{"type":"text","text":"checking"},
				{"type":"tool_use","id":"a1","name":"tl_files__write","input":{"path":"a.txt"}}
			],
			"stop_reason":"tool_use",
			"usage":{"input_tokens":13,"output_tokens":6}
		}`)
	}))
	defer server.Close()

	response, err := newNativeModelClient().Complete(context.Background(), nativeModelRequest{
		System:   "system",
		Provider: tlProviderDefinition{Protocol: "anthropic-messages", BaseURL: server.URL + "/v1"},
		Model:    tlProviderModel{ID: "claude-test", ToolCall: true},
		APIKey:   "anthropic-secret",
		Messages: []nativeConversationMessage{{Role: "user", Text: "task"}},
		Tools:    []nativeModelToolDefinition{nativeTestToolDefinition()},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if response.Text != "checking" || len(response.ToolCalls) != 1 || response.ToolCalls[0].Name != "files.write" {
		t.Fatalf("unexpected normalized response %#v", response)
	}
	if response.Usage.Input != 13 || response.Usage.Output != 6 {
		t.Fatalf("unexpected usage %#v", response.Usage)
	}
}


type nativeModelRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn nativeModelRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestNativeOfficialMistralStreamingUsesUsageAndPromptCache(t *testing.T) {
	var received map[string]any
	client := &nativeHTTPModelClient{
		httpClient: &http.Client{Transport: nativeModelRoundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.URL.String() != "https://api.mistral.ai/v1/chat/completions" {
				t.Fatalf("unexpected Mistral endpoint %s", request.URL.String())
			}
			if err := json.NewDecoder(request.Body).Decode(&received); err != nil {
				t.Fatal(err)
			}
			body := "data: {\"model\":\"mistral-large-4\",\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":1000,\"completion_tokens\":20,\"prompt_tokens_details\":{\"cached_tokens\":896}}}\n\ndata: [DONE]\n\n"
			return &http.Response{
				StatusCode: http.StatusOK,
				Header: http.Header{"Content-Type": []string{"text/event-stream"}},
				Body: io.NopCloser(strings.NewReader(body)),
				Request: request,
			}, nil
		})},
		bridges: map[string]nativeModelBridge{},
	}

	response, err := client.Complete(context.Background(), nativeModelRequest{
		Provider: tlProviderDefinition{Protocol: "openai-compatible", BaseURL: "https://api.mistral.ai/v1"},
		Model: tlProviderModel{ID: "mistral-large-4", ToolCall: true},
		APIKey: "mistral-secret",
		CacheKey: "tlstudio-session-proof",
		Messages: []nativeConversationMessage{{Role: "user", Text: "inspect the project"}},
		Tools: []nativeModelToolDefinition{nativeTestToolDefinition()},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	streamOptions, _ := received["stream_options"].(map[string]any)
	if streamOptions == nil || streamOptions["include_usage"] != true {
		t.Fatalf("Mistral streaming request did not request usage: %#v", received)
	}
	if received["prompt_cache_key"] != "tlstudio-session-proof" {
		t.Fatalf("Mistral prompt cache key missing: %#v", received["prompt_cache_key"])
	}
	if response.Text != "ok" {
		t.Fatalf("unexpected Mistral text %q", response.Text)
	}
	if response.Usage.Input != 1000 || response.Usage.Output != 20 || response.Usage.CacheRead != 896 {
		t.Fatalf("unexpected Mistral usage %#v", response.Usage)
	}
}

func TestNativeNonMistralOpenAICompatibleDoesNotInjectMistralFields(t *testing.T) {
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":1}}`)
	}))
	defer server.Close()

	_, err := newNativeModelClient().Complete(context.Background(), nativeModelRequest{
		Provider: tlProviderDefinition{Protocol: "openai-compatible", BaseURL: server.URL + "/v1"},
		Model: tlProviderModel{ID: "generic-model", ToolCall: true},
		APIKey: "secret",
		CacheKey: "tlstudio-should-not-leak",
		Messages: []nativeConversationMessage{{Role: "user", Text: "hello"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := received["stream_options"]; ok {
		t.Fatalf("generic OpenAI-compatible request received Mistral-only stream_options: %#v", received)
	}
	if _, ok := received["prompt_cache_key"]; ok {
		t.Fatalf("generic OpenAI-compatible request received Mistral-only prompt_cache_key: %#v", received)
	}
}
