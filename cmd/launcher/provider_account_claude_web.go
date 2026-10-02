package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	claudeWebAccountProviderID        = "claude-web"
	claudeWebRuntimeProviderID        = "claude-web-account"
	claudeWebProviderProtocol         = "claude-web-browser"
	claudeWebRuntimeBaseURL           = "https://claude.ai"
	claudeWebRuntimeCredentialSentinel = "claude-web-browser-session"
	claudeWebModelID                   = "claude-sonnet-5-5"
)

type claudeWebConfig struct {
	Connected         bool   `json:"connected,omitempty"`
	OrganizationID    string `json:"organizationId,omitempty"`
	OrganizationName  string `json:"organizationName,omitempty"`
}

func claudeWebConfigPath() string {
	return filepath.Join(tlStudioStateDirectory(), "claude-web.json")
}

func loadClaudeWebConfig() (claudeWebConfig, error) {
	data, err := os.ReadFile(claudeWebConfigPath())
	if errors.Is(err, os.ErrNotExist) {
		return claudeWebConfig{}, nil
	}
	if err != nil {
		return claudeWebConfig{}, err
	}
	var config claudeWebConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return claudeWebConfig{}, fmt.Errorf("decode Claude Web config: %w", err)
	}
	config.OrganizationID = strings.TrimSpace(config.OrganizationID)
	config.OrganizationName = strings.TrimSpace(config.OrganizationName)
	return config, nil
}

func saveClaudeWebConfig(config claudeWebConfig) error {
	path := claudeWebConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	config.OrganizationID = strings.TrimSpace(config.OrganizationID)
	config.OrganizationName = strings.TrimSpace(config.OrganizationName)
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), "claude-web-*.tmp")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempName, path)
}

type claudeWebLoginTransaction struct {
	LoginID   string
	ExpiresAt time.Time
}

type claudeWebAccountAdapter struct {
	state     *appState
	manager   *providerManager
	transport claudeWebTransport

	mu     sync.Mutex
	logins map[string]claudeWebLoginTransaction
}

func newClaudeWebAccountAdapter(state *appState, manager *providerManager) *claudeWebAccountAdapter {
	return newClaudeWebAccountAdapterWithTransport(state, manager, newClaudeWebExtensionBridge(state))
}

func newClaudeWebAccountAdapterWithTransport(state *appState, manager *providerManager, transport claudeWebTransport) *claudeWebAccountAdapter {
	return &claudeWebAccountAdapter{
		state:     state,
		manager:   manager,
		transport: transport,
		logins:    map[string]claudeWebLoginTransaction{},
	}
}

func (a *claudeWebAccountAdapter) ID() string { return claudeWebAccountProviderID }
func (a *claudeWebAccountAdapter) RuntimeProviderID() string { return claudeWebRuntimeProviderID }
func (a *claudeWebAccountAdapter) Protocol() string { return claudeWebProviderProtocol }

func (a *claudeWebAccountAdapter) baseStatus() providerAccountStatus {
	availableErr := errors.New("Claude Web browser transport is unavailable")
	if a != nil && a.transport != nil {
		availableErr = a.transport.Available()
	}
	status := providerAccountStatus{
		ID:           claudeWebAccountProviderID,
		Name:         "Claude Web (Free/Pro)",
		Description:  "Use Claude through the session already signed in inside the user's normal Chrome profile.",
		Available:    availableErr == nil,
		State:        providerAccountDisconnected,
		AuthModes:    []string{"browser_session"},
		Capabilities: []string{"models", "inference"},
		BillingNote:  "Uses the signed-in Claude Web account and its normal web usage limits. No Anthropic API billing is used.",
		Setup: &providerAccountSetupSummary{
			Configurable: false,
			Configured:   availableErr == nil,
			Label:        "Claude Web Chrome extension",
		},
	}
	if availableErr != nil {
		status.Error = availableErr.Error()
	}
	return status
}

func claudeWebModels() []tlProviderModel {
	return []tlProviderModel{{
		ID:        claudeWebModelID,
		Name:      "Claude Sonnet 5.5 (Web)",
		ToolCall:  true,
		Reasoning: true,
	}}
}

