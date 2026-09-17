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

func TestDetectPreviewCapability(t *testing.T) {
	staticProject := t.TempDir()
	if err := os.WriteFile(filepath.Join(staticProject, "index.html"), []byte("<h1>hello</h1>"), 0o600); err != nil {
		t.Fatal(err)
	}
	static := detectPreviewCapability(staticProject)
	if !static.Available || static.Kind != "static" || static.Entry != "index.html" {
		t.Fatalf("unexpected static capability: %#v", static)
	}

	nodeProject := t.TempDir()
	manifest := `{"scripts":{"dev":"vite"}}`
	if err := os.WriteFile(filepath.Join(nodeProject, "package.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nodeProject, "index.html"), []byte("<h1>vite</h1>"), 0o600); err != nil {
		t.Fatal(err)
	}
	node := detectPreviewCapability(nodeProject)
	if !node.Available || node.Kind != "dev-server" || node.Command != "npm run dev" {
		t.Fatalf("unexpected dev capability: %#v", node)
	}

	empty := detectPreviewCapability(t.TempDir())
	if empty.Available || empty.Reason == "" {
		t.Fatalf("unexpected empty capability: %#v", empty)
	}
}

func TestDetectPreviewURLAcceptsLoopbackOnly(t *testing.T) {
	cases := map[string]string{
		"Local: http://localhost:5173/":  "http://localhost:5173/",
		"http://127.0.0.1:3000/app":     "http://127.0.0.1:3000/",
		"http://0.0.0.0:8080":          "http://127.0.0.1:8080/",
		"https://example.com:443":       "",
		"http://192.168.1.12:4173":      "",
		"no preview url in this output": "",
	}
	for input, want := range cases {
		if got := detectPreviewURL(input); got != want {
			t.Fatalf("detectPreviewURL(%q)=%q want %q", input, got, want)
		}
	}
}

func TestStaticPreviewRoutesAndCSP(t *testing.T) {
	project := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, "index.html"), []byte("<!doctype html><title>Preview OK</title>"), 0o600); err != nil {
		t.Fatal(err)
	}
	state := &appState{project: project}
	mux := http.NewServeMux()
	registerLocalProcessRoutes(mux, state)
	server := httptest.NewServer(mux)
	defer server.Close()

	statusRes, err := http.Get(server.URL + "/local/preview")
	if err != nil {
		t.Fatal(err)
	}
	var before previewSnapshot
	if err := json.NewDecoder(statusRes.Body).Decode(&before); err != nil {
		t.Fatal(err)
	}
	statusRes.Body.Close()
	if !before.Available || before.Kind != "static" || before.Running {
		t.Fatalf("before=%#v", before)
	}

	startRes, err := http.Post(server.URL+"/local/preview", "application/json", strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	var started previewSnapshot
	if err := json.NewDecoder(startRes.Body).Decode(&started); err != nil {
		t.Fatal(err)
	}
	startRes.Body.Close()
	if startRes.StatusCode != http.StatusCreated || !started.Running || started.URL == "" {
		t.Fatalf("start status=%d snapshot=%#v", startRes.StatusCode, started)
	}
	if !strings.HasPrefix(started.URL, "http://127.0.0.1:") {
		t.Fatalf("preview URL must be loopback: %q", started.URL)
	}

	previewRes, err := http.Get(started.URL)
	if err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 1024)
	n, _ := previewRes.Body.Read(data)
	previewRes.Body.Close()
	if !strings.Contains(string(data[:n]), "Preview OK") {
		t.Fatalf("unexpected preview body: %q", data[:n])
	}

	req, _ := http.NewRequest(http.MethodDelete, server.URL+"/local/preview", nil)
	stopRes, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	stopRes.Body.Close()
	if stopRes.StatusCode != http.StatusNoContent {
		t.Fatalf("stop status=%d", stopRes.StatusCode)
	}
}

func TestStaticPreviewRejectsSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation commonly requires Windows developer/admin privileges")
	}
	project := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, "index.html"), []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(project, "escape.txt")); err != nil {
		t.Fatal(err)
	}

	handler := safeStaticPreviewHandler(project)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/escape.txt", nil)
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("symlink escape status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "secret") {
		t.Fatal("preview leaked a file outside the selected project")
	}
}
