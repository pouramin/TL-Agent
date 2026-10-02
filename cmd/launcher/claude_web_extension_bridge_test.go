package main

import (
	"context"
	"net/http"
	"strings"
	"net/http/httptest"
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
	if err != nil { t.Fatal(err) }
	if probe.Connected { t.Fatalf("unexpected connected probe: %#v", probe) }
}


func TestClaudeWebExtensionClosesOnlyPairTabAndUsesInactiveCompletionTabs(t *testing.T) {
	source := readRepoText(t, "integrations/claude-web-extension/background.js")
	for _, required := range []string{
		"pairTabId: Number.isInteger(pairTabId) ? pairTabId : null",
		"sender?.tab?.id",
		"const pairTabId = pair.pairTabId",
		"await chrome.tabs.remove(pairTabId)",
		"chrome.tabs.create({ url: \"https://claude.ai/new\", active: false })",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("Claude Web invisible browser transport contract missing %q", required)
		}
	}
	if strings.Contains(source, "chrome.tabs.remove(tab.id)") && !strings.Contains(source, "finally") {
		t.Fatal("completion tab cleanup contract changed unexpectedly")
	}
}
