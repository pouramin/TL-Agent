package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLocalFileRoutesListAndRead(t *testing.T) {
	project := t.TempDir()
	if err := os.Mkdir(filepath.Join(project, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "index.html"), []byte("<h1>Hello</h1>\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "src", "app.js"), []byte("console.log('ok')\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	state := &appState{project: project}
	mux := http.NewServeMux()
	registerLocalFileRoutes(mux, state)
	server := httptest.NewServer(mux)
	defer server.Close()

	res, err := http.Get(server.URL + "/local/files")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected list status: %d", res.StatusCode)
	}
	var list localFileList
	if err := json.NewDecoder(res.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(list.Entries))
	}
	if list.Entries[0].Type != "directory" || list.Entries[0].Name != "src" {
		t.Fatalf("expected directory first, got %#v", list.Entries[0])
	}

	res, err = http.Get(server.URL + "/local/file?path=index.html")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected read status: %d", res.StatusCode)
	}
	var preview localFilePreview
	if err := json.NewDecoder(res.Body).Decode(&preview); err != nil {
		t.Fatal(err)
	}
	if preview.Binary || preview.Content != "<h1>Hello</h1>\n" || preview.Path != "index.html" {
		t.Fatalf("unexpected preview: %#v", preview)
	}
}

func TestResolveProjectEntryRejectsTraversal(t *testing.T) {
	project := t.TempDir()
	outside := filepath.Join(filepath.Dir(project), "outside.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(outside) })

	for _, candidate := range []string{"../outside.txt", "../../outside.txt"} {
		if _, _, err := resolveProjectEntry(project, candidate); err == nil {
			t.Fatalf("expected traversal rejection for %q", candidate)
		}
	}
}

func TestResolveProjectEntryRejectsSymlinkEscape(t *testing.T) {
	project := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(project, "escape")
	if err := os.Symlink(outside, link); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlink unavailable on Windows runner: %v", err)
		}
		t.Fatal(err)
	}
	if _, _, err := resolveProjectEntry(project, "escape/secret.txt"); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("expected symlink escape rejection, got %v", err)
	}
}

func TestLocalFileRouteMarksBinary(t *testing.T) {
	project := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, "blob.bin"), []byte{0x00, 0x01, 0x02}, 0o600); err != nil {
		t.Fatal(err)
	}
	state := &appState{project: project}
	mux := http.NewServeMux()
	registerLocalFileRoutes(mux, state)
	server := httptest.NewServer(mux)
	defer server.Close()

	res, err := http.Get(server.URL + "/local/file?path=blob.bin")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var preview localFilePreview
	if err := json.NewDecoder(res.Body).Decode(&preview); err != nil {
		t.Fatal(err)
	}
	if !preview.Binary || preview.Content != "" {
		t.Fatalf("expected binary preview without content: %#v", preview)
	}
}
