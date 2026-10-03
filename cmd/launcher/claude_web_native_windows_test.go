//go:build windows

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClaudeWebNativeTransportClonesNormalChromeProfile(t *testing.T) {
	source := readRepoText(t, "cmd/launcher/claude_web_native_windows.go")
	for _, required := range []string{
		`Google", "Chrome", "User Data"`,
		`"last_used"`,
		`claude-web-profile-clone`,
		`"Local State"`,
		`"Network"`,
		`"Local Storage"`,
		`"Session Storage"`,
		`"IndexedDB"`,
		`newClaudeWebBrowserTransport(cloneDir, chrome, profileName)`,
		`syncProfileClone`,
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("Claude Web profile-clone transport missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"UIAutomationClient",
		"UIAutomationTypes",
		"powershell.exe",
		"SendKeys",
		"Clipboard",
		"chrome.cookies",
		"sessionKey",
		"claude-web-extension",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("Claude Web clone transport must not depend on %q", forbidden)
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

func TestCopyClaudeWebPathCopiesAuthTree(t *testing.T) {
	src := filepath.Join(t.TempDir(), "Network")
	dst := filepath.Join(t.TempDir(), "Network")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	want := []byte("encrypted-cookie-bytes")
	if err := os.WriteFile(filepath.Join(src, "Cookies"), want, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := copyClaudeWebPath(context.Background(), src, dst); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dst, "Cookies"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("copied cookie DB changed: %q", got)
	}
}

func TestClaudeWebNativeLoginWindowExistsOnlyForAuthentication(t *testing.T) {
	source := readRepoText(t, "cmd/launcher/claude_web_native_windows.go")
	for _, required := range []string{
		`probe, probeErr := browser.Probe(probeCtx)`,
		`if probeErr == nil && probe.Connected`,
		`return t.launchLoginWindow(ctx)`,
		`"--new-window"`,
		`claudeWebCloseNativeWindow(loginHWND)`,
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("Claude Web login-window lifecycle missing %q", required)
		}
	}
}

func TestClaudeWebNativeResetDeletesClonedSession(t *testing.T) {
	root := t.TempDir()
	clone := filepath.Join(root, "clone")
	if err := os.MkdirAll(clone, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(clone, "marker"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	transport := &claudeWebNativeTransport{cloneDir: clone, cloneReady: true}
	if err := transport.Reset(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(clone); !os.IsNotExist(err) {
		t.Fatalf("cloned Claude session still exists after reset: %v", err)
	}
}
