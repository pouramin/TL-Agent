package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	googleGeminiAccountProviderID = "gemini"
	googleGeminiBaseURL           = "https://generativelanguage.googleapis.com/v1beta"
	googleGeminiModelsURL         = "https://generativelanguage.googleapis.com/v1/models"
	googleGeminiSetupVersion      = 1
)

type googleGeminiSetupConfig struct {
	Version   int    `json:"version"`
	ClientID  string `json:"clientId,omitempty"`
	ProjectID string `json:"projectId,omitempty"`
}

var googleGeminiSetupMu sync.Mutex

func googleGeminiSetupPath() string {
	return filepath.Join(tlStudioStateDirectory(), "google-gemini.json")
}

func loadGoogleGeminiSetupConfig() (googleGeminiSetupConfig, error) {
	googleGeminiSetupMu.Lock()
	defer googleGeminiSetupMu.Unlock()
	data, err := os.ReadFile(googleGeminiSetupPath())
	if errors.Is(err, os.ErrNotExist) {
		return googleGeminiSetupConfig{Version: googleGeminiSetupVersion}, nil
	}
	if err != nil {
		return googleGeminiSetupConfig{}, err
	}
	var config googleGeminiSetupConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return googleGeminiSetupConfig{}, fmt.Errorf("decode Google Gemini setup: %w", err)
	}
	if config.Version != 0 && config.Version != googleGeminiSetupVersion {
		return googleGeminiSetupConfig{}, fmt.Errorf("unsupported Google Gemini setup version %d", config.Version)
	}
	config.Version = googleGeminiSetupVersion
	config.ClientID = strings.TrimSpace(config.ClientID)
	config.ProjectID = strings.TrimSpace(config.ProjectID)
	return config, nil
}

func saveGoogleGeminiSetupConfig(config googleGeminiSetupConfig) error {
	config.Version = googleGeminiSetupVersion
	config.ClientID = strings.TrimSpace(config.ClientID)
	config.ProjectID = strings.TrimSpace(config.ProjectID)

	googleGeminiSetupMu.Lock()
	defer googleGeminiSetupMu.Unlock()
	path := googleGeminiSetupPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), "google-gemini-*.tmp")
	if err != nil {
		return err
	}
	name := temp.Name()
	defer os.Remove(name)
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
	if err := os.Rename(name, path); err != nil {
		return os.WriteFile(path, data, 0o600)
	}
	return nil
}

type googleGeminiLoginTransaction struct {
	LoginID     string
	State       string
	Verifier    string
	RedirectURI string
	ExpiresAt   time.Time
	Completed   bool
	Err         string
}

type googleGeminiAccountAdapter struct {
	state      *appState
	manager    *providerManager
	clientID   string
	projectID  string
	client     *http.Client

	authorizeURL string
	tokenURL     string
	userInfoURL  string
	modelsURL    string
	revokeURL    string
	baseURL      string

	mu      sync.Mutex
	logins  map[string]*googleGeminiLoginTransaction
	byState map[string]string
}

func newGoogleGeminiAccountAdapter(state *appState, manager *providerManager) *googleGeminiAccountAdapter {
	saved, _ := loadGoogleGeminiSetupConfig()
	clientID := strings.TrimSpace(os.Getenv("TL_STUDIO_GOOGLE_CLIENT_ID"))
	if clientID == "" {
		clientID = saved.ClientID
	}
	projectID := strings.TrimSpace(os.Getenv("TL_STUDIO_GOOGLE_PROJECT_ID"))
	if projectID == "" {
		projectID = saved.ProjectID
	}
	if projectID == "" && manager != nil && manager.store != nil {
		if provider, ok, err := manager.store.get(googleGeminiAccountProviderID); err == nil && ok {
			projectID = strings.TrimSpace(provider.ProjectID)
		}
	}
	return &googleGeminiAccountAdapter{
		state:        state,
		manager:      manager,
		clientID:     clientID,
		projectID:    projectID,
		client:       &http.Client{Timeout: 20 * time.Second},
		authorizeURL: "https://accounts.google.com/o/oauth2/v2/auth",
		tokenURL:     "https://oauth2.googleapis.com/token",
		userInfoURL:  "https://www.googleapis.com/oauth2/v2/userinfo",
		modelsURL:    googleGeminiModelsURL,
		revokeURL:    "https://oauth2.googleapis.com/revoke",
		baseURL:      googleGeminiBaseURL,
		logins:       map[string]*googleGeminiLoginTransaction{},
		byState:      map[string]string{},
	}
}

