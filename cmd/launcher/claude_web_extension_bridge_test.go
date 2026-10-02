package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClaudeWebExtensionBridgeRejectsNonExtensionOrigin(t *testing.T) {
	bridge := newClaudeWebExtensionBridge(&appState{frontendURL: "http://127.0.0.1:32123"})
	bridge.token = "pair-token"

	mux := http.NewServeMux()
	registerClaudeWebExtensionRoutes(mux, bridge)

	req := httptest.NewRequest(http.MethodGet, "/local/claude-web-extension/poll?token=pair-token", nil)
	req.Header.Set("Origin", "https://claude.ai")
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, req)
	if res.Code != http.StatusForbidden {
		t.Fatalf("non-extension origin returned %d", res.Code)
	}
}

func TestClaudeWebExtensionBridgeProbeRoundTrip(t *testing.T) {
	bridge := newClaudeWebExtensionBridge(&appState{frontendURL: "http://127.0.0.1:32123"})
	bridge.token = "pair-token"
	bridge.paired = true

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result := make(chan claudeWebProbe, 1)
	fail := make(chan error, 1)
	go func() {
		probe, err := bridge.Probe(ctx)
		if err != nil { fail <- err; return }
		result <- probe
	}()

	deadline := time.Now().Add(time.Second)
	var command *claudeWebExtensionCommand
	for time.Now().Before(deadline) {
		bridge.mu.Lock()
		if len(bridge.queue) > 0 {
			item := bridge.queue[0]
			bridge.queue = bridge.queue[1:]
			command = &item
		}
		bridge.mu.Unlock()
		if command != nil { break }
		time.Sleep(10 * time.Millisecond)
	}
	if command == nil || command.Kind != "probe" {
		t.Fatalf("probe command was not queued: %#v", command)
	}
	if !bridge.accept("pair-token", claudeWebExtensionResult{ID: command.ID, OK: true, Connected: true}) {
		t.Fatal("bridge rejected valid result")
	}

	select {
	case err := <-fail:
		t.Fatal(err)
	case probe := <-result:
		if !probe.Connected { t.Fatalf("unexpected probe: %#v", probe) }
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}

func TestClaudeWebExtensionBridgeStartsDisconnectedUntilPaired(t *testing.T) {
	bridge := newClaudeWebExtensionBridge(&appState{frontendURL: "http://127.0.0.1:32123"})
	probe, err := bridge.Probe(context.Background())
	if !errors.Is(err, errClaudeWebExtensionNotPaired) {
		t.Fatalf("expected unpaired bridge error, got probe=%#v err=%v", probe, err)
	}
}


func TestClaudeWebExtensionUsesTablessBackgroundTransport(t *testing.T) {
	source := readRepoText(t, "integrations/claude-web-extension/background.js")
	for _, required := range []string{
		"chrome.cookies.getAll",
		"chrome.declarativeNetRequest.updateSessionRules",
		"https://claude.ai/api",
		"claude-sonnet-5-5",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("Claude Web tabless transport contract missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"chrome.tabs.create",
		"chrome.windows.create",
		"chrome.tabs.remove",
		"chrome.tabs.sendMessage",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("Claude Web must stay inside TL Studio; background transport contains %q", forbidden)
		}
	}
}


func TestClaudeWebExtensionUsesExternallyConnectableDirectPairing(t *testing.T) {
	manifest := readRepoText(t, "integrations/claude-web-extension/manifest.json")
	background := readRepoText(t, "integrations/claude-web-extension/background.js")
	accounts := readBrowserSource(t, "provider-account-ui.ts")

	for _, required := range []string{
		`"version": "0.5.2"`,
		`"key":`,
		`"externally_connectable"`,
		`"http://127.0.0.1/*"`,
		`"http://localhost/*"`,
	} {
		if !strings.Contains(manifest, required) {
			t.Fatalf("Claude Web externally-connectable manifest missing %q", required)
		}
	}
	if strings.Contains(manifest, `"content_scripts"`) || strings.Contains(manifest, `"scripting"`) {
		t.Fatal("Claude Web direct pairing must not require injected content scripts")
	}
	for _, required := range []string{
		`chrome.runtime.onMessageExternal.addListener`,
		`tlstudio-ping`,
		`tlstudio-pair-direct`,
		`validExternalSender`,
		`tlstudio-execute-direct`,
	} {
		if !strings.Contains(background, required) {
			t.Fatalf("Claude Web external pairing background missing %q", required)
		}
	}
	if !strings.Contains(accounts, `CLAUDE_WEB_EXTENSION_ID = "fpphidfmpfiibpbloeecegdlecfbhcla"`) {
		t.Fatal("TL Studio must target the fixed Claude Web extension ID")
	}
}


func TestClaudeWebUIRelayPairsPollsAndReturnsResult(t *testing.T) {
	bridge := newClaudeWebExtensionBridge(&appState{frontendURL: "http://127.0.0.1:32123"})
	bridge.token = "pair-token"

	mux := http.NewServeMux()
	registerClaudeWebUIRelayRoutes(mux, bridge)

	pairReq := httptest.NewRequest(http.MethodPost, "/local/claude-web-ui/pair?token=pair-token", nil)
	pairRes := httptest.NewRecorder()
	mux.ServeHTTP(pairRes, pairReq)
	if pairRes.Code != http.StatusOK {
		t.Fatalf("pair returned HTTP %d: %s", pairRes.Code, pairRes.Body.String())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := bridge.Complete(ctx, "hello")
		done <- err
	}()

	deadline := time.Now().Add(time.Second)
	var command claudeWebExtensionCommand
	for time.Now().Before(deadline) {
		req := httptest.NewRequest(http.MethodGet, "/local/claude-web-ui/poll?token=pair-token", nil)
		res := httptest.NewRecorder()
		mux.ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("poll returned HTTP %d: %s", res.Code, res.Body.String())
		}
		var payload struct {
			Command *claudeWebExtensionCommand `json:"command"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Command != nil {
			command = *payload.Command
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if command.ID == "" || command.Kind != "complete" {
		t.Fatalf("completion command was not relayed: %#v", command)
	}

	body, err := json.Marshal(claudeWebExtensionResult{
		ID: command.ID,
		OK: true,
		Text: "done",
	})
	if err != nil {
		t.Fatal(err)
	}
	resultReq := httptest.NewRequest(
		http.MethodPost,
		"/local/claude-web-ui/result?token=pair-token",
		bytes.NewReader(body),
	)
	resultRes := httptest.NewRecorder()
	mux.ServeHTTP(resultRes, resultReq)
	if resultRes.Code != http.StatusOK {
		t.Fatalf("result returned HTTP %d: %s", resultRes.Code, resultRes.Body.String())
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}


func TestClaudeWebSSEParserReturnsOnTerminalEvent(t *testing.T) {
	source := readRepoText(t, "integrations/claude-web-extension/background.js")
	for _, required := range []string{
		`typeof event.completion === "string"`,
		`event.type === "message_stop"`,
		`stopReason === "stop_sequence"`,
		`stopReason === "end_turn"`,
		`await reader.cancel()`,
		`if (consumeEvent()) return await finish()`,
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("Claude Web SSE completion contract missing %q", required)
		}
	}
	if strings.Contains(source, `event.type === "completion" && typeof event.completion`) {
		t.Fatal("Claude Web parser must accept legacy completion payloads without requiring type=completion")
	}
}
