package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

type providerAccountState string

const (
	providerAccountDisconnected          providerAccountState = "disconnected"
	providerAccountConnecting            providerAccountState = "connecting"
	providerAccountConnected             providerAccountState = "connected"
	providerAccountExpired               providerAccountState = "expired"
	providerAccountNeedsReauthentication providerAccountState = "needs_reauthentication"
	providerAccountError                 providerAccountState = "error"
)

type providerAccountStatus struct {
	ID                string               `json:"id"`
	Name              string               `json:"name"`
	Description       string               `json:"description,omitempty"`
	Available         bool                 `json:"available"`
	Connected         bool                 `json:"connected"`
	State             providerAccountState `json:"state"`
	AuthModes         []string             `json:"authModes"`
	AccountType       string               `json:"accountType,omitempty"`
	AccountLabel      string               `json:"accountLabel,omitempty"`
	OrganizationID    string               `json:"organizationId,omitempty"`
	Models            []string             `json:"models,omitempty"`
	Entitlement       string               `json:"entitlement,omitempty"`
	UnsupportedReason string               `json:"unsupportedReason,omitempty"`
}

type providerAccountLoginChallenge struct {
	LoginID             string `json:"loginId"`
	Flow                string `json:"flow"`
	AuthorizationURL    string `json:"authorizationUrl,omitempty"`
	VerificationURL     string `json:"verificationUrl,omitempty"`
	UserCode            string `json:"userCode,omitempty"`
	Instructions        string `json:"instructions,omitempty"`
	ExpiresAt           string `json:"expiresAt,omitempty"`
	PollIntervalSeconds int    `json:"pollIntervalSeconds,omitempty"`
}

type providerAccountAdapter interface {
	ID() string
	Status(context.Context, string) (providerAccountStatus, error)
	BeginLogin(context.Context, string) (providerAccountLoginChallenge, error)
	CompleteLogin(context.Context, string, string) (providerAccountStatus, error)
	HandleCallback(context.Context, string, string, url.Values) error
	CancelLogin(context.Context, string, string) error
	Refresh(context.Context, string) (providerAccountStatus, error)
	Disconnect(context.Context, string) error
	DiscoverModels(context.Context, string) ([]string, error)
}

type providerAccountService struct {
	adapters map[string]providerAccountAdapter
}

func newProviderAccountService(adapters ...providerAccountAdapter) *providerAccountService {
	service := &providerAccountService{adapters: map[string]providerAccountAdapter{}}
	for _, adapter := range adapters {
		if adapter == nil {
			continue
		}
		id := strings.TrimSpace(adapter.ID())
		if id != "" {
			service.adapters[id] = adapter
		}
	}
	return service
}

func (s *providerAccountService) adapter(id string) (providerAccountAdapter, bool) {
	if s == nil {
		return nil, false
	}
	adapter, ok := s.adapters[strings.TrimSpace(id)]
	return adapter, ok
}

func normalizeProviderAccountStatus(id string, status providerAccountStatus) providerAccountStatus {
	status.ID = strings.TrimSpace(id)
	if status.AuthModes == nil {
		status.AuthModes = []string{"account"}
	}
	if status.State == "" {
		if status.Connected {
			status.State = providerAccountConnected
		} else {
			status.State = providerAccountDisconnected
		}
	}
	status.Connected = status.State == providerAccountConnected
	return status
}

func (s *providerAccountService) list(ctx context.Context, directory string) ([]providerAccountStatus, error) {
	if s == nil {
		return []providerAccountStatus{}, nil
	}
	ids := make([]string, 0, len(s.adapters))
	for id := range s.adapters {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result := make([]providerAccountStatus, 0, len(ids))
	for _, id := range ids {
		status, err := s.adapters[id].Status(ctx, directory)
		if err != nil {
			return nil, err
		}
		result = append(result, normalizeProviderAccountStatus(id, status))
	}
	return result, nil
}

func writeProviderAccountError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, context.Canceled):
		writeJSON(w, http.StatusRequestTimeout, jsonError{Error: "provider account operation cancelled"})
	case errors.Is(err, context.DeadlineExceeded):
		writeJSON(w, http.StatusGatewayTimeout, jsonError{Error: "provider account operation timed out"})
	default:
		writeJSON(w, http.StatusBadGateway, jsonError{Error: err.Error()})
	}
}

