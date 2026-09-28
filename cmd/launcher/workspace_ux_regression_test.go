package main

import (
	"strings"
	"testing"
)

func TestWorkspaceUXEnhancementsContract(t *testing.T) {
	floating := readBrowserSource(t, "preview-floating.ts")
	for _, required := range []string{"tl-studio.preview-window", "pointerdown", "ResizeObserver", "previewWindow", "localStorage"} {
		if !strings.Contains(floating, required) { t.Fatalf("preview-floating.ts missing %q", required) }
	}

	editor := readBrowserSource(t, "editor-enhancements.ts")
	for _, required := range []string{"file-editor-highlight", "tok-keyword", "tok-string", "MutationObserver", "refreshEditorHighlight", "monaco-ready"} {
		if !strings.Contains(editor, required) { t.Fatalf("editor-enhancements.ts missing %q", required) }
	}

	editorCSS, err := webFS.ReadFile("web/editor-enhancements.css")
	if err != nil { t.Fatalf("read editor-enhancements.css: %v", err) }
	if !strings.Contains(string(editorCSS), ".file-editor-surface.monaco-ready .file-editor-highlight") {
		t.Fatal("legacy syntax layer is not hidden when Monaco is active")
	}
	if strings.Contains(string(editorCSS), ".file-editor-surface { position: relative; }") {
		t.Fatal("editor enhancements must not override the workspace surface positioning")
	}

	settings := readBrowserSource(t, "settings-enhancements.ts")
	for _, required := range []string{"Editor color theme", "UI Font", "Code Font", "Terminal Font", "resetPreviewWindow", "tl-studio.editor-theme", "permission-rule-group", "permission-rule-group-body", "openPermissionGroup", "Plugins & MCP"} {
		if !strings.Contains(settings, required) { t.Fatalf("settings-enhancements.ts missing %q", required) }
	}

	product := readBrowserSource(t, "product-ui.ts")
	if !strings.Contains(product, "tl-studio:font-size") {
		t.Fatal("interface size changes must be broadcast to workspace/editor surfaces")
	}
	monaco := readBrowserSource(t, "monaco.ts")
	for _, required := range []string{"editorTypography", "tl-studio:font-size", "fontSize: 15"} {
		if !strings.Contains(monaco, required) { t.Fatalf("monaco.ts missing scalable typography contract %q", required) }
	}

	entry := readBrowserSource(t, "browser.ts")
	for _, required := range []string{`import "./editor-enhancements";`, `import "./preview-floating";`, `import "./settings-enhancements";`} {
		if !strings.Contains(entry, required) { t.Fatalf("Browser module graph missing %s", required) }
	}

	combined := floating + editor + settings
	for _, forbidden := range []string{"unpkg", "jsdelivr", "cdn.jsdelivr", "cdnjs"} {
		if strings.Contains(strings.ToLower(combined), forbidden) { t.Fatalf("workspace UX must not depend on external CDN: %s", forbidden) }
	}
}
