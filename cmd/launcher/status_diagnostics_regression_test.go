package main

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestStatusDiagnosticsRegressions(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not available; browser diagnostics regression harness skipped")
	}

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not resolve test file path")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	script := filepath.Join(repoRoot, "scripts", "check-status-diagnostics.cjs")

	cmd := exec.Command(node, script)
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("status diagnostics regression harness failed: %v\n%s", err, output)
	}
}
