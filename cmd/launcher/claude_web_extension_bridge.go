package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
)

var errClaudeWebExtensionNotPaired = errors.New("Claude Web Chrome extension is not paired")

type claudeWebExtensionCommand struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Prompt string `json:"prompt,omitempty"`
	Model  string `json:"model,omitempty"`
}

type claudeWebExtensionResult struct {
	ID               string `json:"id"`
	OK               bool   `json:"ok"`
	Connected        bool   `json:"connected,omitempty"`
	Status           int    `json:"status,omitempty"`
	Text             string   `json:"text,omitempty"`
	Model            string   `json:"model,omitempty"`
	Models           []string `json:"models,omitempty"`
	Error            string   `json:"error,omitempty"`
	OrganizationID   string   `json:"organizationId,omitempty"`
	OrganizationName string   `json:"organizationName,omitempty"`
}

type claudeWebExtensionBridge struct {
	state   *appState
	mu      sync.Mutex
	token   string
	paired  bool
	queue   []claudeWebExtensionCommand
	waiters map[string]chan claudeWebExtensionResult
}

func newClaudeWebExtensionBridge(state *appState) *claudeWebExtensionBridge {
	return &claudeWebExtensionBridge{state: state, waiters: map[string]chan claudeWebExtensionResult{}}
}

func (b *claudeWebExtensionBridge) Available() error {
	if b == nil || b.state == nil {
		return errors.New("Claude Web extension bridge is unavailable")
	}
	return nil
}

func (b *claudeWebExtensionBridge) frontendURL() string {
	if b == nil || b.state == nil {
		return ""
	}
	b.state.mu.RLock()
	defer b.state.mu.RUnlock()
	return strings.TrimRight(strings.TrimSpace(b.state.frontendURL), "/")
}

func (b *claudeWebExtensionBridge) resetLocked(token string) {
	for id, waiter := range b.waiters {
		delete(b.waiters, id)
		select {
		case waiter <- claudeWebExtensionResult{ID: id, Error: "Claude Web bridge session replaced"}:
		default:
		}
	}
	b.token = token
	b.paired = false
	b.queue = nil
}

