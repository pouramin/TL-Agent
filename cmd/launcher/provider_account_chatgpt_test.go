package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func writeFakeCodexCLI(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake Codex CLI fixture uses a POSIX executable script")
	}
	path := filepath.Join(t.TempDir(), "fake-codex.py")
	script := `#!/usr/bin/env python3
import json
import os
import sys

def send(value):
    sys.stdout.write(json.dumps(value) + "\n")
    sys.stdout.flush()

args = sys.argv[1:]
if args and args[0] == "app-server":
    for line in sys.stdin:
        try:
            msg = json.loads(line)
        except Exception:
            continue
        method = msg.get("method")
        req_id = msg.get("id")
        if method == "initialize":
            params = msg.get("params")
            if not isinstance(params, dict) or "clientInfo" not in params or "capabilities" not in params:
                send({"id": req_id, "error": {"code": -32602, "message": "initialize requires clientInfo and capabilities"}})
                continue
            send({"id": req_id, "result": {"userAgent": "fake-codex", "codexHome": os.environ.get("CODEX_HOME", ""), "platformFamily": "unix", "platformOs": "linux"}})
        elif method == "initialized":
            pass
        elif method == "account/read":
            send({"id": req_id, "result": {"account": {"type": "chatgpt", "email": "tester@example.com", "planType": "plus"}, "requiresOpenaiAuth": True}})
        elif method == "account/login/start":
            send({"id": req_id, "result": {"type": "chatgpt", "loginId": "codex-login-1", "authUrl": "https://chatgpt.com/fake-login"}})
            send({"method": "account/login/completed", "params": {"loginId": "codex-login-1", "success": True, "error": None, "onboardingEntrypoint": None}})
            send({"method": "account/updated", "params": {"authMode": "chatgpt", "planType": "plus"}})
        elif method == "model/list":
            send({"id": req_id, "result": {"data": [
                {"id": "gpt-5.6-sol", "model": "gpt-5.6-sol", "displayName": "GPT-5.6 Sol", "description": "test", "hidden": False, "supportedReasoningEfforts": [], "defaultReasoningEffort": "medium", "inputModalities": ["text"], "supportsPersonality": False, "multiAgentVersion": None, "additionalSpeedTiers": [], "serviceTiers": [], "defaultServiceTier": None, "availableAccessPrograms": None, "isDefault": True, "upgrade": None, "upgradeInfo": None, "availabilityNux": None, "modelSpecialty": None},
                {"id": "gpt-5.6-luna", "model": "gpt-5.6-luna", "displayName": "GPT-5.6 Luna", "description": "test", "hidden": False, "supportedReasoningEfforts": [], "defaultReasoningEffort": "medium", "inputModalities": ["text"], "supportsPersonality": False, "multiAgentVersion": None, "additionalSpeedTiers": [], "serviceTiers": [], "defaultServiceTier": None, "availableAccessPrograms": None, "isDefault": False, "upgrade": None, "upgradeInfo": None, "availabilityNux": None, "modelSpecialty": None}
            ], "nextCursor": None}})
        elif method == "account/login/cancel":
            send({"id": req_id, "result": {}})
        elif method == "account/logout":
            send({"id": req_id, "result": {}})
        elif req_id is not None:
            send({"id": req_id, "error": {"code": -32601, "message": "unknown fake method"}})
    sys.exit(0)

if args and args[0] == "exec":
    output_path = None
    for i, value in enumerate(args):
        if value in ("-o", "--output-last-message") and i + 1 < len(args):
            output_path = args[i + 1]
    if not output_path:
        sys.exit(2)
    value = os.environ.get("FAKE_CODEX_OUTPUT", '{"text":"fake response","toolCalls":[]}')
    with open(output_path, "w", encoding="utf-8") as handle:
        handle.write(value)
    sys.exit(0)

sys.exit(3)
`
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func newChatGPTTestManager(t *testing.T) (*providerManager, *chatGPTAccountAdapter) {
	t.Helper()
	t.Setenv("TL_STUDIO_STATE_DIR", t.TempDir())
	t.Setenv("TL_STUDIO_CODEX_EXECUTABLE", writeFakeCodexCLI(t))
	manager := newProviderManager(&appState{})
	adapter := newChatGPTAccountAdapter(&appState{}, manager)
	manager.registerAccountAdapter(adapter)
	return manager, adapter
}

