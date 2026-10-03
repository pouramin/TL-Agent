//go:build windows

package main

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

const claudeWebNativeURL = "https://claude.ai/new"

type claudeWebNativeTransport struct {
	mu      sync.Mutex
	hwnd    uintptr
	profile string
	chrome  string
}

func newClaudeWebNativeTransport() claudeWebTransport {
	return &claudeWebNativeTransport{}
}

func (t *claudeWebNativeTransport) Available() error {
	chrome, err := resolveClaudeWebNativeChrome()
	if err != nil {
		return err
	}
	profile, err := claudeWebNativeProfile()
	if err != nil {
		return err
	}
	t.mu.Lock()
	t.chrome = chrome
	t.profile = profile
	t.mu.Unlock()
	return nil
}

func resolveClaudeWebNativeChrome() (string, error) {
	if override := strings.TrimSpace(os.Getenv("TL_STUDIO_CLAUDE_WEB_BROWSER")); override != "" {
		if path, ok := resolveExecutableCandidate(override); ok {
			return path, nil
		}
	}
	for _, root := range []string{
		os.Getenv("PROGRAMFILES"),
		os.Getenv("PROGRAMFILES(X86)"),
		os.Getenv("LOCALAPPDATA"),
	} {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		candidate := filepath.Join(root, "Google", "Chrome", "Application", "chrome.exe")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	if path, ok := resolveExecutableCandidate("chrome"); ok {
		return path, nil
	}
	return "", errors.New("Google Chrome was not found for Claude Web")
}

func claudeWebNativeUserDataDir() (string, error) {
	root := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if root == "" {
		return "", errors.New("LOCALAPPDATA is unavailable")
	}
	dir := filepath.Join(root, "Google", "Chrome", "User Data")
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return "", errors.New("the normal Chrome user-data directory was not found")
	}
	return dir, nil
}

func claudeWebNativeProfile() (string, error) {
	if override := strings.TrimSpace(os.Getenv("TL_STUDIO_CLAUDE_WEB_PROFILE")); override != "" {
		return override, nil
	}
	userData, err := claudeWebNativeUserDataDir()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(filepath.Join(userData, "Local State"))
	if err != nil {
		if info, statErr := os.Stat(filepath.Join(userData, "Default")); statErr == nil && info.IsDir() {
			return "Default", nil
		}
		return "", fmt.Errorf("read Chrome Local State: %w", err)
	}
	var state struct {
		Profile struct {
			LastUsed string `json:"last_used"`
		} `json:"profile"`
	}
	if json.Unmarshal(data, &state) == nil {
		if profile := strings.TrimSpace(state.Profile.LastUsed); profile != "" {
			if info, statErr := os.Stat(filepath.Join(userData, profile)); statErr == nil && info.IsDir() {
				return profile, nil
			}
		}
	}
	if info, statErr := os.Stat(filepath.Join(userData, "Default")); statErr == nil && info.IsDir() {
		return "Default", nil
	}
	return "", errors.New("Chrome's active profile could not be determined")
}

func encodePowerShell(script string) string {
	runes := utf16.Encode([]rune(script))
	bytes := make([]byte, len(runes)*2)
	for index, value := range runes {
		binary.LittleEndian.PutUint16(bytes[index*2:], value)
	}
	return base64.StdEncoding.EncodeToString(bytes)
}

func cleanClaudeWebPowerShellError(raw string) string {
	text := strings.TrimSpace(raw)
	if text == "" {
		return ""
	}
	if strings.Contains(text, "#< CLIXML") || strings.Contains(text, "<Objs Version=") {
		return "Windows PowerShell failed while starting the native Chrome bridge"
	}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if len(line) > 320 {
			line = line[:320] + "…"
		}
		return line
	}
	return "Windows PowerShell failed while starting the native Chrome bridge"
}

func runClaudeWebPowerShell(ctx context.Context, script string, env map[string]string) (string, error) {
	cmd := exec.CommandContext(
		ctx,
		"powershell.exe",
		"-NoLogo",
		"-NoProfile",
		"-NonInteractive",
		"-STA",
		"-ExecutionPolicy", "Bypass",
		"-EncodedCommand", encodePowerShell(script),
	)
	cmd.Env = append([]string(nil), os.Environ()...)
	for key, value := range env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	output, err := cmd.Output()
	text := strings.TrimSpace(string(output))
	if err != nil {
		detail := ""
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			detail = cleanClaudeWebPowerShellError(string(exitErr.Stderr))
		}
		if detail != "" {
			return "", fmt.Errorf("Claude Web Chrome bridge: %s", detail)
		}
		return "", fmt.Errorf("Claude Web Chrome bridge: %w", err)
	}
	return text, nil
}

