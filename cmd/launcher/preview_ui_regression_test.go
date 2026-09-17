package main

import (
	"io/fs"
	"strings"
	"testing"
)

func TestEmbeddedLivePreviewUIContract(t *testing.T) {
	assets, err := fs.Sub(webFS, "web")
	if err != nil {
		t.Fatal(err)
	}
	js, err := fs.ReadFile(assets, "preview.js")
	if err != nil {
		t.Fatal(err)
	}
	css, err := fs.ReadFile(assets, "preview.css")
	if err != nil {
		t.Fatal(err)
	}
	app, err := fs.ReadFile(assets, "app.js")
	if err != nil {
		t.Fatal(err)
	}

	text := string(js)
	for _, required := range []string{
		"Live Preview",
		"/local/preview/info",
		"/local/preview/version",
		"/local/preview/bridge",
		"/local/process",
		"window.confirm",
		"K.preview",
		"discoverServerURL",
		"preview-open",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("preview.js missing %q", required)
		}
	}
	if strings.Contains(text, "cdn.") || strings.Contains(text, "unpkg") || strings.Contains(text, "jsdelivr") {
		t.Fatal("live preview UI must not depend on external CDN assets")
	}
	if !strings.Contains(string(css), ".live-preview-pane") || !strings.Contains(string(css), ".files-workspace.preview-open") {
		t.Fatal("preview.css missing docked preview layout")
	}
	if !strings.Contains(string(app), `"/preview.js"`) || !strings.Contains(string(app), "K.__previewInstalled") {
		t.Fatal("app.js does not load the live preview extension")
	}
}
