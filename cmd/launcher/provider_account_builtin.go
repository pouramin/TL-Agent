package main

import (
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
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	huggingFaceAccountProviderID = "huggingface"
	huggingFaceInferenceBaseURL   = "https://router.huggingface.co/v1"
)

type providerAccountStoredCredential struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken,omitempty"`
	TokenType    string `json:"tokenType,omitempty"`
	Scope        string `json:"scope,omitempty"`
	ExpiresAt    string `json:"expiresAt,omitempty"`
	AccountLabel string `json:"accountLabel,omitempty"`
	AccountType  string `json:"accountType,omitempty"`
	Entitlement  string `json:"entitlement,omitempty"`
}

type unavailableProviderAccountAdapter struct {
	id          string
	name        string
	description string
	reason      string
}

func (a *unavailableProviderAccountAdapter) ID() string { return a.id }

func (a *unavailableProviderAccountAdapter) Status(context.Context, string) (providerAccountStatus, error) {
	return providerAccountStatus{
		ID: a.id, Name: a.name, Description: a.description,
		Available: false, State: providerAccountDisconnected,
		AuthModes: []string{"account"}, UnsupportedReason: a.reason,
	}, nil
}

func (a *unavailableProviderAccountAdapter) unsupported() error {
	return errors.New(a.reason)
}

func (a *unavailableProviderAccountAdapter) BeginLogin(context.Context, string) (providerAccountLoginChallenge, error) {
	return providerAccountLoginChallenge{}, a.unsupported()
}
func (a *unavailableProviderAccountAdapter) CompleteLogin(context.Context, string, string) (providerAccountStatus, error) {
	return providerAccountStatus{}, a.unsupported()
}
func (a *unavailableProviderAccountAdapter) HandleCallback(context.Context, string, string, url.Values) error {
	return a.unsupported()
}
func (a *unavailableProviderAccountAdapter) CancelLogin(context.Context, string, string) error {
	return nil
}
func (a *unavailableProviderAccountAdapter) Refresh(context.Context, string) (providerAccountStatus, error) {
	return providerAccountStatus{}, a.unsupported()
}
func (a *unavailableProviderAccountAdapter) Disconnect(context.Context, string) error { return nil }
func (a *unavailableProviderAccountAdapter) DiscoverModels(context.Context, string) ([]string, error) {
	return nil, a.unsupported()
}

type huggingFaceLoginTransaction struct {
	id          string
	verifier    string
	redirectURI string
	expiresAt   time.Time
	done        chan error
}

type huggingFaceAccountAdapter struct {
	state       *appState
	manager     *providerManager
	clientID    string
	httpClient       *http.Client
	authorizeURL      string
	tokenURL          string
	userInfoURL       string
	inferenceBaseURL  string

	mu     sync.Mutex
	logins map[string]*huggingFaceLoginTransaction
}

func newHuggingFaceAccountAdapter(state *appState, manager *providerManager) *huggingFaceAccountAdapter {
	return &huggingFaceAccountAdapter{
		state: state,
		manager: manager,
		clientID: strings.TrimSpace(os.Getenv("TL_STUDIO_HUGGINGFACE_CLIENT_ID")),
		httpClient: &http.Client{Timeout: 20 * time.Second},
		authorizeURL: "https://huggingface.co/oauth/authorize",
		tokenURL: "https://huggingface.co/oauth/token",
		userInfoURL: "https://huggingface.co/oauth/userinfo",
		inferenceBaseURL: huggingFaceInferenceBaseURL,
		logins: map[string]*huggingFaceLoginTransaction{},
	}
}

func (a *huggingFaceAccountAdapter) ID() string { return huggingFaceAccountProviderID }

func (a *huggingFaceAccountAdapter) available() bool {
	return a != nil && a.state != nil && a.manager != nil && a.manager.credentials != nil && strings.TrimSpace(a.clientID) != ""
}

func (a *huggingFaceAccountAdapter) unavailableReason() string {
	if strings.TrimSpace(a.clientID) == "" {
		return "A registered public Hugging Face OAuth client is required. TL Studio does not embed a private client secret."
	}
	return "The Hugging Face account adapter is unavailable in this build."
}

