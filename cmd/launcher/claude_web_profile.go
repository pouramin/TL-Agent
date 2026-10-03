package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

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

func claudeWebSessionDirectory() string {
	return filepath.Join(tlStudioStateDirectory(), "claude-web-session")
}

func claudeWebRuntimeRootDirectory() string {
	return filepath.Join(tlStudioStateDirectory(), "claude-web-runtime")
}

func claudeWebProfileFromUserData(userDataDir string) (string, error) {
	userDataDir = strings.TrimSpace(userDataDir)
	if userDataDir == "" {
		return "", errors.New("Chrome user-data directory is unavailable")
	}
	data, err := os.ReadFile(filepath.Join(userDataDir, "Local State"))
	if err == nil {
		var state struct {
			Profile struct {
				LastUsed string `json:"last_used"`
			} `json:"profile"`
		}
		if json.Unmarshal(data, &state) == nil {
			if profile := strings.TrimSpace(state.Profile.LastUsed); profile != "" {
				if info, statErr := os.Stat(filepath.Join(userDataDir, profile)); statErr == nil && info.IsDir() {
					return profile, nil
				}
			}
		}
	}
	if info, statErr := os.Stat(filepath.Join(userDataDir, "Default")); statErr == nil && info.IsDir() {
		return "Default", nil
	}
	if err != nil {
		return "", fmt.Errorf("read Chrome Local State: %w", err)
	}
	return "", errors.New("Chrome's active profile could not be determined")
}

func claudeWebCloneReady(userDataDir, profileName string) bool {
	if strings.TrimSpace(userDataDir) == "" || strings.TrimSpace(profileName) == "" {
		return false
	}
	if info, err := os.Stat(filepath.Join(userDataDir, "Local State")); err != nil || info.IsDir() {
		return false
	}
	info, err := os.Stat(filepath.Join(userDataDir, profileName))
	return err == nil && info.IsDir()
}

func shouldSkipClaudeWebCloneName(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return false
	}
	switch name {
	case "lock", "log", "log.old", "devtoolsactiveport",
		"singletoncookie", "singletonlock", "singletonsocket",
		"cache", "cache_data", "code cache", "gpucache", "dawncache",
		"grshadercache", "shadercache", "component_crx_cache", "extensions_crx_cache",
		"crashpad":
		return true
	}
	return strings.HasSuffix(name, ".lock") || strings.HasSuffix(name, ".tmp")
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
	if shouldSkipClaudeWebCloneName(filepath.Base(src)) {
		return nil
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
		if shouldSkipClaudeWebCloneName(entry.Name()) {
			continue
		}
		if err := copyClaudeWebPath(ctx, filepath.Join(src, entry.Name()), filepath.Join(dst, entry.Name())); err != nil {
			// Chrome can rotate SQLite helpers and other volatile files while the
			// source profile is live. Individual read failures are tolerated; the
			// page-context session probe is the authority on clone usability.
			continue
		}
	}
	return nil
}

func cloneClaudeWebProfile(ctx context.Context, sourceUserData, targetUserData, profileName string) error {
	sourceUserData = strings.TrimSpace(sourceUserData)
	targetUserData = strings.TrimSpace(targetUserData)
	profileName = strings.TrimSpace(profileName)
	if sourceUserData == "" || targetUserData == "" || profileName == "" {
		return errors.New("Claude Web profile clone paths are incomplete")
	}
	if same, _ := filepath.Abs(sourceUserData); same != "" {
		if target, _ := filepath.Abs(targetUserData); target == same {
			return errors.New("refusing to use the original Chrome profile as a Claude Web clone")
		}
	}
	if err := os.RemoveAll(targetUserData); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(targetUserData, profileName), 0o700); err != nil {
		return err
	}
	if err := copyClaudeWebFile(filepath.Join(sourceUserData, "Local State"), filepath.Join(targetUserData, "Local State")); err != nil {
		_ = os.RemoveAll(targetUserData)
		return fmt.Errorf("copy Chrome Local State for Claude Web: %w", err)
	}
	sourceProfile := filepath.Join(sourceUserData, profileName)
	targetProfile := filepath.Join(targetUserData, profileName)
	for _, relative := range claudeWebCloneEntries {
		if err := copyClaudeWebPath(ctx, filepath.Join(sourceProfile, relative), filepath.Join(targetProfile, relative)); err != nil {
			_ = os.RemoveAll(targetUserData)
			return fmt.Errorf("copy Chrome profile data %s for Claude Web: %w", relative, err)
		}
	}
	for _, name := range []string{"SingletonCookie", "SingletonLock", "SingletonSocket", "DevToolsActivePort"} {
		_ = os.RemoveAll(filepath.Join(targetUserData, name))
	}
	return nil
}

func replaceClaudeWebProfileClone(ctx context.Context, sourceUserData, targetUserData, profileName string) error {
	tempDir := targetUserData + ".sync"
	backupDir := targetUserData + ".old"
	_ = os.RemoveAll(tempDir)
	_ = os.RemoveAll(backupDir)
	if err := cloneClaudeWebProfile(ctx, sourceUserData, tempDir, profileName); err != nil {
		return err
	}
	if _, err := os.Stat(targetUserData); err == nil {
		if err := os.Rename(targetUserData, backupDir); err != nil {
			_ = os.RemoveAll(tempDir)
			return fmt.Errorf("rotate Claude Web profile clone: %w", err)
		}
	}
	if err := os.Rename(tempDir, targetUserData); err != nil {
		if _, statErr := os.Stat(backupDir); statErr == nil {
			_ = os.Rename(backupDir, targetUserData)
		}
		return fmt.Errorf("activate Claude Web profile clone: %w", err)
	}
	_ = os.RemoveAll(backupDir)
	return nil
}
