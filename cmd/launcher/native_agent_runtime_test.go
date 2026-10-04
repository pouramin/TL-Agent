package main

import (
	"context"
	"encoding/json"
	"errors"
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

func (r nativeTestResolver) resolveNativeModel(_ context.Context, providerID, modelID string) (tlProviderDefinition, tlProviderModel, string, error) {
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
		"project-relative paths",
		"Never invent or prefix paths with /workspace",
		"use files.list with an empty path",
		"perform that work instead of stopping at a plan",
		"batch independent read-only tool calls",
		"avoid rereading files already present in the current tool history",
		"totalLines/nextStartLine",
		"non-interactive",
		"do not use the timeout command",
		"Do not blindly retry multiple shell variants",
		"durationMs",
		"Never claim that a requested wait/delay duration completed successfully",
		"ping -n 61 127.0.0.1 > nul",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("native Agent system prompt missing %q", required)
		}
	}
}


func TestNativeToolResultMessageIncludesMeasuredDuration(t *testing.T) {
	result := nativeToolResult{
		ToolID:   "terminal.command",
		CallID:   "call-duration",
		Output:   map[string]any{"exitCode": 0},
		Duration: 1234,
	}
	message := nativeToolResultMessage(result)
	if !strings.Contains(message, `"durationMs":1234`) {
		t.Fatalf("tool result did not expose measured duration: %s", message)
	}
}

func TestTerminalTimeoutSchemaExplainsDeadlineNotDelay(t *testing.T) {
	schema := nativeToolInputSchema("terminal.command")
	props, _ := schema["properties"].(map[string]any)
	timeout, _ := props["timeoutSeconds"].(map[string]any)
	description, _ := timeout["description"].(string)
	if !strings.Contains(strings.ToLower(description), "deadline") || !strings.Contains(strings.ToLower(description), "not a sleep") {
		t.Fatalf("timeoutSeconds description is ambiguous: %q", description)
	}
}


func TestNativeAgentPromptStopsAfterPermissionRejection(t *testing.T) {
	prompt := nativeAgentSystemPrompt()
	for _, required := range []string{
		"rejected by the user",
		"Do not retry it",
		"do not probe for ways around the rejection",
		"do not reinterpret the rejection as a capability or filesystem-access failure",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("native Agent system prompt missing %q", required)
		}
	}
}


type nativeLayaRouteResolver struct {
	mu         sync.Mutex
	providerID string
	modelID    string
}

func (r *nativeLayaRouteResolver) resolveNativeModel(_ context.Context, providerID, modelID string) (tlProviderDefinition, tlProviderModel, string, error) {
	r.mu.Lock()
	r.providerID = providerID
	r.modelID = modelID
	r.mu.Unlock()
	return tlProviderDefinition{
			ID: providerID, Name: "Routed Provider", Protocol: "openai-compatible", BaseURL: "http://127.0.0.1:1/v1",
	}, tlProviderModel{ID: modelID, Name: "Routed Model", ToolCall: true}, "test-key", nil
}

type nativeFakeLayaRouter struct{}

func (nativeFakeLayaRouter) Handles(model *sessionModelRef) bool {
	return model != nil && model.ProviderID == layaRouterProviderID && model.ID == layaRouterModelID
}

func (nativeFakeLayaRouter) Route(_ context.Context, _ string, prompt string) (nativeRouteSelection, error) {
	return nativeRouteSelection{
		ProviderID: "actual-provider",
		ProviderName: "Actual Provider",
		ModelID: "actual-model",
		ModelName: "Actual Model",
		Profile: "cost",
		Group: "budget",
		Quality: 4,
		Speed: 5,
		Reason: "test route for " + prompt,
		Analysis: layaRouteAnalysis{Difficulty: 1.4, Domain: "code", NeedsTools: 0.7, Sensitive: 0.1, LatencyMS: 4.2},
	}, nil
}

