//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type claudeWebBrowserSession interface {
	OpenLogin(context.Context) error
	Probe(context.Context) (claudeWebProbe, error)
	Complete(context.Context, string) (string, error)
	Close(context.Context) error
}

type claudeWebNativeTransport struct {
	mu          sync.Mutex
	operationMu sync.Mutex
	chrome      string
	userDataDir string
	profileName string
	sessionDir  string
	runtimeRoot string
	login       claudeWebBrowserSession
	runtime     claudeWebBrowserSession
	runtimeDir  string
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
	return claudeWebProfileFromUserData(userData)
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
	t.mu.Lock()
	t.chrome = chrome
	t.userDataDir = userDataDir
	t.profileName = profileName
	t.sessionDir = claudeWebSessionDirectory()
	t.runtimeRoot = claudeWebRuntimeRootDirectory()
	t.mu.Unlock()
	return nil
}

func (t *claudeWebNativeTransport) snapshot() (chrome, userDataDir, profileName, sessionDir, runtimeRoot string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.chrome, t.userDataDir, t.profileName, t.sessionDir, t.runtimeRoot
}

func (t *claudeWebNativeTransport) ensureAvailable() error {
	chrome, userDataDir, profileName, sessionDir, _ := t.snapshot()
	if chrome != "" && userDataDir != "" && profileName != "" && sessionDir != "" {
		return nil
	}
	return t.Available()
}

func (t *claudeWebNativeTransport) ensureSessionClone(ctx context.Context) error {
	if err := t.ensureAvailable(); err != nil {
		return err
	}
	_, userDataDir, profileName, sessionDir, _ := t.snapshot()
	if claudeWebCloneReady(sessionDir, profileName) {
		return nil
	}
	if err := replaceClaudeWebProfileClone(ctx, userDataDir, sessionDir, profileName); err != nil {
		return fmt.Errorf("prepare Claude Web session clone: %w", err)
	}
	return nil
}

func (t *claudeWebNativeTransport) refreshSessionCloneFromChrome(ctx context.Context) error {
	if err := t.ensureAvailable(); err != nil {
		return err
	}
	_, userDataDir, profileName, sessionDir, _ := t.snapshot()
	if err := replaceClaudeWebProfileClone(ctx, userDataDir, sessionDir, profileName); err != nil {
		return fmt.Errorf("refresh Claude Web session clone from Chrome: %w", err)
	}
	return nil
}

func (t *claudeWebNativeTransport) makeBrowser(profileDir string) claudeWebBrowserSession {
	chrome, _, profileName, _, _ := t.snapshot()
	return newClaudeWebBrowserTransport(profileDir, chrome, profileName)
}

