package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func writeClaudeWebProfileFixture(t *testing.T, root, profile string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, profile, "Network"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Local State"), []byte(`{"profile":{"last_used":"`+profile+`"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, profile, "Preferences"), []byte("preferences"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, profile, "Network", "Cookies"), []byte("encrypted-cookie-db"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestClaudeWebActiveChromeProfileDiscoveryUsesLastUsed(t *testing.T) {
	root := t.TempDir()
	writeClaudeWebProfileFixture(t, root, "Profile 7")
	if err := os.MkdirAll(filepath.Join(root, "Default"), 0o755); err != nil {
		t.Fatal(err)
	}
	profile, err := claudeWebProfileFromUserData(root)
	if err != nil {
		t.Fatal(err)
	}
	if profile != "Profile 7" {
		t.Fatalf("expected last-used Chrome profile, got %q", profile)
	}
}

func TestClaudeWebActiveChromeProfileDiscoveryFallsBackToDefault(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "Default"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Local State"), []byte(`{"profile":{"last_used":"Missing"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	profile, err := claudeWebProfileFromUserData(root)
	if err != nil {
		t.Fatal(err)
	}
	if profile != "Default" {
		t.Fatalf("expected Default fallback, got %q", profile)
	}
}

func TestClaudeWebProfileCloneExcludesLocksAndCaches(t *testing.T) {
	source := t.TempDir()
	target := filepath.Join(t.TempDir(), "clone")
	writeClaudeWebProfileFixture(t, source, "Profile 1")
	for _, relative := range []string{
		filepath.Join("Profile 1", "Network", "LOCK"),
		filepath.Join("Profile 1", "Network", "Cache", "entry"),
		filepath.Join("Profile 1", "Network", "Code Cache", "entry"),
		"SingletonLock",
		"DevToolsActivePort",
	} {
		path := filepath.Join(source, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("volatile"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := cloneClaudeWebProfile(context.Background(), source, target, "Profile 1"); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(target, "Profile 1", "Network", "Cookies")); err != nil || string(got) != "encrypted-cookie-db" {
		t.Fatalf("session cookie database was not cloned intact: %q err=%v", got, err)
	}
	for _, relative := range []string{
		filepath.Join("Profile 1", "Network", "LOCK"),
		filepath.Join("Profile 1", "Network", "Cache"),
		filepath.Join("Profile 1", "Network", "Code Cache"),
		"SingletonLock",
		"DevToolsActivePort",
	} {
		if _, err := os.Stat(filepath.Join(target, relative)); !os.IsNotExist(err) {
			t.Fatalf("volatile Chrome profile path was copied: %s err=%v", relative, err)
		}
	}
}

func TestClaudeWebProfileCloneNeverModifiesOriginal(t *testing.T) {
	source := t.TempDir()
	target := filepath.Join(t.TempDir(), "clone")
	writeClaudeWebProfileFixture(t, source, "Default")
	cookiePath := filepath.Join(source, "Default", "Network", "Cookies")
	before, err := os.ReadFile(cookiePath)
	if err != nil {
		t.Fatal(err)
	}
	beforeInfo, err := os.Stat(cookiePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := cloneClaudeWebProfile(context.Background(), source, target, "Default"); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(cookiePath)
	if err != nil {
		t.Fatal(err)
	}
	afterInfo, err := os.Stat(cookiePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("original Chrome cookie database content changed during clone")
	}
	if afterInfo.Size() != beforeInfo.Size() || afterInfo.Mode() != beforeInfo.Mode() {
		t.Fatal("original Chrome cookie database metadata changed during clone")
	}
}

func TestClaudeWebProfileCloneRefusesOriginalAsTarget(t *testing.T) {
	source := t.TempDir()
	writeClaudeWebProfileFixture(t, source, "Default")
	if err := cloneClaudeWebProfile(context.Background(), source, source, "Default"); err == nil {
		t.Fatal("Claude Web clone must never target the original Chrome profile")
	}
}