type nativeLayaRouteModel struct {
	finished chan struct{}
}

func (m *nativeLayaRouteModel) Complete(_ context.Context, request nativeModelRequest, _ func(string)) (nativeModelResponse, error) {
	if request.Provider.ID != "actual-provider" || request.Model.ID != "actual-model" {
		return nativeModelResponse{}, errors.New("Laya route was not resolved to the selected real model")
	}
	close(m.finished)
	return nativeModelResponse{Text: "LAYA_ROUTER_OK", FinishReason: "stop", Usage: sessionUsage{Input: 4, Output: 2}}, nil
}

func TestNativeAgentUsesLayaRouterSelectionAndPersistsRoutingActivity(t *testing.T) {
	project := t.TempDir()
	store := newSessionPersistenceStore(filepath.Join(t.TempDir(), "sessions"), "tl-native-laya-router-test")
	session, err := store.createNativeSession(project, sessionCreateInput{Title: "Laya routing proof"})
	if err != nil {
		t.Fatal(err)
	}

	resolver := &nativeLayaRouteResolver{}
	model := &nativeLayaRouteModel{finished: make(chan struct{})}
	executor := newNativeToolExecutor(newProcessManager(func() string { return project }), nativeAllowAuthorizer{})
	runtime := newNativeAgentRuntime(resolver, model, executor, store, newLiveEventBus())
	runtime.setRequestRouter(nativeFakeLayaRouter{})

	input := sessionRunInput{
		Text: "Review the parser structure without modifying files.",
		Agent: "code",
		Model: &sessionModelRef{ProviderID: layaRouterProviderID, ID: layaRouterModelID},
	}
	if err := runtime.Start(project, session.ID, input); err != nil {
		t.Fatal(err)
	}

	select {
	case <-model.finished:
	case <-time.After(3 * time.Second):
		t.Fatal("Laya-routed native Agent did not reach the real model")
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		messages, ok, err := store.getMessages(session.ID)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			for _, message := range messages {
				if message.Role != "assistant" || message.Text != "LAYA_ROUTER_OK" {
					continue
				}
				if message.Model == nil || message.Model.ProviderID != layaRouterProviderID || message.Model.ID != layaRouterModelID {
					t.Fatalf("assistant message should preserve selected Laya Router identity: %#v", message.Model)
				}
				found := false
				for _, activity := range message.Activities {
					if activity.Kind == "model" && activity.Title == "Routed by Laya" &&
						activity.Model != nil && activity.Model.ProviderID == "actual-provider" && activity.Model.ID == "actual-model" {
						found = true
						if activity.Metadata["profile"] != "cost" || activity.Metadata["domain"] != "code" {
							t.Fatalf("Laya routing metadata missing: %#v", activity.Metadata)
						}
					}
				}
				if !found {
					t.Fatalf("Routed by Laya activity was not persisted: %#v", message.Activities)
				}
				resolver.mu.Lock()
				gotProvider, gotModel := resolver.providerID, resolver.modelID
				resolver.mu.Unlock()
				if gotProvider != "actual-provider" || gotModel != "actual-model" {
					t.Fatalf("resolver received %s/%s, want actual-provider/actual-model", gotProvider, gotModel)
				}
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("Laya routed assistant message was not persisted: %#v", messages)
		}
		time.Sleep(20 * time.Millisecond)
	}
}


type nativeFallbackLayaRouter struct{}

func (nativeFallbackLayaRouter) Handles(model *sessionModelRef) bool {
	return model != nil && model.ProviderID == layaRouterProviderID && model.ID == layaRouterModelID
}

func (nativeFallbackLayaRouter) Route(_ context.Context, _ string, _ string) (nativeRouteSelection, error) {
	return nativeRouteSelection{
		ProviderID: "first-provider",
		ProviderName: "First Provider",
		ModelID: "first-model",
		ModelName: "First Model",
		Profile: "cost",
		Group: "free",
		Quality: 4,
		Speed: 4,
		Reason: "initial route",
		Analysis: layaRouteAnalysis{Difficulty: 1.2, Domain: "code", NeedsTools: 0.7},
	}, nil
}