var (
	claudeWebUser32                 = syscall.NewLazyDLL("user32.dll")
	claudeWebEnumWindows            = claudeWebUser32.NewProc("EnumWindows")
	claudeWebGetClassNameW          = claudeWebUser32.NewProc("GetClassNameW")
	claudeWebGetWindowTextLengthW   = claudeWebUser32.NewProc("GetWindowTextLengthW")
	claudeWebGetWindowTextW         = claudeWebUser32.NewProc("GetWindowTextW")
	claudeWebIsWindow               = claudeWebUser32.NewProc("IsWindow")
	claudeWebShowWindow             = claudeWebUser32.NewProc("ShowWindow")
	claudeWebSetWindowPos           = claudeWebUser32.NewProc("SetWindowPos")
	claudeWebSetForegroundWindow    = claudeWebUser32.NewProc("SetForegroundWindow")
	claudeWebPostMessageW           = claudeWebUser32.NewProc("PostMessageW")
)

const (
	claudeWebSWRestore      = 9
	claudeWebWMClose        = 0x0010
	claudeWebSWPNoSize      = 0x0001
	claudeWebSWPNoZOrder    = 0x0004
	claudeWebSWPNoActivate  = 0x0010
	claudeWebSWPShowWindow  = 0x0040
)

func claudeWebWindowClass(hwnd uintptr) string {
	buffer := make([]uint16, 256)
	length, _, _ := claudeWebGetClassNameW.Call(
		hwnd,
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(len(buffer)),
	)
	if length == 0 {
		return ""
	}
	return syscall.UTF16ToString(buffer[:length])
}

func claudeWebWindowTitle(hwnd uintptr) string {
	length, _, _ := claudeWebGetWindowTextLengthW.Call(hwnd)
	if length == 0 {
		return ""
	}
	buffer := make([]uint16, int(length)+1)
	copied, _, _ := claudeWebGetWindowTextW.Call(
		hwnd,
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(len(buffer)),
	)
	if copied == 0 {
		return ""
	}
	return syscall.UTF16ToString(buffer[:copied])
}

func claudeWebChromeWindows() map[uintptr]string {
	windows := map[uintptr]string{}
	callback := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		className := claudeWebWindowClass(hwnd)
		if !strings.HasPrefix(className, "Chrome_WidgetWin_") {
			return 1
		}
		windows[hwnd] = claudeWebWindowTitle(hwnd)
		return 1
	})
	claudeWebEnumWindows.Call(callback, 0)
	return windows
}

func claudeWebNativeWindowAlive(hwnd uintptr) bool {
	if hwnd == 0 {
		return false
	}
	ok, _, _ := claudeWebIsWindow.Call(hwnd)
	return ok != 0
}

func claudeWebSetNativeWindowVisible(hwnd uintptr, visible bool) error {
	if !claudeWebNativeWindowAlive(hwnd) {
		return errors.New("Claude bridge window is unavailable")
	}
	claudeWebShowWindow.Call(hwnd, uintptr(claudeWebSWRestore))
	if visible {
		claudeWebSetWindowPos.Call(
			hwnd,
			0,
			uintptr(int32(80)),
			uintptr(int32(80)),
			uintptr(1100),
			uintptr(800),
			uintptr(claudeWebSWPNoZOrder|claudeWebSWPShowWindow),
		)
		claudeWebSetForegroundWindow.Call(hwnd)
		return nil
	}

	offscreen := int32(-32000)
	claudeWebSetWindowPos.Call(
		hwnd,
		0,
		uintptr(offscreen),
		uintptr(offscreen),
		0,
		0,
		uintptr(claudeWebSWPNoSize|claudeWebSWPNoZOrder|claudeWebSWPNoActivate|claudeWebSWPShowWindow),
	)
	return nil
}

func claudeWebCloseNativeWindow(hwnd uintptr) error {
	if hwnd == 0 || !claudeWebNativeWindowAlive(hwnd) {
		return nil
	}
	ok, _, callErr := claudeWebPostMessageW.Call(
		hwnd,
		uintptr(claudeWebWMClose),
		0,
		0,
	)
	if ok == 0 && callErr != syscall.Errno(0) {
		return callErr
	}
	return nil
}

