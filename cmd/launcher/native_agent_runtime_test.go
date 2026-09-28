package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type nativeTestResolver struct {
	provider tlProviderDefinition
	model    tlProviderModel
}

func (r nativeTestResolver) resolveNativeModel(providerID, modelID string) (tlProviderDefinition, tlProviderModel, string, error) {
	return r.provider, r.model, "test-key", nil
}

type nativeAllowAuthorizer struct{}

func (nativeAllowAuthorizer) AuthorizeNativeTool(context.Context, string, string, toolDescriptor, map[string]any) error {
	return nil
}

type nativeLoopFakeModel struct {
	mu       sync.Mutex
	calls    int
	finished chan struct{}
	t        *testing.T
}

func (m *nativeLoopFakeModel) Complete(_ context.Context, request nativeModelRequest, _ func(string)) (nativeModelResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	switch m.calls {
	case 1:
		return nativeModelResponse{
			ToolCalls: []nativeModelToolCall{{
				ID:        "call-1",
				Name:      "files.write",
				Arguments: json.RawMessage(`{"path":"hello.txt","content":"TL_STUDIO_NATIVE_OK"}`),
			}},
			FinishReason: "tool_calls",
			Usage:        sessionUsage{Input: 7, Output: 3},
		}, nil
	case 2:
		found := false
		for _, message := range request.Messages {
			if message.Role == "tool" && message.ToolCallID == "call-1" &&
				strings.Contains(message.Text, `"path":"hello.txt"`) &&
				strings.Contains(message.Text, `"ok":true`) {
				found = true
				break
			}
		}
		if !found {
			m.t.Errorf("second model turn did not receive the real tool result: %#v", request.Messages)
		}
		close(m.finished)
		return nativeModelResponse{
			Text:         "NATIVE_AGENT_OK",
			FinishReason: "stop",
			Usage:        sessionUsage{Input: 11, Output: 5},
		}, nil
	default:
		m.t.Fatalf("unexpected extra model call %d", m.calls)
		return nativeModelResponse{}, nil
	}
}

