package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestDeferredProviderAccountBoundariesAreVisibleAndUnavailable(t *testing.T) {
	adapters := []providerAccountAdapter{
		newChatGPTAccountBoundaryAdapter(),
		newClaudeAccountBoundaryAdapter(),
		newGitHubCopilotAccountBoundaryAdapter(),
	}
	service := newProviderAccountService(adapters...)

	items, err := service.list(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 deferred provider accounts, got %#v", items)
	}

	wantIDs := []string{"chatgpt", "claude", "github-copilot"}
	for index, want := range wantIDs {
		status := items[index]
		if status.ID != want {
			t.Fatalf("unexpected deferred provider order at %d: got %q want %q", index, status.ID, want)
		}
		if status.Available || status.Connected || status.State != providerAccountDisconnected {
			t.Fatalf("deferred provider must remain explicitly unavailable: %#v", status)
		}
		if strings.TrimSpace(status.Error) == "" {
			t.Fatalf("deferred provider %q must explain why account login is unavailable", status.ID)
		}
		if len(status.Capabilities) == 0 {
			t.Fatalf("deferred provider %q must declare the intended capability boundary", status.ID)
		}
	}
}

func TestDeferredProviderAccountAdaptersCannotYieldRuntimeCredentials(t *testing.T) {
	for _, adapter := range []providerAccountAdapter{
		newChatGPTAccountBoundaryAdapter(),
		newClaudeAccountBoundaryAdapter(),
		newGitHubCopilotAccountBoundaryAdapter(),
	} {
		if _, err := adapter.ResolveCredential(context.Background(), ""); !errors.Is(err, errCredentialNotFound) {
			t.Fatalf("%s must not expose a runtime credential, got %v", adapter.ID(), err)
		}
		if _, err := adapter.BeginLogin(context.Background(), ""); err == nil {
			t.Fatalf("%s must reject login while its documented transport boundary is unresolved", adapter.ID())
		}
	}
}

func TestDeferredProviderAccountStatusHasNoSecretFields(t *testing.T) {
	service := newProviderAccountService(
		newChatGPTAccountBoundaryAdapter(),
		newClaudeAccountBoundaryAdapter(),
		newGitHubCopilotAccountBoundaryAdapter(),
	)
	items, err := service.list(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(items)
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(encoded))
	for _, forbidden := range []string{
		"accesstoken",
		"access_token",
		"refreshtoken",
		"refresh_token",
		"clientsecret",
		"client_secret",
		"codeverifier",
		"code_verifier",
		"authorizationcode",
	} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("deferred provider browser status exposes forbidden field %q: %s", forbidden, lower)
		}
	}
}

func TestDeferredProvidersStayOutOfRuntimeCredentialRegistry(t *testing.T) {
	mainSource, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(mainSource)
	for _, required := range []string{
		"newChatGPTAccountBoundaryAdapter()",
		"newClaudeAccountBoundaryAdapter()",
		"newGitHubCopilotAccountBoundaryAdapter()",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("main.go must expose deferred provider boundary %q", required)
		}
	}
	for _, forbidden := range []string{
		"providerManager.registerAccountAdapter(newChatGPTAccountBoundaryAdapter()",
		"providerManager.registerAccountAdapter(newClaudeAccountBoundaryAdapter()",
		"providerManager.registerAccountAdapter(newGitHubCopilotAccountBoundaryAdapter()",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("deferred provider must not enter runtime credential resolution: %q", forbidden)
		}
	}
}
