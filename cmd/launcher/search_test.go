package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func writeSearchFixture(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestSearchProjectFindsGroupedMatchesAndLines(t *testing.T) {
	project := t.TempDir()
	writeSearchFixture(t, project, "src/a.js", "first\nNeedle here\nneedle again\n")
	writeSearchFixture(t, project, "src/b.js", "before\nNEEDLE other\n")
	writeSearchFixture(t, project, "README.md", "nothing here\n")

	result, err := searchProject(context.Background(), project, projectSearchOptions{Query: "needle", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if result.MatchCount != 3 || result.FileCount != 2 {
		t.Fatalf("expected 3 matches in 2 files, got %#v", result)
	}
	if result.Files[0].Path != "src/a.js" || result.Files[1].Path != "src/b.js" {
		t.Fatalf("expected deterministic path order, got %#v", result.Files)
	}
	if result.Files[0].Matches[0].Line != 2 || result.Files[0].Matches[0].Column != 1 {
		t.Fatalf("unexpected first match: %#v", result.Files[0].Matches[0])
	}
}

func TestSearchProjectCaseSensitiveAndFilters(t *testing.T) {
	project := t.TempDir()
	writeSearchFixture(t, project, "src/app.js", "Needle\nneedle\n")
	writeSearchFixture(t, project, "src/app.go", "Needle\n")
	writeSearchFixture(t, project, "test/app.js", "Needle\n")

	result, err := searchProject(context.Background(), project, projectSearchOptions{
		Query:         "Needle",
		CaseSensitive: true,
		Includes:      []string{"*.js"},
		Excludes:      []string{"test/*"},
		Limit:         50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.MatchCount != 1 || result.FileCount != 1 || result.Files[0].Path != "src/app.js" {
		t.Fatalf("unexpected filtered result: %#v", result)
	}
}

func TestSearchProjectIgnoresHeavyAndBinaryContent(t *testing.T) {
	project := t.TempDir()
	writeSearchFixture(t, project, ".git/config", "needle\n")
	writeSearchFixture(t, project, "node_modules/pkg/index.js", "needle\n")
	writeSearchFixture(t, project, "dist/bundle.js", "needle\n")
	writeSearchFixture(t, project, "src/ok.txt", "needle\n")
	if err := os.WriteFile(filepath.Join(project, "blob.bin"), []byte{'n', 'e', 'e', 'd', 'l', 'e', 0, 1}, 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := searchProject(context.Background(), project, projectSearchOptions{Query: "needle", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if result.MatchCount != 1 || result.FileCount != 1 || result.Files[0].Path != "src/ok.txt" {
		t.Fatalf("ignored content leaked into results: %#v", result)
	}
}

func TestSearchProjectResultLimit(t *testing.T) {
	project := t.TempDir()
	writeSearchFixture(t, project, "many.txt", "x x x x x\n")
	result, err := searchProject(context.Background(), project, projectSearchOptions{Query: "x", Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if result.MatchCount != 2 || !result.Truncated {
		t.Fatalf("expected limited truncated result, got %#v", result)
	}
}

func TestSearchProjectDoesNotEscapeThroughPatternsOrSymlinks(t *testing.T) {
	project := t.TempDir()
	outside := t.TempDir()
	writeSearchFixture(t, outside, "secret.txt", "outside-secret-token\n")

	result, err := searchProject(context.Background(), project, projectSearchOptions{
		Query:    "outside-secret-token",
		Includes: []string{"../*"},
		Limit:    50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.MatchCount != 0 {
		t.Fatalf("traversal-like include escaped project: %#v", result)
	}

	link := filepath.Join(project, "escape")
	if err := os.Symlink(outside, link); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlink unavailable on Windows runner: %v", err)
		}
		t.Fatal(err)
	}
	result, err = searchProject(context.Background(), project, projectSearchOptions{Query: "outside-secret-token", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if result.MatchCount != 0 {
		t.Fatalf("symlink escape was searched: %#v", result)
	}
}

func TestSearchProjectHonorsCancellation(t *testing.T) {
	project := t.TempDir()
	writeSearchFixture(t, project, "a.txt", "needle\n")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := searchProject(ctx, project, projectSearchOptions{Query: "needle", Limit: 50}); err == nil {
		t.Fatal("expected canceled search to return an error")
	}
}

func TestProjectSearchRouteUsesCurrentProject(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	writeSearchFixture(t, first, "first.txt", "project-token\n")
	writeSearchFixture(t, second, "second.txt", "project-token\n")

	state := &appState{project: first}
	mux := http.NewServeMux()
	registerProjectSearchRoutes(mux, state)
	server := httptest.NewServer(mux)
	defer server.Close()

	search := func() projectSearchResponse {
		res, err := http.Get(server.URL + "/local/search?q=project-token")
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("unexpected search status: %d", res.StatusCode)
		}
		var payload projectSearchResponse
		if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		return payload
	}

	if got := search(); got.FileCount != 1 || got.Files[0].Path != "first.txt" {
		t.Fatalf("unexpected first project result: %#v", got)
	}
	state.setProject(second)
	if got := search(); got.FileCount != 1 || got.Files[0].Path != "second.txt" {
		t.Fatalf("unexpected switched project result: %#v", got)
	}
}

func TestProjectSearchRouteRejectsInvalidQueryAndCapsLimit(t *testing.T) {
	project := t.TempDir()
	writeSearchFixture(t, project, "a.txt", "needle needle needle\n")
	state := &appState{project: project}
	mux := http.NewServeMux()
	registerProjectSearchRoutes(mux, state)
	server := httptest.NewServer(mux)
	defer server.Close()

	res, err := http.Get(server.URL + "/local/search?q=%20%20")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for blank query, got %d", res.StatusCode)
	}

	res, err = http.Get(server.URL + "/local/search?q=needle&limit=1")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var payload projectSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.MatchCount != 1 || !payload.Truncated {
		t.Fatalf("expected route limit to apply, got %#v", payload)
	}
}
