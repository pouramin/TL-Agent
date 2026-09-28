package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNativeAgentInteractiveQuestionE2E(t *testing.T) {
	project := t.TempDir()
	store := newSessionPersistenceStore(filepath.Join(t.TempDir(), "sessions"), "native-question-proof")
	session, err := store.createNativeSession(project, sessionCreateInput{Title: "Question proof"})
	if err != nil {
		t.Fatal(err)
	}

	var calls atomic.Int32
	modelServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected model path %s", r.URL.Path)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		n := calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if n == 1 {
			_, _ = io.WriteString(w, "{\n\"choices\":[{\"message\":{\"content\":null,\"tool_calls\":[{\"id\":\"question-call\",\"type\":\"function\",\"function\":{\"name\":\"tl_interaction__question\",\"arguments\":\"{\\\"questions\\\":[{\\\"header\\\":\\\"Choice\\\",\\\"question\\\":\\\"Which path?\\\",\\\"options\\\":[{\\\"label\\\":\\\"A\\\"},{\\\"label\\\":\\\"B\\\"}],\\\"custom\\\":true}]}\"}}]},\"finish_reason\":\"tool_calls\"}],\n\"usage\":{\"prompt_tokens\":8,\"completion_tokens\":3}\n}")
			return
		}
		if n == 2 {
			messages, _ := payload["messages"].([]any)
			sawAnswer := false
			for _, raw := range messages {
				message, _ := raw.(map[string]any)
				if message["role"] != "tool" {
					continue
				}
				body, _ := message["content"].(string)
				if strings.Contains(body, "\"answers\":[[\"B\"]]") {
					sawAnswer = true
				}
			}
			if !sawAnswer {
				t.Fatalf("model did not receive native question answer: %#v", payload)
			}
			_, _ = io.WriteString(w, "{\"choices\":[{\"message\":{\"content\":\"QUESTION_RESUMED_OK\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":3}}")
			return
		}
		t.Fatalf("unexpected model call %d", n)
	}))
	defer modelServer.Close()

	resolver := nativeTestResolver{
		provider: tlProviderDefinition{ID: "proof", Protocol: "openai-compatible", BaseURL: modelServer.URL + "/v1"},
		model:    tlProviderModel{ID: "proof-model", ToolCall: true},
	}
	questions := newQuestionContract(&appState{project: project}, newLiveEventBus())
	executor := newNativeToolExecutor(newProcessManager(func() string { return project }), nativeAllowAuthorizer{})
	executor.setQuestionManager(questions)
	runtime := newNativeAgentRuntime(resolver, newNativeModelClient(), executor, store, newLiveEventBus())

	if err := runtime.Start(project, session.ID, sessionRunInput{
		Text: "Ask me which path to use.",
		Model: &sessionModelRef{ProviderID: "proof", ID: "proof-model"},
		Agent: "code",
	}); err != nil {
		t.Fatal(err)
	}

	var pending questionRequestView
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		items := questions.list(session.ID)
		if len(items) == 1 {
			pending = items[0]
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pending.ID == "" {
		t.Fatal("native Agent question did not become pending")
	}
	if err := questions.resolve(pending.ID, session.ID, questionResolution{answers: [][]string{{"B"}}}); err != nil {
		t.Fatal(err)
	}

	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		messages, _, readErr := store.getMessages(session.ID)
		if readErr != nil {
			t.Fatal(readErr)
		}
		for _, message := range messages {
			if message.Role == "assistant" && message.Text == "QUESTION_RESUMED_OK" {
				if calls.Load() != 2 {
					t.Fatalf("expected 2 model turns, got %d", calls.Load())
				}
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("native Agent did not resume after question answer")
}
