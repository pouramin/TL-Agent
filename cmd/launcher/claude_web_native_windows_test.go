//go:build windows

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClaudeWebNativeTransportUsesNormalChromeProfileWithoutExtension(t *testing.T) {
	source := readRepoText(t, "cmd/launcher/claude_web_native_windows.go")
	for _, required := range []string{
		`Google", "Chrome", "User Data"`,
		`"last_used"`,
		`--profile-directory=`,
		`--app=https://claude.ai/new`,
		`UIAutomationClient`,
		`TLStudioClaudeBridge`,
		`TLSTUDIO_BEGIN_`,
		`TLSTUDIO_END_`,
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("Claude Web native Chrome transport missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"--remote-debugging-port",
		"chrome.cookies",
		"sessionKey",
		"claude-web-extension",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("Claude Web native Chrome transport must not depend on %q", forbidden)
		}
	}
}

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
