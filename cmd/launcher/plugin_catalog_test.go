package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAvailablePluginCatalogIncludesGraphifyJevDirectAndLaya(t *testing.T) {
	catalog := availablePluginCatalog()
	if len(catalog) != 3 {
		t.Fatalf("expected three curated plugin presets, got %#v", catalog)
	}
	byID := map[string]pluginCatalogEntry{}
	for _, entry := range catalog {
		byID[entry.ID] = entry
		if entry.Category == "" || entry.Icon == "" || entry.Upstream == "" {
			t.Fatalf("catalog entry is missing presentation metadata: %#v", entry)
		}
		if entry.Type == pluginTypeMCP {
			if entry.Transport != pluginTransportStdio || entry.PackageSpec == "" || entry.Executable == "" || entry.Module == "" {
				t.Fatalf("MCP catalog entry is missing install/runtime metadata: %#v", entry)
			}
		}
	}

	graphify := byID["graphify"]
	if graphify.Scope != "global" || graphify.PackageSpec != "graphifyy[mcp]" ||
		len(graphify.Arguments) != 1 || graphify.Arguments[0] != "graphify-out/graph.json" ||
		graphify.Metadata["integration"] != "graphify" {
		t.Fatalf("unexpected Graphify preset: %#v", graphify)
	}

	jevDirect := byID["jev-direct"]
	if jevDirect.Scope != "global" || jevDirect.Type != pluginTypeRouter ||
		jevDirect.Environment[jevDirectAPIKeyEnv] != "" ||
		jevDirect.Metadata["endpoint"] != jevDirectEndpoint ||
		jevDirect.Metadata["model"] != jevDirectModel {
		t.Fatalf("unexpected JEV Direct preset: %#v", jevDirect)
	}

	laya := byID["laya"]
	if laya.Scope != "global" || laya.PackageSpec != "laya[mcp]" ||
		laya.Environment["LAYA_PRELOAD"] != "0" {
		t.Fatalf("unexpected Laya preset: %#v", laya)
	}
}

func TestPluginCatalogResponseHidesInstallerCommands(t *testing.T) {
	encoded, err := json.Marshal(availablePluginCatalog())
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, forbidden := range []string{"packageSpec", "executable", "module", "arguments", "installHint", "graphify-mcp", "laya-mcp-server"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("catalog response leaked backend install details %q: %s", forbidden, text)
		}
	}
	for _, required := range []string{"category", "icon", "/plugin-graphify.svg", "/plugin-jev.svg", "/plugin-laya.svg"} {
		if !strings.Contains(text, required) {
			t.Fatalf("catalog response missing presentation field %q: %s", required, text)
		}
	}
}


func TestGraphifyCatalogInstallsGloballyButKeepsProjectGraph(t *testing.T) {
	entry, ok := pluginCatalogEntryByID("graphify")
	if !ok {
		t.Fatal("Graphify catalog entry missing")
	}
	if entry.Scope != "global" {
		t.Fatalf("Graphify scope = %q, want global", entry.Scope)
	}
	if entry.Metadata["graphPath"] != "graphify-out/graph.json" {
		t.Fatalf("Graphify graph path = %q", entry.Metadata["graphPath"])
	}
	config := pluginConfig{
		ID: "graphify",
		Name: "Graphify",
		Type: pluginTypeMCP,
		Scope: "global",
		Transport: pluginTransportStdio,
		Command: "graphify-mcp",
		Arguments: []string{"graphify-out/graph.json"},
		Metadata: map[string]string{"integration": "graphify", "graphPath": "graphify-out/graph.json"},
	}
	first := t.TempDir()
	second := t.TempDir()
	if got, err := pluginWorkingDirectory(config, first); err != nil || got != first {
		t.Fatalf("global Graphify cwd for first project = %q, %v", got, err)
	}
	if got, err := pluginWorkingDirectory(config, second); err != nil || got != second {
		t.Fatalf("global Graphify cwd for second project = %q, %v", got, err)
	}
}


func TestGraphifyInstallDoesNotBuildProjectGraphImplicitly(t *testing.T) {
	source, err := os.ReadFile(filepath.Join(releaseRepoRoot(t), "cmd", "launcher", "plugin_catalog.go"))
	if err != nil { t.Fatal(err) }
	text := string(source)
	if strings.Contains(text, `if entry.ID == "graphify" {
		if _, err := runGraphifyBuild`) {
		t.Fatal("installing global Graphify must not implicitly build the current project graph")
	}
	if !strings.Contains(text, `entry.Type == pluginTypeRouter || entry.ID == "graphify"`) {
		t.Fatal("Graphify must remain installed but disabled until its project graph is built")
	}
}

func TestGraphifyBuildExportsInteractiveHTML(t *testing.T) {
	source, err := os.ReadFile(filepath.Join(releaseRepoRoot(t), "cmd", "launcher", "plugin_graphify.go"))
	if err != nil { t.Fatal(err) }
	text := string(source)
	for _, required := range []string{
		`"extract", ".", "--code-only"`,
		`"export", "html"`,
		`Graphify graph was built, but HTML export failed`,
		`manager.SetEnabled(project, config.ID, true)`,
		`"graphify-out/graph.html"`,
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("Graphify build/export contract missing %q", required)
		}
	}
}
