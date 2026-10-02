package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func writeFakeClaudeCLI(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake Claude CLI fixture uses a POSIX executable script")
	}
	path := filepath.Join(t.TempDir(), "fake-claude.py")
	script := `#!/usr/bin/env python3
import json
import os
import sys
import time

args = sys.argv[1:]
config_dir = os.environ.get("CLAUDE_CONFIG_DIR", "")
if not config_dir:
    print("missing CLAUDE_CONFIG_DIR", file=sys.stderr)
    sys.exit(90)
os.makedirs(config_dir, exist_ok=True)
auth_file = os.path.join(config_dir, "fake-auth.json")

def account_payload(logged_in):
    if not logged_in:
        return {"loggedIn": False, "authMethod": "none", "apiProvider": "firstParty"}
    return {
        "loggedIn": True,
        "authMethod": "claude.ai",
        "apiProvider": "firstParty",
        "email": "tester@example.com",
        "orgId": "org_fake",
        "orgName": "Test Org",
        "subscriptionType": "max",
    }

if len(args) >= 2 and args[0] == "auth" and args[1] == "status":
    logged_in = os.path.exists(auth_file)
    print(json.dumps(account_payload(logged_in)))
    sys.exit(0 if logged_in else 1)

if len(args) >= 2 and args[0] == "auth" and args[1] == "login":
    if os.environ.get("FAKE_CLAUDE_LOGIN_HANG") == "1":
        time.sleep(30)
        sys.exit(2)
    time.sleep(float(os.environ.get("FAKE_CLAUDE_LOGIN_DELAY", "0.03")))
    with open(auth_file, "w", encoding="utf-8") as handle:
        json.dump(account_payload(True), handle)
    print("Login successful")
    sys.exit(0)

if len(args) >= 2 and args[0] == "auth" and args[1] == "logout":
    try:
        os.remove(auth_file)
    except FileNotFoundError:
        pass
    print("Logged out")
    sys.exit(0)

if "-p" in args or "--print" in args:
    capture = os.environ.get("FAKE_CLAUDE_ARGS_FILE")
    if capture:
        with open(capture, "w", encoding="utf-8") as handle:
            json.dump(args, handle)
    env_capture = os.environ.get("FAKE_CLAUDE_ENV_FILE")
    if env_capture:
        keys = [
            "CLAUDE_CONFIG_DIR",
            "ANTHROPIC_API_KEY",
            "ANTHROPIC_AUTH_TOKEN",
            "ANTHROPIC_BASE_URL",
            "ANTHROPIC_PROFILE",
            "CLAUDE_CODE_OAUTH_TOKEN",
            "CLAUDE_CODE_USE_BEDROCK",
            "CLAUDE_CODE_USE_VERTEX",
            "CLAUDE_CODE_USE_FOUNDRY",
        ]
        with open(env_capture, "w", encoding="utf-8") as handle:
            json.dump({key: os.environ.get(key) for key in keys}, handle)
    prompt_capture = os.environ.get("FAKE_CLAUDE_PROMPT_FILE")
    prompt = sys.stdin.read()
    if prompt_capture:
        with open(prompt_capture, "w", encoding="utf-8") as handle:
            handle.write(prompt)
    if not os.path.exists(auth_file):
        print(json.dumps({"type":"result","is_error":True,"result":"not logged in"}))
        sys.exit(1)
    value = json.loads(os.environ.get("FAKE_CLAUDE_OUTPUT", '{"text":"fake response","toolCalls":[]}'))
    print(json.dumps({
        "type": "result",
        "subtype": "success",
        "is_error": False,
        "result": "",
        "session_id": "fake-session",
        "structured_output": value,
    }))
    sys.exit(0)

if "--version" in args or "-v" in args:
    print("2.1.999 (Claude Code)")
    sys.exit(0)

print("unsupported fake claude args: " + repr(args), file=sys.stderr)
sys.exit(3)
`
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func newClaudeTestManager(t *testing.T) (*providerManager, *claudeAccountAdapter, string) {
	t.Helper()
	stateDir := t.TempDir()
	t.Setenv("TL_STUDIO_STATE_DIR", stateDir)
	t.Setenv("TL_STUDIO_CLAUDE_EXECUTABLE", writeFakeClaudeCLI(t))
	manager := newProviderManager(&appState{})
	adapter := newClaudeAccountAdapter(&appState{}, manager)
	manager.registerAccountAdapter(adapter)
	return manager, adapter, stateDir
}