func (t *claudeWebNativeTransport) cleanupRuntime(ctx context.Context) error {
	t.mu.Lock()
	browser := t.runtime
	dir := t.runtimeDir
	runtimeRoot := t.runtimeRoot
	t.runtime = nil
	t.runtimeDir = ""
	t.mu.Unlock()

	var firstErr error
	if browser != nil {
		if err := browser.Close(ctx); err != nil {
			firstErr = err
		}
	}
	if strings.TrimSpace(dir) != "" {
		if err := os.RemoveAll(dir); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if strings.TrimSpace(runtimeRoot) != "" {
		_ = os.Remove(runtimeRoot)
	}
	return firstErr
}

func (t *claudeWebNativeTransport) withRuntime(ctx context.Context, fn func(claudeWebBrowserSession) error) error {
	t.operationMu.Lock()
	defer t.operationMu.Unlock()

	if err := t.cleanupRuntime(context.Background()); err != nil {
		return err
	}
	if err := t.ensureSessionClone(ctx); err != nil {
		return err
	}
	_, _, profileName, sessionDir, runtimeRoot := t.snapshot()
	if err := os.MkdirAll(runtimeRoot, 0o700); err != nil {
		return fmt.Errorf("create Claude Web runtime directory: %w", err)
	}
	runtimeDir, err := os.MkdirTemp(runtimeRoot, "run-")
	if err != nil {
		return fmt.Errorf("create Claude Web temporary runtime: %w", err)
	}
	if err := cloneClaudeWebProfile(ctx, sessionDir, runtimeDir, profileName); err != nil {
		_ = os.RemoveAll(runtimeDir)
		return fmt.Errorf("clone Claude Web runtime profile: %w", err)
	}
	browser := t.makeBrowser(runtimeDir)
	t.mu.Lock()
	t.runtime = browser
	t.runtimeDir = runtimeDir
	t.mu.Unlock()
	defer t.cleanupRuntime(context.Background())
	return fn(browser)
}

func (t *claudeWebNativeTransport) probeHeadless(ctx context.Context) (claudeWebProbe, error) {
	var probe claudeWebProbe
	err := t.withRuntime(ctx, func(browser claudeWebBrowserSession) error {
		var err error
		probe, err = browser.Probe(ctx)
		return err
	})
	return probe, err
}

func (t *claudeWebNativeTransport) startLoginBrowser(ctx context.Context) error {
	if err := t.ensureSessionClone(ctx); err != nil {
		return err
	}
	_, _, _, sessionDir, _ := t.snapshot()
	browser := t.makeBrowser(sessionDir)
	if err := browser.OpenLogin(ctx); err != nil {
		return err
	}
	t.mu.Lock()
	t.login = browser
	t.mu.Unlock()
	return nil
}

func (t *claudeWebNativeTransport) OpenLogin(ctx context.Context) error {
	if err := t.Close(context.Background()); err != nil {
		return err
	}
	if err := t.ensureSessionClone(ctx); err != nil {
		return err
	}
	if probe, err := t.probeHeadless(ctx); err == nil && probe.Connected {
		return nil
	}

	// A stale dedicated session clone is allowed to fall back to a fresh
	// read-only clone of the user's current Chrome profile before any visible
	// login UI is shown. The original profile is never used as a browser
	// user-data-dir by TL Studio and is never modified.
	if err := t.refreshSessionCloneFromChrome(ctx); err == nil {
		if probe, probeErr := t.probeHeadless(ctx); probeErr == nil && probe.Connected {
			return nil
		}
	}
	return t.startLoginBrowser(ctx)
}

func (t *claudeWebNativeTransport) Probe(ctx context.Context) (claudeWebProbe, error) {
	t.mu.Lock()
	login := t.login
	t.mu.Unlock()
	if login != nil {
		return login.Probe(ctx)
	}
	return t.probeHeadless(ctx)
}

func (t *claudeWebNativeTransport) Complete(ctx context.Context, prompt string) (string, error) {
	var response string
	err := t.withRuntime(ctx, func(browser claudeWebBrowserSession) error {
		var err error
		response, err = browser.Complete(ctx, prompt)
		return err
	})
	return response, err
}

func (t *claudeWebNativeTransport) Close(ctx context.Context) error {
	t.mu.Lock()
	login := t.login
	t.login = nil
	t.mu.Unlock()

	var firstErr error
	if login != nil {
		if err := login.Close(ctx); err != nil {
			firstErr = err
		}
	}
	if err := t.cleanupRuntime(ctx); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (t *claudeWebNativeTransport) Reset(ctx context.Context) error {
	if err := t.Close(ctx); err != nil {
		return err
	}
	t.mu.Lock()
	sessionDir := t.sessionDir
	runtimeRoot := t.runtimeRoot
	if strings.TrimSpace(sessionDir) == "" {
		sessionDir = claudeWebSessionDirectory()
	}
	if strings.TrimSpace(runtimeRoot) == "" {
		runtimeRoot = claudeWebRuntimeRootDirectory()
	}
	t.mu.Unlock()
	var firstErr error
	if err := os.RemoveAll(sessionDir); err != nil {
		firstErr = err
	}
	if err := os.RemoveAll(runtimeRoot); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}