func (t *claudeWebNativeTransport) launchWindow(ctx context.Context, visible bool) (uintptr, error) {
	if err := t.Available(); err != nil {
		return 0, err
	}
	t.mu.Lock()
	chrome := t.chrome
	profile := t.profile
	t.mu.Unlock()

	before := claudeWebChromeWindows()
	args := []string{
		"--profile-directory=" + profile,
		"--new-window",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-features=PwaNavigationCapturing",
		"--disable-backgrounding-occluded-windows",
		"--disable-renderer-backgrounding",
		"--disable-background-timer-throttling",
		"--force-renderer-accessibility",
	}
	if !visible {
		args = append(args,
			"--window-position=-32000,-32000",
			"--window-size=1100,800",
		)
	}
	args = append(args, claudeWebNativeURL)

	cmd := exec.CommandContext(ctx, chrome, args...)
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("start Chrome for Claude Web: %w", err)
	}
	_ = cmd.Process.Release()

	deadline := time.Now().Add(20 * time.Second)
	var fallback uintptr
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}
		for hwnd, title := range claudeWebChromeWindows() {
			if _, existed := before[hwnd]; existed {
				continue
			}
			if fallback == 0 {
				fallback = hwnd
			}
			if strings.Contains(strings.ToLower(title), "claude") {
				_ = claudeWebSetNativeWindowVisible(hwnd, visible)
				return hwnd, nil
			}
		}
		if fallback != 0 && time.Until(deadline) < 17*time.Second {
			_ = claudeWebSetNativeWindowVisible(fallback, visible)
			return fallback, nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	if fallback != 0 {
		_ = claudeWebSetNativeWindowVisible(fallback, visible)
		return fallback, nil
	}
	return 0, errors.New("Chrome did not create the TL Studio Claude bridge window")
}

const claudeWebProbeScript = `
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
$handle = [IntPtr]([Int64]::Parse($env:TL_CLAUDE_HWND))
$window = [System.Windows.Automation.AutomationElement]::FromHandle($handle)
if ($null -eq $window) {
  Write-Output '{"connected":false,"error":"Claude bridge window is unavailable"}'
  exit 0
}
$deadline = [DateTime]::UtcNow.AddSeconds(12)
while ([DateTime]::UtcNow -lt $deadline) {
  try {
    $document = $window.FindFirst(
      [System.Windows.Automation.TreeScope]::Descendants,
      (New-Object System.Windows.Automation.PropertyCondition(
        [System.Windows.Automation.AutomationElement]::ControlTypeProperty,
        [System.Windows.Automation.ControlType]::Document
      ))
    )
    $text = ""
    if ($null -ne $document) {
      $pattern = $null
      if ($document.TryGetCurrentPattern([System.Windows.Automation.TextPattern]::Pattern, [ref]$pattern)) {
        $text = $pattern.DocumentRange.GetText(-1)
      }
    }

    $edits = $window.FindAll(
      [System.Windows.Automation.TreeScope]::Descendants,
      (New-Object System.Windows.Automation.PropertyCondition(
        [System.Windows.Automation.AutomationElement]::ControlTypeProperty,
        [System.Windows.Automation.ControlType]::Edit
      ))
    )
    $composer = $false
    foreach ($edit in $edits) {
      $name = [string]$edit.Current.Name
      $rect = $edit.Current.BoundingRectangle
      if ($name -match "(How can I help|Message|Ask Claude|Talk to Claude)" -or ($rect.Width -gt 300 -and $rect.Height -gt 35)) {
        $composer = $true
        break
      }
    }

    if ($composer -and ($text -match "(Claude|New chat|Projects|How can I help)")) {
      Write-Output '{"connected":true,"status":200}'
      exit 0
    }
  } catch {}
  Start-Sleep -Milliseconds 200
}
Write-Output '{"connected":false,"status":401,"error":"Claude is not signed in or the Claude composer is not ready in this Chrome profile."}'
`

