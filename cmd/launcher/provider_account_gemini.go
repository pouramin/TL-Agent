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
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	googleGeminiAccountProviderID = "gemini"
	googleGeminiBaseURL           = "https://generativelanguage.googleapis.com/v1beta"
	googleGeminiModelsURL         = "https://generativelanguage.googleapis.com/v1/models"
)

type googleGeminiLoginTransaction struct {
	id          string
	verifier    string
	redirectURI string
	expiresAt   time.Time
	done        chan error
}

type googleGeminiAccountAdapter struct {
	state      *appState
	manager    *providerManager
	clientID   string
	projectID  string
	httpClient *http.Client

	authorizeURL string
	tokenURL     string
	userInfoURL  string
	modelsURL    string
	revokeURL    string
	baseURL      string

	mu     sync.Mutex
	logins map[string]*googleGeminiLoginTransaction
}

func newGoogleGeminiAccountAdapter(state *appState, manager *providerManager) *googleGeminiAccountAdapter {
	return &googleGeminiAccountAdapter{
		state:        state,
		manager:      manager,
		clientID:     strings.TrimSpace(os.Getenv("TL_STUDIO_GOOGLE_CLIENT_ID")),
		projectID:    strings.TrimSpace(os.Getenv("TL_STUDIO_GOOGLE_PROJECT_ID")),
		httpClient:   &http.Client{Timeout: 20 * time.Second},
		authorizeURL: "https://accounts.google.com/o/oauth2/v2/auth",
		tokenURL:     "https://oauth2.googleapis.com/token",
		userInfoURL:  "https://www.googleapis.com/oauth2/v2/userinfo",
		modelsURL:    googleGeminiModelsURL,
		revokeURL:    "https://oauth2.googleapis.com/revoke",
		baseURL:      googleGeminiBaseURL,
		logins:       map[string]*googleGeminiLoginTransaction{},
	}
}

func (a *googleGeminiAccountAdapter) ID() string { return googleGeminiAccountProviderID }

func (a *googleGeminiAccountAdapter) available() bool {
	return a != nil &&
		a.state != nil &&
		a.manager != nil &&
		a.manager.credentials != nil &&
		strings.TrimSpace(a.clientID) != "" &&
		strings.TrimSpace(a.projectID) != ""
}

func (a *googleGeminiAccountAdapter) unavailableReason() string {
	switch {
	case strings.TrimSpace(a.clientID) == "":
		return "TL Studio needs its registered Google Desktop OAuth client ID before Google account login can be enabled."
	case strings.TrimSpace(a.projectID) == "":
		return "Gemini OAuth requires a Google Cloud project with the Gemini API enabled for API quota and billing. A Gemini consumer subscription is not an API entitlement."
	default:
		return "The Google / Gemini account adapter is unavailable in this build."
	}
}

func (a *googleGeminiAccountAdapter) callbackURI() (string, error) {
	if a.state == nil {
		return "", errors.New("TL Studio local server is unavailable")
	}
	a.state.mu.RLock()
	base := strings.TrimRight(a.state.frontendURL, "/")
	a.state.mu.RUnlock()
	if base == "" {
		return "", errors.New("TL Studio local callback URL is unavailable")
	}
	return base + "/local/provider-accounts/" + googleGeminiAccountProviderID + "/oauth/callback", nil
}

func (a *googleGeminiAccountAdapter) loadCredential() (providerAccountStoredCredential, error) {
	if a == nil || a.manager == nil || a.manager.credentials == nil {
		return providerAccountStoredCredential{}, errCredentialNotFound
	}
	raw, err := a.manager.credentials.Get(providerAccountCredentialID(googleGeminiAccountProviderID))
	if err != nil {
		return providerAccountStoredCredential{}, err
	}
	var credential providerAccountStoredCredential
	if err := json.Unmarshal([]byte(raw), &credential); err != nil {
		return providerAccountStoredCredential{}, errors.New("invalid Google account credential")
	}
	credential.AccessToken = strings.TrimSpace(credential.AccessToken)
	credential.RefreshToken = strings.TrimSpace(credential.RefreshToken)
	if credential.AccessToken == "" {
		return providerAccountStoredCredential{}, errCredentialNotFound
	}
	return credential, nil
}

func (a *googleGeminiAccountAdapter) saveCredential(credential providerAccountStoredCredential) error {
	encoded, err := encodeProviderAccountCredential(credential)
	if err != nil {
		return err
	}
	return a.manager.credentials.Put(providerAccountCredentialID(googleGeminiAccountProviderID), encoded)
}

