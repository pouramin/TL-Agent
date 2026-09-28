package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
)

type geminiFunctionCall struct {
	ID   string         `json:"id,omitempty"`
	Name string         `json:"name"`
	Args map[string]any `json:"args,omitempty"`
}

type geminiFunctionResponse struct {
	ID       string         `json:"id,omitempty"`
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

type geminiPart struct {
	Text             string                  `json:"text,omitempty"`
	FunctionCall     *geminiFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *geminiFunctionResponse `json:"functionResponse,omitempty"`
	Thought          bool                    `json:"thought,omitempty"`
	ThoughtSignature string                  `json:"thoughtSignature,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiFunctionDeclaration struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type geminiTool struct {
	FunctionDeclarations []geminiFunctionDeclaration `json:"functionDeclarations,omitempty"`
}

type geminiGenerateRequest struct {
	SystemInstruction *geminiContent `json:"systemInstruction,omitempty"`
	Contents          []geminiContent `json:"contents"`
	Tools             []geminiTool    `json:"tools,omitempty"`
}

type geminiGenerateResponse struct {
	Candidates []struct {
		Content      geminiContent `json:"content"`
		FinishReason string        `json:"finishReason"`
	} `json:"candidates"`
	PromptFeedback struct {
		BlockReason string `json:"blockReason"`
	} `json:"promptFeedback"`
	UsageMetadata struct {
		PromptTokenCount     int64 `json:"promptTokenCount"`
		CandidatesTokenCount int64 `json:"candidatesTokenCount"`
		ThoughtsTokenCount    int64 `json:"thoughtsTokenCount"`
		CachedTokenCount      int64 `json:"cachedContentTokenCount"`
	} `json:"usageMetadata"`
	ModelVersion string `json:"modelVersion"`
}

func geminiToolResponseObject(text string) map[string]any {
	text = strings.TrimSpace(text)
	if text == "" {
		return map[string]any{"result": ""}
	}
	var decoded any
	if json.Unmarshal([]byte(text), &decoded) == nil {
		if object, ok := decoded.(map[string]any); ok {
			return object
		}
		return map[string]any{"result": decoded}
	}
	return map[string]any{"result": text}
}

func geminiProviderState(signature string) json.RawMessage {
	signature = strings.TrimSpace(signature)
	if signature == "" {
		return nil
	}
	encoded, err := json.Marshal(map[string]string{"thoughtSignature": signature})
	if err != nil {
		return nil
	}
	return encoded
}

func geminiThoughtSignature(state json.RawMessage) string {
	if len(state) == 0 {
		return ""
	}
	var payload struct {
		ThoughtSignature string `json:"thoughtSignature"`
	}
	if json.Unmarshal(state, &payload) != nil {
		return ""
	}
	return strings.TrimSpace(payload.ThoughtSignature)
}

func geminiRequestContents(messages []nativeConversationMessage) []geminiContent {
	result := make([]geminiContent, 0, len(messages))
	for _, message := range messages {
		switch strings.ToLower(strings.TrimSpace(message.Role)) {
		case "user":
			text := strings.TrimSpace(message.Text)
			if text != "" {
				result = append(result, geminiContent{
					Role:  "user",
					Parts: []geminiPart{{Text: text}},
				})
			}
		case "assistant":
			parts := []geminiPart{}
			if text := strings.TrimSpace(message.Text); text != "" {
				parts = append(parts, geminiPart{Text: text})
			}
			for _, call := range message.ToolCalls {
				args := map[string]any{}
				if len(call.Arguments) > 0 {
					_ = json.Unmarshal(call.Arguments, &args)
				}
				parts = append(parts, geminiPart{
					FunctionCall: &geminiFunctionCall{
						ID:   strings.TrimSpace(call.ID),
						Name: nativeToolWireName(call.Name),
						Args: args,
					},
					ThoughtSignature: geminiThoughtSignature(call.ProviderState),
				})
			}
			if len(parts) > 0 {
				result = append(result, geminiContent{Role: "model", Parts: parts})
			}
		case "tool":
			name := strings.TrimSpace(message.ToolName)
			if name == "" {
				continue
			}
			result = append(result, geminiContent{
				Role: "user",
				Parts: []geminiPart{{
					FunctionResponse: &geminiFunctionResponse{
						ID:       strings.TrimSpace(message.ToolCallID),
						Name:     nativeToolWireName(name),
						Response: geminiToolResponseObject(message.Text),
					},
				}},
			})
		}
	}
	return result
}

func geminiRequestTools(tools []nativeModelToolDefinition) []geminiTool {
	if len(tools) == 0 {
		return nil
	}
	declarations := make([]geminiFunctionDeclaration, 0, len(tools))
	for _, tool := range tools {
		name := strings.TrimSpace(tool.ID)
		if name == "" {
			continue
		}
		parameters := tool.InputSchema
		if parameters == nil {
			parameters = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		declarations = append(declarations, geminiFunctionDeclaration{
			Name:        nativeToolWireName(name),
			Description: strings.TrimSpace(tool.Description),
			Parameters:  parameters,
		})
	}
	if len(declarations) == 0 {
		return nil
	}
	return []geminiTool{{FunctionDeclarations: declarations}}
}

func geminiGenerateEndpoint(provider tlProviderDefinition, modelID string) (string, error) {
	base, err := url.Parse(strings.TrimSpace(provider.BaseURL))
	if err != nil || base.Scheme == "" || base.Host == "" {
		return "", errors.New("invalid Gemini provider base URL")
	}
	modelID = strings.TrimPrefix(strings.TrimSpace(modelID), "models/")
	if modelID == "" {
		return "", errors.New("Gemini model id is required")
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/models/" + url.PathEscape(modelID) + ":generateContent"
	base.RawQuery = ""
	base.Fragment = ""
	return base.String(), nil
}

func (c *nativeHTTPModelClient) completeGemini(ctx context.Context, request nativeModelRequest, onTextDelta func(string)) (nativeModelResponse, error) {
	endpoint, err := geminiGenerateEndpoint(request.Provider, request.Model.ID)
	if err != nil {
		return nativeModelResponse{}, err
	}
	payload := geminiGenerateRequest{
		Contents: geminiRequestContents(request.Messages),
		Tools:    geminiRequestTools(request.Tools),
	}
	if system := strings.TrimSpace(request.System); system != "" {
		payload.SystemInstruction = &geminiContent{Parts: []geminiPart{{Text: system}}}
	}
	if len(payload.Contents) == 0 {
		return nativeModelResponse{}, errors.New("Gemini request has no conversation content")
	}

	headers := map[string]string{
		"Authorization": "Bearer " + request.APIKey,
	}
	if projectID := strings.TrimSpace(request.Provider.ProjectID); projectID != "" {
		headers["x-goog-user-project"] = projectID
	}
	res, err := c.doJSON(ctx, endpoint, headers, payload)
	if err != nil {
		return nativeModelResponse{}, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(io.LimitReader(res.Body, nativeModelMaxResponseBytes+1))
	if err != nil {
		return nativeModelResponse{}, err
	}
	if len(body) > nativeModelMaxResponseBytes {
		return nativeModelResponse{}, errors.New("Gemini model response is too large")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nativeModelResponse{}, fmt.Errorf("Gemini model request failed with status %d", res.StatusCode)
	}

	var payloadResponse geminiGenerateResponse
	if err := json.Unmarshal(body, &payloadResponse); err != nil {
		return nativeModelResponse{}, errors.New("Gemini model response was invalid")
	}
	if len(payloadResponse.Candidates) == 0 {
		if reason := strings.TrimSpace(payloadResponse.PromptFeedback.BlockReason); reason != "" {
			return nativeModelResponse{}, fmt.Errorf("Gemini blocked the prompt: %s", reason)
		}
		return nativeModelResponse{}, errors.New("Gemini returned no candidates")
	}

	candidate := payloadResponse.Candidates[0]
	textParts := []string{}
	toolCalls := []nativeModelToolCall{}
	for _, part := range candidate.Content.Parts {
		if part.Thought {
			continue
		}
		if text := strings.TrimSpace(part.Text); text != "" {
			textParts = append(textParts, text)
			if onTextDelta != nil {
				onTextDelta(text)
			}
		}
		if part.FunctionCall == nil || strings.TrimSpace(part.FunctionCall.Name) == "" {
			continue
		}
		args, err := json.Marshal(part.FunctionCall.Args)
		if err != nil {
			return nativeModelResponse{}, errors.New("Gemini returned invalid tool arguments")
		}
		toolCalls = append(toolCalls, nativeModelToolCall{
			ID:            strings.TrimSpace(part.FunctionCall.ID),
			Name:          nativeToolIDFromWire(part.FunctionCall.Name, request.Tools),
			Arguments:     args,
			ProviderState: geminiProviderState(part.ThoughtSignature),
		})
	}

	return nativeModelResponse{
		Text:         strings.TrimSpace(strings.Join(textParts, "\n")),
		ToolCalls:    toolCalls,
		FinishReason: strings.TrimSpace(candidate.FinishReason),
		RoutedModel:  strings.TrimSpace(payloadResponse.ModelVersion),
		Usage: sessionUsage{
			Input:     payloadResponse.UsageMetadata.PromptTokenCount,
			Output:    payloadResponse.UsageMetadata.CandidatesTokenCount,
			Reasoning: payloadResponse.UsageMetadata.ThoughtsTokenCount,
			CacheRead: payloadResponse.UsageMetadata.CachedTokenCount,
		},
	}, nil
}
