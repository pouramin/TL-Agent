package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type claudeBridgeToolCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type claudeBridgeOutput struct {
	Text      string                 `json:"text"`
	ToolCalls []claudeBridgeToolCall `json:"toolCalls"`
}

type claudeCLIJSONResult struct {
	Type             string          `json:"type,omitempty"`
	Subtype          string          `json:"subtype,omitempty"`
	IsError          bool            `json:"is_error,omitempty"`
	Result           string          `json:"result,omitempty"`
	SessionID        string          `json:"session_id,omitempty"`
	StructuredOutput json.RawMessage `json:"structured_output,omitempty"`
}

func claudeBridgeSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"additionalProperties": false,
		"required": []string{"text", "toolCalls"},
		"properties": map[string]any{
			"text": map[string]any{"type": "string"},
			"toolCalls": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"additionalProperties": false,
					"required": []string{"name", "arguments"},
					"properties": map[string]any{
						"name": map[string]any{"type": "string"},
						"arguments": map[string]any{
							"type": "string",
							"description": "JSON-encoded object containing the TL Studio tool arguments.",
						},
					},
				},
			},
		},
	}
}

func claudeBridgeSystemPrompt() string {
	return strings.TrimSpace(`
You are the model transport for TL Studio. TL Studio itself owns the coding-agent loop, permissions, project access, and tool execution.

For every turn:
- Do not inspect the filesystem.
- Do not execute shell commands.
- Do not browse the web.
- Do not invoke Claude Code built-in tools, MCP tools, subagents, plugins, skills, or file-edit tools.
- Do not modify any files.
- Decide only the next assistant output for the supplied TL Studio conversation.
- The model identity for this turn is the selectedModel supplied by TL Studio. If the user asks which model is being used, report selectedModel.id exactly.
- If a TL Studio tool is needed, return it in toolCalls and stop. TL Studio will execute it and provide the result on the next turn.
- Every toolCalls[].name must exactly match one of the supplied tool IDs.
- toolCalls[].arguments must be a JSON string encoding one object that matches that tool's input schema.
- If no tool is needed, return assistant text in text and an empty toolCalls array.
- Return only data matching the required JSON schema.
`)
}

func claudeBridgePrompt(request nativeModelRequest) (string, error) {
	payload := map[string]any{
		"selectedModel": map[string]any{
			"id": strings.TrimSpace(request.Model.ID),
			"name": strings.TrimSpace(request.Model.Name),
			"provider": claudeAccountProviderID,
		},
		"system": request.System,
		"messages": request.Messages,
		"tools": request.Tools,
	}
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}
	return "TL Studio turn payload:\n" + string(encoded), nil
}

func claudeBridgeResponse(request nativeModelRequest, output claudeBridgeOutput, onTextDelta func(string)) (nativeModelResponse, error) {
	allowed := map[string]nativeModelToolDefinition{}
	for _, tool := range request.Tools {
		allowed[tool.ID] = tool
		allowed[nativeToolWireName(tool.ID)] = tool
	}
	response := nativeModelResponse{
		Text: strings.TrimSpace(output.Text),
		FinishReason: "completed",
		RoutedModel: strings.TrimSpace(request.Model.ID),
		ToolCalls: []nativeModelToolCall{},
	}
	for _, call := range output.ToolCalls {
		name := strings.TrimSpace(call.Name)
		tool, ok := allowed[name]
		if !ok {
			return nativeModelResponse{}, fmt.Errorf("Claude bridge requested unknown TL Studio tool %q", name)
		}
		argumentsText := strings.TrimSpace(call.Arguments)
		if argumentsText == "" {
			argumentsText = "{}"
		}
		var argumentsObject map[string]any
		if err := json.Unmarshal([]byte(argumentsText), &argumentsObject); err != nil {
			return nativeModelResponse{}, fmt.Errorf("Claude bridge returned invalid JSON arguments for TL Studio tool %q", name)
		}
		arguments, err := json.Marshal(argumentsObject)
		if err != nil {
			return nativeModelResponse{}, err
		}
		callID, err := randomBase64URL(12)
		if err != nil {
			return nativeModelResponse{}, err
		}
		response.ToolCalls = append(response.ToolCalls, nativeModelToolCall{
			ID: "claude_" + callID,
			Name: tool.ID,
			Arguments: arguments,
		})
	}
	if len(response.ToolCalls) > 0 {
		response.FinishReason = "tool_calls"
	}
	if response.Text == "" && len(response.ToolCalls) == 0 {
		return nativeModelResponse{}, errors.New("official Claude Code bridge returned an empty response")
	}
	if onTextDelta != nil && response.Text != "" {
		onTextDelta(response.Text)
	}
	return response, nil
}

