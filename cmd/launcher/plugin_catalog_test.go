package main

import "testing"

func TestAvailablePluginCatalogIncludesGraphifyAndLaya(t *testing.T) {
	catalog := availablePluginCatalog()
	if len(catalog) != 2 {
		t.Fatalf("expected two curated plugin presets, got %#v", catalog)
	}
	byID := map[string]pluginCatalogEntry{}
	for _, entry := range catalog {
		byID[entry.ID] = entry
		if entry.Type != pluginTypeMCP || entry.Transport != pluginTransportStdio {
			t.Fatalf("catalog entry must stay on the generic stdio MCP path: %#v", entry)
		}
		if entry.Command == "" || entry.InstallHint == "" || entry.Upstream == "" {
			t.Fatalf("catalog entry is missing setup metadata: %#v", entry)
		}
	}

	graphify := byID["graphify"]
	if graphify.Scope != "project" || graphify.Command != "graphify-mcp" ||
		len(graphify.Arguments) != 1 || graphify.Arguments[0] != "graphify-out/graph.json" {
		t.Fatalf("unexpected Graphify preset: %#v", graphify)
	}

	laya := byID["laya"]
	if laya.Scope != "global" || laya.Command != "laya-mcp-server" || len(laya.Arguments) != 0 {
		t.Fatalf("unexpected Laya preset: %#v", laya)
	}
}
