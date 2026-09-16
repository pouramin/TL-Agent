package main

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestProvidersUIRegressions(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not available; custom provider regression harness skipped")
	}

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not resolve test file path")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))

	for _, name := range []string{"check-providers-ui.cjs", "check-provider-api.cjs"} {
		script := filepath.Join(repoRoot, "scripts", name)
		cmd := exec.Command(node, script)
		cmd.Dir = repoRoot
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s regression harness failed: %v\n%s", name, err, output)
		}
	}
}
