package main

import (
	"io/fs"
	"strings"
	"testing"
)

func TestEmbeddedTerminalUIContract(t *testing.T) {
	assets, err := fs.Sub(webFS, "web")
	if err != nil { t.Fatal(err) }
	js, err := fs.ReadFile(assets, "terminal.js")
	if err != nil { t.Fatal(err) }
	css, err := fs.ReadFile(assets, "terminal.css")
	if err != nil { t.Fatal(err) }
	text := string(js)
	for _, required := range []string{"Terminal", "/local/process", "terminalStop", "historyIndex", "K.terminal", "stoppedByUser", "[stopped]"} {
		if !strings.Contains(text, required) { t.Fatalf("terminal.js missing %q", required) }
	}
	if strings.Contains(text, "cdn.") || strings.Contains(text, "unpkg") || strings.Contains(text, "jsdelivr") {
		t.Fatal("terminal UI must not depend on external CDN assets")
	}
	if !strings.Contains(string(css), ".terminal-panel") { t.Fatal("terminal.css missing terminal panel styles") }
}