func registerProviderAccountRoutes(mux *http.ServeMux, service *providerAccountService) {
	mux.HandleFunc("GET /local/provider-accounts", func(w http.ResponseWriter, r *http.Request) {
		items, err := service.list(r.Context(), r.URL.Query().Get("directory"))
		if err != nil {
			writeProviderAccountError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, items)
	})

	mux.HandleFunc("GET /local/provider-accounts/{id}", func(w http.ResponseWriter, r *http.Request) {
		adapter, ok := service.adapter(r.PathValue("id"))
		if !ok {
			writeJSON(w, http.StatusNotFound, jsonError{Error: "provider account adapter not found"})
			return
		}
		status, err := adapter.Status(r.Context(), r.URL.Query().Get("directory"))
		if err != nil {
			writeProviderAccountError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, normalizeProviderAccountStatus(adapter.ID(), status))
	})

	mux.HandleFunc("POST /local/provider-accounts/{id}/authorize", func(w http.ResponseWriter, r *http.Request) {
		adapter, ok := service.adapter(r.PathValue("id"))
		if !ok {
			writeJSON(w, http.StatusNotFound, jsonError{Error: "provider account adapter not found"})
			return
		}
		challenge, err := adapter.BeginLogin(r.Context(), r.URL.Query().Get("directory"))
		if err != nil {
			writeProviderAccountError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, challenge)
	})

	complete := func(w http.ResponseWriter, r *http.Request) {
		adapter, ok := service.adapter(r.PathValue("id"))
		if !ok {
			writeJSON(w, http.StatusNotFound, jsonError{Error: "provider account adapter not found"})
			return
		}
		loginID := strings.TrimSpace(r.URL.Query().Get("login"))
		if loginID == "" {
			writeJSON(w, http.StatusBadRequest, jsonError{Error: "login id is required"})
			return
		}
		status, err := adapter.CompleteLogin(r.Context(), r.URL.Query().Get("directory"), loginID)
		if err != nil {
			writeProviderAccountError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, normalizeProviderAccountStatus(adapter.ID(), status))
	}
	mux.HandleFunc("POST /local/provider-accounts/{id}/complete", complete)
	// Compatibility alias for the pre-0.6 Browser contract.
	mux.HandleFunc("POST /local/provider-accounts/{id}/callback", complete)

	mux.HandleFunc("GET /local/provider-accounts/{id}/oauth/callback", func(w http.ResponseWriter, r *http.Request) {
		adapter, ok := service.adapter(r.PathValue("id"))
		if !ok {
			http.Error(w, "Provider account adapter not found.", http.StatusNotFound)
			return
		}
		loginID := strings.TrimSpace(r.URL.Query().Get("state"))
		if err := adapter.HandleCallback(r.Context(), r.URL.Query().Get("directory"), loginID, r.URL.Query()); err != nil {
			http.Error(w, "TL Studio could not complete provider authorization. You can close this tab and return to TL Studio.", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(`<!doctype html><meta charset="utf-8"><title>TL Studio</title><style>body{font-family:system-ui;margin:40px;max-width:680px;line-height:1.5}h1{font-size:20px}</style><h1>Provider connected</h1><p>You can close this tab and return to TL Studio.</p>`))
	})

	mux.HandleFunc("POST /local/provider-accounts/{id}/cancel", func(w http.ResponseWriter, r *http.Request) {
		adapter, ok := service.adapter(r.PathValue("id"))
		if !ok {
			writeJSON(w, http.StatusNotFound, jsonError{Error: "provider account adapter not found"})
			return
		}
		loginID := strings.TrimSpace(r.URL.Query().Get("login"))
		if err := adapter.CancelLogin(r.Context(), r.URL.Query().Get("directory"), loginID); err != nil {
			writeProviderAccountError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"cancelled": true})
	})

	mux.HandleFunc("POST /local/provider-accounts/{id}/refresh", func(w http.ResponseWriter, r *http.Request) {
		adapter, ok := service.adapter(r.PathValue("id"))
		if !ok {
			writeJSON(w, http.StatusNotFound, jsonError{Error: "provider account adapter not found"})
			return
		}
		status, err := adapter.Refresh(r.Context(), r.URL.Query().Get("directory"))
		if err != nil {
			writeProviderAccountError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, normalizeProviderAccountStatus(adapter.ID(), status))
	})

	mux.HandleFunc("GET /local/provider-accounts/{id}/models", func(w http.ResponseWriter, r *http.Request) {
		adapter, ok := service.adapter(r.PathValue("id"))
		if !ok {
			writeJSON(w, http.StatusNotFound, jsonError{Error: "provider account adapter not found"})
			return
		}
		models, err := adapter.DiscoverModels(r.Context(), r.URL.Query().Get("directory"))
		if err != nil {
			writeProviderAccountError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"models": models})
	})

	mux.HandleFunc("DELETE /local/provider-accounts/{id}", func(w http.ResponseWriter, r *http.Request) {
		adapter, ok := service.adapter(r.PathValue("id"))
		if !ok {
			writeJSON(w, http.StatusNotFound, jsonError{Error: "provider account adapter not found"})
			return
		}
		if err := adapter.Disconnect(r.Context(), r.URL.Query().Get("directory")); err != nil {
			writeProviderAccountError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"disconnected": true, "at": time.Now().UTC().Format(time.RFC3339)})
	})
}

// providerAccountCredentialID namespaces account credentials away from API-key
// credentials without changing the existing provider credential store contract.
func providerAccountCredentialID(providerID string) string {
	return strings.TrimSpace(providerID) + "--account"
}

func encodeProviderAccountCredential(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