func (nativeFallbackLayaRouter) Fallback(
	_ context.Context,
	_ string,
	_ string,
	previous nativeRouteSelection,
	failure error,
) (nativeRouteSelection, bool, error) {
	if previous.ProviderID != "first-provider" || previous.ModelID != "first-model" {
		return nativeRouteSelection{}, false, nil
	}
	if failure == nil {
		return nativeRouteSelection{}, false, nil
	}
	failureText := strings.ToLower(failure.Error())
	if !strings.Contains(failureText, "429") && !strings.Contains(failureText, "empty response") {
		return nativeRouteSelection{}, false, nil
	}
	return nativeRouteSelection{
		ProviderID: "second-provider",
		ProviderName: "Second Provider",
		ModelID: "second-model",
		ModelName: "Second Model",
		Profile: previous.Profile,
		Group: "free",
		Quality: 4,
		Speed: 3,
		Reason: "automatic fallback",
		Analysis: previous.Analysis,
	}, true, nil
}

type nativeFallbackRouteModel struct {
	mu       sync.Mutex
	calls    int
	finished chan struct{}
	t        *testing.T
}

func (m *nativeFallbackRouteModel) Complete(_ context.Context, request nativeModelRequest, _ func(string)) (nativeModelResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	switch m.calls {
	case 1:
		if request.Provider.ID != "first-provider" || request.Model.ID != "first-model" {
			m.t.Fatalf("first Laya route resolved to %s/%s", request.Provider.ID, request.Model.ID)
		}
		return nativeModelResponse{}, errors.New("model request failed with status 429: temporarily rate-limited upstream")
	case 2:
		if request.Provider.ID != "second-provider" || request.Model.ID != "second-model" {
			m.t.Fatalf("fallback Laya route resolved to %s/%s", request.Provider.ID, request.Model.ID)
		}
		close(m.finished)
		return nativeModelResponse{
			Text: "LAYA_FALLBACK_OK",
			FinishReason: "stop",
			Usage: sessionUsage{Input: 6, Output: 2},
		}, nil
	default:
		m.t.Fatalf("unexpected extra routed model call %d", m.calls)
		return nativeModelResponse{}, nil
	}
}

func TestNativeAgentFallsBackToNextLayaRouteAfterTransientModelFailure(t *testing.T) {
	project := t.TempDir()
	store := newSessionPersistenceStore(filepath.Join(t.TempDir(), "sessions"), "tl-native-laya-fallback-test")
	session, err := store.createNativeSession(project, sessionCreateInput{Title: "Laya fallback proof"})
	if err != nil {
		t.Fatal(err)
	}

	resolver := &nativeLayaRouteResolver{}
	model := &nativeFallbackRouteModel{finished: make(chan struct{}), t: t}
	executor := newNativeToolExecutor(newProcessManager(func() string { return project }), nativeAllowAuthorizer{})
	runtime := newNativeAgentRuntime(resolver, model, executor, store, newLiveEventBus())
	runtime.setRequestRouter(nativeFallbackLayaRouter{})

	input := sessionRunInput{
		Text: "Inspect the project",
		Agent: "code",
		Model: &sessionModelRef{ProviderID: layaRouterProviderID, ID: layaRouterModelID},
	}
	if err := runtime.Start(project, session.ID, input); err != nil {
		t.Fatal(err)
	}

	select {
	case <-model.finished:
	case <-time.After(3 * time.Second):
		t.Fatal("Laya fallback did not reach the second routed model")
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		messages, ok, err := store.getMessages(session.ID)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			for _, message := range messages {
				if message.Role != "assistant" || message.Text != "LAYA_FALLBACK_OK" {
					continue
				}
				if message.Model == nil || message.Model.ProviderID != layaRouterProviderID || message.Model.ID != layaRouterModelID {
					t.Fatalf("assistant message should preserve selected Laya Router identity: %#v", message.Model)
				}
				if len(message.Activities) < 2 {
					t.Fatalf("expected failed and fallback route activities, got %#v", message.Activities)
				}
				first := message.Activities[0]
				second := message.Activities[1]
				if first.Status != "failed" || first.Model == nil ||
					first.Model.ProviderID != "first-provider" || first.Model.ID != "first-model" ||
					first.Error == nil || !strings.Contains(strings.ToLower(first.Error.Message), "rate-limited") {
					t.Fatalf("initial failed route was not preserved: %#v", first)
				}
				if second.Status != "completed" || second.Title != "Routed by Laya" || second.Model == nil ||
					second.Model.ProviderID != "second-provider" || second.Model.ID != "second-model" {
					t.Fatalf("fallback route was not persisted: %#v", second)
				}
				model.mu.Lock()
				calls := model.calls
				model.mu.Unlock()
				if calls != 2 {
					t.Fatalf("expected exactly two routed model calls, got %d", calls)
				}
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("Laya fallback result was not persisted: %#v", messages)
		}
		time.Sleep(20 * time.Millisecond)
	}
}