func (a *googleGeminiAccountAdapter) statusFromCredential(credential providerAccountStoredCredential) providerAccountStatus {
	state := providerAccountConnected
	if accountCredentialExpired(credential, 0) {
		if credential.RefreshToken != "" {
			state = providerAccountExpired
		} else {
			state = providerAccountNeedsReauthentication
		}
	}
	models := []string{}
	if provider, found, err := a.manager.store.get(googleGeminiAccountProviderID); err == nil && found {
		for _, model := range provider.Models {
			models = append(models, model.ID)
		}
	}
	return providerAccountStatus{
		ID:          googleGeminiAccountProviderID,
		Name:        "Google / Gemini",
		Description: "Official Google OAuth for the Gemini API. API quota and billing remain tied to the configured Google Cloud project.",
		Available:   a.available(),
		Connected:   state == providerAccountConnected,
		State:       state,
		AuthModes:   []string{"authorization_code_pkce"},
		AccountLabel: credential.AccountLabel,
		AccountType:  credential.AccountType,
		Entitlement:  credential.Entitlement,
		Models:       models,
	}
}

func (a *googleGeminiAccountAdapter) Status(ctx context.Context, _ string) (providerAccountStatus, error) {
	if !a.available() {
		return providerAccountStatus{
			ID:                googleGeminiAccountProviderID,
			Name:              "Google / Gemini",
			Description:       "Official Google account authorization for Gemini API access.",
			Available:         false,
			State:             providerAccountDisconnected,
			AuthModes:         []string{"authorization_code_pkce"},
			UnsupportedReason: a.unavailableReason(),
		}, nil
	}
	credential, err := a.loadCredential()
	if errors.Is(err, errCredentialNotFound) {
		return providerAccountStatus{
			ID:          googleGeminiAccountProviderID,
			Name:        "Google / Gemini",
			Description: "Official Google account authorization for Gemini API access.",
			Available:   true,
			State:       providerAccountDisconnected,
			AuthModes:   []string{"authorization_code_pkce"},
		}, nil
	}
	if err != nil {
		return providerAccountStatus{}, err
	}
	if accountCredentialExpired(credential, 2*time.Minute) && credential.RefreshToken != "" {
		if refreshed, refreshErr := a.refreshCredential(ctx, credential); refreshErr == nil {
			credential = refreshed
		}
	}
	return a.statusFromCredential(credential), nil
}

