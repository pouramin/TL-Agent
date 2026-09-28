package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	openRouterAccountProviderID = "openrouter"
	openRouterAccountBaseURL    = "https://openrouter.ai/api/v1"
	openRouterAuthBaseURL       = "https://openrouter.ai"
)

type openRouterLoginTransaction struct {
	LoginID   string
	State     string
	Verifier  string
	ExpiresAt time.Time
	Completed bool
	Err       string
}

type openRouterAccountAdapter struct {
	state       *appState
	manager     *providerManager
	client      *http.Client
	authBaseURL string
	apiBaseURL  string

	mu      sync.Mutex
	logins  map[string]*openRouterLoginTransaction
	byState map[string]string
}

func newOpenRouterAccountAdapter(state *appState, manager *providerManager) *openRouterAccountAdapter {
	return &openRouterAccountAdapter{
		state: state, manager: manager,
		client: &http.Client{Timeout: 20 * time.Second},
		authBaseURL: openRouterAuthBaseURL,
		apiBaseURL: openRouterAccountBaseURL,
		logins: map[string]*openRouterLoginTransaction{},
		byState: map[string]string{},
	}
}

func (a *openRouterAccountAdapter) ID() string { return openRouterAccountProviderID }

func (a *openRouterAccountAdapter) Status(context.Context, string) (providerAccountStatus, error) {
	status := providerAccountStatus{
		ID: openRouterAccountProviderID,
		Name: "OpenRouter",
		Description: "Connect your OpenRouter account without copying an API key.",
		Available: true,
		State: providerAccountDisconnected,
		AuthModes: []string{"authorization_code_pkce"},
		Capabilities: []string{"models", "inference"},
		BillingNote: "Model usage is billed to the connected OpenRouter account.",
	}
	if a.manager == nil || a.manager.credentials == nil {
		status.Available = false
		status.Error = "TL Studio credential vault is unavailable"
		return status, nil
	}
	if value, err := getProviderCredentialSlot(a.manager.credentials, openRouterAccountProviderID, providerCredentialSlotAccount); err == nil && strings.TrimSpace(value) != "" {
		status.State = providerAccountConnected
		status.Connected = true
		status.AccountType = "OpenRouter account"
	} else if err != nil && !errors.Is(err, errCredentialNotFound) {
		return providerAccountStatus{}, err
	}
	if provider, ok, err := a.manager.store.get(openRouterAccountProviderID); err == nil && ok {
		status.Models = make([]string, 0, len(provider.Models))
		for _, model := range provider.Models {
			status.Models = append(status.Models, model.ID)
		}
	} else if err != nil {
		return providerAccountStatus{}, err
	}
	return status, nil
}

