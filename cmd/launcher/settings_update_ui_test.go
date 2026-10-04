package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSettingsAboutExposesVerifiedUpdater(t *testing.T) {
	root := releaseRepoRoot(t)
	indexBytes, err := os.ReadFile(filepath.Join(root, "cmd", "launcher", "web", "index.html"))
	if err != nil { t.Fatal(err) }
	index := string(indexBytes)
	for _, required := range []string{
		`id="aboutUpdateCard"`,
		`id="aboutUpdateStatus"`,
		`id="aboutUpdateCheck"`,
		`id="aboutUpdateApply"`,
	} {
		if !strings.Contains(index, required) {
			t.Fatalf("About updater UI missing %q", required)
		}
	}

	product := readBrowserSource(t, "product-ui.ts")
	for _, required := range []string{
		"K.api.updates.status()",
		"K.api.updates.apply()",
		"Checking GitHub Releases",
		"PowerShell updater started",
	} {
		if !strings.Contains(product, required) {
			t.Fatalf("About updater behavior missing %q", required)
		}
	}

	api := readBrowserSource(t, "runtime-api.ts")
	for _, required := range []string{
		`status: () => K.request("/local/update")`,
		`apply: () => K.request("/local/update", { method: "POST" })`,
	} {
		if !strings.Contains(api, required) {
			t.Fatalf("Runtime updater API missing %q", required)
		}
	}

	cssBytes, err := os.ReadFile(filepath.Join(root, "cmd", "launcher", "web", "settings.css"))
	if err != nil { t.Fatal(err) }
	css := string(cssBytes)
	if !strings.Contains(css, ".settings-window { box-sizing:border-box; width:920px; min-width:920px; max-width:calc(100vw - 36px);") {
		t.Fatal("Settings must keep one stable desktop width across sections")
	}
	if strings.Contains(css, "settings-window-plugins") {
		t.Fatal("Settings must not change width for the Plugins section")
	}
}