func TestNativeAgentLoopExecutesToolAndContinuesWithoutKilo(t *testing.T) {
	project := t.TempDir()
	store := newSessionPersistenceStore(filepath.Join(t.TempDir(), "sessions"), "tl-native-test")
	session, err := store.createNativeSession(project, sessionCreateInput{Title: "Native proof"})
	if err != nil {
		t.Fatal(err)
	}

	model := &nativeLoopFakeModel{finished: make(chan struct{}), t: t}
	resolver := nativeTestResolver{
		provider: tlProviderDefinition{
			ID: "test",
			Protocol: "openai-compatible",
			BaseURL: "http://127.0.0.1:1/v1",
		},
		model: tlProviderModel{ID: "test-model", ToolCall: true},
	}
	executor := newNativeToolExecutor(newProcessManager(func() string { return project }), nativeAllowAuthorizer{})
	runtime := newNativeAgentRuntime(resolver, model, executor, store, newLiveEventBus())

	input := sessionRunInput{
		Text:  "Create hello.txt",
		Agent: "code",
		Model: &sessionModelRef{ProviderID: "test", ID: "test-model"},
	}
	if err := runtime.Start(project, session.ID, input); err != nil {
		t.Fatal(err)
	}

	select {
	case <-model.finished:
	case <-time.After(3 * time.Second):
		t.Fatal("native Agent did not complete model/tool/model loop")
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		messages, ok, err := store.getMessages(session.ID)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			for _, message := range messages {
				if message.Role == "assistant" && message.Text == "NATIVE_AGENT_OK" {
					data, err := os.ReadFile(filepath.Join(project, "hello.txt"))
					if err != nil {
						t.Fatal(err)
					}
					if string(data) != "TL_STUDIO_NATIVE_OK" {
						t.Fatalf("unexpected file contents %q", string(data))
					}
					if model.calls != 2 {
						t.Fatalf("expected 2 model turns, got %d", model.calls)
					}
					return
				}
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("final native assistant message was not persisted: %#v", messages)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

type nativeEmptyResponseModel struct {
	finished chan struct{}
}

func (m *nativeEmptyResponseModel) Complete(context.Context, nativeModelRequest, func(string)) (nativeModelResponse, error) {
	close(m.finished)
	return nativeModelResponse{FinishReason: "stop"}, nil
}

func TestNativeAgentTurnsEmptyModelResponseIntoVisibleFailure(t *testing.T) {
	project := t.TempDir()
	store := newSessionPersistenceStore(filepath.Join(t.TempDir(), "sessions"), "tl-native-empty-test")
	session, err := store.createNativeSession(project, sessionCreateInput{Title: "Empty response proof"})
	if err != nil {
		t.Fatal(err)
	}

	model := &nativeEmptyResponseModel{finished: make(chan struct{})}
	resolver := nativeTestResolver{
		provider: tlProviderDefinition{
			ID: "test", Protocol: "openai-compatible", BaseURL: "http://127.0.0.1:1/v1",
		},
		model: tlProviderModel{ID: "test-model", ToolCall: true},
	}
	executor := newNativeToolExecutor(newProcessManager(func() string { return project }), nativeAllowAuthorizer{})
	runtime := newNativeAgentRuntime(resolver, model, executor, store, newLiveEventBus())

	input := sessionRunInput{
		Text: "Say hello", Agent: "code",
		Model: &sessionModelRef{ProviderID: "test", ID: "test-model"},
	}
	if err := runtime.Start(project, session.ID, input); err != nil {
		t.Fatal(err)
	}
	select {
	case <-model.finished:
	case <-time.After(3 * time.Second):
		t.Fatal("native Agent did not receive the model response")
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		messages, ok, err := store.getMessages(session.ID)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			for _, message := range messages {
				if message.Role == "assistant" && message.Error != nil &&
					strings.Contains(message.Error.Message, "model returned an empty response") {
					return
				}
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("empty response failure was not persisted visibly: %#v", messages)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

type nativeBlockingModel struct{}

func (nativeBlockingModel) Complete(ctx context.Context, _ nativeModelRequest, _ func(string)) (nativeModelResponse, error) {
	<-ctx.Done()
	return nativeModelResponse{}, ctx.Err()
}

func TestNativeAgentPersistsVisibleModelTimeout(t *testing.T) {
	project := t.TempDir()
	store := newSessionPersistenceStore(filepath.Join(t.TempDir(), "sessions"), "tl-native-timeout-test")
	session, err := store.createNativeSession(project, sessionCreateInput{Title: "Timeout proof"})
	if err != nil {
		t.Fatal(err)
	}

	resolver := nativeTestResolver{
		provider: tlProviderDefinition{
			ID: "test", Protocol: "openai-compatible", BaseURL: "http://127.0.0.1:1/v1",
		},
		model: tlProviderModel{ID: "test-model", ToolCall: true},
	}
	executor := newNativeToolExecutor(newProcessManager(func() string { return project }), nativeAllowAuthorizer{})
	runtime := newNativeAgentRuntime(resolver, nativeBlockingModel{}, executor, store, newLiveEventBus())
	runtime.modelTurnTimeout = 30 * time.Millisecond

	input := sessionRunInput{
		Text: "Say hello", Agent: "code",
		Model: &sessionModelRef{ProviderID: "test", ID: "test-model"},
	}
	if err := runtime.Start(project, session.ID, input); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		messages, ok, err := store.getMessages(session.ID)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			for _, message := range messages {
				if message.Role == "assistant" && message.Error != nil &&
					strings.Contains(message.Error.Message, "model request timed out after") {
					return
				}
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("model timeout failure was not persisted visibly: %#v", messages)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestNativeToolExecutorRejectsTraversalAndUnknownTools(t *testing.T) {
	project := t.TempDir()
	executor := newNativeToolExecutor(newProcessManager(func() string { return project }), nativeAllowAuthorizer{})

	traversal := executor.Execute(context.Background(), "s1", project, nativeToolCall{
		ID: "files.write",
		Arguments: json.RawMessage(`{"path":"../escape.txt","content":"no"}`),
	})
	if traversal.Error == "" {
		t.Fatal("expected path traversal to be rejected")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(project), "escape.txt")); !os.IsNotExist(err) {
		t.Fatalf("escape file should not exist, stat err=%v", err)
	}

	unknown := executor.Execute(context.Background(), "s1", project, nativeToolCall{
		ID:        "runtime.mystery",
		Arguments: json.RawMessage(`{}`),
	})
	if unknown.Error == "" {
		t.Fatal("expected unknown tool to be rejected")
	}
}


func TestNativeConversationKeepsCancellationBoundary(t *testing.T) {
	messages := []sessionMessageView{
		{Role: "user", Text: "Run a command that waits for 60 seconds."},
		{Role: "assistant", Error: &sessionErrorView{Type: "cancelled", Message: "context canceled"}},
		{Role: "user", Text: "reply with: abort test passed"},
	}
	conversation := nativeConversationFromMessages(messages)
	if len(conversation) != 3 {
		t.Fatalf("expected cancellation boundary to remain in model context, got %#v", conversation)
	}
	if conversation[1].Role != "assistant" || !strings.Contains(strings.ToLower(conversation[1].Text), "cancelled by the user") {
		t.Fatalf("cancelled turn was not represented as an assistant boundary: %#v", conversation[1])
	}
	if strings.Contains(strings.ToLower(conversation[2].Text), "60 seconds") {
		t.Fatalf("new user turn was contaminated by cancelled task context: %#v", conversation[2])
	}
	if conversation[2].Text != "reply with: abort test passed" {
		t.Fatalf("unexpected new user turn %q", conversation[2].Text)
	}
}

func TestNativeAgentPromptExplainsNonInteractiveShellRetries(t *testing.T) {
	prompt := nativeAgentSystemPrompt()
	for _, required := range []string{
		"non-interactive",
		"do not use the timeout command",
		"Do not blindly retry multiple shell variants",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("native Agent system prompt missing %q", required)
		}
	}
}