const claudeWebCompleteScript = `
$ErrorActionPreference = "Stop"
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes

$handle = [IntPtr]([Int64]::Parse($env:TL_CLAUDE_HWND))
$window = [System.Windows.Automation.AutomationElement]::FromHandle($handle)
if ($null -eq $window) { throw "Claude bridge window is unavailable." }

$prompt = Get-Content -LiteralPath $env:TL_CLAUDE_PROMPT_FILE -Raw -Encoding UTF8
$beginMarker = $env:TL_CLAUDE_BEGIN
$endMarker = $env:TL_CLAUDE_END
$wrapped = @"
Begin your visible response with this exact marker on its own line:
$beginMarker
End your visible response with this exact marker on its own line:
$endMarker
Do not put either marker inside a code fence. Everything between the markers must be the response requested below.

$prompt
"@

function Get-DocumentText {
  $document = $window.FindFirst(
    [System.Windows.Automation.TreeScope]::Descendants,
    (New-Object System.Windows.Automation.PropertyCondition(
      [System.Windows.Automation.AutomationElement]::ControlTypeProperty,
      [System.Windows.Automation.ControlType]::Document
    ))
  )
  if ($null -eq $document) { return "" }
  $pattern = $null
  if ($document.TryGetCurrentPattern([System.Windows.Automation.TextPattern]::Pattern, [ref]$pattern)) {
    return [string]$pattern.DocumentRange.GetText(-1)
  }
  return ""
}

function Count-Marker([string]$text, [string]$marker) {
  if ([string]::IsNullOrEmpty($text) -or [string]::IsNullOrEmpty($marker)) { return 0 }
  $count = 0
  $index = 0
  while ($true) {
    $index = $text.IndexOf($marker, $index, [System.StringComparison]::Ordinal)
    if ($index -lt 0) { break }
    $count++
    $index += $marker.Length
  }
  return $count
}

function Find-Composer {
  $edits = $window.FindAll(
    [System.Windows.Automation.TreeScope]::Descendants,
    (New-Object System.Windows.Automation.PropertyCondition(
      [System.Windows.Automation.AutomationElement]::ControlTypeProperty,
      [System.Windows.Automation.ControlType]::Edit
    ))
  )
  $fallback = $null
  foreach ($edit in $edits) {
    $name = [string]$edit.Current.Name
    $rect = $edit.Current.BoundingRectangle
    if ($name -match "(How can I help|Message|Ask Claude|Talk to Claude)") { return $edit }
    if ($rect.Width -gt 300 -and $rect.Height -gt 35) { $fallback = $edit }
  }
  return $fallback
}

function Set-ComposerText($composer, [string]$text) {
  $valuePattern = $null
  if ($composer.TryGetCurrentPattern([System.Windows.Automation.ValuePattern]::Pattern, [ref]$valuePattern)) {
    try {
      if (-not $valuePattern.Current.IsReadOnly) {
        $valuePattern.SetValue($text)
        return $true
      }
    } catch {}
  }

  $legacyPattern = $null
  if ($composer.TryGetCurrentPattern([System.Windows.Automation.LegacyIAccessiblePattern]::Pattern, [ref]$legacyPattern)) {
    try {
      $legacyPattern.SetValue($text)
      return $true
    } catch {}
  }
  return $false
}

$composer = Find-Composer
if ($null -eq $composer) { throw "Claude message composer was not found." }

# Count the unique markers before inserting this turn. The submitted user
# message itself adds one begin/end pair; the assistant response adds the
# second pair. Waiting for both prevents TL Studio from mistaking its own
# prompt instructions for Claude's answer.
$baselineText = Get-DocumentText
$baselineBeginCount = Count-Marker $baselineText $beginMarker
$baselineEndCount = Count-Marker $baselineText $endMarker

if (-not (Set-ComposerText $composer $wrapped)) {
  throw "Claude message composer does not expose a background-edit automation pattern. TL Studio will not bring Chrome to the foreground as a fallback."
}

Start-Sleep -Milliseconds 120

$sendButton = $null
$buttons = $window.FindAll(
  [System.Windows.Automation.TreeScope]::Descendants,
  (New-Object System.Windows.Automation.PropertyCondition(
    [System.Windows.Automation.AutomationElement]::ControlTypeProperty,
    [System.Windows.Automation.ControlType]::Button
  ))
)
foreach ($button in $buttons) {
  $name = [string]$button.Current.Name
  if ($name -match "^(Send|Send message|Send Message)$") {
    $sendButton = $button
    break
  }
}
if ($null -eq $sendButton) {
  throw "Claude send button was not found for background automation."
}
$invoke = $null
if (-not $sendButton.TryGetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern, [ref]$invoke)) {
  throw "Claude send button does not expose a background invoke pattern."
}
$invoke.Invoke()

$deadline = [DateTime]::UtcNow.AddSeconds(180)
while ([DateTime]::UtcNow -lt $deadline) {
  Start-Sleep -Milliseconds 180
  $text = Get-DocumentText
  $beginCount = Count-Marker $text $beginMarker
  $endCount = Count-Marker $text $endMarker
  if ($beginCount -lt ($baselineBeginCount + 2) -or $endCount -lt ($baselineEndCount + 2)) { continue }

  $begin = $text.LastIndexOf($beginMarker, [System.StringComparison]::Ordinal)
  $end = $text.LastIndexOf($endMarker, [System.StringComparison]::Ordinal)
  if ($begin -lt 0 -or $end -le $begin) { continue }

  $contentStart = $begin + $beginMarker.Length
  $result = $text.Substring($contentStart, $end - $contentStart).Trim()
  Write-Output $result
  exit 0
}
throw "Timed out waiting for Claude to finish the TL Studio response."
`

