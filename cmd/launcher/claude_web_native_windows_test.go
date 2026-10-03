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
		`--new-window`,
		`--window-position=-32000,-32000`,
		`--window-size=1100,800`,
		`--disable-features=PwaNavigationCapturing`,
		`--disable-backgrounding-occluded-windows`,
		`--disable-renderer-backgrounding`,
		`--force-renderer-accessibility`,
		`claudeWebNativeURL`,
		`syscall.NewLazyDLL("user32.dll")`,
		`Chrome_WidgetWin_`,
		`UIAutomationClient`,
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
		"--app=",
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


func TestClaudeWebNativeWindowLifecycleUsesWin32NotPowerShell(t *testing.T) {
	source := readRepoText(t, "cmd/launcher/claude_web_native_windows.go")
	for _, required := range []string{
		`NewProc("EnumWindows")`,
		`NewProc("GetClassNameW")`,
		`NewProc("IsWindow")`,
		`NewProc("ShowWindow")`,
		`NewProc("SetWindowPos")`,
		`NewProc("PostMessageW")`,
		`func (t *claudeWebNativeTransport) launchWindow`,
		`exec.CommandContext(ctx, chrome, args...)`,
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("native Claude Chrome lifecycle missing %q", required)
		}
	}
	for _, forbidden := range []string{
		`const claudeWebWindowScript`,
		`Start-Process -FilePath`,
		`TLStudioClaudeBridge`,
		`Get-ChromeWindows`,
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("native Claude Chrome lifecycle must not depend on PowerShell window management: found %q", forbidden)
		}
	}
}

func TestClaudeWebPowerShellErrorsNeverExposeRawCLIXML(t *testing.T) {
	got := cleanClaudeWebPowerShellError("#< CLIXML\n<Objs Version=\"1.1.0.1\"><S S=\"Error\">boom</S></Objs>")
	if strings.Contains(got, "CLIXML") || strings.Contains(got, "<Objs") {
		t.Fatalf("raw PowerShell CLIXML leaked into user-facing error: %q", got)
	}
	if !strings.Contains(got, "Windows UI automation failed") {
		t.Fatalf("unexpected sanitized PowerShell error: %q", got)
	}
}


func TestClaudeWebNativeCompletionNeverForegroundsChrome(t *testing.T) {
	source := readRepoText(t, "cmd/launcher/claude_web_native_windows.go")
	start := strings.Index(source, "const claudeWebCompleteScript = `")
	end := strings.Index(source[start:], "const claudeWebVisibilityScript = `")
	if start < 0 || end < 0 {
		t.Fatal("Claude Web completion script bounds were not found")
	}
	complete := source[start : start+end]
	for _, forbidden := range []string{
		"System.Windows.Forms",
		"SendKeys",
		"Clipboard",
		"SetForegroundWindow",
		"ShowWindow",
	} {
		if strings.Contains(complete, forbidden) {
			t.Fatalf("Claude Web background completion must not use %q", forbidden)
		}
	}
	for _, required := range []string{
		"ValuePattern",
		"LegacyIAccessiblePattern",
		"InvokePattern",
		"$baselineBeginCount",
		"$baselineEndCount",
		"$beginCount -lt ($baselineBeginCount + 2)",
		"$endCount -lt ($baselineEndCount + 2)",
		"$text.LastIndexOf($beginMarker",
		"$text.LastIndexOf($endMarker",
	} {
		if !strings.Contains(complete, required) {
			t.Fatalf("Claude Web background completion contract missing %q", required)
		}
	}
}

func TestClaudeWebNativeLoginOnlyShowsChromeWhenAuthenticationIsNeeded(t *testing.T) {
	source := readRepoText(t, "cmd/launcher/claude_web_native_windows.go")
	for _, required := range []string{
		"hwnd, err := t.ensureWindow(ctx, false)",
		"if json.Unmarshal([]byte(strings.TrimSpace(output)), &probe) == nil && probe.Connected",
		"return t.setWindowVisibility(ctx, hwnd, true)",
		"if probe.Connected {",
		"_ = t.setWindowVisibility(ctx, hwnd, false)",
		"return claudeWebSetNativeWindowVisible(hwnd, visible)",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("Claude Web hidden-login contract missing %q", required)
		}
	}
}


func TestClaudeWebNativeHiddenWindowIsParkedOffScreenNotMinimized(t *testing.T) {
	source := readRepoText(t, "cmd/launcher/claude_web_native_windows.go")
	for _, required := range []string{
		`NewProc("SetWindowPos")`,
		`offscreen := int32(-32000)`,
		`--window-position=-32000,-32000`,
		`--window-size=1100,800`,
		`claudeWebSWPNoActivate`,
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("Claude Web off-screen bridge contract missing %q", required)
		}
	}
	if strings.Contains(source, "--start-minimized") || strings.Contains(source, "claudeWebSWMinimize") {
		t.Fatal("Claude Web background bridge must stay rendered off-screen instead of being minimized")
	}
}

func TestClaudeWebNativeMarkerParserWaitsForAssistantPair(t *testing.T) {
	source := readRepoText(t, "cmd/launcher/claude_web_native_windows.go")
	baseline := strings.Index(source, "$baselineText = Get-DocumentText")
	insert := strings.Index(source, "Set-ComposerText $composer $wrapped")
	if baseline < 0 || insert < 0 || baseline > insert {
		t.Fatal("marker baseline must be captured before inserting the TL Studio prompt")
	}
	for _, required := range []string{
		`$beginCount -lt ($baselineBeginCount + 2)`,
		`$endCount -lt ($baselineEndCount + 2)`,
		`$text.LastIndexOf($beginMarker`,
		`$text.LastIndexOf($endMarker`,
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("Claude Web response-marker contract missing %q", required)
		}
	}
}


func TestClaudeWebNativeBackgroundBridgeIsHiddenFromTaskbar(t *testing.T) {
	source := readRepoText(t, "cmd/launcher/claude_web_native_windows.go")
	for _, required := range []string{
		`NewProc("GetWindowLongPtrW")`,
		`NewProc("SetWindowLongPtrW")`,
		`claudeWebWSExToolWindow`,
		`claudeWebWSExAppWindow`,
		`claudeWebSWPFrameChanged`,
		`claudeWebSetTaskbarVisible(hwnd, visible)`,
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("Claude Web taskbar-hiding contract missing %q", required)
		}
	}
}
