package main

import (
	"io/fs"
	"strings"
	"testing"
)

func TestProjectSearchUIContract(t *testing.T) {
	assets, err := fs.Sub(webFS, "web")
	if err != nil {
		t.Fatal(err)
	}
	searchJS, err := fs.ReadFile(assets, "search.js")
	if err != nil {
		t.Fatal(err)
	}
	filesJS, err := fs.ReadFile(assets, "files.js")
	if err != nil {
		t.Fatal(err)
	}
	appJS, err := fs.ReadFile(assets, "app.js")
	if err != nil {
		t.Fatal(err)
	}

	searchText := string(searchJS)
	for _, required := range []string{
		"/local/search",
		"projectSearchQuery",
		"projectSearchInclude",
		"projectSearchExclude",
		"projectSearchCase",
		"event.ctrlKey || event.metaKey",
		"event.shiftKey",
		"event.key.toLowerCase() === \"f\"",
		"AbortController",
		"K.openWorkspaceFileAt",
		"match.line",
		"match.column",
	} {
		if !strings.Contains(searchText, required) {
			t.Fatalf("search UI is missing contract marker %q", required)
		}
	}

	filesText := string(filesJS)
	for _, required := range []string{
		"K.openWorkspaceFileAt = openEditorAt",
		"setSelectionRange",
		"openEditor(path)",
	} {
		if !strings.Contains(filesText, required) {
			t.Fatalf("workspace editor is missing search navigation marker %q", required)
		}
	}

	if !strings.Contains(string(appJS), `"/search.js"`) {
		t.Fatal("project search extension is not loaded by app.js")
	}
}

func TestProjectSearchAddsNoRuntimeCDNDependency(t *testing.T) {
	assets, err := fs.Sub(webFS, "web")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"search.js", "files.js", "app.js"} {
		content, err := fs.ReadFile(assets, name)
		if err != nil {
			t.Fatal(err)
		}
		text := strings.ToLower(string(content))
		for _, forbidden := range []string{"https://cdn.", "https://unpkg.com", "https://esm.sh", "https://jsdelivr.net"} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s introduced runtime CDN dependency %q", name, forbidden)
			}
		}
	}
}
