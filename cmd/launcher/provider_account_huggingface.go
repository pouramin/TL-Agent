package main

import (
	"context"
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
	huggingFaceDefaultClientID    = "https://pouramin.dev/.well-known/oauth-cimd"
)

type huggingFaceLoginTransaction struct {
	LoginID     string
	State       string
	Verifier    string
	RedirectURI string
	ExpiresAt   time.Time
	Completed   bool
	Err         string
}

type huggingFaceAccountAdapter struct {
	state            *appState
	manager          *providerManager
	clientID         string
	client           *http.Client
	authorizeURL     string
	tokenURL         string
	userInfoURL      string
	inferenceBaseURL string

	mu      sync.Mutex
	logins  map[string]*huggingFaceLoginTransaction
	byState map[string]string
}

func newHuggingFaceAccountAdapter(state *appState, manager *providerManager) *huggingFaceAccountAdapter {
	clientID := strings.TrimSpace(os.Getenv("TL_STUDIO_HUGGINGFACE_CLIENT_ID"))
	if clientID == "" {
		clientID = huggingFaceDefaultClientID
	}
	return &huggingFaceAccountAdapter{
		state:            state,
		manager:          manager,
		clientID:         clientID,
		client:           &http.Client{Timeout: 20 * time.Second},
		authorizeURL:     "https://huggingface.co/oauth/authorize",
		tokenURL:         "https://huggingface.co/oauth/token",
		userInfoURL:      "https://huggingface.co/oauth/userinfo",
		inferenceBaseURL: huggingFaceInferenceBaseURL,
		logins:           map[string]*huggingFaceLoginTransaction{},
		byState:          map[string]string{},
	}
}

func (a *huggingFaceAccountAdapter) ID() string { return huggingFaceAccountProviderID }

func (a *huggingFaceAccountAdapter) available() bool {
	return a != nil && a.state != nil && a.manager != nil && a.manager.credentials != nil && strings.TrimSpace(a.clientID) != ""
}

func (a *huggingFaceAccountAdapter) baseStatus() providerAccountStatus {
	return providerAccountStatus{
		ID:           huggingFaceAccountProviderID,
		Name:         "Hugging Face",
		Description:  "Connect a Hugging Face account for Inference Providers.",
		Available:    a.available(),
		State:        providerAccountDisconnected,
		AuthModes:    []string{"authorization_code_pkce"},
		Capabilities: []string{"models", "inference", "refresh"},
		BillingNote:  "Inference usage is billed to the connected Hugging Face account and its available credits.",
	}
}

func (a *huggingFaceAccountAdapter) loadCredential() (providerOAuthCredential, error) {
	if a == nil || a.manager == nil || a.manager.credentials == nil {
		return providerOAuthCredential{}, errCredentialNotFound
	}
	raw, err := getProviderCredentialSlot(a.manager.credentials, huggingFaceAccountProviderID, providerCredentialSlotAccount)
	if err != nil {
		return providerOAuthCredential{}, err
	}
	credential, structured, err := decodeProviderOAuthCredential(raw)
	if err != nil {
		return providerOAuthCredential{}, err
	}
	if !structured {
		return providerOAuthCredential{}, errors.New("invalid Hugging Face account credential")
	}
	return credential, nil
}

func (a *huggingFaceAccountAdapter) saveCredential(credential providerOAuthCredential) error {
	if a == nil || a.manager == nil || a.manager.credentials == nil {
		return errors.New("TL Studio credential vault is unavailable")
	}
	encoded, err := encodeProviderOAuthCredential(credential)
	if err != nil {
		return err
	}
	return putProviderCredentialSlot(a.manager.credentials, huggingFaceAccountProviderID, providerCredentialSlotAccount, encoded)
}

func (a *huggingFaceAccountAdapter) statusFromCredential(credential providerOAuthCredential) providerAccountStatus {
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
		if provider, ok, err := a.manager.store.get(huggingFaceAccountProviderID); err == nil && ok {
			status.Models = make([]string, 0, len(provider.Models))
			for _, model := range provider.Models {
				status.Models = append(status.Models, model.ID)
			}
		}
	}
	status.Connected = status.State == providerAccountConnected
	return status
}

