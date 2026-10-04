package main

import (
	"strings"
	"testing"
)

func TestMainWorkspaceContract(t *testing.T) {
	source := readBrowserSource(t, "workbench.ts")
	entry := readBrowserSource(t, "browser.ts")

	if !strings.Contains(entry, "import \"./workbench\";") {
		t.Fatal("Browser module graph does not include the main workspace")
	}

	for _, required := range []string{
		"workspace-topbar",
		"workspace-activity-rail",
		"workspace-activity-icon",
		"activityIcon(\"terminal\")",
		"workspace-context-sidebar",
		"workspace-agent-panel",
		"workspace-statusbar",
		"tl-studio.workspace-layout.v1",
		"[\"collapsed\", \"compact\", \"focused\"]",
		"document.addEventListener(\"pointermove\"",
		"beginResize(\"sidebar\"",
		"beginResize(\"agent\"",
		"beginResize(\"terminal\"",
		"event.key.toLowerCase() === \"k\"",
		"\"filesButton\"",
		"\"searchButton\"",
		"\"changesButton\"",
		"\"previewButton\"",
		"\"terminalButton\"",
		"\"modelSelect\"",
		"\"settingsButton\"",
		"K.openWorkspace",
		"AGENT_COMPACT_MAX_WIDTH = 420",
		"AGENT_FOCUSED_MIN_WIDTH = 520",
		"Math.max(layout.agentWidth, AGENT_FOCUSED_MIN_WIDTH)",
		"Math.min(layout.agentWidth, AGENT_COMPACT_MAX_WIDTH)",
		"compactAgent.setAttribute(\"aria-pressed\"",
		"focusAgent.setAttribute(\"aria-pressed\"",
		"layout.agentWidth = defaults.agentWidth",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("workbench.ts missing %q", required)
		}
	}

	for _, forbidden := range []string{
		"Extensions marketplace",
		"Debug Console",
		"Ports panel",
		"Replace in Files",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("workbench exposes unsupported prototype feature %q", forbidden)
		}
	}

	cssBytes, err := webFS.ReadFile("web/workbench.css")
	if err != nil {
		t.Fatalf("read embedded workbench.css: %v", err)
	}
	css := string(cssBytes)
	for _, required := range []string{
		"--tl-brand-navy: #212E4E",
		"--tl-brand-green: #008036",
		"--tl-bg: #0B0F17",
		"--workspace-context-width",
		"--workspace-agent-width",
		"--workspace-terminal-height",
		"--workspace-rail-width: 54px",
		"padding: 12px 18px 12px 12px",
		"overflow-wrap: anywhere",
		"minmax(0, 1fr)",
		".workspace-context-sidebar",
		".workspace-agent-panel",
		".workspace-terminal-panel.terminal-panel",
		".workspace-terminal-activity",
		"var(--tl-message-size)",
		"var(--tl-terminal-size)",
		".workspace-command-palette",
		"html[data-resolved-theme=\"light\"]",
		"@media (max-width: 1350px)",
		"@media (max-width: 1120px)",
		"@media (max-width: 880px)",
		"flex-wrap: wrap",
		"flex: 1 1 180px",
		"max-width: 100%",
		"minmax(0, 1fr)",
	} {
		if !strings.Contains(css, required) {
			t.Fatalf("workbench.css missing %q", required)
		}
	}

	indexBytes, err := webFS.ReadFile("web/index.html")
	if err != nil {
		t.Fatalf("read embedded index.html: %v", err)
	}
	index := string(indexBytes)
	if !strings.Contains(index, `href="https://pouramin.dev"`) || !strings.Contains(index, ">pouramin.dev</small>") {
		t.Fatal("About panel must link to the TL Studio website")
	}
	if strings.Contains(index, "<small>Coming soon</small>") {
		t.Fatal("About panel still contains the old website placeholder")
	}
}

func TestPreviewUsesDocumentLevelPointerLifecycle(t *testing.T) {
	source := readBrowserSource(t, "preview-floating.ts")
	for _, required := range []string{
		"document.addEventListener(\"pointermove\", moveResize)",
		"document.addEventListener(\"pointerup\", endEdgeResize)",
		"document.addEventListener(\"pointercancel\", endEdgeResize)",
		"document.addEventListener(\"pointermove\", moveDrag)",
		"document.addEventListener(\"pointerup\", endDrag)",
		"document.addEventListener(\"pointercancel\", endDrag)",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("preview pointer lifecycle missing %q", required)
		}
	}
}
