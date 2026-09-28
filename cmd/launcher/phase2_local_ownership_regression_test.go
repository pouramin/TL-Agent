package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionLauncherContainsNoCompatibilityRuntimePath(t *testing.T) {
	root := releaseRepoRoot(t)
	paths := []string{
		"cmd/launcher/main.go",
		"cmd/launcher/session_contract.go",
		"cmd/launcher/session_command_contract.go",
		"cmd/launcher/question_contract.go",
		"cmd/launcher/permission_engine.go",
		"cmd/launcher/live_event_contract.go",
		"cmd/launcher/native_agent_runtime.go",
		"cmd/launcher/runtime_providers.go",
	}
	for _, relative := range paths {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		source := string(data)
		for _, forbidden := range []string{"runtimeBackend", "kiloRuntime", "x-kilo-directory", "KILO_SERVER_", "--native-only", "--runtime-bin"} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s contains removed compatibility marker %q", relative, forbidden)
			}
		}
	}
}

func TestNativeOwnershipContractsArePresent(t *testing.T) {
	root := releaseRepoRoot(t)
	required := []string{
		"cmd/launcher/native_agent_runtime.go",
		"cmd/launcher/native_tool_executor.go",
		"cmd/launcher/session_contract.go",
		"cmd/launcher/question_contract.go",
		"cmd/launcher/permission_engine.go",
		"cmd/launcher/live_event_contract.go",
		"cmd/launcher/provider_account_auth.go",
	}
	for _, relative := range required {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); err != nil {
			t.Fatalf("native ownership contract missing: %s: %v", relative, err)
		}
	}
}
