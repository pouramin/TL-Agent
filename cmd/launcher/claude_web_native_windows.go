//go:build windows

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const claudeWebNativeURL = "https://claude.ai/new"

type claudeWebNativeTransport struct {
	mu          sync.Mutex
	chrome      string
	userDataDir string
	profileName string
	cloneDir    string
	browser     *claudeWebBrowserTransport
	loginHWND   uintptr
	cloneReady  bool
}

func newClaudeWebNativeTransport() claudeWebTransport {
	return &claudeWebNativeTransport{}
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

func claudeWebCloneDirectory() string {
	return filepath.Join(tlStudioStateDirectory(), "claude-web-profile-clone")
}

func (t *claudeWebNativeTransport) Available() error {
	chrome, err := resolveClaudeWebNativeChrome()
	if err != nil {
		return err
	}
	userDataDir, err := claudeWebNativeUserDataDir()
	if err != nil {
		return err
	}
	profileName, err := claudeWebNativeProfile()
	if err != nil {
		return err
	}
	cloneDir := claudeWebCloneDirectory()

	t.mu.Lock()
	t.chrome = chrome
	t.userDataDir = userDataDir
	t.profileName = profileName
	t.cloneDir = cloneDir
	if t.browser == nil ||
		t.browser.profileDir != cloneDir ||
		t.browser.executable != chrome ||
		t.browser.profileName != profileName {
		t.browser = newClaudeWebBrowserTransport(cloneDir, chrome, profileName)
	}
	t.mu.Unlock()
	return nil
}

var (
	claudeWebUser32               = syscall.NewLazyDLL("user32.dll")
	claudeWebEnumWindows          = claudeWebUser32.NewProc("EnumWindows")
	claudeWebGetClassNameW        = claudeWebUser32.NewProc("GetClassNameW")
	claudeWebGetWindowTextLengthW = claudeWebUser32.NewProc("GetWindowTextLengthW")
	claudeWebGetWindowTextW       = claudeWebUser32.NewProc("GetWindowTextW")
	claudeWebIsWindow             = claudeWebUser32.NewProc("IsWindow")
	claudeWebPostMessageW         = claudeWebUser32.NewProc("PostMessageW")
)

const claudeWebWMClose = 0x0010

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
		if !strings.HasPrefix(claudeWebWindowClass(hwnd), "Chrome_WidgetWin_") {
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

func claudeWebCloseNativeWindow(hwnd uintptr) error {
	if hwnd == 0 || !claudeWebNativeWindowAlive(hwnd) {
		return nil
	}
	ok, _, callErr := claudeWebPostMessageW.Call(hwnd, uintptr(claudeWebWMClose), 0, 0)
	if ok == 0 && callErr != syscall.Errno(0) {
		return callErr
	}
	return nil
}

func (t *claudeWebNativeTransport) launchLoginWindow(ctx context.Context) error {
	if err := t.Available(); err != nil {
		return err
	}
	t.mu.Lock()
	chrome := t.chrome
	profile := t.profileName
	t.mu.Unlock()

	before := claudeWebChromeWindows()
	cmd := exec.CommandContext(
		ctx,
		chrome,
		"--profile-directory="+profile,
		"--new-window",
		"--no-first-run",
		"--no-default-browser-check",
		claudeWebNativeURL,
	)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open Claude sign-in in Chrome: %w", err)
	}
	_ = cmd.Process.Release()

	deadline := time.Now().Add(15 * time.Second)
	var fallback uintptr
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
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
				t.mu.Lock()
				t.loginHWND = hwnd
				t.mu.Unlock()
				return nil
			}
		}
		if fallback != 0 && time.Until(deadline) < 12*time.Second {
			t.mu.Lock()
			t.loginHWND = fallback
			t.mu.Unlock()
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	return errors.New("Chrome did not create the temporary Claude sign-in window")
}

var claudeWebCloneEntries = []string{
	"Preferences",
	"Secure Preferences",
	"Network",
	"Local Storage",
	"Session Storage",
	"IndexedDB",
	"Storage",
	"Shared Dictionary",
	"SharedStorage",
	"SharedStorage-wal",
	"Web Data",
	"Web Data-journal",
	"Cookies",
	"Cookies-journal",
	"Trust Tokens",
	"Trust Tokens-journal",
}

func copyClaudeWebFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	temp := dst + ".tlstudio-copy"
	_ = os.Remove(temp)
	target, err := os.OpenFile(temp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(target, source)
	closeErr := target.Close()
	if copyErr != nil {
		_ = os.Remove(temp)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(temp)
		return closeErr
	}
	_ = os.Chmod(temp, info.Mode().Perm())
	_ = os.Remove(dst)
	if err := os.Rename(temp, dst); err != nil {
		_ = os.Remove(temp)
		return err
	}
	return nil
}

func copyClaudeWebPath(ctx context.Context, src, dst string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	info, err := os.Lstat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil
	}
	if !info.IsDir() {
		return copyClaudeWebFile(src, dst)
	}
	if err := os.MkdirAll(dst, 0o700); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		switch name {
		case "LOCK", "LOG", "LOG.old", "DevToolsActivePort", "SingletonCookie", "SingletonLock", "SingletonSocket":
			continue
		}
		if err := copyClaudeWebPath(ctx, filepath.Join(src, name), filepath.Join(dst, name)); err != nil {
			// Chrome can rotate cache or SQLite helper files while the main
			// profile is live. Skip individual volatile files; the probe below
			// is the authoritative validation of whether the clone is usable.
			continue
		}
	}
	return nil
}

