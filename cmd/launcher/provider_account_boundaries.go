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
