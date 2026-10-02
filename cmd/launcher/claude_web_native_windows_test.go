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


func TestClaudeWebNativePowerShellDoesNotUseReservedPIDVariable(t *testing.T) {
	source := readRepoText(t, "cmd/launcher/claude_web_native_windows.go")
	if strings.Contains(source, "$pid =") || strings.Contains(source, "$PID =") {
		t.Fatal("native Claude Chrome bridge must not assign to PowerShell's reserved PID variable")
	}
	if !strings.Contains(source, "$processId = $window.Current.ProcessId") {
		t.Fatal("native Claude Chrome bridge must use a non-reserved process-id variable")
	}
}

func TestClaudeWebPowerShellErrorsNeverExposeRawCLIXML(t *testing.T) {
	got := cleanClaudeWebPowerShellError("#< CLIXML\n<Objs Version=\"1.1.0.1\"><S S=\"Error\">boom</S></Objs>")
	if strings.Contains(got, "CLIXML") || strings.Contains(got, "<Objs") {
		t.Fatalf("raw PowerShell CLIXML leaked into user-facing error: %q", got)
	}
	if !strings.Contains(got, "Windows PowerShell failed") {
		t.Fatalf("unexpected sanitized PowerShell error: %q", got)
	}
}
