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
	Setup          *providerAccountSetupSummary   `json:"setup,omitempty"`
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

type providerAccountCallback struct {
	Code  string
	State string
	Error string
}

type providerAccountSetupSummary struct {
	Configurable bool   `json:"configurable"`
	Configured   bool   `json:"configured"`
	Label        string `json:"label,omitempty"`
}

type providerAccountSetupField struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
	Value       string `json:"value,omitempty"`
	Required    bool   `json:"required,omitempty"`
	ReadOnly    bool   `json:"readOnly,omitempty"`
}

type providerAccountSetup struct {
	Title       string                      `json:"title"`
	Description string                      `json:"description,omitempty"`
	Fields      []providerAccountSetupField `json:"fields"`
}

type providerAccountConfigurable interface {
	Setup(context.Context, string) (providerAccountSetup, error)
	Configure(context.Context, string, map[string]string) (providerAccountStatus, error)
}

type providerAccountAdapter interface {
	ID() string
	Status(context.Context, string) (providerAccountStatus, error)
	BeginLogin(context.Context, string) (providerAccountLogin, error)
	CompleteLogin(context.Context, string, providerAccountCallback) error
	PollLogin(context.Context, string, string) (providerAccountStatus, error)
	CancelLogin(context.Context, string, string) error
	Refresh(context.Context, string) (providerAccountStatus, error)
	ResolveCredential(context.Context, string) (string, error)
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

	mux.HandleFunc("GET /local/provider-accounts/{id}/setup", func(w http.ResponseWriter, r *http.Request) {
		adapter, ok := service.adapter(r.PathValue("id"))
		if !ok {
			writeJSON(w, http.StatusNotFound, jsonError{Error: "provider account adapter not found"})
			return
		}
		configurable, ok := adapter.(providerAccountConfigurable)
		if !ok {
			writeJSON(w, http.StatusNotFound, jsonError{Error: "provider account setup is not configurable"})
			return
		}
		setup, err := configurable.Setup(r.Context(), r.URL.Query().Get("directory"))
		if err != nil {
			writeProviderAccountError(w, err)
			return
		}
		if setup.Fields == nil {
			setup.Fields = []providerAccountSetupField{}
		}
		writeJSON(w, http.StatusOK, setup)
	})

	mux.HandleFunc("PUT /local/provider-accounts/{id}/setup", func(w http.ResponseWriter, r *http.Request) {
		adapter, ok := service.adapter(r.PathValue("id"))
		if !ok {
			writeJSON(w, http.StatusNotFound, jsonError{Error: "provider account adapter not found"})
			return
		}
		configurable, ok := adapter.(providerAccountConfigurable)
		if !ok {
			writeJSON(w, http.StatusNotFound, jsonError{Error: "provider account setup is not configurable"})
			return
		}
		var input struct {
			Values map[string]string `json:"values"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
		if err := decoder.Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, jsonError{Error: "invalid provider account setup JSON body"})
			return
		}
		if input.Values == nil {
			input.Values = map[string]string{}
		}
		status, err := configurable.Configure(r.Context(), r.URL.Query().Get("directory"), input.Values)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, jsonError{Error: err.Error()})
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

	mux.HandleFunc("GET /local/provider-accounts/{id}/oauth/callback", func(w http.ResponseWriter, r *http.Request) {
		adapter, ok := service.adapter(r.PathValue("id"))
		if !ok {
			http.Error(w, "Provider account adapter not found.", http.StatusNotFound)
			return
		}
		callback := providerAccountCallback{
			Code: strings.TrimSpace(r.URL.Query().Get("code")),
			State: strings.TrimSpace(r.URL.Query().Get("state")),
			Error: strings.TrimSpace(r.URL.Query().Get("error")),
		}
		if callback.State == "" {
			http.Error(w, "Missing OAuth state.", http.StatusBadRequest)
			return
		}
		if err := adapter.CompleteLogin(r.Context(), r.URL.Query().Get("directory"), callback); err != nil {
			http.Error(w, "Provider sign-in could not be completed. Return to TL Studio and try again.", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<!doctype html><meta charset=\"utf-8\"><title>TL Studio</title><p>Sign-in complete. You can close this window and return to TL Studio.</p>"))
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
