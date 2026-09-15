package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestProjectHistoryRememberPersistsAndDeduplicates(t *testing.T) {
	file := filepath.Join(t.TempDir(), "projects.json")
	first := t.TempDir()
	second := t.TempDir()

	store := &projectHistoryStore{filePath: file}
	store.remember(first)
	store.remember(second)
	store.remember(first)

	got := store.list()
	if len(got) != 2 {
		t.Fatalf("expected 2 projects, got %#v", got)
	}
	if !sameProjectPath(got[0], first) || !sameProjectPath(got[1], second) {
		t.Fatalf("expected most recently opened project first, got %#v", got)
	}

	reloaded := &projectHistoryStore{filePath: file}
	persisted := reloaded.list()
	if len(persisted) != 2 || !sameProjectPath(persisted[0], first) || !sameProjectPath(persisted[1], second) {
		t.Fatalf("project history did not persist: %#v", persisted)
	}
}

func TestLocalProjectsEndpointTracksProjectSwitches(t *testing.T) {
	old := recentProjects
	recentProjects = &projectHistoryStore{filePath: filepath.Join(t.TempDir(), "projects.json")}
	defer func() { recentProjects = old }()

	first := t.TempDir()
	second := t.TempDir()
	state := &appState{project: first}
	mux := http.NewServeMux()
	registerProjectHistoryRoute(mux, state)
	server := httptest.NewServer(mux)
	defer server.Close()

	readProjects := func() []string {
		t.Helper()
		res, err := http.Get(server.URL + "/local/projects")
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("unexpected status: %d", res.StatusCode)
		}
		var payload projectHistoryFile
		if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		return payload.Projects
	}

	projects := readProjects()
	if len(projects) != 1 || !sameProjectPath(projects[0], first) {
		t.Fatalf("initial project missing from history: %#v", projects)
	}

	state.setProject(second)
	projects = readProjects()
	if len(projects) != 2 || !sameProjectPath(projects[0], second) || !sameProjectPath(projects[1], first) {
		t.Fatalf("project switch was not tracked: %#v", projects)
	}
}