func (a *googleGeminiAccountAdapter) ID() string { return googleGeminiAccountProviderID }

func (a *googleGeminiAccountAdapter) setupValues() (string, string) {
	if a == nil {
		return "", ""
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return strings.TrimSpace(a.clientID), strings.TrimSpace(a.projectID)
}

func (a *googleGeminiAccountAdapter) Setup(context.Context, string) (providerAccountSetup, error) {
	clientID, projectID := a.setupValues()
	return providerAccountSetup{
		Title: "Configure Google / Gemini",
		Description: "Use a Google Cloud project with the Generative Language API enabled. Until TL Studio ships its own registered OAuth client, alpha builds can use a Desktop OAuth client ID from that Google project.",
		Fields: []providerAccountSetupField{
			{
				ID: "projectId", Label: "Google Cloud Project ID",
				Description: "Used as the Gemini API quota and billing project.",
				Placeholder: "my-google-cloud-project", Value: projectID, Required: true,
				ReadOnly: strings.TrimSpace(os.Getenv("TL_STUDIO_GOOGLE_PROJECT_ID")) != "",
			},
			{
				ID: "clientId", Label: "Desktop OAuth Client ID",
				Description: "Non-secret OAuth client identifier for the TL Studio alpha login flow.",
				Placeholder: "1234567890-example.apps.googleusercontent.com", Value: clientID, Required: true,
				ReadOnly: strings.TrimSpace(os.Getenv("TL_STUDIO_GOOGLE_CLIENT_ID")) != "",
			},
		},
	}, nil
}

func (a *googleGeminiAccountAdapter) Configure(ctx context.Context, directory string, values map[string]string) (providerAccountStatus, error) {
	if a == nil {
		return providerAccountStatus{}, errors.New("Google Gemini account adapter is unavailable")
	}
	currentClientID, currentProjectID := a.setupValues()
	clientID := strings.TrimSpace(values["clientId"])
	projectID := strings.TrimSpace(values["projectId"])
	if env := strings.TrimSpace(os.Getenv("TL_STUDIO_GOOGLE_CLIENT_ID")); env != "" {
		clientID = env
	} else if clientID == "" {
		clientID = currentClientID
	}
	if env := strings.TrimSpace(os.Getenv("TL_STUDIO_GOOGLE_PROJECT_ID")); env != "" {
		projectID = env
	} else if projectID == "" {
		projectID = currentProjectID
	}
	if clientID == "" || !strings.HasSuffix(strings.ToLower(clientID), ".apps.googleusercontent.com") {
		return providerAccountStatus{}, errors.New("enter a valid Google Desktop OAuth client ID")
	}
	if projectID == "" || strings.ContainsAny(projectID, " \t\r\n") {
		return providerAccountStatus{}, errors.New("enter a valid Google Cloud project ID")
	}
	if err := saveGoogleGeminiSetupConfig(googleGeminiSetupConfig{
		ClientID: clientID, ProjectID: projectID,
	}); err != nil {
		return providerAccountStatus{}, err
	}
	a.mu.Lock()
	a.clientID = clientID
	a.projectID = projectID
	a.mu.Unlock()
	return a.Status(ctx, directory)
}

func (a *googleGeminiAccountAdapter) available() bool {
	clientID, projectID := a.setupValues()
	return a != nil &&
		a.state != nil &&
		a.manager != nil &&
		a.manager.credentials != nil &&
		clientID != "" &&
		projectID != ""
}

func (a *googleGeminiAccountAdapter) unavailableReason() string {
	clientID, projectID := a.setupValues()
	switch {
	case clientID == "":
		return "Configure a Google Desktop OAuth client ID before Google account login can be enabled."
	case projectID == "":
		return "Configure a Google Cloud project with the Generative Language API enabled for quota and billing."
	default:
		return "Google account login is unavailable in this build."
	}
}

func (a *googleGeminiAccountAdapter) baseStatus() providerAccountStatus {
	description := "Official Google OAuth for Gemini API access. API quota and billing remain tied to a Google Cloud project."
	status := providerAccountStatus{
		ID:           googleGeminiAccountProviderID,
		Name:         "Google / Gemini",
		Description:  description,
		Available:    a.available(),
		State:        providerAccountDisconnected,
		AuthModes:    []string{"authorization_code_pkce"},
		Capabilities: []string{"models", "inference", "refresh", "revocation"},
		BillingNote:  "A Gemini consumer subscription is separate from Gemini API quota and billing.",
		Setup: &providerAccountSetupSummary{
			Configurable: true,
			Configured:   a.available(),
			Label:        "Google API setup",
		},
	}
	if !status.Available {
		status.Error = a.unavailableReason()
		status.Description = description + " " + status.Error
	}
	return status
}

func (a *googleGeminiAccountAdapter) loadCredential() (providerOAuthCredential, error) {
	if a == nil || a.manager == nil || a.manager.credentials == nil {
		return providerOAuthCredential{}, errCredentialNotFound
	}
	raw, err := getProviderCredentialSlot(a.manager.credentials, googleGeminiAccountProviderID, providerCredentialSlotAccount)
	if err != nil {
		return providerOAuthCredential{}, err
	}
	credential, structured, err := decodeProviderOAuthCredential(raw)
	if err != nil {
		return providerOAuthCredential{}, err
	}
	if !structured {
		return providerOAuthCredential{}, errors.New("invalid Google account credential")
	}
	return credential, nil
}

func (a *googleGeminiAccountAdapter) saveCredential(credential providerOAuthCredential) error {
	if a == nil || a.manager == nil || a.manager.credentials == nil {
		return errors.New("TL Studio credential vault is unavailable")
	}
	encoded, err := encodeProviderOAuthCredential(credential)
	if err != nil {
		return err
	}
	return putProviderCredentialSlot(a.manager.credentials, googleGeminiAccountProviderID, providerCredentialSlotAccount, encoded)
}

func (a *googleGeminiAccountAdapter) statusFromCredential(credential providerOAuthCredential) providerAccountStatus {
	status := a.baseStatus()
	status.AccountLabel = credential.AccountLabel
	status.AccountType = credential.AccountType
	status.State = providerAccountConnected
	if credential.needsRefresh(time.Now()) {
		if strings.TrimSpace(credential.RefreshToken) != "" {
			status.State = providerAccountExpired
		} else {
			status.State = providerAccountNeedsReauthentication
		}
	}
	if a.manager != nil && a.manager.store != nil {
		if provider, ok, err := a.manager.store.get(googleGeminiAccountProviderID); err == nil && ok {
			status.Models = make([]string, 0, len(provider.Models))
			for _, model := range provider.Models {
				status.Models = append(status.Models, model.ID)
			}
		}
	}
	status.Connected = status.State == providerAccountConnected
	return status
}

func (a *googleGeminiAccountAdapter) Status(ctx context.Context, _ string) (providerAccountStatus, error) {
	status := a.baseStatus()
	credential, err := a.loadCredential()
	if errors.Is(err, errCredentialNotFound) {
		return status, nil
	}
	if err != nil {
		return providerAccountStatus{}, err
	}
	clientID, _ := a.setupValues()
	if credential.needsRefresh(time.Now()) && strings.TrimSpace(credential.RefreshToken) != "" && clientID != "" {
		if refreshed, refreshErr := a.refreshCredential(ctx, credential); refreshErr == nil {
			credential = refreshed
		} else {
			status = a.statusFromCredential(credential)
			status.State = providerAccountNeedsReauthentication
			status.Connected = false
			status.Error = "Google account needs reauthentication"
			return status, nil
		}
	}
	return a.statusFromCredential(credential), nil
}

func (a *googleGeminiAccountAdapter) callbackURL() (string, error) {
	if a == nil || a.state == nil {
		return "", errors.New("TL Studio local server is unavailable")
	}
	a.state.mu.RLock()
	base := strings.TrimRight(a.state.frontendURL, "/")
	a.state.mu.RUnlock()
	if base == "" {
		return "", errors.New("TL Studio local callback URL is unavailable")
	}
	callback := base + "/local/provider-accounts/" + googleGeminiAccountProviderID + "/oauth/callback"
	parsed, err := url.Parse(callback)
	if err != nil || parsed.Scheme != "http" || !isLoopbackHost(parsed.Hostname()) {
		return "", errors.New("Google account login requires the TL Studio loopback server")
	}
	return callback, nil
}

func (a *googleGeminiAccountAdapter) BeginLogin(context.Context, string) (providerAccountLogin, error) {
	if !a.available() {
		return providerAccountLogin{}, errors.New(a.unavailableReason())
	}
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
	redirectURI, err := a.callbackURL()
	if err != nil {
		return providerAccountLogin{}, err
	}
	expiresAt := time.Now().UTC().Add(10 * time.Minute)
	transaction := &googleGeminiLoginTransaction{
		LoginID: loginID, State: state, Verifier: verifier,
		RedirectURI: redirectURI, ExpiresAt: expiresAt,
	}
	a.mu.Lock()
	a.logins[loginID] = transaction
	a.byState[state] = loginID
	a.mu.Unlock()

	authorize, err := url.Parse(a.authorizeURL)
	if err != nil {
		return providerAccountLogin{}, err
	}
	clientID, _ := a.setupValues()
	query := authorize.Query()
	query.Set("client_id", clientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("response_type", "code")
	query.Set("scope", strings.Join([]string{
		"https://www.googleapis.com/auth/userinfo.profile",
		"https://www.googleapis.com/auth/userinfo.email",
		"https://www.googleapis.com/auth/generative-language.retriever",
	}, " "))
	query.Set("state", state)
	query.Set("code_challenge", oauthPKCEChallenge(verifier))
	query.Set("code_challenge_method", "S256")
	query.Set("access_type", "offline")
	query.Set("prompt", "consent")
	authorize.RawQuery = query.Encode()

	return providerAccountLogin{
		LoginID: loginID,
		Flow: "authorization_code_pkce",
		AuthorizationURL: authorize.String(),
		Instructions: "Sign in with Google and authorize Gemini API access. Usage is charged to the configured Google Cloud project.",
		ExpiresAt: expiresAt.Format(time.RFC3339),
		PollIntervalSeconds: 2,
	}, nil
}

func (a *googleGeminiAccountAdapter) CompleteLogin(ctx context.Context, _ string, callback providerAccountCallback) error {
	a.mu.Lock()
	loginID := a.byState[strings.TrimSpace(callback.State)]
	transaction := a.logins[loginID]
	a.mu.Unlock()
	if transaction == nil || loginID == "" {
		return errors.New("invalid OAuth state")
	}
	if time.Now().UTC().After(transaction.ExpiresAt) {
		a.setLoginError(loginID, "authorization expired")
		return errors.New("authorization expired")
	}
	if strings.TrimSpace(callback.Error) != "" {
		a.setLoginError(loginID, "authorization was rejected")
		return errors.New("authorization was rejected")
	}
	if strings.TrimSpace(callback.Code) == "" {
		a.setLoginError(loginID, "authorization code is missing")
		return errors.New("authorization code is missing")
	}

	credential, err := a.exchangeAuthorizationCode(ctx, transaction, callback.Code)
	if err == nil {
		err = a.populateUserInfo(ctx, &credential)
	}
	if err != nil {
		a.setLoginError(loginID, "Google token exchange failed")
		return err
	}
	if err := a.saveCredential(credential); err != nil {
		a.setLoginError(loginID, "Google credential storage failed")
		return err
	}
	if _, err := a.syncProvider(ctx, credential.AccessToken); err != nil {
		_ = deleteProviderCredentialSlot(a.manager.credentials, googleGeminiAccountProviderID, providerCredentialSlotAccount)
		a.setLoginError(loginID, "Gemini model discovery failed")
		return err
	}

	a.mu.Lock()
	transaction.Completed = true
	transaction.Verifier = ""
	delete(a.byState, transaction.State)
	a.mu.Unlock()
	return nil
}

func (a *googleGeminiAccountAdapter) setLoginError(loginID, message string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if transaction := a.logins[loginID]; transaction != nil {
		transaction.Err = message
		transaction.Verifier = ""
		delete(a.byState, transaction.State)
	}
}

func (a *googleGeminiAccountAdapter) PollLogin(ctx context.Context, directory, loginID string) (providerAccountStatus, error) {
	a.mu.Lock()
	transaction := a.logins[strings.TrimSpace(loginID)]
	if transaction == nil {
		a.mu.Unlock()
		return providerAccountStatus{}, errors.New("provider login transaction not found")
	}
	completed, loginErr, expiresAt := transaction.Completed, transaction.Err, transaction.ExpiresAt
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
	if time.Now().UTC().After(expiresAt) {
		status.State = providerAccountExpired
		status.Connected = false
		return status, nil
	}
	status.State = providerAccountConnecting
	status.Connected = false
	return status, nil
}

func (a *googleGeminiAccountAdapter) CancelLogin(_ context.Context, _ string, loginID string) error {
	loginID = strings.TrimSpace(loginID)
	if loginID == "" {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	transaction := a.logins[loginID]
	if transaction == nil {
		return nil
	}
	delete(a.byState, transaction.State)
	delete(a.logins, loginID)
	return nil
}

func (a *googleGeminiAccountAdapter) exchangeAuthorizationCode(ctx context.Context, transaction *googleGeminiLoginTransaction, code string) (providerOAuthCredential, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	clientID, _ := a.setupValues()
	form.Set("client_id", clientID)
	form.Set("code", strings.TrimSpace(code))
	form.Set("redirect_uri", transaction.RedirectURI)
	form.Set("code_verifier", transaction.Verifier)
	return a.exchangeToken(ctx, form)
}

func (a *googleGeminiAccountAdapter) exchangeToken(ctx context.Context, form url.Values) (providerOAuthCredential, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, a.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return providerOAuthCredential{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := a.client.Do(request)
	if err != nil {
		return providerOAuthCredential{}, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return providerOAuthCredential{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return providerOAuthCredential{}, fmt.Errorf("Google token exchange failed with status %d", response.StatusCode)
	}
	var payload struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		Scope        string `json:"scope"`
		ExpiresIn    any    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return providerOAuthCredential{}, errors.New("Google token response was invalid")
	}
	if strings.TrimSpace(payload.AccessToken) == "" {
		return providerOAuthCredential{}, errors.New("Google token response did not include an access token")
	}
	expiresAt := ""
	switch value := payload.ExpiresIn.(type) {
	case float64:
		if value > 0 {
			expiresAt = time.Now().UTC().Add(time.Duration(value) * time.Second).Format(time.RFC3339)
		}
	case string:
		if seconds, parseErr := strconv.Atoi(value); parseErr == nil && seconds > 0 {
			expiresAt = time.Now().UTC().Add(time.Duration(seconds) * time.Second).Format(time.RFC3339)
		}
	}
	return providerOAuthCredential{
		AccessToken:  strings.TrimSpace(payload.AccessToken),
		RefreshToken: strings.TrimSpace(payload.RefreshToken),
		TokenType:    strings.TrimSpace(payload.TokenType),
		ExpiresAt:    expiresAt,
		Scopes:       strings.Fields(strings.TrimSpace(payload.Scope)),
		AccountType:  "Google account",
	}, nil
}

func (a *googleGeminiAccountAdapter) populateUserInfo(ctx context.Context, credential *providerOAuthCredential) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, a.userInfoURL, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+credential.AccessToken)
	response, err := a.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("Google user info failed with status %d", response.StatusCode)
	}
	var info struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&info); err != nil {
		return errors.New("Google user info response was invalid")
	}
	if strings.TrimSpace(info.Email) != "" {
		credential.AccountLabel = strings.TrimSpace(info.Email)
	} else {
		credential.AccountLabel = strings.TrimSpace(info.Name)
	}
	return nil
}

