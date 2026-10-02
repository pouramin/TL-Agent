package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

const claudeWebExtensionBridgePath = "/local/claude-web-extension/"

var errClaudeWebExtensionNotPaired = errors.New("Claude Web Chrome extension is not paired")

type claudeWebProbe struct {
	Connected        bool   `json:"connected"`
	Status           int    `json:"status,omitempty"`
	OrganizationID   string `json:"organizationId,omitempty"`
	OrganizationName string `json:"organizationName,omitempty"`
	Error            string `json:"error,omitempty"`
}

type claudeWebTransport interface {
	Available() error
	OpenLogin(context.Context) error
	Probe(context.Context) (claudeWebProbe, error)
	Complete(context.Context, string) (string, error)
	Close(context.Context) error
}

type claudeWebExtensionCommand struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Prompt string `json:"prompt,omitempty"`
}

type claudeWebExtensionResult struct {
	ID        string `json:"id"`
	OK        bool   `json:"ok"`
	Connected bool   `json:"connected,omitempty"`
	Text      string `json:"text,omitempty"`
	Error     string `json:"error,omitempty"`
}

type claudeWebExtensionBridge struct {
	state *appState
	mu sync.Mutex
	token string
	paired bool
	queue []claudeWebExtensionCommand
	waiters map[string]chan claudeWebExtensionResult
}

func newClaudeWebExtensionBridge(state *appState) *claudeWebExtensionBridge {
	return &claudeWebExtensionBridge{state: state, waiters: map[string]chan claudeWebExtensionResult{}}
}

func (b *claudeWebExtensionBridge) Available() error {
	if b == nil || b.state == nil { return errors.New("Claude Web extension bridge is unavailable") }
	return nil
}

func (b *claudeWebExtensionBridge) frontendURL() string {
	b.state.mu.RLock()
	defer b.state.mu.RUnlock()
	return strings.TrimRight(strings.TrimSpace(b.state.frontendURL), "/")
}

func (b *claudeWebExtensionBridge) resetLocked(token string) {
	for id, waiter := range b.waiters {
		delete(b.waiters, id)
		select { case waiter <- claudeWebExtensionResult{ID:id, Error:"Claude Web bridge session replaced"}: default: }
	}
	b.token = token
	b.paired = false
	b.queue = nil
}

func (b *claudeWebExtensionBridge) OpenLogin(ctx context.Context) error {
	if err := b.Available(); err != nil { return err }
	origin := b.frontendURL()
	if origin == "" { return errors.New("TL Studio local URL is unavailable") }
	token, err := randomSecret(32)
	if err != nil { return err }
	b.mu.Lock()
	b.resetLocked(token)
	b.mu.Unlock()

	values := url.Values{}
	values.Set("tlstudio_pair", token)
	values.Set("tlstudio_origin", origin)
	if err := openBrowser("https://claude.ai/#" + values.Encode()); err != nil {
		return fmt.Errorf("open Claude in the default browser: %w", err)
	}
	select { case <-ctx.Done(): return ctx.Err(); default: return nil }
}

func (b *claudeWebExtensionBridge) Probe(ctx context.Context) (claudeWebProbe, error) {
	b.mu.Lock(); paired := b.paired; b.mu.Unlock()
	if !paired { return claudeWebProbe{}, nil }
	result, err := b.send(ctx, claudeWebExtensionCommand{Kind:"probe"})
	if err != nil { return claudeWebProbe{}, err }
	if !result.OK { return claudeWebProbe{}, errors.New(strings.TrimSpace(result.Error)) }
	return claudeWebProbe{Connected:result.Connected, Status:200}, nil
}

func (b *claudeWebExtensionBridge) Complete(ctx context.Context, prompt string) (string, error) {
	result, err := b.send(ctx, claudeWebExtensionCommand{Kind:"complete", Prompt:prompt})
	if err != nil { return "", err }
	if !result.OK {
		detail := strings.TrimSpace(result.Error)
		if detail == "" { detail = "Claude Web request failed" }
		return "", errors.New(detail)
	}
	if strings.TrimSpace(result.Text) == "" { return "", errors.New("Claude Web returned an empty response") }
	return result.Text, nil
}