func (a *claudeAccountAdapter) CompleteModelTurn(ctx context.Context, request nativeModelRequest, onTextDelta func(string)) (nativeModelResponse, error) {
	if a == nil {
		return nativeModelResponse{}, errors.New("Claude account transport is unavailable")
	}
	command, err := a.resolveCommand()
	if err != nil {
		return nativeModelResponse{}, err
	}
	prompt, err := claudeBridgePrompt(request)
	if err != nil {
		return nativeModelResponse{}, err
	}
	schemaBytes, err := json.Marshal(claudeBridgeSchema())
	if err != nil {
		return nativeModelResponse{}, err
	}
	tempDir, err := os.MkdirTemp("", "tl-studio-claude-turn-*")
	if err != nil {
		return nativeModelResponse{}, err
	}
	defer os.RemoveAll(tempDir)
	systemPath := filepath.Join(tempDir, "system.txt")
	if err := os.WriteFile(systemPath, []byte(claudeBridgeSystemPrompt()), 0o600); err != nil {
		return nativeModelResponse{}, err
	}

	args := []string{
		"-p",
		"--safe-mode",
		"--no-session-persistence",
		"--disable-slash-commands",
		"--tools", "",
		"--disallowedTools", "mcp__*",
		"--permission-prompts", "none",
		"--no-chrome",
		"--strict-mcp-config",
		"--mcp-config", `{"mcpServers":{}}`,
		"--output-format", "json",
		"--json-schema", string(schemaBytes),
		"--model", strings.TrimSpace(request.Model.ID),
		"--system-prompt-file", systemPath,
	}
	cmd := claudeProcess(ctx, command, args...)
	if err := prepareClaudeCommand(cmd); err != nil {
		return nativeModelResponse{}, err
	}
	cmd.Dir = tempDir
	cmd.Stdin = strings.NewReader(prompt)
	var stdout bytes.Buffer
	stderr := newBoundedTextBuffer(16 << 10)
	cmd.Stdout = &stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(stdout.String())
		}
		if detail != "" {
			return nativeModelResponse{}, fmt.Errorf("official Claude Code model bridge failed: %w — %s", err, detail)
		}
		return nativeModelResponse{}, fmt.Errorf("official Claude Code model bridge failed: %w", err)
	}
	if stdout.Len() > 4<<20 {
		return nativeModelResponse{}, errors.New("official Claude Code bridge response is too large")
	}
	var result claudeCLIJSONResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nativeModelResponse{}, fmt.Errorf("decode official Claude Code JSON result: %w", err)
	}
	if result.IsError {
		detail := strings.TrimSpace(result.Result)
		if detail == "" {
			detail = "Claude Code returned an error"
		}
		return nativeModelResponse{}, errors.New(detail)
	}
	if len(result.StructuredOutput) == 0 || string(result.StructuredOutput) == "null" {
		return nativeModelResponse{}, errors.New("official Claude Code did not return structured output")
	}
	var output claudeBridgeOutput
	if err := json.Unmarshal(result.StructuredOutput, &output); err != nil {
		return nativeModelResponse{}, errors.New("official Claude Code returned invalid structured output")
	}
	return claudeBridgeResponse(request, output, onTextDelta)
}
