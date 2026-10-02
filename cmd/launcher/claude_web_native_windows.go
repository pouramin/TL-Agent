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
	"time"
	"unicode/utf16"
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
	output, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(output))
	if err != nil {
		if text != "" {
			return "", fmt.Errorf("Claude Web Chrome bridge: %s", text)
		}
		return "", fmt.Errorf("Claude Web Chrome bridge: %w", err)
	}
	return text, nil
}

const claudeWebWindowScript = `
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
Add-Type @"
using System;
using System.Runtime.InteropServices;
public static class TLStudioClaudeWin {
  [DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern IntPtr GetProp(IntPtr hWnd, string lpString);
  [DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern bool SetProp(IntPtr hWnd, string lpString, IntPtr hData);
  [DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr hWnd, int nCmdShow);
  [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr hWnd);
  [DllImport("user32.dll")] public static extern bool PostMessage(IntPtr hWnd, uint Msg, IntPtr wParam, IntPtr lParam);
}
"@

function Get-ChromeWindows {
  $items = @()
  $root = [System.Windows.Automation.AutomationElement]::RootElement
  $windows = $root.FindAll(
    [System.Windows.Automation.TreeScope]::Children,
    [System.Windows.Automation.Condition]::TrueCondition
  )
  foreach ($window in $windows) {
    try {
      $pid = $window.Current.ProcessId
      if ($pid -le 0) { continue }
      $process = Get-Process -Id $pid -ErrorAction Stop
      if ($process.ProcessName -notin @("chrome", "chrome_proxy")) { continue }
      $items += $window
    } catch {}
  }
  return $items
}

$visible = $env:TL_CLAUDE_VISIBLE -eq "1"
foreach ($window in (Get-ChromeWindows)) {
  $handle = [IntPtr]$window.Current.NativeWindowHandle
  if ($handle -eq [IntPtr]::Zero) { continue }
  if ([TLStudioClaudeWin]::GetProp($handle, "TLStudioClaudeBridge") -ne [IntPtr]::Zero) {
    if ($visible) {
      [void][TLStudioClaudeWin]::ShowWindow($handle, 9)
      [void][TLStudioClaudeWin]::SetForegroundWindow($handle)
    } else {
      [void][TLStudioClaudeWin]::ShowWindow($handle, 6)
    }
    Write-Output $handle.ToInt64()
    exit 0
  }
}

$before = @{}
foreach ($window in (Get-ChromeWindows)) {
  $before[[string]$window.Current.NativeWindowHandle] = $true
}

$args = @(
  "--profile-directory=" + $env:TL_CLAUDE_PROFILE,
  "--new-window",
  "--app=https://claude.ai/new",
  "--no-first-run",
  "--no-default-browser-check"
)
Start-Process -FilePath $env:TL_CLAUDE_CHROME -ArgumentList $args | Out-Null

$deadline = [DateTime]::UtcNow.AddSeconds(20)
while ([DateTime]::UtcNow -lt $deadline) {
  Start-Sleep -Milliseconds 150
  $candidate = $null
  foreach ($window in (Get-ChromeWindows)) {
    $handleValue = [string]$window.Current.NativeWindowHandle
    if ($before.ContainsKey($handleValue)) { continue }
    if ($window.Current.NativeWindowHandle -eq 0) { continue }
    if ($window.Current.Name -match "Claude") {
      $candidate = $window
      break
    }
    if ($null -eq $candidate) { $candidate = $window }
  }
  if ($null -ne $candidate) {
    $handle = [IntPtr]$candidate.Current.NativeWindowHandle
    [void][TLStudioClaudeWin]::SetProp($handle, "TLStudioClaudeBridge", [IntPtr]1)
    if ($visible) {
      [void][TLStudioClaudeWin]::ShowWindow($handle, 9)
      [void][TLStudioClaudeWin]::SetForegroundWindow($handle)
    } else {
      [void][TLStudioClaudeWin]::ShowWindow($handle, 6)
    }
    Write-Output $handle.ToInt64()
    exit 0
  }
}
Write-Error "Chrome did not create the TL Studio Claude bridge window."
exit 1
`

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
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
Add-Type -AssemblyName System.Windows.Forms
Add-Type @"
using System;
using System.Runtime.InteropServices;
public static class TLStudioClaudeInput {
  [DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr hWnd, int nCmdShow);
  [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr hWnd);
}
"@

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

$composer = Find-Composer
if ($null -eq $composer) { throw "Claude message composer was not found." }

