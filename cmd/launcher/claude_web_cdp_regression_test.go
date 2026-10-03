package main

import (
	"context"
	"strings"
	"testing"
)

func TestClaudeWebBrowserArgsKeepCDPLoopbackAndHeadless(t *testing.T) {
	headless := strings.Join(claudeWebBrowserArgs(`C:\\runtime`, "Profile 2", false), " ")
	for _, required := range []string{
		`--user-data-dir=C:\\runtime`,
		`--profile-directory=Profile 2`,
		`--remote-debugging-address=127.0.0.1`,
		`--remote-debugging-port=0`,
		`--headless=new`,
	} {
		if !strings.Contains(headless, required) {
			t.Fatalf("headless Claude Web browser args missing %q: %s", required, headless)
		}
	}
	if strings.Contains(headless, "--new-window") {
		t.Fatalf("normal Claude Web runtime must stay headless: %s", headless)
	}

	visible := strings.Join(claudeWebBrowserArgs(`C:\\session`, "Profile 2", true), " ")
	if !strings.Contains(visible, "--new-window") || strings.Contains(visible, "--headless=new") {
		t.Fatalf("login browser args are not isolated to visible authentication: %s", visible)
	}
}

func TestClaudeWebDevToolsRejectsNonLoopbackWebSocket(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := openLocalWebSocket(ctx, "ws://example.com/devtools/browser/test"); err == nil || !strings.Contains(err.Error(), "non-loopback") {
		t.Fatalf("non-loopback DevTools endpoint was not rejected: %v", err)
	}
}

func TestClaudeWebCompletionFlowCreatesStreamsDeletesAndStopsEarly(t *testing.T) {
	source := readRepoText(t, "cmd/launcher/claude_web_browser.go")
	for _, required := range []string{
		`/api/organizations`,
		`/chat_conversations`,
		`/completion`,
		`method:"DELETE"`,
		`credentials:"include"`,
		`event.type === "message_stop"`,
		`event.type === "completion_stop"`,
		`stopReason === "stop_sequence"`,
		`stopReason === "end_turn"`,
		`stopReason === "max_tokens"`,
		`await reader.cancel()`,
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("Claude Web completion regression contract missing %q", required)
		}
	}
}

func TestClaudeWebProductionTransportHasNoExtensionUIAutomationOrPlaintextCookieExtraction(t *testing.T) {
	source := strings.Join([]string{
		readRepoText(t, "cmd/launcher/claude_web_browser.go"),
		readRepoText(t, "cmd/launcher/claude_web_profile.go"),
		readRepoText(t, "cmd/launcher/claude_web_native_windows.go"),
	}, "\n")
	for _, forbidden := range []string{
		"claude-web-extension",
		"chrome.runtime",
		"content_script",
		"service_worker bridge",
		"UIAutomationClient",
		"UIAutomationTypes",
		"powershell.exe",
		"SendKeys",
		"Clipboard",
		"user32.dll",
		"EnumWindows",
		"GetWindowText",
		"Network.getAllCookies",
		"Storage.getCookies",
		"document.cookie",
		"CryptUnprotectData",
		"sessionKey",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("Claude Web production transport contains forbidden dependency %q", forbidden)
		}
	}
}

func TestClaudeWebNativeTransportUsesEphemeralRuntimeClone(t *testing.T) {
	source := readRepoText(t, "cmd/launcher/claude_web_native_windows.go")
	for _, required := range []string{
		`claudeWebRuntimeRootDirectory()`,
		`os.MkdirTemp(runtimeRoot, "run-")`,
		`cloneClaudeWebProfile(ctx, sessionDir, runtimeDir, profileName)`,
		`defer t.cleanupRuntime(context.Background())`,
		`os.RemoveAll(dir)`,
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("Claue Web temporary runtime lifecycle missing %q", required)
		}
	}
}
