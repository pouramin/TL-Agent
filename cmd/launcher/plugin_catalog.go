package main

type pluginCatalogEntry struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Type        string   `json:"type"`
	Scope       string   `json:"scope"`
	Transport   string   `json:"transport"`
	Command     string   `json:"command"`
	Arguments   []string `json:"arguments,omitempty"`
	Upstream    string   `json:"upstream,omitempty"`
	InstallHint string   `json:"installHint,omitempty"`
}

func availablePluginCatalog() []pluginCatalogEntry {
	return []pluginCatalogEntry{
		{
			ID:          "graphify",
			Name:        "Graphify",
			Description: "Local code knowledge graph exposed to the Agent over MCP.",
			Type:        pluginTypeMCP,
			Scope:       "project",
			Transport:   pluginTransportStdio,
			Command:     "graphify-mcp",
			Arguments:   []string{"graphify-out/graph.json"},
			Upstream:    "https://github.com/Graphify-Labs/graphify",
			InstallHint: `uv tool install "graphifyy[mcp]"`,
		},
		{
			ID:          "laya",
			Name:        "Laya",
			Description: "Local typed decision engine for routing, scoring, yes/no, triage, and guardrail decisions.",
			Type:        pluginTypeMCP,
			Scope:       "global",
			Transport:   pluginTransportStdio,
			Command:     "laya-mcp-server",
			Upstream:    "https://github.com/NandhaKishorM/laya",
			InstallHint: `python -m pip install "laya[mcp]"`,
		},
	}
}