func (t *claudeWebNativeTransport) syncProfileClone(ctx context.Context) error {
	if err := t.Available(); err != nil {
		return err
	}
	t.mu.Lock()
	browser := t.browser
	userDataDir := t.userDataDir
	profileName := t.profileName
	cloneDir := t.cloneDir
	t.mu.Unlock()

	if browser != nil {
		_ = browser.Close(ctx)
	}

	tempDir := cloneDir + ".sync"
	_ = os.RemoveAll(tempDir)
	if err := os.MkdirAll(filepath.Join(tempDir, profileName), 0o700); err != nil {
		return err
	}
	if err := copyClaudeWebFile(filepath.Join(userDataDir, "Local State"), filepath.Join(tempDir, "Local State")); err != nil {
		return fmt.Errorf("copy Chrome Local State for Claude Web: %w", err)
	}

	sourceProfile := filepath.Join(userDataDir, profileName)
	targetProfile := filepath.Join(tempDir, profileName)
	for _, relative := range claudeWebCloneEntries {
		if err := copyClaudeWebPath(ctx, filepath.Join(sourceProfile, relative), filepath.Join(targetProfile, relative)); err != nil {
			_ = os.RemoveAll(tempDir)
			return fmt.Errorf("copy Chrome profile data %s for Claude Web: %w", relative, err)
		}
	}

	_ = os.RemoveAll(filepath.Join(tempDir, "SingletonCookie"))
	_ = os.RemoveAll(filepath.Join(tempDir, "SingletonLock"))
	_ = os.RemoveAll(filepath.Join(tempDir, "SingletonSocket"))
	_ = os.Remove(filepath.Join(tempDir, "DevToolsActivePort"))

	backupDir := cloneDir + ".old"
	_ = os.RemoveAll(backupDir)
	if _, err := os.Stat(cloneDir); err == nil {
		if err := os.Rename(cloneDir, backupDir); err != nil {
			_ = os.RemoveAll(tempDir)
			return fmt.Errorf("rotate Claude Web profile clone: %w", err)
		}
	}
	if err := os.Rename(tempDir, cloneDir); err != nil {
		if _, statErr := os.Stat(backupDir); statErr == nil {
			_ = os.Rename(backupDir, cloneDir)
		}
		return fmt.Errorf("activate Claude Web profile clone: %w", err)
	}
	_ = os.RemoveAll(backupDir)

	t.mu.Lock()
	t.cloneReady = true
	t.browser = newClaudeWebBrowserTransport(cloneDir, t.chrome, profileName)
	t.mu.Unlock()
	return nil
}

func (t *claudeWebNativeTransport) browserTransport() (*claudeWebBrowserTransport, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.browser, t.cloneReady && t.browser != nil
}

func (t *claudeWebNativeTransport) OpenLogin(ctx context.Context) error {
	if err := t.Close(ctx); err != nil {
		return err
	}
	if err := t.syncProfileClone(ctx); err == nil {
		if browser, ok := t.browserTransport(); ok {
			probeCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
			probe, probeErr := browser.Probe(probeCtx)
			cancel()
			_ = browser.Close(context.Background())
			if probeErr == nil && probe.Connected {
				return nil
			}
		}
	}
	return t.launchLoginWindow(ctx)
}

func (t *claudeWebNativeTransport) Probe(ctx context.Context) (claudeWebProbe, error) {
	t.mu.Lock()
	loginOpen := t.loginHWND != 0 && claudeWebNativeWindowAlive(t.loginHWND)
	cloneReady := t.cloneReady
	t.mu.Unlock()

	if loginOpen || !cloneReady {
		if err := t.syncProfileClone(ctx); err != nil {
			return claudeWebProbe{}, err
		}
	}
	browser, ok := t.browserTransport()
	if !ok {
		return claudeWebProbe{}, errors.New("Claude Web profile clone is unavailable")
	}
	return browser.Probe(ctx)
}

func (t *claudeWebNativeTransport) Complete(ctx context.Context, prompt string) (string, error) {
	browser, ok := t.browserTransport()
	if !ok {
		if err := t.syncProfileClone(ctx); err != nil {
			return "", err
		}
		browser, ok = t.browserTransport()
		if !ok {
			return "", errors.New("Claude Web profile clone is unavailable")
		}
	}
	return browser.Complete(ctx, prompt)
}

func (t *claudeWebNativeTransport) Close(ctx context.Context) error {
	t.mu.Lock()
	browser := t.browser
	loginHWND := t.loginHWND
	t.loginHWND = 0
	t.mu.Unlock()

	var firstErr error
	if browser != nil {
		if err := browser.Close(ctx); err != nil {
			firstErr = err
		}
	}
	if err := claudeWebCloseNativeWindow(loginHWND); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (t *claudeWebNativeTransport) Reset(ctx context.Context) error {
	if err := t.Close(ctx); err != nil {
		return err
	}
	t.mu.Lock()
	cloneDir := t.cloneDir
	t.cloneReady = false
	t.mu.Unlock()
	if strings.TrimSpace(cloneDir) == "" {
		cloneDir = claudeWebCloneDirectory()
	}
	return os.RemoveAll(cloneDir)
}