func (b *claudeWebExtensionBridge) OpenLogin(ctx context.Context) error {
	if err := b.Available(); err != nil {
		return err
	}
	if b.frontendURL() == "" {
		return errors.New("TL Studio local URL is unavailable")
	}
	token, err := randomSecret(32)
	if err != nil {
		return err
	}
	b.mu.Lock()
	b.resetLocked(token)
	b.mu.Unlock()
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func (b *claudeWebExtensionBridge) PairingToken() string {
	if b == nil {
		return ""
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.token
}

func (b *claudeWebExtensionBridge) PairingOrigin() string {
	if b == nil {
		return ""
	}
	return b.frontendURL()
}

func (b *claudeWebExtensionBridge) Paired() bool {
	if b == nil {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.TrimSpace(b.token) != "" && b.paired
}

func (b *claudeWebExtensionBridge) Probe(ctx context.Context) (claudeWebProbe, error) {
	b.mu.Lock()
	paired := b.paired
	b.mu.Unlock()
	if !paired {
		return claudeWebProbe{}, errClaudeWebExtensionNotPaired
	}
	result, err := b.send(ctx, claudeWebExtensionCommand{Kind: "probe"})
	if err != nil {
		return claudeWebProbe{}, err
	}
	if !result.OK {
		detail := strings.TrimSpace(result.Error)
		if detail == "" {
			detail = "Claude Web session probe failed"
		}
		return claudeWebProbe{}, errors.New(detail)
	}
	return claudeWebProbe{
		Connected:        result.Connected,
		Status:           result.Status,
		OrganizationID:   strings.TrimSpace(result.OrganizationID),
		OrganizationName: strings.TrimSpace(result.OrganizationName),
		Models:           append([]string(nil), result.Models...),
		Error:            strings.TrimSpace(result.Error),
	}, nil
}

func (b *claudeWebExtensionBridge) Complete(ctx context.Context, prompt, model string) (claudeWebCompletion, error) {
	result, err := b.send(ctx, claudeWebExtensionCommand{Kind: "complete", Prompt: prompt, Model: strings.TrimSpace(model)})
	if err != nil {
		return claudeWebCompletion{}, err
	}
	if !result.OK {
		detail := strings.TrimSpace(result.Error)
		if detail == "" {
			detail = "Claude Web request failed"
		}
		return claudeWebCompletion{}, errors.New(detail)
	}
	if strings.TrimSpace(result.Text) == "" {
		return claudeWebCompletion{}, errors.New("Claude Web returned an empty response")
	}
	return claudeWebCompletion{Text: result.Text, Model: strings.TrimSpace(result.Model)}, nil
}

func (b *claudeWebExtensionBridge) Close(context.Context) error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	b.resetLocked("")
	b.mu.Unlock()
	return nil
}

func (b *claudeWebExtensionBridge) send(ctx context.Context, command claudeWebExtensionCommand) (claudeWebExtensionResult, error) {
	if command.Kind != "probe" && command.Kind != "complete" {
		return claudeWebExtensionResult{}, errors.New("unsupported Claude Web transport command")
	}
	id, err := randomSecret(12)
	if err != nil {
		return claudeWebExtensionResult{}, err
	}
	command.ID = id
	waiter := make(chan claudeWebExtensionResult, 1)
	b.mu.Lock()
	if b.token == "" || !b.paired {
		b.mu.Unlock()
		return claudeWebExtensionResult{}, errClaudeWebExtensionNotPaired
	}
	b.waiters[id] = waiter
	b.queue = append(b.queue, command)
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		delete(b.waiters, id)
		b.mu.Unlock()
	}()
	select {
	case <-ctx.Done():
		return claudeWebExtensionResult{}, ctx.Err()
	case result := <-waiter:
		return result, nil
	}
}

func (b *claudeWebExtensionBridge) pair(token string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if token == "" || token != b.token {
		return false
	}
	b.paired = true
	return true
}

func (b *claudeWebExtensionBridge) poll(token string) (*claudeWebExtensionCommand, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if token == "" || token != b.token {
		return nil, false
	}
	b.paired = true
	if len(b.queue) == 0 {
		return nil, true
	}
	command := b.queue[0]
	b.queue = append([]claudeWebExtensionCommand(nil), b.queue[1:]...)
	return &command, true
}

func (b *claudeWebExtensionBridge) accept(token string, result claudeWebExtensionResult) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if token == "" || token != b.token {
		return false
	}
	waiter, ok := b.waiters[strings.TrimSpace(result.ID)]
	if ok {
		select {
		case waiter <- result:
		default:
		}
	}
	return true
}

// registerClaudeWebUIRelayRoutes is intentionally same-origin only. The Chrome
// extension never receives access to TL Studio's file, Terminal, Permission,
// Session, or Tool endpoints. The local UI relays only probe/complete commands
// protected by the per-pair random token.
func registerClaudeWebUIRelayRoutes(mux *http.ServeMux, bridge *claudeWebExtensionBridge) {
	mux.HandleFunc("POST /local/claude-web-ui/pair", func(w http.ResponseWriter, r *http.Request) {
		if !bridge.pair(strings.TrimSpace(r.URL.Query().Get("token"))) {
			writeJSON(w, http.StatusUnauthorized, jsonError{Error: "invalid Claude Web bridge token"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"paired": true})
	})
	mux.HandleFunc("GET /local/claude-web-ui/poll", func(w http.ResponseWriter, r *http.Request) {
		command, ok := bridge.poll(strings.TrimSpace(r.URL.Query().Get("token")))
		if !ok {
			writeJSON(w, http.StatusUnauthorized, jsonError{Error: "invalid Claude Web bridge token"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"command": command})
	})
	mux.HandleFunc("POST /local/claude-web-ui/result", func(w http.ResponseWriter, r *http.Request) {
		var result claudeWebExtensionResult
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<20)).Decode(&result); err != nil {
			writeJSON(w, http.StatusBadRequest, jsonError{Error: "invalid Claude Web bridge result"})
			return
		}
		if !bridge.accept(strings.TrimSpace(r.URL.Query().Get("token")), result) {
			writeJSON(w, http.StatusUnauthorized, jsonError{Error: "invalid Claude Web bridge token"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"accepted": true})
	})
}