func (a *claudeWebAccountAdapter) syncProvider(config claudeWebConfig) ([]string, error) {
	if a == nil || a.manager == nil || a.manager.store == nil {
		return nil, errors.New("TL Studio Provider Registry is unavailable")
	}
	models := claudeWebModels()
	definition := tlProviderDefinition{
		ID:        claudeWebRuntimeProviderID,
		Name:      "Claude Web / Account",
		Protocol:  claudeWebProviderProtocol,
		BaseURL:   claudeWebRuntimeBaseURL,
		ManagedBy: "account",
		Models:    models,
	}
	if existing, found, err := a.manager.store.get(claudeWebRuntimeProviderID); err != nil {
		return nil, err
	} else if found && existing.ManagedBy != "account" {
		return nil, errors.New("provider ID claude-web-account is already used by a different provider")
	}
	if err := a.manager.store.put(definition); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(models))
	for _, model := range models {
		ids = append(ids, model.ID)
	}
	return ids, nil
}

func (a *claudeWebAccountAdapter) removeManagedProvider() {
	if a == nil || a.manager == nil || a.manager.store == nil {
		return
	}
	if provider, found, err := a.manager.store.get(claudeWebRuntimeProviderID); err == nil && found && provider.ManagedBy == "account" {
		_ = a.manager.store.remove(claudeWebRuntimeProviderID)
	}
}

func (a *claudeWebAccountAdapter) Status(context.Context, string) (providerAccountStatus, error) {
	status := a.baseStatus()
	if !status.Available {
		return status, nil
	}
	config, err := loadClaudeWebConfig()
	if err != nil {
		return providerAccountStatus{}, err
	}
	if !config.Connected {
		a.removeManagedProvider()
		return status, nil
	}
	if paired, ok := a.transport.(interface{ Paired() bool }); ok && !paired.Paired() {
		a.removeManagedProvider()
		status.State = providerAccountNeedsReauthentication
		status.AccountType = "Claude Web"
		status.AccountLabel = strings.TrimSpace(config.OrganizationName)
		status.OrganizationID = strings.TrimSpace(config.OrganizationID)
		status.Error = "Claude Web is still signed in, but the TL Studio browser bridge needs to reconnect."
		return status, nil
	}
	ids, err := a.syncProvider(config)
	if err != nil {
		return providerAccountStatus{}, err
	}
	status.State = providerAccountConnected
	status.Connected = true
	status.AccountType = "Claude Web"
	status.AccountLabel = strings.TrimSpace(config.OrganizationName)
	status.OrganizationID = strings.TrimSpace(config.OrganizationID)
	status.Models = ids
	return status, nil
}

func (a *claudeWebAccountAdapter) Setup(context.Context, string) (providerAccountSetup, error) {
	return providerAccountSetup{
		Title:       "Claude Web Chrome extension",
		Description: "Claude Web uses the bundled Chrome extension and the signed-in Claude session from the normal Chrome profile.",
		Fields:      []providerAccountSetupField{},
	}, nil
}

func (a *claudeWebAccountAdapter) Configure(ctx context.Context, directory string, _ map[string]string) (providerAccountStatus, error) {
	return a.Status(ctx, directory)
}

func (a *claudeWebAccountAdapter) BeginLogin(ctx context.Context, _ string) (providerAccountLogin, error) {
	if a == nil || a.transport == nil {
		return providerAccountLogin{}, errors.New("Claude Web browser transport is unavailable")
	}
	if err := a.transport.OpenLogin(ctx); err != nil {
		return providerAccountLogin{}, err
	}
	loginID, err := randomBase64URL(18)
	if err != nil {
		return providerAccountLogin{}, err
	}
	expiresAt := time.Now().UTC().Add(10 * time.Minute)
	a.mu.Lock()
	a.logins[loginID] = claudeWebLoginTransaction{LoginID: loginID, ExpiresAt: expiresAt}
	a.mu.Unlock()
	login := providerAccountLogin{
		LoginID:             loginID,
		Flow:                "claude_web_extension",
		Instructions:        "Pairing with the Claude session already signed in inside this Chrome profile. Claude will stay inside TL Studio; no Claude tab is required.",
		ExpiresAt:           expiresAt.Format(time.RFC3339),
		PollIntervalSeconds: 1,
	}
	if pairing, ok := a.transport.(interface {
		PairingToken() string
		PairingOrigin() string
	}); ok {
		login.BridgeToken = strings.TrimSpace(pairing.PairingToken())
		login.BridgeOrigin = strings.TrimSpace(pairing.PairingOrigin())
	}
	if login.BridgeToken == "" || login.BridgeOrigin == "" {
		return providerAccountLogin{}, errors.New("Claude Web extension pairing data is unavailable")
	}
	return login, nil
}