func (a *huggingFaceAccountAdapter) Status(ctx context.Context, _ string) (providerAccountStatus, error) {
	status := a.baseStatus()
	if !status.Available {
		status.Error = "TL Studio credential vault is unavailable"
		return status, nil
	}
	credential, err := a.loadCredential()
	if errors.Is(err, errCredentialNotFound) {
		return status, nil
	}
	if err != nil {
		return providerAccountStatus{}, err
	}
	if credential.needsRefresh(time.Now()) && strings.TrimSpace(credential.RefreshToken) != "" {
		if refreshed, refreshErr := a.refreshCredential(ctx, credential); refreshErr == nil {
			credential = refreshed
		} else {
			status = a.statusFromCredential(credential)
			status.State = providerAccountNeedsReauthentication
			status.Connected = false
			status.Error = "Hugging Face account needs reauthentication"
			return status, nil
		}
	}
	return a.statusFromCredential(credential), nil
}

func (a *huggingFaceAccountAdapter) callbackURL() (string, error) {
	if a == nil || a.state == nil {
		return "", errors.New("TL Studio local server is unavailable")
	}
	a.state.mu.RLock()
	base := strings.TrimRight(a.state.frontendURL, "/")
	a.state.mu.RUnlock()
	if base == "" {
		return "", errors.New("TL Studio local callback URL is unavailable")
	}
	callback := base + "/local/provider-accounts/" + huggingFaceAccountProviderID + "/oauth/callback"
	parsed, err := url.Parse(callback)
	if err != nil || parsed.Scheme != "http" || !isLoopbackHost(parsed.Hostname()) {
		return "", errors.New("Hugging Face account login requires the TL Studio loopback server")
	}
	return callback, nil
}

func oauthPKCEChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func (a *huggingFaceAccountAdapter) BeginLogin(context.Context, string) (providerAccountLogin, error) {
	if !a.available() {
		return providerAccountLogin{}, errors.New("Hugging Face account login is unavailable")
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
	transaction := &huggingFaceLoginTransaction{
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
	query := authorize.Query()
	query.Set("client_id", a.clientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("response_type", "code")
	query.Set("scope", "openid profile email inference-api")
	query.Set("state", state)
	query.Set("code_challenge", oauthPKCEChallenge(verifier))
	query.Set("code_challenge_method", "S256")
	authorize.RawQuery = query.Encode()

	return providerAccountLogin{
		LoginID: loginID,
		Flow: "authorization_code_pkce",
		AuthorizationURL: authorize.String(),
		Instructions: "Open Hugging Face, sign in, and approve TL Studio.",
		ExpiresAt: expiresAt.Format(time.RFC3339),
		PollIntervalSeconds: 2,
	}, nil
}

func (a *huggingFaceAccountAdapter) CompleteLogin(ctx context.Context, _ string, callback providerAccountCallback) error {
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
	if err != nil {
		a.setLoginError(loginID, "Hugging Face token exchange failed")
		return err
	}
	if err := a.saveCredential(credential); err != nil {
		a.setLoginError(loginID, "Hugging Face credential storage failed")
		return err
	}
	if _, err := a.syncProvider(ctx, credential.AccessToken); err != nil {
		_ = deleteProviderCredentialSlot(a.manager.credentials, huggingFaceAccountProviderID, providerCredentialSlotAccount)
		a.setLoginError(loginID, "Hugging Face model discovery failed")
		return err
	}

	a.mu.Lock()
	transaction.Completed = true
	transaction.Verifier = ""
	delete(a.byState, transaction.State)
	a.mu.Unlock()
	return nil
}

func (a *huggingFaceAccountAdapter) setLoginError(loginID, message string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if transaction := a.logins[loginID]; transaction != nil {
		transaction.Err = message
		transaction.Verifier = ""
		delete(a.byState, transaction.State)
	}
}

func (a *huggingFaceAccountAdapter) PollLogin(ctx context.Context, directory, loginID string) (providerAccountStatus, error) {
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

func (a *huggingFaceAccountAdapter) CancelLogin(_ context.Context, _ string, loginID string) error {
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

func (a *huggingFaceAccountAdapter) exchangeAuthorizationCode(ctx context.Context, transaction *huggingFaceLoginTransaction, code string) (providerOAuthCredential, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", a.clientID)
	form.Set("code", strings.TrimSpace(code))
	form.Set("redirect_uri", transaction.RedirectURI)
	form.Set("code_verifier", transaction.Verifier)
	credential, err := a.exchangeToken(ctx, form)
	if err != nil {
		return providerOAuthCredential{}, err
	}
	if err := a.populateUserInfo(ctx, &credential); err != nil {
		return providerOAuthCredential{}, err
	}
	return credential, nil
}

func (a *huggingFaceAccountAdapter) exchangeToken(ctx context.Context, form url.Values) (providerOAuthCredential, error) {
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
		return providerOAuthCredential{}, fmt.Errorf("Hugging Face token exchange failed with status %d", response.StatusCode)
	}
	var payload struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		Scope        string `json:"scope"`
		ExpiresIn    any    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return providerOAuthCredential{}, errors.New("Hugging Face token response was invalid")
	}
	if strings.TrimSpace(payload.AccessToken) == "" {
		return providerOAuthCredential{}, errors.New("Hugging Face token response did not include an access token")
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
	scopes := strings.Fields(strings.TrimSpace(payload.Scope))
	return providerOAuthCredential{
		AccessToken: strings.TrimSpace(payload.AccessToken),
		RefreshToken: strings.TrimSpace(payload.RefreshToken),
		TokenType: strings.TrimSpace(payload.TokenType),
		ExpiresAt: expiresAt,
		Scopes: scopes,
	}, nil
}

func (a *huggingFaceAccountAdapter) populateUserInfo(ctx context.Context, credential *providerOAuthCredential) error {
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
		return fmt.Errorf("Hugging Face user info failed with status %d", response.StatusCode)
	}
	var info struct {
		Name              string `json:"name"`
		PreferredUsername string `json:"preferred_username"`
		Email             string `json:"email"`
		IsPro             bool   `json:"isPro"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&info); err != nil {
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
	return nil
}

func (a *huggingFaceAccountAdapter) refreshCredential(ctx context.Context, current providerOAuthCredential) (providerOAuthCredential, error) {
	if strings.TrimSpace(current.RefreshToken) == "" {
		return providerOAuthCredential{}, errors.New("Hugging Face account needs reauthentication")
	}
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", a.clientID)
	form.Set("refresh_token", current.RefreshToken)
	next, err := a.exchangeToken(ctx, form)
	if err != nil {
		return providerOAuthCredential{}, err
	}
	if strings.TrimSpace(next.RefreshToken) == "" {
		next.RefreshToken = current.RefreshToken
	}
	next.AccountLabel = current.AccountLabel
	next.AccountType = current.AccountType
	if err := a.saveCredential(next); err != nil {
		return providerOAuthCredential{}, err
	}
	return next, nil
}

func (a *huggingFaceAccountAdapter) Refresh(ctx context.Context, _ string) (providerAccountStatus, error) {
	credential, err := a.loadCredential()
	if err != nil {
		return providerAccountStatus{}, err
	}
	if strings.TrimSpace(credential.RefreshToken) == "" {
		return a.statusFromCredential(credential), nil
	}
	credential, err = a.refreshCredential(ctx, credential)
	if err != nil {
		return providerAccountStatus{}, err
	}
	if _, err := a.syncProvider(ctx, credential.AccessToken); err != nil {
		return providerAccountStatus{}, err
	}
	return a.statusFromCredential(credential), nil
}

func (a *huggingFaceAccountAdapter) ResolveCredential(ctx context.Context, _ string) (string, error) {
	credential, err := a.loadCredential()
	if err != nil {
		return "", err
	}
	if credential.needsRefresh(time.Now()) {
		if strings.TrimSpace(credential.RefreshToken) == "" {
			return "", errors.New("Hugging Face account needs reauthentication")
		}
		credential, err = a.refreshCredential(ctx, credential)
		if err != nil {
			return "", err
		}
	}
	return strings.TrimSpace(credential.AccessToken), nil
}

func (a *huggingFaceAccountAdapter) DiscoverModels(ctx context.Context, directory string) ([]string, error) {
	credential, err := a.ResolveCredential(ctx, directory)
	if err != nil {
		return nil, err
	}
	return a.syncProvider(ctx, credential)
}

func (a *huggingFaceAccountAdapter) syncProvider(ctx context.Context, accessToken string) ([]string, error) {
	discovered, err := discoverOpenAICompatibleModelsWithClient(ctx, a.inferenceBaseURL, accessToken, a.client)
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
		toolCall := item.ToolCall != nil && *item.ToolCall
		reasoning := item.Reasoning != nil && *item.Reasoning
		models = append(models, tlProviderModel{
			ID: id,
			Name: item.Name,
			Kind: item.Kind,
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
	if err := deleteProviderCredentialSlot(a.manager.credentials, huggingFaceAccountProviderID, providerCredentialSlotAccount); err != nil {
		return err
	}
	if provider, found, err := a.manager.store.get(huggingFaceAccountProviderID); err == nil && found && provider.ManagedBy == "account" {
		_ = a.manager.store.remove(huggingFaceAccountProviderID)
	}
	return nil
}
