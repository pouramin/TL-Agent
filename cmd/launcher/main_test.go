package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestNativeServerExposesOnlyLocalProductHealth(t *testing.T) {
	project := t.TempDir()
	t.Setenv("TL_STUDIO_STATE_DIR", t.TempDir())
	state := &appState{project: project, frontendURL: "http://127.0.0.1"}
	handler, err := newServer(state)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()

	res, err := http.Get(server.URL + "/local/health")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("health status=%d", res.StatusCode)
	}
	var health map[string]any
	if err := json.NewDecoder(res.Body).Decode(&health); err != nil {
		t.Fatal(err)
	}
	if health["healthy"] != true || health["mode"] != "native" {
		t.Fatalf("unexpected health %#v", health)
	}

	res, err = http.Get(server.URL + "/runtime/global/health")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("removed runtime proxy unexpectedly answered with %d", res.StatusCode)
	}
}

func TestNormalizeProjectUsesDirectory(t *testing.T) {
	project := t.TempDir()
	got, err := normalizeProject(project)
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(project)
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Clean(want) {
		t.Fatalf("normalizeProject=%q want %q", got, filepath.Clean(want))
	}
}

func TestLocalOnlyRejectsNonLoopbackHost(t *testing.T) {
	handler := localOnly(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "http://example.test/local/health", nil)
	req.Host = "example.test"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-loopback host status=%d", rec.Code)
	}
}