func TestChatGPTAccountLoginUsesOfficialCodexAndSyncsModels(t *testing.T) {
	manager, adapter := newChatGPTTestManager(t)
	ctx := context.Background()

	status, err := adapter.Status(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if !status.Available || status.Connected {
		t.Fatalf("expected available disconnected ChatGPT account, got %#v", status)
	}
	if status.Setup == nil || !status.Setup.Configurable || !status.Setup.Configured {
		t.Fatalf("expected configured Codex setup surface, got %#v", status.Setup)
	}

	login, err := adapter.BeginLogin(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if login.Flow != "chatgpt" || login.AuthorizationURL != "https://chatgpt.com/fake-login" {
		t.Fatalf("unexpected ChatGPT login challenge: %#v", login)
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		status, err = adapter.PollLogin(ctx, "", login.LoginID)
		if err != nil {
			t.Fatal(err)
		}
		if status.Connected {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("ChatGPT login did not complete: %#v", status)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if status.AccountLabel != "tester@example.com" || !strings.Contains(strings.ToLower(status.AccountType), "plus") {
		t.Fatalf("unexpected ChatGPT account metadata: %#v", status)
	}
	if len(status.Models) != 2 {
		t.Fatalf("expected Codex model catalog to sync, got %#v", status.Models)
	}

	provider, ok, err := manager.store.get(chatGPTAccountProviderID)
	if err != nil || !ok {
		t.Fatalf("expected managed ChatGPT provider: ok=%v err=%v", ok, err)
	}
	if provider.Protocol != codexChatGPTProviderProtocol || provider.ManagedBy != "account" {
		t.Fatalf("unexpected ChatGPT provider definition: %#v", provider)
	}
	if len(provider.Models) != 2 || provider.Models[0].ID == "" {
		t.Fatalf("unexpected ChatGPT models: %#v", provider.Models)
	}

	credential, err := adapter.ResolveCredential(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if credential != "official-codex-chatgpt-account" {
		t.Fatalf("expected non-secret transport sentinel, got %q", credential)
	}

	registryBytes, err := os.ReadFile(providerRegistryPath())
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(registryBytes))
	for _, forbidden := range []string{"access_token", "refresh_token", "chatgptauthtokens", "cookie"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("provider registry leaked forbidden auth material %q: %s", forbidden, lower)
		}
	}
}

func TestChatGPTCodexBridgeReturnsTLStudioToolCall(t *testing.T) {
	_, adapter := newChatGPTTestManager(t)
	t.Setenv("FAKE_CODEX_OUTPUT", `{"text":"","toolCalls":[{"name":"files.read","arguments":{"path":"README.md"}}]}`)

	client := newNativeModelClient(adapter)
	response, err := client.Complete(context.Background(), nativeModelRequest{
		System: "system",
		Provider: tlProviderDefinition{
			ID: chatGPTAccountProviderID,
			Name: "ChatGPT / Codex",
			Protocol: codexChatGPTProviderProtocol,
			BaseURL: chatGPTAccountBaseURL,
			ManagedBy: "account",
		},
		Model: tlProviderModel{ID: "gpt-5.6-sol", Name: "GPT-5.6 Sol", ToolCall: true},
		APIKey: "official-codex-chatgpt-account",
		Messages: []nativeConversationMessage{{Role: "user", Text: "read the readme"}},
		Tools: []nativeModelToolDefinition{{
			ID: "files.read",
			Name: "Read file",
			Description: "Read a project file",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{"path": map[string]any{"type": "string"}},
			},
		}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if response.FinishReason != "tool_calls" || len(response.ToolCalls) != 1 {
		t.Fatalf("expected one TL Studio tool call, got %#v", response)
	}
	if response.ToolCalls[0].Name != "files.read" {
		t.Fatalf("unexpected tool call: %#v", response.ToolCalls[0])
	}
	var args map[string]any
	if err := json.Unmarshal(response.ToolCalls[0].Arguments, &args); err != nil {
		t.Fatal(err)
	}
	if args["path"] != "README.md" {
		t.Fatalf("unexpected tool arguments: %#v", args)
	}
}

func TestChatGPTAdapterDisconnectRemovesManagedProvider(t *testing.T) {
	manager, adapter := newChatGPTTestManager(t)
	if err := manager.store.put(tlProviderDefinition{
		ID: chatGPTAccountProviderID,
		Name: "ChatGPT / Codex",
		Protocol: codexChatGPTProviderProtocol,
		BaseURL: chatGPTAccountBaseURL,
		ManagedBy: "account",
		Models: []tlProviderModel{{ID: "gpt-5.6-sol", Name: "GPT-5.6 Sol", ToolCall: true}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := saveChatGPTCodexConfig(chatGPTCodexConfig{Email: "tester@example.com", PlanType: "plus"}); err != nil {
		t.Fatal(err)
	}
	if err := adapter.Disconnect(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	if _, found, err := manager.store.get(chatGPTAccountProviderID); err != nil || found {
		t.Fatalf("managed ChatGPT provider should be removed on sign-out: found=%v err=%v", found, err)
	}
	config, err := loadChatGPTCodexConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.Email != "" || config.PlanType != "" {
		t.Fatalf("account metadata must be cleared on sign-out: %#v", config)
	}
}

func TestChatGPTIntegrationAvoidsPrivateAuthSurfaces(t *testing.T) {
	for _, path := range []string{"provider_account_chatgpt.go", "codex_chatgpt_bridge.go"} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		lower := strings.ToLower(string(data))
		for _, forbidden := range []string{
			"chatgptauthtokens",
			"backend-api",
			"session_token",
			"sessiontoken",
			"set-cookie",
			"document.cookie",
		} {
			if strings.Contains(lower, forbidden) {
				t.Fatalf("%s must not use private ChatGPT auth surface %q", path, forbidden)
			}
		}
	}
	adapterSource, err := os.ReadFile("provider_account_chatgpt.go")
	if err != nil {
		t.Fatal(err)
	}
	bridgeSource, err := os.ReadFile("codex_chatgpt_bridge.go")
	if err != nil {
		t.Fatal(err)
	}
	combined := string(adapterSource) + "\n" + string(bridgeSource)
	for _, required := range []string{
		`"account/login/start"`,
		`map[string]any{"type": "chatgpt"}`,
		`"account/read"`,
		`"model/list"`,
		`"account/logout"`,
		`"--ignore-user-config"`,
		`"--ignore-rules"`,
		`"--sandbox", "read-only"`,
		`approval_policy="never"`,
		`web_search="disabled"`,
	} {
		if !strings.Contains(combined, required) {
			t.Fatalf("official Codex account lifecycle missing %q", required)
		}
	}
}


func TestRealCodexAppServerInitializeSmoke(t *testing.T) {
	if os.Getenv("TL_STUDIO_REAL_CODEX_SMOKE") != "1" {
		t.Skip("real Codex smoke is opt-in")
	}
	t.Setenv("TL_STUDIO_STATE_DIR", t.TempDir())
	adapter := newChatGPTAccountAdapter(&appState{}, nil)
	command, err := adapter.resolveCommand()
	if err != nil {
		t.Fatal(err)
	}
	server, err := startCodexAppServer(command)
	if err != nil {
		t.Fatal(err)
	}
	server.Close()
}

func TestWindowsBatchCommandLineKeepsOuterAndExecutableQuotes(t *testing.T) {
	line := windowsBatchCommandLine(
		`C:\Program Files\nodejs\npx.cmd`,
		[]string{"--yes", "@openai/codex", "app-server", "--stdio"},
	)
	if !strings.HasPrefix(line, `""C:\Program Files\nodejs\npx.cmd"`) {
		t.Fatalf("batch command must keep cmd.exe outer and executable quotes: %q", line)
	}
	if !strings.HasSuffix(line, `"--stdio""`) {
		t.Fatalf("batch command must close the outer cmd.exe quote pair: %q", line)
	}
}