func waitForClaudeConnected(t *testing.T, adapter *claudeAccountAdapter, login providerAccountLogin) providerAccountStatus {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for {
		status, err := adapter.PollLogin(context.Background(), "", login.LoginID)
		if err != nil {
			t.Fatal(err)
		}
		if status.Connected {
			return status
		}
		if status.State == providerAccountErrorState || status.State == providerAccountExpired {
			t.Fatalf("Claude login failed: %#v", status)
		}
		if time.Now().After(deadline) {
			t.Fatalf("Claude login did not complete: %#v", status)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func seedManualClaudeAPI(t *testing.T, manager *providerManager) {
	t.Helper()
	if err := manager.store.put(tlProviderDefinition{
		ID:       claudeAccountProviderID,
		Name:     "Claude",
		Protocol: "anthropic-messages",
		BaseURL:  "https://api.anthropic.com/v1",
		Models: []tlProviderModel{{
			ID: "claude-sonnet-test",
			Name: "Claude Sonnet Test",
			ToolCall: true,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := putProviderCredentialSlot(manager.credentials, claudeAccountProviderID, providerCredentialSlotAPI, "manual-api-secret"); err != nil {
		t.Fatal(err)
	}
}

func TestClaudeAccountBrowserLoginPreservesManualAPIAndSyncsModels(t *testing.T) {
	manager, adapter, _ := newClaudeTestManager(t)
	seedManualClaudeAPI(t, manager)

	status, err := adapter.Status(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if !status.Available || status.Connected {
		t.Fatalf("expected available disconnected Claude account, got %#v", status)
	}
	if status.Setup == nil || !status.Setup.Configurable || !status.Setup.Configured {
		t.Fatalf("expected configured Claude Code setup surface, got %#v", status.Setup)
	}

	login, err := adapter.BeginLogin(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if login.Flow != "claude_browser" || login.LoginID == "" {
		t.Fatalf("unexpected Claude login challenge: %#v", login)
	}
	status = waitForClaudeConnected(t, adapter, login)
	if status.AccountLabel != "tester@example.com" || !strings.Contains(strings.ToLower(status.AccountType), "max") {
		t.Fatalf("unexpected Claude account metadata: %#v", status)
	}
	if len(status.Models) != 3 {
		t.Fatalf("expected Claude account model aliases, got %#v", status.Models)
	}

	accountProvider, ok, err := manager.store.get(claudeAccountRuntimeProviderID)
	if err != nil || !ok {
		t.Fatalf("expected managed Claude account provider: ok=%v err=%v", ok, err)
	}
	if accountProvider.Protocol != claudeAccountProviderProtocol || accountProvider.ManagedBy != "account" {
		t.Fatalf("unexpected Claude account provider definition: %#v", accountProvider)
	}
	manualProvider, ok, err := manager.store.get(claudeAccountProviderID)
	if err != nil || !ok || manualProvider.Protocol != "anthropic-messages" {
		t.Fatalf("manual Claude API provider was overwritten: ok=%v err=%v provider=%#v", ok, err, manualProvider)
	}
	apiSecret, err := getProviderCredentialSlot(manager.credentials, claudeAccountProviderID, providerCredentialSlotAPI)
	if err != nil || apiSecret != "manual-api-secret" {
		t.Fatalf("manual Claude API credential changed: secret=%q err=%v", apiSecret, err)
	}

	accountCredential, err := manager.effectiveCredential(context.Background(), claudeAccountRuntimeProviderID, "")
	if err != nil || accountCredential != "official-claude-account" {
		t.Fatalf("unexpected Claude account transport sentinel %q err=%v", accountCredential, err)
	}
	apiCredential, err := manager.effectiveCredential(context.Background(), claudeAccountProviderID, "")
	if err != nil || apiCredential != "manual-api-secret" {
		t.Fatalf("manual Claude API resolution changed %q err=%v", apiCredential, err)
	}

	registryBytes, err := os.ReadFile(providerRegistryPath())
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(registryBytes))
	for _, forbidden := range []string{"oauth", "access_token", "refresh_token", "manual-api-secret", "cookie"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("provider registry leaked forbidden auth material %q: %s", forbidden, lower)
		}
	}
}

func TestClaudeAccountBridgeReturnsTLStudioToolCallAndLocksDownClaudeTools(t *testing.T) {
	_, adapter, stateDir := newClaudeTestManager(t)
	authDir := claudeConfigDirectory()
	if err := os.MkdirAll(authDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(authDir, "fake-auth.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	argsFile := filepath.Join(t.TempDir(), "args.json")
	envFile := filepath.Join(t.TempDir(), "env.json")
	promptFile := filepath.Join(t.TempDir(), "prompt.txt")
	t.Setenv("FAKE_CLAUDE_ARGS_FILE", argsFile)
	t.Setenv("FAKE_CLAUDE_ENV_FILE", envFile)
	t.Setenv("FAKE_CLAUDE_PROMPT_FILE", promptFile)
	t.Setenv("FAKE_CLAUDE_OUTPUT", `{"text":"","toolCalls":[{"name":"files.read","arguments":"{\"path\":\"README.md\"}"}]}`)
	t.Setenv("ANTHROPIC_API_KEY", "must-not-leak")
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "must-not-leak")
	t.Setenv("ANTHROPIC_BASE_URL", "https://must-not-leak.invalid")
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "must-not-leak")
	t.Setenv("CLAUDE_CODE_USE_BEDROCK", "1")

	client := newNativeModelClient(adapter)
	response, err := client.Complete(context.Background(), nativeModelRequest{
		System: "system",
		Provider: tlProviderDefinition{
			ID: claudeAccountRuntimeProviderID,
			Name: "Claude / Account",
			Protocol: claudeAccountProviderProtocol,
			BaseURL: claudeAccountBaseURL,
			ManagedBy: "account",
		},
		Model: tlProviderModel{ID: "sonnet", Name: "Claude Sonnet", ToolCall: true},
		APIKey: "official-claude-account",
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
	if response.FinishReason != "tool_calls" || len(response.ToolCalls) != 1 || response.ToolCalls[0].Name != "files.read" {
		t.Fatalf("expected one TL Studio file tool call, got %#v", response)
	}

	var args []string
	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &args); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, "\n")
	for _, required := range []string{
		"-p",
		"--safe-mode",
		"--no-session-persistence",
		"--disable-slash-commands",
		"--tools",
		"--disallowedTools",
		"mcp__*",
		"--permission-prompts",
		"none",
		"--no-chrome",
		"--strict-mcp-config",
		"--output-format",
		"json",
		"--json-schema",
		"--model",
		"sonnet",
		"--system-prompt-file",
	} {
		if !strings.Contains(joined, required) {
			t.Fatalf("Claude bridge missing safety/runtime flag %q: %#v", required, args)
		}
	}
	if strings.Contains(joined, "--bare") {
		t.Fatalf("Claude subscription bridge must not use --bare because bare mode ignores subscription OAuth credentials: %#v", args)
	}

	var env map[string]*string
	envData, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(envData, &env); err != nil {
		t.Fatal(err)
	}
	configDir := ""
	if env["CLAUDE_CONFIG_DIR"] != nil {
		configDir = *env["CLAUDE_CONFIG_DIR"]
	}
	if configDir == "" || !strings.HasPrefix(filepath.Clean(configDir), filepath.Clean(stateDir)) {
		t.Fatalf("Claude credentials were not isolated under TL Studio state: %#v", env)
	}
	for _, key := range []string{"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_BASE_URL", "CLAUDE_CODE_OAUTH_TOKEN", "CLAUDE_CODE_USE_BEDROCK"} {
		if env[key] != nil {
			t.Fatalf("Claude account bridge inherited forbidden credential source %s=%q", key, *env[key])
		}
	}
	promptData, err := os.ReadFile(promptFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(promptData), `"id": "sonnet"`) || !strings.Contains(string(promptData), `"provider": "claude"`) {
		t.Fatalf("Claude bridge prompt missing selected model identity: %s", promptData)
	}
}

func TestClaudeAccountRestartAndDisconnectPersistence(t *testing.T) {
	manager, adapter, stateDir := newClaudeTestManager(t)
	seedManualClaudeAPI(t, manager)
	login, err := adapter.BeginLogin(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	_ = waitForClaudeConnected(t, adapter, login)

	// Simulate a full TL Studio restart against the same persisted state.
	t.Setenv("TL_STUDIO_STATE_DIR", stateDir)
	restartedManager := newProviderManager(&appState{})
	restarted := newClaudeAccountAdapter(&appState{}, restartedManager)
	restartedManager.registerAccountAdapter(restarted)
	status, err := restarted.Status(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if !status.Connected {
		t.Fatalf("Claude account did not survive restart: %#v", status)
	}

	if err := restarted.Disconnect(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	status, err = restarted.Status(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if status.Connected {
		t.Fatalf("Claude account remained connected after logout: %#v", status)
	}
	if _, found, err := restartedManager.store.get(claudeAccountRuntimeProviderID); err != nil || found {
		t.Fatalf("Claude account provider remained after logout: found=%v err=%v", found, err)
	}
	manualProvider, found, err := restartedManager.store.get(claudeAccountProviderID)
	if err != nil || !found || manualProvider.Protocol != "anthropic-messages" {
		t.Fatalf("manual Claude API provider was removed on account logout: found=%v err=%v provider=%#v", found, err, manualProvider)
	}
	apiSecret, err := getProviderCredentialSlot(restartedManager.credentials, claudeAccountProviderID, providerCredentialSlotAPI)
	if err != nil || apiSecret != "manual-api-secret" {
		t.Fatalf("manual Claude API credential was removed on account logout: %q err=%v", apiSecret, err)
	}

	// A second restart must remain signed out.
	finalManager := newProviderManager(&appState{})
	finalAdapter := newClaudeAccountAdapter(&appState{}, finalManager)
	finalManager.registerAccountAdapter(finalAdapter)
	finalStatus, err := finalAdapter.Status(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if finalStatus.Connected {
		t.Fatalf("Claude logout did not persist across restart: %#v", finalStatus)
	}
}

func TestClaudeCancelLoginStopsOfficialHelper(t *testing.T) {
	_, adapter, _ := newClaudeTestManager(t)
	t.Setenv("FAKE_CLAUDE_LOGIN_HANG", "1")
	login, err := adapter.BeginLogin(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if err := adapter.CancelLogin(context.Background(), "", login.LoginID); err != nil {
		t.Fatal(err)
	}
	time.Sleep(30 * time.Millisecond)
	adapter.mu.Lock()
	_, exists := adapter.logins[login.LoginID]
	adapter.mu.Unlock()
	if exists {
		t.Fatal("cancelled Claude login transaction remained registered")
	}
}

func TestClaudeCredentialEnvironmentScrubsAlternateAuthSources(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "x")
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "x")
	t.Setenv("ANTHROPIC_BASE_URL", "https://example.invalid")
	t.Setenv("ANTHROPIC_PROFILE", "profile")
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "x")
	t.Setenv("CLAUDE_CODE_USE_BEDROCK", "1")
	t.Setenv("CLAUDE_CODE_USE_VERTEX", "1")
	t.Setenv("CLAUDE_CODE_USE_FOUNDRY", "1")
	t.Setenv("TL_STUDIO_STATE_DIR", t.TempDir())
	env, err := claudeCredentialSafeEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	joined := "\n" + strings.Join(env, "\n") + "\n"
	for _, key := range []string{
		"ANTHROPIC_API_KEY=",
		"ANTHROPIC_AUTH_TOKEN=",
		"ANTHROPIC_BASE_URL=",
		"ANTHROPIC_PROFILE=",
		"CLAUDE_CODE_OAUTH_TOKEN=",
		"CLAUDE_CODE_USE_BEDROCK=",
		"CLAUDE_CODE_USE_VERTEX=",
		"CLAUDE_CODE_USE_FOUNDRY=",
	} {
		if strings.Contains(joined, "\n"+key) {
			t.Fatalf("forbidden Claude credential source survived environment scrub: %s", key)
		}
	}
}

func TestClaudeAuthStatusExitOneMeansSignedOut(t *testing.T) {
	_, adapter, _ := newClaudeTestManager(t)
	command, err := adapter.resolveCommand()
	if err != nil {
		t.Fatal(err)
	}
	payload, connected, err := adapter.authStatus(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if connected || payload.LoggedIn {
		t.Fatalf("expected fake Claude account to start signed out: %#v", payload)
	}
}

func TestClaudeRuntimeProviderAliasIsRegistered(t *testing.T) {
	manager, adapter, _ := newClaudeTestManager(t)
	if got := manager.accountAdapter(claudeAccountRuntimeProviderID); got != adapter {
		t.Fatalf("Claude runtime provider alias did not resolve account adapter: %#v", got)
	}
	if got := manager.accountAdapter(claudeAccountProviderID); got != nil {
		t.Fatalf("manual Claude API provider must not be intercepted by account adapter: %#v", got)
	}
}

func TestClaudeManualAPICredentialRemainsWhenAccountCredentialMissing(t *testing.T) {
	manager, _, _ := newClaudeTestManager(t)
	seedManualClaudeAPI(t, manager)
	value, err := manager.effectiveCredential(context.Background(), claudeAccountProviderID, "")
	if err != nil {
		t.Fatal(err)
	}
	if value != "manual-api-secret" {
		t.Fatalf("unexpected manual Claude API credential %q", value)
	}
	if _, err := manager.effectiveCredential(context.Background(), claudeAccountRuntimeProviderID, ""); !errors.Is(err, errCredentialNotFound) {
		t.Fatalf("disconnected Claude account must not yield runtime credential, got %v", err)
	}
}


func TestRealClaudeCLIContractSmoke(t *testing.T) {
	if os.Getenv("TL_STUDIO_REAL_CLAUDE_SMOKE") != "1" {
		t.Skip("real Claude Code smoke is opt-in")
	}
	t.Setenv("TL_STUDIO_STATE_DIR", t.TempDir())
	adapter := newClaudeAccountAdapter(&appState{}, newProviderManager(&appState{}))
	command, err := adapter.resolveCommand()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := claudeProcess(ctx, command, "--version")
	if err := prepareClaudeCommand(cmd); err != nil {
		t.Fatal(err)
	}
	var output boundedTextBuffer
	output.max = 16 << 10
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		t.Fatalf("official Claude Code CLI did not start: %v — %s", err, output.String())
	}
	if strings.TrimSpace(output.String()) == "" {
		t.Fatal("official Claude Code --version returned no output")
	}

	statusCtx, statusCancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer statusCancel()
	_, _, err = adapter.authStatus(statusCtx, command)
	if err != nil {
		t.Fatalf("official Claude auth status contract failed: %v", err)
	}
}
