//go:build windows

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClaudeWebNativeChromeProfileOverride(t *testing.T) {
	t.Setenv("TL_STUDIO_CLAUDE_WEB_PROFILE", "Profile 9")
	got, err := claudeWebNativeProfile()
	if err != nil {
		t.Fatal(err)
	}
	if got != "Profile 9" {
		t.Fatalf("unexpected Chrome profile override %q", got)
	}
}

func TestClaudeWebNativeUserDataDirUsesLocalAppData(t *testing.T) {
	root := t.TempDir()
	t.Setenv("LOCALAPPDATA", root)
	expected := filepath.Join(root, "Google", "Chrome", "User Data")
	if err := os.MkdirAll(expected, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := claudeWebNativeUserDataDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != expected {
		t.Fatalf("unexpected Chrome user data dir %q", got)
	}
}

func TestClaudeWebNativeLoginUsesDedicatedSessionCloneAndCDP(t *testing.T) {
	source := readRepoText(t, "cmd/launcher/claude_web_native_windows.go")
	for _, required := range []string{
		`sessionDir = claudeWebSessionDirectory()`,
		`runtimeRoot = claudeWebRuntimeRootDirectory()`,
		`browser := t.makeBrowser(sessionDir)`,
		`browser.OpenLogin(ctx)`,
		`probeHeadless`,
		`refreshSessionCloneFromChrome`,
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("Claude Web native clone transport missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"user32.dll",
		"EnumWindows",
		"GetWindowText",
		"WM_CLOSE",
		"UIAutomationClient",
		"powershell.exe",
		"SendKeys",
		"Clipboard",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("Claude Web native transport must not depend on UI automation primitive %q", forbidden)
		}
	}
}

func TestClaudeWebNativeResetDeletesSessionAndRuntimeClones(t *testing.T) {
	root := t.TempDir()
	session := filepath.Join(root, "session")
	runtimeRoot := filepath.Join(root, "runtime")
	for _, dir := range []string{session, filepath.Join(runtimeRoot, "run-test")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "marker"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	transport := &claudeWebNativeTransport{
		chrome:      "chrome.exe",
		userDataDir: root,
		profileName: "Default",
		sessionDir:  session,
		runtimeRoot: runtimeRoot,
	}
	if err := transport.Reset(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{session, runtimeRoot} {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatalf("Claude Web clone still exists after reset: %s err=%v", dir, err)
		}
	}
}