func (a *claudeWebAccountAdapter) CompleteLogin(context.Context, string, providerAccountCallback) error {
	return errors.New("Claude Web login is completed through the local Chrome extension bridge")
}

func (a *claudeWebAccountAdapter) PollLogin(ctx context.Context, directory, loginID string) (providerAccountStatus, error) {
	loginID = strings.TrimSpace(loginID)
	a.mu.Lock()
	transaction, found := a.logins[loginID]
	a.mu.Unlock()
	if !found {
		return providerAccountStatus{}, errors.New("provider login transaction not found")
	}
	if time.Now().UTC().After(transaction.ExpiresAt) {
		a.mu.Lock()
		delete(a.logins, loginID)
		a.mu.Unlock()
		status := a.baseStatus()
		status.State = providerAccountExpired
		return status, nil
	}
	probe, err := a.transport.Probe(ctx)
	if err != nil {
		if errors.Is(err, errClaudeWebExtensionNotPaired) {
			status := a.baseStatus()
			status.State = providerAccountConnecting
			return status, nil
		}
		return providerAccountStatus{}, err
	}
	if !probe.Connected {
		status := a.baseStatus()
		status.State = providerAccountConnecting
		if probe.Error != "" && probe.Status >= 400 && probe.Status != 401 && probe.Status != 403 {
			status.Error = probe.Error
		}
		return status, nil
	}
	config, err := loadClaudeWebConfig()
	if err != nil {
		return providerAccountStatus{}, err
	}
	config.Connected = true
	config.OrganizationID = strings.TrimSpace(probe.OrganizationID)
	config.OrganizationName = strings.TrimSpace(probe.OrganizationName)
	if err := saveClaudeWebConfig(config); err != nil {
		return providerAccountStatus{}, err
	}
	if _, err := a.syncProvider(config); err != nil {
		return providerAccountStatus{}, err
	}
	a.mu.Lock()
	delete(a.logins, loginID)
	a.mu.Unlock()
	return a.Status(ctx, directory)
}

func (a *claudeWebAccountAdapter) CancelLogin(ctx context.Context, _ string, loginID string) error {
	a.mu.Lock()
	delete(a.logins, strings.TrimSpace(loginID))
	a.mu.Unlock()
	if a.transport != nil {
		return a.transport.Close(ctx)
	}
	return nil
}

func (a *claudeWebAccountAdapter) Refresh(ctx context.Context, directory string) (providerAccountStatus, error) {
	config, err := loadClaudeWebConfig()
	if err != nil {
		return providerAccountStatus{}, err
	}
	if !config.Connected {
		return a.Status(ctx, directory)
	}
	probe, err := a.transport.Probe(ctx)
	if err != nil {
		if errors.Is(err, errClaudeWebExtensionNotPaired) {
			status := a.baseStatus()
			status.State = providerAccountNeedsReauthentication
			status.AccountType = "Claude Web"
			status.AccountLabel = strings.TrimSpace(config.OrganizationName)
			status.OrganizationID = strings.TrimSpace(config.OrganizationID)
			status.Error = "Claude Web bridge needs to reconnect to this TL Studio page."
			return status, nil
		}
		return providerAccountStatus{}, err
	}
	if !probe.Connected {
		config.Connected = false
		config.OrganizationID = ""
		config.OrganizationName = ""
		_ = saveClaudeWebConfig(config)
		a.removeManagedProvider()
		status := a.baseStatus()
		status.State = providerAccountNeedsReauthentication
		return status, nil
	}
	config.OrganizationID = strings.TrimSpace(probe.OrganizationID)
	config.OrganizationName = strings.TrimSpace(probe.OrganizationName)
	if err := saveClaudeWebConfig(config); err != nil {
		return providerAccountStatus{}, err
	}
	return a.Status(ctx, directory)
}