func (t *claudeWebNativeTransport) ensureWindow(ctx context.Context, visible bool) (uintptr, error) {
	t.mu.Lock()
	hwnd := t.hwnd
	t.mu.Unlock()

	if hwnd != 0 && claudeWebNativeWindowAlive(hwnd) {
		if err := claudeWebSetNativeWindowVisible(hwnd, visible); err != nil {
			t.mu.Lock()
			t.hwnd = 0
			t.mu.Unlock()
			return 0, err
		}
		return hwnd, nil
	}

	hwnd, err := t.launchWindow(ctx, visible)
	if err != nil {
		return 0, err
	}
	t.mu.Lock()
	t.hwnd = hwnd
	t.mu.Unlock()
	return hwnd, nil
}

func (t *claudeWebNativeTransport) setWindowVisibility(_ context.Context, hwnd uintptr, visible bool) error {
	return claudeWebSetNativeWindowVisible(hwnd, visible)
}

func (t *claudeWebNativeTransport) OpenLogin(ctx context.Context) error {
	hwnd, err := t.ensureWindow(ctx, false)
	if err != nil {
		return err
	}
	probeCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	output, probeErr := runClaudeWebPowerShell(probeCtx, claudeWebProbeScript, map[string]string{
		"TL_CLAUDE_HWND": strconv.FormatUint(uint64(hwnd), 10),
	})
	if probeErr == nil {
		var probe claudeWebProbe
		if json.Unmarshal([]byte(strings.TrimSpace(output)), &probe) == nil && probe.Connected {
			return nil
		}
	}
	return t.setWindowVisibility(ctx, hwnd, true)
}

func (t *claudeWebNativeTransport) Probe(ctx context.Context) (claudeWebProbe, error) {
	hwnd, err := t.ensureWindow(ctx, false)
	if err != nil {
		return claudeWebProbe{}, err
	}
	probeCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	output, err := runClaudeWebPowerShell(probeCtx, claudeWebProbeScript, map[string]string{
		"TL_CLAUDE_HWND": strconv.FormatUint(uint64(hwnd), 10),
	})
	if err != nil {
		t.mu.Lock()
		t.hwnd = 0
		t.mu.Unlock()
		return claudeWebProbe{}, err
	}
	var probe claudeWebProbe
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &probe); err != nil {
		return claudeWebProbe{}, fmt.Errorf("decode Claude Web Chrome probe: %w", err)
	}
	if probe.Connected {
		_ = t.setWindowVisibility(ctx, hwnd, false)
	}
	return probe, nil
}

func (t *claudeWebNativeTransport) Complete(ctx context.Context, prompt string) (string, error) {
	hwnd, err := t.ensureWindow(ctx, false)
	if err != nil {
		return "", err
	}
	nonce, err := randomBase64URL(18)
	if err != nil {
		return "", err
	}
	beginMarker := "TLSTUDIO_BEGIN_" + nonce
	endMarker := "TLSTUDIO_END_" + nonce
	dir := filepath.Join(tlStudioStateDirectory(), "claude-web-native")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(dir, "prompt-*.txt")
	if err != nil {
		return "", err
	}
	path := file.Name()
	defer os.Remove(path)
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return "", err
	}
	if _, err := file.WriteString(prompt); err != nil {
		_ = file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}

	callCtx, cancel := context.WithTimeout(ctx, 190*time.Second)
	defer cancel()
	output, err := runClaudeWebPowerShell(callCtx, claudeWebCompleteScript, map[string]string{
		"TL_CLAUDE_HWND":        strconv.FormatUint(uint64(hwnd), 10),
		"TL_CLAUDE_PROMPT_FILE": path,
		"TL_CLAUDE_BEGIN":       beginMarker,
		"TL_CLAUDE_END":         endMarker,
	})
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(output) == "" {
		return "", errors.New("Claude Web returned an empty response")
	}
	return strings.TrimSpace(output), nil
}

func (t *claudeWebNativeTransport) Close(context.Context) error {
	t.mu.Lock()
	hwnd := t.hwnd
	t.hwnd = 0
	t.mu.Unlock()
	return claudeWebCloseNativeWindow(hwnd)
}
