package main

import (
	"context"
	"errors"
)

type unavailableProviderAccountAdapter struct {
	id           string
	name         string
	description  string
	billingNote  string
	reason       string
	authModes    []string
	capabilities []string
}

func (a *unavailableProviderAccountAdapter) ID() string { return a.id }

func (a *unavailableProviderAccountAdapter) Status(context.Context, string) (providerAccountStatus, error) {
	return providerAccountStatus{
		ID:           a.id,
		Name:         a.name,
		Description:  a.description,
		Available:    false,
		State:        providerAccountDisconnected,
		AuthModes:    append([]string(nil), a.authModes...),
		Capabilities: append([]string(nil), a.capabilities...),
		BillingNote:  a.billingNote,
		Error:        a.reason,
	}, nil
}

func (a *unavailableProviderAccountAdapter) unavailableError() error {
	if a == nil || a.reason == "" {
		return errors.New("provider account login is unavailable")
	}
	return errors.New(a.reason)
}

func (a *unavailableProviderAccountAdapter) BeginLogin(context.Context, string) (providerAccountLogin, error) {
	return providerAccountLogin{}, a.unavailableError()
}

func (a *unavailableProviderAccountAdapter) CompleteLogin(context.Context, string, providerAccountCallback) error {
	return a.unavailableError()
}

func (a *unavailableProviderAccountAdapter) PollLogin(context.Context, string, string) (providerAccountStatus, error) {
	return providerAccountStatus{}, a.unavailableError()
}

func (a *unavailableProviderAccountAdapter) CancelLogin(context.Context, string, string) error {
	return nil
}

func (a *unavailableProviderAccountAdapter) Refresh(context.Context, string) (providerAccountStatus, error) {
	return providerAccountStatus{}, a.unavailableError()
}

func (a *unavailableProviderAccountAdapter) ResolveCredential(context.Context, string) (string, error) {
	return "", errCredentialNotFound
}

func (a *unavailableProviderAccountAdapter) DiscoverModels(context.Context, string) ([]string, error) {
	return nil, a.unavailableError()
}

func (a *unavailableProviderAccountAdapter) Disconnect(context.Context, string) error {
	return nil
}

func newChatGPTAccountBoundaryAdapter() providerAccountAdapter {
	return &unavailableProviderAccountAdapter{
		id:   "chatgpt",
		name: "ChatGPT / Codex",
		description: "ChatGPT account usage is a required TL Studio 0.6 target. OpenAI publicly supports ChatGPT login through Codex clients, but the documented third-party surface currently routes inference through the Codex thread/turn runtime rather than a raw model transport.",
		billingNote: "ChatGPT/Codex plan usage is separate from normal OpenAI API billing.",
		reason: "Waiting for an official third-party ChatGPT/Codex model transport that can preserve TL Studio Native Agent ownership. TL Studio will not copy private OAuth clients or undocumented ChatGPT backend endpoints.",
		authModes: []string{"chatgpt"},
		capabilities: []string{"models", "inference"},
	}
}

func newClaudeAccountBoundaryAdapter() providerAccountAdapter {
	return &unavailableProviderAccountAdapter{
		id:   "claude",
		name: "Claude",
		description: "Anthropic account OAuth exists in Anthropic-owned tools, but no documented public third-party consumer-account client registration suitable for TL Studio is currently used.",
		billingNote: "Claude consumer subscriptions and Anthropic API billing are separate products.",
		reason: "Account login is not enabled until Anthropic exposes a documented third-party authorization contract. Anthropic-compatible API configuration remains supported.",
		authModes: []string{"account"},
		capabilities: []string{"models", "inference"},
	}
}

func newGitHubCopilotAccountBoundaryAdapter() providerAccountAdapter {
	return &unavailableProviderAccountAdapter{
		id:   "github-copilot",
		name: "GitHub Copilot",
		description: "GitHub provides official account authentication for Copilot integrations, but the current model-access integration is coupled to the Copilot SDK/runtime.",
		billingNote: "Eligible Copilot usage is tied to the connected GitHub Copilot entitlement.",
		reason: "Deferred until Copilot can be integrated without introducing a second Agent runtime beneath TL Studio.",
		authModes: []string{"oauth_pkce", "device_code"},
		capabilities: []string{"models", "inference"},
	}
}
