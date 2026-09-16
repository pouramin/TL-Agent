package main

import (
	"strings"
	"testing"
)

func TestBrowserIDEFoundationIsEmbeddedAndWired(t *testing.T) {
	filesJS, err := webFS.ReadFile("web/files.js")
	if err != nil {
		t.Fatalf("read embedded files.js: %v", err)
	}
	filesText := string(filesJS)
	for _, expected := range []string{
		`K.state.editorTabs = []`,
		`expectedSha256: tab.sha256`,
		`error.status === 409`,
		`method: "POST"`,
		`method: "PATCH"`,
		`method: "DELETE"`,
		`event.key.toLowerCase() === "s"`,
		`Ask Agent`,
	} {
		if !strings.Contains(filesText, expected) {
			t.Fatalf("files.js is missing IDE behavior %q", expected)
		}
	}

	hardeningJS, err := webFS.ReadFile("web/ide-foundation.js")
	if err != nil {
		t.Fatalf("read embedded ide-foundation.js: %v", err)
	}
	hardeningText := string(hardeningJS)
	for _, expected := range []string{
		`refreshOpenTabs`,
		`beforeunload`,
		`externalChanged`,
		`Switch projects and discard all unsaved editor changes?`,
		`K.workspaceFiles`,
		`K.state.local?.platform === "windows"`,
	} {
		if !strings.Contains(hardeningText, expected) {
			t.Fatalf("ide-foundation.js is missing reconciliation behavior %q", expected)
		}
	}
	if strings.Contains(filesText+hardeningText, "cdn.") || strings.Contains(filesText+hardeningText, "unpkg.com") || strings.Contains(filesText+hardeningText, "jsdelivr.net") {
		t.Fatal("browser IDE must not depend on a runtime CDN")
	}

	appJS, err := webFS.ReadFile("web/app.js")
	if err != nil {
		t.Fatalf("read embedded app.js: %v", err)
	}
	if !strings.Contains(string(appJS), `"/ide-foundation.js"`) {
		t.Fatal("app.js does not load the browser IDE reconciliation extension")
	}

	css, err := webFS.ReadFile("web/files.css")
	if err != nil {
		t.Fatalf("read embedded files.css: %v", err)
	}
	styles := string(css)
	for _, expected := range []string{".file-tabs", ".file-editor-input", ".file-tab.external-change"} {
		if !strings.Contains(styles, expected) {
			t.Fatalf("files.css is missing IDE style %q", expected)
		}
	}
}