func (a *googleGeminiAccountAdapter) refreshCredential(ctx context.Context, current providerOAuthCredential) (providerOAuthCredential, error) {
	if strings.TrimSpace(current.RefreshToken) == "" {
		return providerOAuthCredential{}, errors.New("Google account needs reauthentication")
	}
	clientID, _ := a.setupValues()
	if clientID == "" {
		return providerOAuthCredential{}, errors.New("TL Studio Google OAuth client is not configured")
	}
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", clientID)
	form.Set("refresh_token", current.RefreshToken)
	next, err := a.exchangeToken(ctx, form)
	if err != nil {
		return providerOAuthCredential{}, err
	}
	if strings.TrimSpace(next.RefreshToken) == "" {
		next.RefreshToken = current.RefreshToken
	}
	if len(next.Scopes) == 0 {
		next.Scopes = append([]string(nil), current.Scopes...)
	}
	next.AccountLabel = current.AccountLabel
	next.AccountType = current.AccountType
	if err := a.saveCredential(next); err != nil {
		return providerOAuthCredential{}, err
	}
	return next, nil
}

func (a *googleGeminiAccountAdapter) Refresh(ctx context.Context, _ string) (providerAccountStatus, error) {
	credential, err := a.loadCredential()
	if err != nil {
		return providerAccountStatus{}, err
	}
	if strings.TrimSpace(credential.RefreshToken) != "" {
		credential, err = a.refreshCredential(ctx, credential)
		if err != nil {
			return providerAccountStatus{}, err
		}
	}
	if _, err := a.syncProvider(ctx, credential.AccessToken); err != nil {
		return providerAccountStatus{}, err
	}
	return a.statusFromCredential(credential), nil
}