func (a *claudeWebAccountAdapter) ResolveCredential(context.Context, string) (string, error) {
	if a == nil || a.manager == nil || a.manager.store == nil {
		return "", errCredentialNotFound
	}
	config, err := loadClaudeWebConfig()
	if err != nil || !config.Connected {
		return "", errCredentialNotFound
	}
	provider, found, err := a.manager.store.get(claudeWebRuntimeProviderID)
	if err != nil {
		return "", err
	}
	if !found || provider.ManagedBy != "account" || provider.Protocol != claudeWebProviderProtocol {
		return "", errCredentialNotFound
	}
	return claudeWebRuntimeCredentialSentinel, nil
}

func (a *claudeWebAccountAdapter) DiscoverModels(ctx context.Context, directory string) ([]string, error) {
	status, err := a.Status(ctx, directory)
	if err != nil {
		return nil, err
	}
	if !status.Connected {
		return nil, errCredentialNotFound
	}
	return append([]string(nil), status.Models...), nil
}

func (a *claudeWebAccountAdapter) Disconnect(ctx context.Context, _ string) error {
	a.mu.Lock()
	a.logins = map[string]claudeWebLoginTransaction{}
	a.mu.Unlock()
	if a.transport != nil {
		if err := a.transport.Close(ctx); err != nil {
			return err
		}
	}
	config, err := loadClaudeWebConfig()
	if err != nil {
		return err
	}
	config.Connected = false
	config.OrganizationID = ""
	config.OrganizationName = ""
	if err := saveClaudeWebConfig(config); err != nil {
		return err
	}
	a.removeManagedProvider()
	return nil
}

func claudeWebBridgePrompt(request nativeModelRequest) (string, error) {
	turn, err := claudeBridgePrompt(request)
	if err != nil {
		return "", err
	}
	schemaBytes, err := json.MarshalIndent(claudeBridgeSchema(), "", "  ")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(claudeBridgeSystemPrompt()) +
		"\n\n" + turn +
		"\n\nReturn exactly one JSON object and no markdown fences. Required schema:\n" +
		string(schemaBytes), nil
}

func parseClaudeWebBridgeOutput(raw string) claudeBridgeOutput {
	text := strings.TrimSpace(raw)
	if strings.HasPrefix(text, "```") {
		lines := strings.Split(text, "\n")
		if len(lines) >= 3 && strings.HasPrefix(strings.TrimSpace(lines[0]), "```") && strings.TrimSpace(lines[len(lines)-1]) == "```" {
			text = strings.TrimSpace(strings.Join(lines[1:len(lines)-1], "\n"))
		}
	}
	var output claudeBridgeOutput
	if json.Unmarshal([]byte(text), &output) == nil {
		return output
	}
	start := strings.IndexByte(text, '{')
	end := strings.LastIndexByte(text, '}')
	if start >= 0 && end > start {
		if json.Unmarshal([]byte(text[start:end+1]), &output) == nil {
			return output
		}
	}
	return claudeBridgeOutput{Text: strings.TrimSpace(raw), ToolCalls: []claudeBridgeToolCall{}}
}

func (a *claudeWebAccountAdapter) CompleteModelTurn(ctx context.Context, request nativeModelRequest, onTextDelta func(string)) (nativeModelResponse, error) {
	if a == nil || a.transport == nil {
		return nativeModelResponse{}, errors.New("Claude Web account transport is unavailable")
	}
	config, err := loadClaudeWebConfig()
	if err != nil {
		return nativeModelResponse{}, err
	}
	if !config.Connected {
		return nativeModelResponse{}, errors.New("Claude Web account requires browser sign-in")
	}
	probe, err := a.transport.Probe(ctx)
	if err != nil {
		if errors.Is(err, errClaudeWebExtensionNotPaired) {
			return nativeModelResponse{}, errors.New("Claude Web bridge is not connected to this TL Studio page yet; reconnecting is required")
		}
		return nativeModelResponse{}, err
	}
	if !probe.Connected {
		config.Connected = false
		config.OrganizationID = ""
		config.OrganizationName = ""
		_ = saveClaudeWebConfig(config)
		a.removeManagedProvider()
		return nativeModelResponse{}, errors.New("Claude Web browser session expired; sign in again from Provider Settings")
	}
	prompt, err := claudeWebBridgePrompt(request)
	if err != nil {
		return nativeModelResponse{}, err
	}
	raw, err := a.transport.Complete(ctx, prompt)
	if err != nil {
		return nativeModelResponse{}, err
	}
	output := parseClaudeWebBridgeOutput(raw)
	return claudeBridgeResponse(request, output, onTextDelta)
}
