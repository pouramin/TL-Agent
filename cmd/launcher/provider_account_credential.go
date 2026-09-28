package main

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const providerOAuthCredentialPrefix = "TLSOAUTH1:"

type providerOAuthCredential struct {
	Version      int      `json:"version"`
	AccessToken  string   `json:"accessToken"`
	RefreshToken string   `json:"refreshToken,omitempty"`
	TokenType    string   `json:"tokenType,omitempty"`
	ExpiresAt    string   `json:"expiresAt,omitempty"`
	Scopes       []string `json:"scopes,omitempty"`
	AccountLabel string   `json:"accountLabel,omitempty"`
}

func encodeProviderOAuthCredential(credential providerOAuthCredential) (string, error) {
	credential.Version = 1
	credential.AccessToken = strings.TrimSpace(credential.AccessToken)
	credential.RefreshToken = strings.TrimSpace(credential.RefreshToken)
	credential.TokenType = strings.TrimSpace(credential.TokenType)
	credential.ExpiresAt = strings.TrimSpace(credential.ExpiresAt)
	credential.AccountLabel = strings.TrimSpace(credential.AccountLabel)
	if credential.AccessToken == "" {
		return "", errors.New("OAuth access token is required")
	}
	encoded, err := json.Marshal(credential)
	if err != nil {
		return "", err
	}
	return providerOAuthCredentialPrefix + string(encoded), nil
}

func decodeProviderOAuthCredential(value string) (providerOAuthCredential, bool, error) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, providerOAuthCredentialPrefix) {
		return providerOAuthCredential{}, false, nil
	}
	var credential providerOAuthCredential
	if err := json.Unmarshal([]byte(strings.TrimPrefix(value, providerOAuthCredentialPrefix)), &credential); err != nil {
		return providerOAuthCredential{}, true, errors.New("invalid TL Studio OAuth credential")
	}
	if credential.Version != 1 || strings.TrimSpace(credential.AccessToken) == "" {
		return providerOAuthCredential{}, true, errors.New("invalid TL Studio OAuth credential")
	}
	return credential, true, nil
}

func (credential providerOAuthCredential) expiry() (time.Time, bool) {
	if strings.TrimSpace(credential.ExpiresAt) == "" {
		return time.Time{}, false
	}
	value, err := time.Parse(time.RFC3339, credential.ExpiresAt)
	return value, err == nil
}

func (credential providerOAuthCredential) needsRefresh(now time.Time) bool {
	expiresAt, ok := credential.expiry()
	return ok && !expiresAt.After(now.Add(60*time.Second))
}