func (a *googleGeminiAccountAdapter) ResolveCredential(ctx context.Context, _ string) (string, error) {
	credential, err := a.loadCredential()
	if err != nil {
		return "", err
	}
	if credential.needsRefresh(time.Now()) {
		if strings.TrimSpace(credential.RefreshToken) == "" {
			return "", errors.New("Google account needs reauthentication")
		}
		credential, err = a.refreshCredential(ctx, credential)
		if err != nil {
			return "", err
		}
	}
	return strings.TrimSpace(credential.AccessToken), nil
}

func (a *googleGeminiAccountAdapter) DiscoverModels(ctx context.Context, directory string) ([]string, error) {
	token, err := a.ResolveCredential(ctx, directory)
	if err != nil {
		return nil, err
	}
	return a.syncProvider(ctx, token)
}

func (a *googleGeminiAccountAdapter) syncProvider(ctx context.Context, accessToken string) ([]string, error) {
	_, projectID := a.setupValues()
	if projectID == "" {
		return nil, errors.New("Google Cloud project ID is required for Gemini API quota and billing")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, a.modelsURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("x-goog-user-project", projectID)
	request.Header.Set("x-goog-api-client", "tl-studio/"+strings.TrimSpace(version))
	response, err := a.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, providerDiscoveryMaxBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > providerDiscoveryMaxBodyBytes {
		return nil, errors.New("Gemini model catalog response is too large")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("Gemini model discovery failed with status %d", response.StatusCode)
	}
	var payload struct {
		Models []struct {
			Name                       string   `json:"name"`
			DisplayName                string   `json:"displayName"`
			InputTokenLimit            int      `json:"inputTokenLimit"`
			OutputTokenLimit           int      `json:"outputTokenLimit"`
			SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, errors.New("Gemini model catalog response was invalid")
	}

	models := make([]tlProviderModel, 0, len(payload.Models))
	ids := make([]string, 0, len(payload.Models))
	for _, item := range payload.Models {
		supportsGenerateContent := false
		for _, method := range item.SupportedGenerationMethods {
			if method == "generateContent" {
				supportsGenerateContent = true
				break
			}
		}
		if !supportsGenerateContent {
			continue
		}
		id := strings.TrimPrefix(strings.TrimSpace(item.Name), "models/")
		if id == "" {
			continue
		}
		name := strings.TrimSpace(item.DisplayName)
		if name == "" {
			name = id
		}
		models = append(models, tlProviderModel{
			ID:           id,
			Name:         name,
			ToolCall:     true,
			ContextLimit: item.InputTokenLimit,
			OutputLimit:  item.OutputTokenLimit,
		})
		ids = append(ids, id)
	}
	if len(models) == 0 {
		return nil, errors.New("Gemini returned no generateContent models")
	}
	definition := tlProviderDefinition{
		ID:        googleGeminiAccountProviderID,
		Name:      "Google / Gemini",
		Protocol:  "gemini-generate-content",
		BaseURL:   a.baseURL,
		ManagedBy: "account",
		ProjectID: projectID,
		Models:    models,
	}
	if err := a.manager.store.put(definition); err != nil {
		return nil, err
	}
	return ids, nil
}

func (a *googleGeminiAccountAdapter) revoke(ctx context.Context, credential providerOAuthCredential) {
	token := strings.TrimSpace(credential.RefreshToken)
	if token == "" {
		token = strings.TrimSpace(credential.AccessToken)
	}
	if token == "" {
		return
	}
	form := url.Values{}
	form.Set("token", token)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, a.revokeURL, strings.NewReader(form.Encode()))
	if err != nil {
		return
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := a.client.Do(request)
	if err == nil && response != nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
		_ = response.Body.Close()
	}
}

func (a *googleGeminiAccountAdapter) Disconnect(ctx context.Context, _ string) error {
	if a == nil || a.manager == nil || a.manager.credentials == nil {
		return nil
	}
	if credential, err := a.loadCredential(); err == nil {
		a.revoke(ctx, credential)
	}
	if err := deleteProviderCredentialSlot(a.manager.credentials, googleGeminiAccountProviderID, providerCredentialSlotAccount); err != nil {
		return err
	}
	if provider, found, err := a.manager.store.get(googleGeminiAccountProviderID); err == nil && found && provider.ManagedBy == "account" {
		_ = a.manager.store.remove(googleGeminiAccountProviderID)
	}
	return nil
}
