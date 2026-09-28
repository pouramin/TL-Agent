package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSettingsExposeGenericPluginsSurface(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(releaseRepoRoot(t), "cmd", "launcher", "web", "index.html"))
	if err != nil { t.Fatal(err) }
	index := string(data)
	source := readBrowserSource(t, "plugins.ts")

	for _, expected := range []string{
		`data-settings-section="plugins"`,
		`data-settings-panel="plugins"`,
		`id="pluginAddButton"`,
		`id="pluginCommandInput"`,
		`id="pluginArgsInput"`,
		`placeholder="/c&#10;npx&#10;-y&#10;package-name"`,
		`One argument per line. Enter only arguments here; the executable belongs in Command.`,
		`settings-primary-action`,
		`id="pluginEnvInput"`,
		`id="pluginScopeSelect"`,
		`Test Connection`,
	} {
		if !strings.Contains(index, expected) {
			t.Fatalf("Plugins settings surface is missing %q", expected)
		}
	}
	for _, expected := range []string{
		"K.api.plugins.testConfig",
		"K.api.plugins.create",
		"K.api.plugins.update",
		"K.api.plugins.setEnabled",
		"K.api.plugins.remove",
		"K.api.plugins.saved()",
		"K.api.plugins.attach",
		"Saved for another project",
		"Use in current project",
		`transport: transportSelect.value || "stdio"`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("generic Plugins UI is missing %q", expected)
		}
	}

	cssData, err := os.ReadFile(filepath.Join(releaseRepoRoot(t), "cmd", "launcher", "web", "settings.css"))
	if err != nil { t.Fatal(err) }
	css := string(cssData)
	for _, expected := range []string{
		".plugin-arguments-field {",
		".plugin-arguments-field textarea:focus",
		"border: 1px solid var(--line);",
		"background: var(--panel-2);",
		".plugin-form-grid input::placeholder,",
		"color: #606975;",
		"border-color: #485260;",
		".plugin-empty { display: grid; gap: 3px; padding: 11px 13px;",
	} {
		if !strings.Contains(css, expected) {
			t.Fatalf("Plugins settings styling is missing %q", expected)
		}
	}
	for _, forbidden := range []string{
		"border: 1px solid color-mix(in srgb,var(--accent),var(--line) 78%);",
		"box-shadow: 0 0 0 2px color-mix(in srgb,var(--accent),transparent 82%);",
	} {
		if strings.Contains(css, forbidden) {
			t.Fatalf("Arguments field must not keep the rejected double/accent border treatment: %q", forbidden)
		}
	}
}

func TestPluginsUIKeepsIntegrationsGeneric(t *testing.T) {
	source := readBrowserSource(t, "plugins.ts")
	for _, expected := range []string{
		"plugin.integration?.actions",
		"K.api.plugins.action",
		`descriptor.kind === "preview"`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("generic integration action handling is missing %q", expected)
		}
	}
	for _, forbidden := range []string{
		"buildGraphify",
		"mcp.graphify.query_graph",
		"mcp.graphify.shortest_path",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("browser must not hardcode Graphify integration internals: found %q", forbidden)
		}
	}
}

func TestPluginPreviewActionsUseExistingPreview(t *testing.T) {
	source := readBrowserSource(t, "plugins.ts")
	for _, expected := range []string{
		"K.preview?.open?.()",
		"K.preview?.selectEntry?.(path)",
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("Plugin preview actions must reuse TL Studio Preview: missing %q", expected)
		}
	}
}


func TestPluginEditorResetsAfterCompletion(t *testing.T) {
	source := readBrowserSource(t, "plugins.ts")
	for _, expected := range []string{
		"const resetEditor = () =>",
		"nameInput.value = \"\"",
		"commandInput.value = \"\"",
		"argsInput.value = \"\"",
		"envInput.value = \"\"",
		"settingsDialog?.addEventListener(\"close\", closeEditor)",
		"closeEditor();",
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("Plugin editor reset behavior is missing %q", expected)
		}
	}
}


func TestPluginsUIDistinguishesBundledAndUserAddedWithoutForkingExecution(t *testing.T) {
	source := readBrowserSource(t, "plugins.ts")
	for _, expected := range []string{
		`plugin.origin === "bundled"`,
		`Included with TL Studio`,
		`Added by you`,
		`actionButton(plugin.enabled ? "Disable" : "Enable"`,
		`actionButton("Test Connection", "test", plugin.id)`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("bundled/user Plugin UI distinction is missing %q", expected)
		}
	}
	if strings.Contains(source, "bundledPluginExecute") || strings.Contains(source, "executeBundled") {
		t.Fatal("bundled plugins must not gain a browser-side execution path")
	}
}