$setDirectly = $false
$valuePattern = $null
if ($composer.TryGetCurrentPattern([System.Windows.Automation.ValuePattern]::Pattern, [ref]$valuePattern)) {
  try {
    if (-not $valuePattern.Current.IsReadOnly) {
      $valuePattern.SetValue($wrapped)
      $setDirectly = $true
    }
  } catch {}
}

if (-not $setDirectly) {
  $oldClipboard = $null
  try { $oldClipboard = [System.Windows.Forms.Clipboard]::GetText() } catch {}
  [void][TLStudioClaudeInput]::ShowWindow($handle, 9)
  [void][TLStudioClaudeInput]::SetForegroundWindow($handle)
  $composer.SetFocus()
  Start-Sleep -Milliseconds 120
  [System.Windows.Forms.Clipboard]::SetText($wrapped)
  [System.Windows.Forms.SendKeys]::SendWait("^a")
  [System.Windows.Forms.SendKeys]::SendWait("^v")
  Start-Sleep -Milliseconds 80
  if ($null -ne $oldClipboard) {
    try { [System.Windows.Forms.Clipboard]::SetText($oldClipboard) } catch {}
  }
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
  }
}
if ($null -ne $sendButton) {
  $invoke = $null
  if ($sendButton.TryGetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern, [ref]$invoke)) {
    $invoke.Invoke()
  } else {
    $composer.SetFocus()
    [System.Windows.Forms.SendKeys]::SendWait("{ENTER}")
  }
} else {
  $composer.SetFocus()
  [System.Windows.Forms.SendKeys]::SendWait("{ENTER}")
}

$deadline = [DateTime]::UtcNow.AddSeconds(180)
while ([DateTime]::UtcNow -lt $deadline) {
  Start-Sleep -Milliseconds 180
  $text = Get-DocumentText
  $begin = $text.LastIndexOf($beginMarker)
  if ($begin -lt 0) { continue }
  $start = $begin + $beginMarker.Length
  $end = $text.IndexOf($endMarker, $start)
  if ($end -lt 0) { continue }
  $result = $text.Substring($start, $end - $start).Trim()
  [void][TLStudioClaudeInput]::ShowWindow($handle, 6)
  Write-Output $result
  exit 0
}
throw "Timed out waiting for Claude to finish the TL Studio response."
`

const claudeWebCloseScript = `
Add-Type @"
using System;
using System.Runtime.InteropServices;
public static class TLStudioClaudeClose {
  [DllImport("user32.dll")] public static extern bool PostMessage(IntPtr hWnd, uint Msg, IntPtr wParam, IntPtr lParam);
}
"@
$handle = [IntPtr]([Int64]::Parse($env:TL_CLAUDE_HWND))
[void][TLStudioClaudeClose]::PostMessage($handle, 0x0010, [IntPtr]::Zero, [IntPtr]::Zero)
`

func (t *claudeWebNativeTransport) ensureWindow(ctx context.Context, visible bool) (uintptr, error) {
	t.mu.Lock()
	if t.hwnd != 0 {
		hwnd := t.hwnd
		t.mu.Unlock()
		return hwnd, nil
	}
	t.mu.Unlock()

	if err := t.Available(); err != nil {
		return 0, err
	}
	t.mu.Lock()
	chrome := t.chrome
	profile := t.profile
	t.mu.Unlock()

	output, err := runClaudeWebPowerShell(ctx, claudeWebWindowScript, map[string]string{
		"TL_CLAUDE_CHROME":  chrome,
		"TL_CLAUDE_PROFILE": profile,
		"TL_CLAUDE_VISIBLE": map[bool]string{true: "1", false: "0"}[visible],
	})
	if err != nil {
		return 0, err
	}
	lines := strings.Fields(output)
	if len(lines) == 0 {
		return 0, errors.New("Claude Web Chrome bridge did not return a window handle")
	}
	value, err := strconv.ParseUint(lines[len(lines)-1], 10, 64)
	if err != nil || value == 0 {
		return 0, fmt.Errorf("invalid Claude Web Chrome bridge window handle %q", output)
	}
	t.mu.Lock()
	t.hwnd = uintptr(value)
	t.mu.Unlock()
	return uintptr(value), nil
}

func (t *claudeWebNativeTransport) OpenLogin(ctx context.Context) error {
	_, err := t.ensureWindow(ctx, true)
	return err
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

func (t *claudeWebNativeTransport) Close(ctx context.Context) error {
	t.mu.Lock()
	hwnd := t.hwnd
	t.hwnd = 0
	t.mu.Unlock()
	if hwnd == 0 {
		return nil
	}
	closeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := runClaudeWebPowerShell(closeCtx, claudeWebCloseScript, map[string]string{
		"TL_CLAUDE_HWND": strconv.FormatUint(uint64(hwnd), 10),
	})
	return err
}
