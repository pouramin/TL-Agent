package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"
)

type providerAccountConnectionState string

const (
	providerAccountDisconnected          providerAccountConnectionState = "disconnected"
	providerAccountConnecting            providerAccountConnectionState = "connecting"
	providerAccountConnected             providerAccountConnectionState = "connected"
	providerAccountExpired               providerAccountConnectionState = "expired"
	providerAccountNeedsReauthentication providerAccountConnectionState = "needs_reauthentication"
	providerAccountErrorState            providerAccountConnectionState = "error"
)

type providerAccountStatus struct {
	ID             string                         `json:"id"`
	Name           string                         `json:"name"`
	Description    string                         `json:"description,omitempty"`
	Available      bool                           `json:"available"`
	Connected      bool                           `json:"connected"`
	State          providerAccountConnectionState `json:"state"`
	AuthModes      []string                       `json:"authModes"`
	AccountType    string                         `json:"accountType,omitempty"`
	AccountLabel   string                         `json:"accountLabel,omitempty"`
	OrganizationID string                         `json:"organizationId,omitempty"`
	Models         []string                       `json:"models,omitempty"`
	Capabilities   []string                       `json:"capabilities,omitempty"`
	BillingNote    string                         `json:"billingNote,omitempty"`
	Error          string                         `json:"error,omitempty"`
}

type providerAccountLogin struct {
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
	BeginLogin(context.Context, string) (providerAccountLogin, error)
	PollLogin(context.Context, string, string) (providerAccountStatus, error)
	CancelLogin(context.Context, string, string) error
	Refresh(context.Context, string) (providerAccountStatus, error)
	DiscoverModels(context.Context, string) ([]string, error)
	Disconnect(context.Context, string) error
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
	if errors.Is(err, context.Canceled) {
		writeJSON(w, http.StatusRequestTimeout, jsonError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusBadGateway, jsonError{Error: err.Error()})
}

func providerAccountLoginExpired(login providerAccountLogin) bool {
	if strings.TrimSpace(login.ExpiresAt) == "" {
		return false
	}
	expiresAt, err := time.Parse(time.RFC3339, login.ExpiresAt)
	return err == nil && time.Now().After(expiresAt)
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

	mux.HandleFunc("POST /local/provider-accounts/{id}/login", func(w http.ResponseWriter, r *http.Request) {
		adapter, ok := service.adapter(r.PathValue("id"))
		if !ok {
			writeJSON(w, http.StatusNotFound, jsonError{Error: "provider account adapter not found"})
			return
		}
		login, err := adapter.BeginLogin(r.Context(), r.URL.Query().Get("directory"))
		if err != nil {
			writeProviderAccountError(w, err)
			return
		}
		if strings.TrimSpace(login.LoginID) == "" || strings.TrimSpace(login.Flow) == "" {
			writeJSON(w, http.StatusBadGateway, jsonError{Error: "provider account adapter returned an invalid login challenge"})
			return
		}
		if providerAccountLoginExpired(login) {
			writeJSON(w, http.StatusBadGateway, jsonError{Error: "provider account adapter returned an expired login challenge"})
			return
		}
		writeJSON(w, http.StatusOK, login)
	})

	mux.HandleFunc("GET /local/provider-accounts/{id}/login/{loginID}", func(w http.ResponseWriter, r *http.Request) {
		adapter, ok := service.adapter(r.PathValue("id"))
		if !ok {
			writeJSON(w, http.StatusNotFound, jsonError{Error: "provider account adapter not found"})
			return
		}
		status, err := adapter.PollLogin(r.Context(), r.URL.Query().Get("directory"), r.PathValue("loginID"))
		if err != nil {
			writeProviderAccountError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, normalizeProviderAccountStatus(adapter.ID(), status))
	})

	mux.HandleFunc("DELETE /local/provider-accounts/{id}/login/{loginID}", func(w http.ResponseWriter, r *http.Request) {
		adapter, ok := service.adapter(r.PathValue("id"))
		if !ok {
			writeJSON(w, http.StatusNotFound, jsonError{Error: "provider account adapter not found"})
			return
		}
		if err := adapter.CancelLogin(r.Context(), r.URL.Query().Get("directory"), r.PathValue("loginID")); err != nil {
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
		if models == nil {
			models = []string{}
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
		writeJSON(w, http.StatusOK, map[string]any{"disconnected": true})
	})
}

// Keep encoding/json referenced in this file so accidental raw-provider payloads
// cannot creep back into the HTTP contract without an explicit review.
var _ = json.Valid
