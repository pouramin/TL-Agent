package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeProject(t *testing.T) {
	dir := t.TempDir()
	got, err := normalizeProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.Abs(dir)
	if got != filepath.Clean(want) {
		t.Fatalf("got %q want %q", got, want)
	}

	file := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := normalizeProject(file); err == nil {
		t.Fatal("expected a non-directory error")
	}
}

func TestIsLoopbackHost(t *testing.T) {
	for _, host := range []string{"localhost", "127.0.0.1", "::1", "[::1]"} {
		if !isLoopbackHost(host) {
			t.Fatalf("expected loopback: %s", host)
		}
	}
	for _, host := range []string{"0.0.0.0", "192.168.1.10", "example.com", ""} {
		if isLoopbackHost(host) {
			t.Fatalf("expected non-loopback: %s", host)
		}
	}
}

func TestProxyAddsAuthAndProject(t *testing.T) {
	project := t.TempDir()
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/global/health" {
			t.Fatalf("unexpected backend path: %s", r.URL.Path)
		}
		user, pass, ok := r.BasicAuth()
		if !ok || user != "kilo" || pass != "secret" {
			t.Fatalf("bad backend auth: %q %q %v", user, pass, ok)
		}
		if got := r.Header.Get("x-kilo-directory"); got == "" || !strings.Contains(got, filepath.Base(project)) {
			t.Fatalf("missing project header: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer backend.Close()

	state := &appState{project: project, backendURL: backend.URL, frontendURL: "http://127.0.0.1"}
	handler, err := newServer(state, backend.URL, "kilo", "secret")
	if err != nil {
		t.Fatal(err)
	}
	frontend := httptest.NewServer(handler)
	defer frontend.Close()

	res, err := http.Get(frontend.URL + "/runtime/global/health")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", res.StatusCode)
	}
}

func TestRootServesIndexWithoutRedirect(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer backend.Close()

	state := &appState{project: t.TempDir(), backendURL: backend.URL, frontendURL: "http://127.0.0.1"}
	handler, err := newServer(state, backend.URL, "kilo", "secret")
	if err != nil {
		t.Fatal(err)
	}
	frontend := httptest.NewServer(handler)
	defer frontend.Close()

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	res, err := client.Get(frontend.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 at root, got %d", res.StatusCode)
	}
	if location := res.Header.Get("Location"); location != "" {
		t.Fatalf("root must not redirect, got Location %q", location)
	}
	if contentType := res.Header.Get("Content-Type"); !strings.Contains(contentType, "text/html") {
		t.Fatalf("expected HTML root, got %q", contentType)
	}
}

func TestLocalProjectEndpoint(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer backend.Close()

	original := t.TempDir()
	next := t.TempDir()
	state := &appState{project: original, backendURL: backend.URL, frontendURL: "http://127.0.0.1"}
	handler, err := newServer(state, backend.URL, "kilo", "secret")
	if err != nil {
		t.Fatal(err)
	}
	frontend := httptest.NewServer(handler)
	defer frontend.Close()

	body := strings.NewReader(`{"path":` + mustJSON(t, next) + `}`)
	req, _ := http.NewRequest(http.MethodPost, frontend.URL+"/local/project", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", frontend.URL)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", res.StatusCode)
	}
	if state.projectPath() != filepath.Clean(next) {
		t.Fatalf("project was not updated: %q", state.projectPath())
	}
}

func TestBlocksCrossOriginRequests(t *testing.T) {
	handler := localOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:1234/local/project", strings.NewReader("{}"))
	req.Host = "127.0.0.1:1234"
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func mustJSON(t *testing.T, value string) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