func TestNativeAgentLayaRouteUsesAdaptiveDefaultModelTimeout(t *testing.T) {
	runtime := &nativeAgentRuntime{modelTurnTimeout: nativeAgentModelTurnTimeout}
	if got := runtime.modelRequestTimeout(false, false); got != nativeAgentModelTurnTimeout {
		t.Fatalf("direct model timeout = %s, want %s", got, nativeAgentModelTurnTimeout)
	}
	if got := runtime.modelRequestTimeout(true, false); got != nativeAgentModelTurnTimeout {
		t.Fatalf("read-only Laya timeout = %s, want %s", got, nativeAgentModelTurnTimeout)
	}
	if got := runtime.modelRequestTimeout(true, true); got != nativeAgentLayaModelTurnTimeout {
		t.Fatalf("execution Laya timeout = %s, want %s", got, nativeAgentLayaModelTurnTimeout)
	}

	runtime.modelTurnTimeout = 30 * time.Millisecond
	if got := runtime.modelRequestTimeout(true, true); got != 30*time.Millisecond {
		t.Fatalf("explicit test/runtime override must win, got %s", got)
	}
}


type nativeEmptyFallbackRouteModel struct {
	mu       sync.Mutex
	calls    int
	finished chan struct{}
	t        *testing.T
}

func (m *nativeEmptyFallbackRouteModel) Complete(_ context.Context, request nativeModelRequest, _ func(string)) (nativeModelResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	switch m.calls {
	case 1:
		if request.Provider.ID != "first-provider" || request.Model.ID != "first-model" {
			m.t.Fatalf("first empty-response route resolved to %s/%s", request.Provider.ID, request.Model.ID)
		}
		return nativeModelResponse{FinishReason: "stop"}, nil
	case 2:
		if request.Provider.ID != "second-provider" || request.Model.ID != "second-model" {
			m.t.Fatalf("fallback after empty response resolved to %s/%s", request.Provider.ID, request.Model.ID)
		}
		close(m.finished)
		return nativeModelResponse{Text: "EMPTY_FALLBACK_OK", FinishReason: "stop"}, nil
	default:
		m.t.Fatalf("unexpected extra empty-response call %d", m.calls)
		return nativeModelResponse{}, nil
	}
}