func randomBase64URL(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func (a *openRouterAccountAdapter) frontendURL() string {
	if a.state == nil {
		return ""
	}
	a.state.mu.RLock()
	defer a.state.mu.RUnlock()
	return strings.TrimRight(a.state.frontendURL, "/")
}

func (a *openRouterAccountAdapter) BeginLogin(context.Context, string) (providerAccountLogin, error) {
	loginID, err := randomBase64URL(18)
	if err != nil {
		return providerAccountLogin{}, err
	}
	state, err := randomBase64URL(24)
	if err != nil {
		return providerAccountLogin{}, err
	}
	verifier, err := randomBase64URL(48)
	if err != nil {
		return providerAccountLogin{}, err
	}
	challengeHash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(challengeHash[:])
	callbackURL := a.frontendURL() + "/local/provider-accounts/openrouter/oauth/callback"
	if !strings.HasPrefix(callbackURL, "http://127.0.0.1:") && !strings.HasPrefix(callbackURL, "http://localhost:") && !strings.HasPrefix(callbackURL, "http://[::1]:") {
		return providerAccountLogin{}, errors.New("OpenRouter account login requires the TL Studio loopback server")
	}
	authorize, err := url.Parse(strings.TrimRight(a.authBaseURL, "/") + "/auth")
	if err != nil {
		return providerAccountLogin{}, err
	}
	query := authorize.Query()
	query.Set("callback_url", callbackURL)
	query.Set("code_challenge", challenge)
	query.Set("code_challenge_method", "S256")
	query.Set("state", state)
	authorize.RawQuery = query.Encode()

	expiresAt := time.Now().Add(10 * time.Minute)
	txn := &openRouterLoginTransaction{
		LoginID: loginID, State: state, Verifier: verifier, ExpiresAt: expiresAt,
	}
	a.mu.Lock()
	a.logins[loginID] = txn
	a.byState[state] = loginID
	a.mu.Unlock()

	return providerAccountLogin{
		LoginID: loginID,
		Flow: "authorization_code_pkce",
		AuthorizationURL: authorize.String(),
		Instructions: "Open OpenRouter, sign in, and approve TL Studio.",
		ExpiresAt: expiresAt.UTC().Format(time.RFC3339),
		PollIntervalSeconds: 2,
	}, nil
}

func (a *openRouterAccountAdapter) CompleteLogin(ctx context.Context, _ string, callback providerAccountCallback) error {
	a.mu.Lock()
	loginID := a.byState[callback.State]
	txn := a.logins[loginID]
	a.mu.Unlock()
	if txn == nil || loginID == "" {
		return errors.New("invalid OAuth state")
	}
	if time.Now().After(txn.ExpiresAt) {
		a.setLoginError(loginID, "authorization expired")
		return errors.New("authorization expired")
	}
	if callback.Error != "" {
		a.setLoginError(loginID, "authorization was rejected")
		return errors.New("authorization was rejected")
	}
	if callback.Code == "" {
		a.setLoginError(loginID, "authorization code is missing")
		return errors.New("authorization code is missing")
	}

	payload, err := json.Marshal(map[string]string{
		"code": callback.Code,
		"code_verifier": txn.Verifier,
		"code_challenge_method": "S256",
	})
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(a.apiBaseURL, "/")+"/auth/keys", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := a.client.Do(request)
	if err != nil {
		a.setLoginError(loginID, "OpenRouter token exchange failed")
		return err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		a.setLoginError(loginID, fmt.Sprintf("OpenRouter token exchange failed with status %d", response.StatusCode))
		return fmt.Errorf("OpenRouter token exchange failed with status %d", response.StatusCode)
	}
	var exchanged struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(body, &exchanged); err != nil {
		return errors.New("OpenRouter token exchange returned invalid JSON")
	}
	exchanged.Key = strings.TrimSpace(exchanged.Key)
	if exchanged.Key == "" {
		return errors.New("OpenRouter token exchange returned no API key")
	}

	models, err := discoverOpenAICompatibleModelsWithClient(ctx, a.apiBaseURL, exchanged.Key, a.client)
	if err != nil {
		a.setLoginError(loginID, "OpenRouter model discovery failed")
		return err
	}
	definition := tlProviderDefinition{
		ID: openRouterAccountProviderID,
		Name: "OpenRouter",
		Protocol: "openai-compatible",
		BaseURL: a.apiBaseURL,
		Models: make([]tlProviderModel, 0, len(models)),
	}
	for _, model := range models {
		definition.Models = append(definition.Models, tlProviderModel{
			ID: model.ID,
			Name: model.Name,
			Kind: model.Kind,
			ToolCall: model.ToolCall != nil && *model.ToolCall,
			Reasoning: model.Reasoning != nil && *model.Reasoning,
			ContextLimit: model.ContextLimit,
			OutputLimit: model.OutputLimit,
		})
	}
	if existing, ok, getErr := a.manager.store.get(openRouterAccountProviderID); getErr != nil {
		return getErr
	} else if ok && !isOpenRouterBaseURL(existing.BaseURL) {
		return errors.New("provider ID openrouter is already used by a different provider")
	}
	if err := putProviderCredentialSlot(a.manager.credentials, openRouterAccountProviderID, providerCredentialSlotAccount, exchanged.Key); err != nil {
		return err
	}
	if err := a.manager.store.put(definition); err != nil {
		_ = deleteProviderCredentialSlot(a.manager.credentials, openRouterAccountProviderID, providerCredentialSlotAccount)
		return err
	}

	a.mu.Lock()
	txn.Completed = true
	txn.Verifier = ""
	delete(a.byState, txn.State)
	a.mu.Unlock()
	return nil
}

func (a *openRouterAccountAdapter) setLoginError(loginID, message string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if txn := a.logins[loginID]; txn != nil {
		txn.Err = message
		txn.Verifier = ""
		delete(a.byState, txn.State)
	}
}

func (a *openRouterAccountAdapter) PollLogin(ctx context.Context, directory, loginID string) (providerAccountStatus, error) {
	a.mu.Lock()
	txn := a.logins[strings.TrimSpace(loginID)]
	if txn == nil {
		a.mu.Unlock()
		return providerAccountStatus{}, errors.New("provider login transaction not found")
	}
	completed, loginErr, expiresAt := txn.Completed, txn.Err, txn.ExpiresAt
	a.mu.Unlock()
	if loginErr != "" {
		status, err := a.Status(ctx, directory)
		if err != nil {
			return providerAccountStatus{}, err
		}
		status.State = providerAccountErrorState
		status.Connected = false
		status.Error = loginErr
		return status, nil
	}
	if completed {
		return a.Status(ctx, directory)
	}
	status, err := a.Status(ctx, directory)
	if err != nil {
		return providerAccountStatus{}, err
	}
	if time.Now().After(expiresAt) {
		status.State = providerAccountExpired
		status.Connected = false
		return status, nil
	}
	status.State = providerAccountConnecting
	status.Connected = false
	return status, nil
}

func (a *openRouterAccountAdapter) CancelLogin(context.Context, string, string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	for loginID, txn := range a.logins {
		delete(a.byState, txn.State)
		delete(a.logins, loginID)
	}
	return nil
}

func (a *openRouterAccountAdapter) Refresh(ctx context.Context, directory string) (providerAccountStatus, error) {
	return a.Status(ctx, directory)
}

func (a *openRouterAccountAdapter) DiscoverModels(ctx context.Context, _ string) ([]string, error) {
	if a.manager == nil || a.manager.credentials == nil {
		return nil, errors.New("TL Studio credential vault is unavailable")
	}
	key, err := getProviderCredentialSlot(a.manager.credentials, openRouterAccountProviderID, providerCredentialSlotAccount)
	if err != nil {
		return nil, err
	}
	models, err := discoverOpenAICompatibleModelsWithClient(ctx, a.apiBaseURL, key, a.client)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(models))
	for _, model := range models {
		result = append(result, model.ID)
	}
	return result, nil
}

func (a *openRouterAccountAdapter) Disconnect(context.Context, string) error {
	if a.manager == nil || a.manager.credentials == nil {
		return nil
	}
	return deleteProviderCredentialSlot(a.manager.credentials, openRouterAccountProviderID, providerCredentialSlotAccount)
}