func (a *googleGeminiAccountAdapter) BeginLogin(context.Context, string) (providerAccountLoginChallenge, error) {
	if !a.available() {
		return providerAccountLoginChallenge{}, errors.New(a.unavailableReason())
	}
	loginID, err := randomOAuthValue(24)
	if err != nil {
		return providerAccountLoginChallenge{}, err
	}
	verifier, err := randomOAuthValue(48)
	if err != nil {
		return providerAccountLoginChallenge{}, err
	}
	redirectURI, err := a.callbackURI()
	if err != nil {
		return providerAccountLoginChallenge{}, err
	}
	expiresAt := time.Now().UTC().Add(10 * time.Minute)
	transaction := &googleGeminiLoginTransaction{
		id: loginID, verifier: verifier, redirectURI: redirectURI,
		expiresAt: expiresAt, done: make(chan error, 1),
	}
	a.mu.Lock()
	a.logins[loginID] = transaction
	a.mu.Unlock()

	query := url.Values{}
	query.Set("client_id", a.clientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("response_type", "code")
	query.Set("scope", strings.Join([]string{
		"https://www.googleapis.com/auth/userinfo.profile",
		"https://www.googleapis.com/auth/userinfo.email",
		"https://www.googleapis.com/auth/generative-language.retriever",
	}, " "))
	query.Set("state", loginID)
	query.Set("code_challenge", oauthPKCEChallenge(verifier))
	query.Set("code_challenge_method", "S256")
	query.Set("access_type", "offline")
	query.Set("prompt", "consent")

	return providerAccountLoginChallenge{
		LoginID:          loginID,
		Flow:             "authorization_code_pkce",
		AuthorizationURL: a.authorizeURL + "?" + query.Encode(),
		Instructions:     "Sign in with Google and authorize Gemini API access. Gemini API quota and billing use the configured Google Cloud project.",
		ExpiresAt:        expiresAt.Format(time.RFC3339),
	}, nil
}

func (a *googleGeminiAccountAdapter) CompleteLogin(ctx context.Context, _ string, loginID string) (providerAccountStatus, error) {
	a.mu.Lock()
	transaction, ok := a.logins[loginID]
	a.mu.Unlock()
	if !ok {
		return providerAccountStatus{}, errors.New("provider login transaction was not found or already completed")
	}
	timer := time.NewTimer(time.Until(transaction.expiresAt))
	defer timer.Stop()
	select {
	case err := <-transaction.done:
		a.mu.Lock()
		delete(a.logins, loginID)
		a.mu.Unlock()
		if err != nil {
			return providerAccountStatus{}, err
		}
		return a.Status(ctx, "")
	case <-ctx.Done():
		return providerAccountStatus{}, ctx.Err()
	case <-timer.C:
		a.cancelLogin(loginID)
		return providerAccountStatus{}, context.DeadlineExceeded
	}
}

func (a *googleGeminiAccountAdapter) finishLogin(transaction *googleGeminiLoginTransaction, err error) {
	select {
	case transaction.done <- err:
	default:
	}
}

func (a *googleGeminiAccountAdapter) HandleCallback(ctx context.Context, _ string, loginID string, values url.Values) error {
	a.mu.Lock()
	transaction, ok := a.logins[loginID]
	a.mu.Unlock()
	if !ok {
		return errors.New("provider login state is invalid")
	}
	if time.Now().UTC().After(transaction.expiresAt) {
		err := errors.New("provider login expired")
		a.finishLogin(transaction, err)
		return err
	}
	if returnedState := strings.TrimSpace(values.Get("state")); returnedState == "" || returnedState != transaction.id {
		err := errors.New("provider login state validation failed")
		a.finishLogin(transaction, err)
		return err
	}
	if providerError := strings.TrimSpace(values.Get("error")); providerError != "" {
		err := fmt.Errorf("provider authorization failed: %s", providerError)
		a.finishLogin(transaction, err)
		return err
	}
	code := strings.TrimSpace(values.Get("code"))
	if code == "" {
		err := errors.New("provider authorization code is missing")
		a.finishLogin(transaction, err)
		return err
	}
	credential, err := a.exchangeAuthorizationCode(ctx, transaction, code)
	if err == nil {
		err = a.populateUserInfo(ctx, &credential)
	}
	if err == nil {
		err = a.saveCredential(credential)
	}
	if err == nil {
		_, err = a.syncProvider(ctx, credential.AccessToken)
	}
	a.finishLogin(transaction, err)
	return err
}

func (a *googleGeminiAccountAdapter) cancelLogin(loginID string) {
	a.mu.Lock()
	transaction, ok := a.logins[loginID]
	if ok {
		delete(a.logins, loginID)
	}
	a.mu.Unlock()
	if ok {
		a.finishLogin(transaction, context.Canceled)
	}
}

func (a *googleGeminiAccountAdapter) exchangeAuthorizationCode(ctx context.Context, transaction *googleGeminiLoginTransaction, code string) (providerAccountStoredCredential, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", a.clientID)
	form.Set("code", code)
	form.Set("redirect_uri", transaction.redirectURI)
	form.Set("code_verifier", transaction.verifier)
	return a.exchangeToken(ctx, form)
}

func (a *googleGeminiAccountAdapter) exchangeToken(ctx context.Context, form url.Values) (providerAccountStoredCredential, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return providerAccountStoredCredential{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := a.httpClient.Do(req)
	if err != nil {
		return providerAccountStoredCredential{}, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return providerAccountStoredCredential{}, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return providerAccountStoredCredential{}, fmt.Errorf("Google token exchange failed with status %d", res.StatusCode)
	}
	var payload struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		Scope        string `json:"scope"`
		ExpiresIn    any    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return providerAccountStoredCredential{}, errors.New("Google token response was invalid")
	}
	if strings.TrimSpace(payload.AccessToken) == "" {
		return providerAccountStoredCredential{}, errors.New("Google token response did not include an access token")
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
	return providerAccountStoredCredential{
		AccessToken:  strings.TrimSpace(payload.AccessToken),
		RefreshToken: strings.TrimSpace(payload.RefreshToken),
		TokenType:    strings.TrimSpace(payload.TokenType),
		Scope:        strings.TrimSpace(payload.Scope),
		ExpiresAt:    expiresAt,
		AccountType:  "Google account",
		Entitlement:  "Gemini API · Google Cloud project " + a.projectID,
	}, nil
}

func (a *googleGeminiAccountAdapter) populateUserInfo(ctx context.Context, credential *providerAccountStoredCredential) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.userInfoURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+credential.AccessToken)
	res, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("Google user info failed with status %d", res.StatusCode)
	}
	var info struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&info); err != nil {
		return errors.New("Google user info response was invalid")
	}
	if strings.TrimSpace(info.Email) != "" {
		credential.AccountLabel = strings.TrimSpace(info.Email)
	} else {
		credential.AccountLabel = strings.TrimSpace(info.Name)
	}
	return nil
}