func TestNativeAgentFallsBackAfterEmptyRoutedResponse(t *testing.T) {
	project := t.TempDir()
	store := newSessionPersistenceStore(filepath.Join(t.TempDir(), "sessions"), "tl-native-empty-route-fallback-test")
	session, err := store.createNativeSession(project, sessionCreateInput{Title: "Empty route fallback proof"})
	if err != nil {
		t.Fatal(err)
	}
	model := &nativeEmptyFallbackRouteModel{finished: make(chan struct{}), t: t}
	executor := newNativeToolExecutor(newProcessManager(func() string { return project }), nativeAllowAuthorizer{})
	runtime := newNativeAgentRuntime(&nativeLayaRouteResolver{}, model, executor, store, newLiveEventBus())
	runtime.setRequestRouter(nativeFallbackLayaRouter{})

	input := sessionRunInput{
		Text: "Explain this project",
		Agent: "code",
		Model: &sessionModelRef{ProviderID: layaRouterProviderID, ID: layaRouterModelID},
	}
	if err := runtime.Start(project, session.ID, input); err != nil {
		t.Fatal(err)
	}
	select {
	case <-model.finished:
	case <-time.After(3 * time.Second):
		t.Fatal("empty routed response did not fall back")
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		messages, ok, err := store.getMessages(session.ID)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			for _, message := range messages {
				if message.Role == "assistant" && message.Text == "EMPTY_FALLBACK_OK" {
					if len(message.Activities) < 2 || message.Activities[0].Status != "failed" {
						t.Fatalf("empty-response fallback activities missing: %#v", message.Activities)
					}
					if message.Activities[0].Error == nil || !strings.Contains(message.Activities[0].Error.Message, "empty response") {
						t.Fatalf("empty-response failure reason missing: %#v", message.Activities[0])
					}
					return
				}
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("empty-response fallback result was not persisted: %#v", messages)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

type nativeExecutionGuardModel struct {
	mu       sync.Mutex
	calls    int
	finished chan struct{}
	t        *testing.T
}

func (m *nativeExecutionGuardModel) Complete(_ context.Context, request nativeModelRequest, _ func(string)) (nativeModelResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	switch m.calls {
	case 1:
		if !strings.Contains(request.System, "This specific turn is an execution task") {
			m.t.Fatalf("execution-specific system contract missing: %s", request.System)
		}
		return nativeModelResponse{Text: "Here is my implementation plan.", FinishReason: "stop"}, nil
	case 2:
		if !strings.Contains(request.System, "previous answer was plan-only") {
			m.t.Fatalf("execution retry guard missing: %s", request.System)
		}
		return nativeModelResponse{
			ToolCalls: []nativeModelToolCall{{
				ID: "guard-write",
				Name: "files.write",
				Arguments: json.RawMessage(`{"path":"guard.txt","content":"done"}`),
			}},
			FinishReason: "tool_calls",
		}, nil
	case 3:
		close(m.finished)
		return nativeModelResponse{Text: "EXECUTION_GUARD_OK", FinishReason: "stop"}, nil
	default:
		m.t.Fatalf("unexpected execution-guard model call %d", m.calls)
		return nativeModelResponse{}, nil
	}
}

func TestNativeAgentRejectsPlanOnlyAnswerForExplicitExecutionTask(t *testing.T) {
	project := t.TempDir()
	store := newSessionPersistenceStore(filepath.Join(t.TempDir(), "sessions"), "tl-native-execution-guard-test")
	session, err := store.createNativeSession(project, sessionCreateInput{Title: "Execution guard proof"})
	if err != nil {
		t.Fatal(err)
	}

	model := &nativeExecutionGuardModel{finished: make(chan struct{}), t: t}
	resolver := nativeTestResolver{
		provider: tlProviderDefinition{ID: "test", Protocol: "openai-compatible", BaseURL: "http://127.0.0.1:1/v1"},
		model: tlProviderModel{ID: "test-model", ToolCall: true},
	}
	executor := newNativeToolExecutor(newProcessManager(func() string { return project }), nativeAllowAuthorizer{})
	runtime := newNativeAgentRuntime(resolver, model, executor, store, newLiveEventBus())

	input := sessionRunInput{
		Text: "Inspect the project, fix the bug, run tests, and verify it works.",
		Agent: "code",
		Model: &sessionModelRef{ProviderID: "test", ID: "test-model"},
	}
	if err := runtime.Start(project, session.ID, input); err != nil {
		t.Fatal(err)
	}
	select {
	case <-model.finished:
	case <-time.After(3 * time.Second):
		t.Fatal("execution guard did not continue past the plan-only answer")
	}

	data, err := os.ReadFile(filepath.Join(project, "guard.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "done" {
		t.Fatalf("execution guard did not perform requested work: %q", string(data))
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, running := runtime.NativeStatuses(project)[session.ID]; !running {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("execution-guard native run did not finish cleanly")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestNativePromptExecutionDetectionHonorsReadOnlyRequests(t *testing.T) {
	if !nativePromptRequiresExecution("Inspect the project, fix the bugs, run the relevant tests.") {
		t.Fatal("explicit implementation request should require execution")
	}
	if nativePromptRequiresExecution("Review the project and tell me what you would improve. Do not modify any files.") {
		t.Fatal("read-only review must not be forced into execution")
	}
}


func TestNativeFilesReadUsesBoundedLineRanges(t *testing.T) {
	project := t.TempDir()
	lines := make([]string, 0, 900)
	for index := 1; index <= 900; index++ {
		lines = append(lines, fmt.Sprintf("line-%03d", index))
	}
	if err := os.WriteFile(filepath.Join(project, "large.txt"), []byte(strings.Join(lines, "\n")), 0o600); err != nil {
		t.Fatal(err)
	}

	first, err := nativeReadFile(project, map[string]any{"path": "large.txt"})
	if err != nil {
		t.Fatal(err)
	}
	firstMap, _ := first.(map[string]any)
	if got := firstMap["startLine"]; got != 1 {
		t.Fatalf("default startLine = %#v, want 1", got)
	}
	if got := firstMap["endLine"]; got != nativeAgentReadDefaultLines {
		t.Fatalf("default endLine = %#v, want %d", got, nativeAgentReadDefaultLines)
	}
	if got := firstMap["totalLines"]; got != 900 {
		t.Fatalf("totalLines = %#v, want 900", got)
	}
	if got := firstMap["nextStartLine"]; got != nativeAgentReadDefaultLines+1 {
		t.Fatalf("nextStartLine = %#v, want %d", got, nativeAgentReadDefaultLines+1)
	}
	if truncated, _ := firstMap["truncated"].(bool); !truncated {
		t.Fatal("large default read should be marked truncated")
	}
	content, _ := firstMap["content"].(string)
	if !strings.Contains(content, "line-001") || !strings.Contains(content, "line-400") || strings.Contains(content, "line-401") {
		t.Fatalf("unexpected first page content")
	}

	ranged, err := nativeReadFile(project, map[string]any{
		"path": "large.txt",
		"startLine": json.Number("401"),
		"endLine": json.Number("450"),
	})
	if err != nil {
		t.Fatal(err)
	}
	rangeMap, _ := ranged.(map[string]any)
	if rangeMap["startLine"] != 401 || rangeMap["endLine"] != 450 {
		t.Fatalf("unexpected explicit range %#v", rangeMap)
	}
	rangeContent, _ := rangeMap["content"].(string)
	if !strings.HasPrefix(rangeContent, "line-401") || !strings.HasSuffix(rangeContent, "line-450") {
		t.Fatalf("unexpected ranged content")
	}
}

func TestNativeFilesReadKeepsSmallFilesComplete(t *testing.T) {
	project := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, "small.txt"), []byte("one\ntwo\nthree"), 0o600); err != nil {
		t.Fatal(err)
	}
	output, err := nativeReadFile(project, map[string]any{"path": "small.txt"})
	if err != nil {
		t.Fatal(err)
	}
	result, _ := output.(map[string]any)
	if result["content"] != "one\ntwo\nthree" || result["truncated"] != false {
		t.Fatalf("small file should remain complete: %#v", result)
	}
}