func randomOAuthValue(bytes int) (string, error) {
	value := make([]byte, bytes)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func oauthPKCEChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func (a *huggingFaceAccountAdapter) loadCredential() (providerAccountStoredCredential, error) {
	if a == nil || a.manager == nil || a.manager.credentials == nil {
		return providerAccountStoredCredential{}, errCredentialNotFound
	}
	raw, err := a.manager.credentials.Get(providerAccountCredentialID(huggingFaceAccountProviderID))
	if err != nil {
		return providerAccountStoredCredential{}, err
	}
	var credential providerAccountStoredCredential
	if err := json.Unmarshal([]byte(raw), &credential); err != nil {
		return providerAccountStoredCredential{}, errors.New("invalid Hugging Face account credential")
	}
	credential.AccessToken = strings.TrimSpace(credential.AccessToken)
	credential.RefreshToken = strings.TrimSpace(credential.RefreshToken)
	if credential.AccessToken == "" {
		return providerAccountStoredCredential{}, errCredentialNotFound
	}
	return credential, nil
}

func (a *huggingFaceAccountAdapter) saveCredential(credential providerAccountStoredCredential) error {
	encoded, err := encodeProviderAccountCredential(credential)
	if err != nil {
		return err
	}
	return a.manager.credentials.Put(providerAccountCredentialID(huggingFaceAccountProviderID), encoded)
}

func accountCredentialExpired(credential providerAccountStoredCredential, margin time.Duration) bool {
	if strings.TrimSpace(credential.ExpiresAt) == "" {
		return false
	}
	expiresAt, err := time.Parse(time.RFC3339, credential.ExpiresAt)
	if err != nil {
		return true
	}
	return time.Now().UTC().Add(margin).After(expiresAt)
}

func (a *huggingFaceAccountAdapter) statusFromCredential(credential providerAccountStoredCredential) providerAccountStatus {
	state := providerAccountConnected
	if accountCredentialExpired(credential, 0) {
		if credential.RefreshToken != "" {
			state = providerAccountExpired
		} else {
			state = providerAccountNeedsReauthentication
		}
	}
	models := []string{}
	if provider, found, err := a.manager.store.get(huggingFaceAccountProviderID); err == nil && found {
		for _, model := range provider.Models {
			models = append(models, model.ID)
		}
	}
	return providerAccountStatus{
		ID: huggingFaceAccountProviderID,
		Name: "Hugging Face",
		Description: "Uses official Hugging Face OAuth with the inference-api scope.",
		Available: a.available(),
		State: state,
		Connected: state == providerAccountConnected,
		AuthModes: []string{"authorization_code_pkce"},
		AccountLabel: credential.AccountLabel,
		AccountType: credential.AccountType,
		Entitlement: credential.Entitlement,
		Models: models,
	}
}

func (a *huggingFaceAccountAdapter) Status(ctx context.Context, directory string) (providerAccountStatus, error) {
	if !a.available() {
		return providerAccountStatus{
			ID: huggingFaceAccountProviderID,
			Name: "Hugging Face",
			Description: "Official account login for Hugging Face Inference Providers.",
			Available: false,
			State: providerAccountDisconnected,
			AuthModes: []string{"authorization_code_pkce"},
			UnsupportedReason: a.unavailableReason(),
		}, nil
	}
	credential, err := a.loadCredential()
	if errors.Is(err, errCredentialNotFound) {
		return providerAccountStatus{
			ID: huggingFaceAccountProviderID,
			Name: "Hugging Face",
			Description: "Official account login for Hugging Face Inference Providers.",
			Available: true,
			State: providerAccountDisconnected,
			AuthModes: []string{"authorization_code_pkce"},
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

func (a *huggingFaceAccountAdapter) callbackURI() (string, error) {
	if a.state == nil {
		return "", errors.New("TL Studio local server is unavailable")
	}
	a.state.mu.RLock()
	base := strings.TrimRight(a.state.frontendURL, "/")
	a.state.mu.RUnlock()
	if base == "" {
		return "", errors.New("TL Studio local callback URL is unavailable")
	}
	return base + "/local/provider-accounts/" + huggingFaceAccountProviderID + "/oauth/callback", nil
}

func (a *huggingFaceAccountAdapter) BeginLogin(context.Context, string) (providerAccountLoginChallenge, error) {
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
	transaction := &huggingFaceLoginTransaction{
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
	query.Set("scope", "openid profile email inference-api")
	query.Set("state", loginID)
	query.Set("code_challenge", oauthPKCEChallenge(verifier))
	query.Set("code_challenge_method", "S256")

	return providerAccountLoginChallenge{
		LoginID: loginID,
		Flow: "authorization_code_pkce",
		AuthorizationURL: a.authorizeURL + "?" + query.Encode(),
		Instructions: "Complete the Hugging Face authorization in your browser. TL Studio will finish automatically.",
		ExpiresAt: expiresAt.Format(time.RFC3339),
	}, nil
}

func (a *huggingFaceAccountAdapter) CompleteLogin(ctx context.Context, _ string, loginID string) (providerAccountStatus, error) {
	a.mu.Lock()
	transaction, ok := a.logins[loginID]
	a.mu.Unlock()
	if !ok {
		return providerAccountStatus{}, errors.New("provider login transaction was not found or already completed")
	}
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
	case <-time.After(time.Until(transaction.expiresAt)):
		_ = a.CancelLogin(context.Background(), "", loginID)
		return providerAccountStatus{}, context.DeadlineExceeded
	}
}

func (a *huggingFaceAccountAdapter) finishLogin(transaction *huggingFaceLoginTransaction, err error) {
	select {
	case transaction.done <- err:
	default:
	}
}

func (a *huggingFaceAccountAdapter) HandleCallback(ctx context.Context, _ string, loginID string, values url.Values) error {
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
		err = a.saveCredential(credential)
	}
	if err == nil {
		_, err = a.syncProvider(ctx, credential.AccessToken)
	}
	a.finishLogin(transaction, err)
	return err
}

func (a *huggingFaceAccountAdapter) cancelLogin(loginID string) {
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

func (a *huggingFaceAccountAdapter) exchangeAuthorizationCode(ctx context.Context, transaction *huggingFaceLoginTransaction, code string) (providerAccountStoredCredential, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", a.clientID)
	form.Set("code", code)
	form.Set("redirect_uri", transaction.redirectURI)
	form.Set("code_verifier", transaction.verifier)
	credential, err := a.exchangeToken(ctx, form)
	if err != nil {
		return providerAccountStoredCredential{}, err
	}
	if err := a.populateUserInfo(ctx, &credential); err != nil {
		return providerAccountStoredCredential{}, err
	}
	return credential, nil
}

func (a *huggingFaceAccountAdapter) exchangeToken(ctx context.Context, form url.Values) (providerAccountStoredCredential, error) {
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
		return providerAccountStoredCredential{}, fmt.Errorf("Hugging Face token exchange failed with status %d", res.StatusCode)
	}
	var payload struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		Scope        string `json:"scope"`
		ExpiresIn    any    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return providerAccountStoredCredential{}, errors.New("Hugging Face token response was invalid")
	}
	if strings.TrimSpace(payload.AccessToken) == "" {
		return providerAccountStoredCredential{}, errors.New("Hugging Face token response did not include an access token")
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
		AccessToken: strings.TrimSpace(payload.AccessToken),
		RefreshToken: strings.TrimSpace(payload.RefreshToken),
		TokenType: strings.TrimSpace(payload.TokenType),
		Scope: strings.TrimSpace(payload.Scope),
		ExpiresAt: expiresAt,
	}, nil
}

func (a *huggingFaceAccountAdapter) populateUserInfo(ctx context.Context, credential *providerAccountStoredCredential) error {
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
		return fmt.Errorf("Hugging Face user info failed with status %d", res.StatusCode)
	}
	var info struct {
		Name              string `json:"name"`
		PreferredUsername string `json:"preferred_username"`
		Email             string `json:"email"`
		IsPro             bool   `json:"isPro"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&info); err != nil {
		return errors.New("Hugging Face user info response was invalid")
	}
	switch {
	case strings.TrimSpace(info.Email) != "":
		credential.AccountLabel = strings.TrimSpace(info.Email)
	case strings.TrimSpace(info.PreferredUsername) != "":
		credential.AccountLabel = strings.TrimSpace(info.PreferredUsername)
	default:
		credential.AccountLabel = strings.TrimSpace(info.Name)
	}
	if info.IsPro {
		credential.AccountType = "PRO"
	} else {
		credential.AccountType = "Free"
	}
	credential.Entitlement = "Inference Providers"
	return nil
}

func (a *huggingFaceAccountAdapter) refreshCredential(ctx context.Context, current providerAccountStoredCredential) (providerAccountStoredCredential, error) {
	if strings.TrimSpace(current.RefreshToken) == "" {
		return providerAccountStoredCredential{}, errors.New("Hugging Face account needs reauthentication")
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

func (a *huggingFaceAccountAdapter) RuntimeCredential(ctx context.Context) (string, error) {
	credential, err := a.loadCredential()
	if err != nil {
		return "", err
	}
	if accountCredentialExpired(credential, 2*time.Minute) {
		if credential.RefreshToken == "" {
			return "", errors.New("Hugging Face account needs reauthentication")
		}
		credential, err = a.refreshCredential(ctx, credential)
		if err != nil {
			return "", err
		}
	}
	return strings.TrimSpace(credential.AccessToken), nil
}

func (a *huggingFaceAccountAdapter) Refresh(ctx context.Context, _ string) (providerAccountStatus, error) {
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

func (a *huggingFaceAccountAdapter) DiscoverModels(ctx context.Context, _ string) ([]string, error) {
	credential, err := a.loadCredential()
	if err != nil {
		return nil, err
	}
	if accountCredentialExpired(credential, 2*time.Minute) && credential.RefreshToken != "" {
		credential, err = a.refreshCredential(ctx, credential)
		if err != nil {
			return nil, err
		}
	}
	return a.syncProvider(ctx, credential.AccessToken)
}

func (a *huggingFaceAccountAdapter) syncProvider(ctx context.Context, accessToken string) ([]string, error) {
	discovered, err := discoverOpenAICompatibleModels(ctx, a.inferenceBaseURL, accessToken)
	if err != nil {
		return nil, err
	}
	models := make([]tlProviderModel, 0, len(discovered))
	ids := make([]string, 0, len(discovered))
	for _, item := range discovered {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		toolCall := true
		if item.ToolCall != nil {
			toolCall = *item.ToolCall
		}
		reasoning := false
		if item.Reasoning != nil {
			reasoning = *item.Reasoning
		}
		models = append(models, tlProviderModel{
			ID: id,
			Name: item.Name,
			ToolCall: toolCall,
			Reasoning: reasoning,
			ContextLimit: item.ContextLimit,
			OutputLimit: item.OutputLimit,
		})
		ids = append(ids, id)
	}
	if len(models) == 0 {
		return nil, errors.New("Hugging Face returned no usable models")
	}
	definition := tlProviderDefinition{
		ID: huggingFaceAccountProviderID,
		Name: "Hugging Face",
		Protocol: "openai-compatible",
		BaseURL: a.inferenceBaseURL,
		ManagedBy: "account",
		Models: models,
	}
	if err := a.manager.store.put(definition); err != nil {
		return nil, err
	}
	return ids, nil
}

func (a *huggingFaceAccountAdapter) Disconnect(context.Context, string) error {
	if a == nil || a.manager == nil || a.manager.credentials == nil {
		return nil
	}
	if err := a.manager.credentials.Delete(providerAccountCredentialID(huggingFaceAccountProviderID)); err != nil {
		return err
	}
	if provider, found, err := a.manager.store.get(huggingFaceAccountProviderID); err == nil && found && provider.ManagedBy == "account" {
		_ = a.manager.store.remove(huggingFaceAccountProviderID)
	}
	return nil
}

func (a *huggingFaceAccountAdapter) CancelLogin(ctx context.Context, directory, loginID string) error {
	a.cancelLogin(strings.TrimSpace(loginID))
	return nil
}
