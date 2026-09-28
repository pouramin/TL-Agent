package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseWorkflowsNeverBundleCompatibilityRuntime(t *testing.T) {
	root := releaseRepoRoot(t)
	for _, relative := range []string{".github/workflows/preview-build.yml", ".github/workflows/release.yml"} {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		source := string(data)
		for _, forbidden := range []string{"KILO_VERSION", "kilo-windows", "kilo-linux", "kilo-darwin", "bin/kilo", "/runtime/global/health"} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s still contains removed runtime packaging marker %q", relative, forbidden)
			}
		}
		if !strings.Contains(source, "Assert package contains no Kilo runtime") {
			t.Fatalf("%s is missing the no-Kilo package assertion", relative)
		}
	}
}

func TestDevelopmentVersionIsNextMilestone(t *testing.T) {
	root := releaseRepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "0.6.0-alpha.1" {
		t.Fatalf("unexpected milestone version %q", strings.TrimSpace(string(data)))
	}
}
