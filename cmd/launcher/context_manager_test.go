package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNativeContextPlanKeepsConversationWhenWithinBudget(t *testing.T) {
	messages := []nativeConversationMessage{
		{Role: "user", Text: "Inspect the project."},
		{Role: "assistant", Text: "I inspected it."},
		{Role: "user", Text: "Now fix the tests."},
	}
	model := tlProviderModel{ID: "test", ContextLimit: 32768, OutputLimit: 4096}
	plan := buildNativeContextPlan(messages, "system", []nativeModelToolDefinition{nativeTestToolDefinition()}, model)

	if plan.CheckpointInserted || plan.OmittedMessages != 0 || plan.CompactedToolResults != 0 || plan.CompactedToolCalls != 0 {
		t.Fatalf("small conversation should remain untouched: %#v", plan)
	}
	if len(plan.Messages) != len(messages) {
		t.Fatalf("expected %d messages, got %d", len(messages), len(plan.Messages))
	}
	for i := range messages {
		if plan.Messages[i].Role != messages[i].Role || plan.Messages[i].Text != messages[i].Text {
			t.Fatalf("message %d changed unexpectedly: %#v", i, plan.Messages[i])
		}
	}
}

func TestNativeContextPlanWindowsOldTurnsAndKeepsLatestTurn(t *testing.T) {
	large := strings.Repeat("historical context ", 500)
	messages := []nativeConversationMessage{
		{Role: "user", Text: "old request one " + large},
		{Role: "assistant", Text: "old answer one " + large},
		{Role: "user", Text: "old request two " + large},
		{Role: "assistant", Text: "old answer two " + large},
		{Role: "user", Text: "LATEST USER REQUEST MUST SURVIVE"},
		{Role: "assistant", Text: "latest assistant state"},
	}
	model := tlProviderModel{ID: "small", ContextLimit: 4096, OutputLimit: 1024}
	plan := buildNativeContextPlan(messages, "system", nil, model)

	if !plan.CheckpointInserted || plan.OmittedMessages == 0 {
		t.Fatalf("expected old history to be windowed: %#v", plan)
	}
	joined := ""
	for _, message := range plan.Messages {
		joined += "\n" + message.Text
	}
	if !strings.Contains(joined, "[TL Studio context checkpoint]") {
		t.Fatalf("checkpoint marker missing: %#v", plan.Messages)
	}
	if !strings.Contains(joined, "LATEST USER REQUEST MUST SURVIVE") {
		t.Fatalf("latest user turn was dropped: %#v", plan.Messages)
	}
	if strings.Contains(joined, "old request one") {
		t.Fatalf("oldest turn should have been omitted: %#v", plan.Messages)
	}
	if plan.EstimatedMessageTokens > plan.MessageBudget {
		t.Fatalf("windowed context still exceeds message budget: %#v", plan)
	}
}

func TestNativeContextPlanCompactsCompletedToolPayloads(t *testing.T) {
	hugeArguments, _ := json.Marshal(map[string]any{"content": strings.Repeat("A", 9000)})
	hugeResult, _ := json.Marshal(map[string]any{
		"ok":     true,
		"toolID": "files.write",
		"callID": "call-1",
		"output": map[string]any{"content": strings.Repeat("B", 9000)},
		"changes": []map[string]any{{"file": "big.txt", "additions": 1, "deletions": 0}},
	})
	messages := []nativeConversationMessage{
		{Role: "user", Text: "Write a large generated file and continue."},
		{Role: "assistant", ToolCalls: []nativeModelToolCall{{
			ID: "call-1", Name: "files.write", Arguments: hugeArguments,
		}}},
		{Role: "tool", ToolCallID: "call-1", ToolName: "files.write", Text: string(hugeResult)},
		{Role: "assistant", Text: "The write completed; now continue with the task."},
	}
	model := tlProviderModel{ID: "small", ContextLimit: 4096, OutputLimit: 1024}
	plan := buildNativeContextPlan(messages, "system", nil, model)

	if plan.CompactedToolResults == 0 {
		t.Fatalf("expected large tool output to be compacted: %#v", plan)
	}
	if plan.CompactedToolCalls == 0 {
		t.Fatalf("expected completed large tool arguments to be compacted: %#v", plan)
	}
	var sawCompactedResult bool
	var sawCompactedCall bool
	for _, message := range plan.Messages {
		if message.Role == "tool" && strings.Contains(message.Text, "\"compacted\":true") {
			sawCompactedResult = true
		}
		for _, call := range message.ToolCalls {
			if strings.Contains(string(call.Arguments), "_tlStudioContextCompacted") {
				sawCompactedCall = true
			}
		}
	}
	if !sawCompactedResult || !sawCompactedCall {
		t.Fatalf("compacted structures missing from model context: %#v", plan.Messages)
	}
}

func TestNativeContextBudgetUsesAdvertisedModelLimits(t *testing.T) {
	model := tlProviderModel{ID: "bounded", ContextLimit: 16000, OutputLimit: 4000}
	contextLimit, budget, fixed := nativeContextMessageBudget("system prompt", []nativeModelToolDefinition{nativeTestToolDefinition()}, model)

	if contextLimit != 16000 {
		t.Fatalf("expected advertised context limit, got %d", contextLimit)
	}
	if budget <= 0 || budget >= contextLimit {
		t.Fatalf("invalid message budget %d for context limit %d", budget, contextLimit)
	}
	if fixed <= nativeContextSafetyReserve {
		t.Fatalf("fixed budget should include system/tools plus safety reserve, got %d", fixed)
	}
}

func TestNativeContextPlanFallsBackToConservativeDefaultLimit(t *testing.T) {
	plan := buildNativeContextPlan(
		[]nativeConversationMessage{{Role: "user", Text: "hello"}},
		"system",
		nil,
		tlProviderModel{ID: "unknown-limit"},
	)
	if plan.ContextLimit != nativeContextDefaultLimit {
		t.Fatalf("expected default context limit %d, got %d", nativeContextDefaultLimit, plan.ContextLimit)
	}
}

func TestNativeContextActivityReportsManagementMetadata(t *testing.T) {
	plan := nativeContextPlan{
		ContextLimit: 8192, MessageBudget: 4096, EstimatedMessageTokens: 3000,
		OmittedMessages: 6, OmittedTokens: 2200, CompactedToolResults: 2,
		CompactedToolCalls: 1, CheckpointInserted: true,
	}
	activity := nativeContextActivity(plan)
	if activity.Kind != "context" || activity.Status != "completed" {
		t.Fatalf("unexpected context activity %#v", activity)
	}
	if activity.Metadata["omittedMessages"] != 6 || activity.Metadata["compactedToolResults"] != 2 {
		t.Fatalf("context metadata missing: %#v", activity.Metadata)
	}
}