func (a *googleGeminiAccountAdapter) refreshCredential(ctx context.Context, current providerAccountStoredCredential) (providerAccountStoredCredential, error) {
	if strings.TrimSpace(current.RefreshToken) == "" {
		return providerAccountStoredCredential{}, errors.New("Google account needs reauthentication")
	}
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", a.clientID)
	form.Set("refresh_token", current.RefreshToken)
	next, err := a.exchangeToken(ctx, form)
	if err != nil {
		return providerAccountStoredCredential{}, err
	}
	if next.RefreshToken == "" {
		next.RefreshToken = current.RefreshToken
	}
	next.AccountLabel = current.AccountLabel
	next.AccountType = current.AccountType
	next.Entitlement = current.Entitlement
	if err := a.saveCredential(next); err != nil {
		return providerAccountStoredCredential{}, err
	}
	return next, nil
}

func (a *googleGeminiAccountAdapter) RuntimeCredential(ctx context.Context) (string, error) {
	credential, err := a.loadCredential()
	if err != nil {
		return "", err
	}
	if accountCredentialExpired(credential, 2*time.Minute) {
		if credential.RefreshToken == "" {
			return "", errors.New("Google account needs reauthentication")
		}
		credential, err = a.refreshCredential(ctx, credential)
		if err != nil {
			return "", err
		}
	}
	return strings.TrimSpace(credential.AccessToken), nil
}

func (a *googleGeminiAccountAdapter) Refresh(ctx context.Context, _ string) (providerAccountStatus, error) {
	credential, err := a.loadCredential()
	if err != nil {
		return providerAccountStatus{}, err
	}
	if credential.RefreshToken != "" {
		credential, err = a.refreshCredential(ctx, credential)
		if err != nil {
			return providerAccountStatus{}, err
		}
	}
	_, _ = a.syncProvider(ctx, credential.AccessToken)
	return a.statusFromCredential(credential), nil
}

func (a *googleGeminiAccountAdapter) DiscoverModels(ctx context.Context, _ string) ([]string, error) {
	token, err := a.RuntimeCredential(ctx)
	if err != nil {
		return nil, err
	}
	return a.syncProvider(ctx, token)
}

func (a *googleGeminiAccountAdapter) syncProvider(ctx context.Context, accessToken string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.modelsURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("x-goog-user-project", a.projectID)
	res, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, providerDiscoveryMaxBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > providerDiscoveryMaxBodyBytes {
		return nil, errors.New("Gemini model catalog response is too large")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("Gemini model discovery failed with status %d", res.StatusCode)
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
		ProjectID: a.projectID,
		Models:    models,
	}
	if err := a.manager.store.put(definition); err != nil {
		return nil, err
	}
	return ids, nil
}

func (a *googleGeminiAccountAdapter) revoke(ctx context.Context, credential providerAccountStoredCredential) {
	token := strings.TrimSpace(credential.RefreshToken)
	if token == "" {
		token = strings.TrimSpace(credential.AccessToken)
	}
	if token == "" {
		return
	}
	form := url.Values{}
	form.Set("token", token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.revokeURL, strings.NewReader(form.Encode()))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := a.httpClient.Do(req)
	if err == nil && res != nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 64<<10))
		_ = res.Body.Close()
	}
}

func (a *googleGeminiAccountAdapter) Disconnect(ctx context.Context, _ string) error {
	if a == nil || a.manager == nil || a.manager.credentials == nil {
		return nil
	}
	if credential, err := a.loadCredential(); err == nil {
		a.revoke(ctx, credential)
	}
	if err := a.manager.credentials.Delete(providerAccountCredentialID(googleGeminiAccountProviderID)); err != nil {
		return err
	}
	if provider, found, err := a.manager.store.get(googleGeminiAccountProviderID); err == nil && found && provider.ManagedBy == "account" {
		_ = a.manager.store.remove(googleGeminiAccountProviderID)
	}
	return nil
}

func (a *googleGeminiAccountAdapter) CancelLogin(_ context.Context, _ string, loginID string) error {
	a.cancelLogin(strings.TrimSpace(loginID))
	return nil
}