func (b *claudeWebExtensionBridge) Close(context.Context) error {
	if b == nil { return nil }
	b.mu.Lock(); b.resetLocked(""); b.mu.Unlock()
	return nil
}

func (b *claudeWebExtensionBridge) send(ctx context.Context, command claudeWebExtensionCommand) (claudeWebExtensionResult, error) {
	id, err := randomSecret(12)
	if err != nil { return claudeWebExtensionResult{}, err }
	command.ID = id
	waiter := make(chan claudeWebExtensionResult, 1)
	b.mu.Lock()
	if b.token == "" || !b.paired { b.mu.Unlock(); return claudeWebExtensionResult{}, errClaudeWebExtensionNotPaired }
	b.waiters[id] = waiter
	b.queue = append(b.queue, command)
	b.mu.Unlock()
	defer func(){ b.mu.Lock(); delete(b.waiters,id); b.mu.Unlock() }()
	select {
	case <-ctx.Done(): return claudeWebExtensionResult{}, ctx.Err()
	case result := <-waiter: return result, nil
	}
}

func (b *claudeWebExtensionBridge) poll(token string) (*claudeWebExtensionCommand, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if token == "" || token != b.token { return nil, false }
	b.paired = true
	if len(b.queue)==0 { return nil, true }
	command := b.queue[0]
	b.queue = append([]claudeWebExtensionCommand(nil), b.queue[1:]...)
	return &command, true
}

func (b *claudeWebExtensionBridge) accept(token string, result claudeWebExtensionResult) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if token == "" || token != b.token { return false }
	waiter, ok := b.waiters[strings.TrimSpace(result.ID)]
	if ok { select { case waiter <- result: default: } }
	return true
}

func claudeWebExtensionOriginAllowed(origin string) bool {
	u, err := url.Parse(strings.TrimSpace(origin))
	return err == nil && strings.EqualFold(u.Scheme,"chrome-extension") && u.Host != ""
}

func isClaudeWebExtensionBridgeRequest(r *http.Request) bool {
	return r != nil && strings.HasPrefix(r.URL.Path, claudeWebExtensionBridgePath)
}

func claudeWebExtensionCORS(w http.ResponseWriter, r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if !claudeWebExtensionOriginAllowed(origin) { return false }
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Vary", "Origin")
	return true
}

func registerClaudeWebExtensionRoutes(mux *http.ServeMux, bridge *claudeWebExtensionBridge) {
	mux.HandleFunc("OPTIONS /local/claude-web-extension/poll", func(w http.ResponseWriter, r *http.Request) {
		if !claudeWebExtensionCORS(w,r) { http.Error(w,"extension origin required",http.StatusForbidden); return }
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /local/claude-web-extension/poll", func(w http.ResponseWriter, r *http.Request) {
		if !claudeWebExtensionCORS(w,r) { http.Error(w,"extension origin required",http.StatusForbidden); return }
		command, ok := bridge.poll(strings.TrimSpace(r.URL.Query().Get("token")))
		if !ok { writeJSON(w,http.StatusUnauthorized,jsonError{Error:"invalid Claude Web bridge token"}); return }
		writeJSON(w,http.StatusOK,map[string]any{"command":command})
	})
	mux.HandleFunc("OPTIONS /local/claude-web-extension/result", func(w http.ResponseWriter, r *http.Request) {
		if !claudeWebExtensionCORS(w,r) { http.Error(w,"extension origin required",http.StatusForbidden); return }
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /local/claude-web-extension/result", func(w http.ResponseWriter, r *http.Request) {
		if !claudeWebExtensionCORS(w,r) { http.Error(w,"extension origin required",http.StatusForbidden); return }
		var result claudeWebExtensionResult
		if err := json.NewDecoder(http.MaxBytesReader(w,r.Body,8<<20)).Decode(&result); err != nil {
			writeJSON(w,http.StatusBadRequest,jsonError{Error:"invalid Claude Web bridge result"}); return
		}
		if !bridge.accept(strings.TrimSpace(r.URL.Query().Get("token")),result) {
			writeJSON(w,http.StatusUnauthorized,jsonError{Error:"invalid Claude Web bridge token"}); return
		}
		writeJSON(w,http.StatusOK,map[string]any{"accepted":true})
	})
}
