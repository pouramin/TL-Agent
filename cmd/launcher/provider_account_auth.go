package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

type providerAccountStatus struct {
	ID                    string   `json:"id"`
	Name                  string   `json:"name"`
	Description           string   `json:"description,omitempty"`
	Available             bool     `json:"available"`
	Connected             bool     `json:"connected"`
	AuthModes             []string `json:"authModes"`
	RequiresCompatibility bool     `json:"requiresCompatibility,omitempty"`
	AccountType           string   `json:"accountType,omitempty"`
	OrganizationID        string   `json:"organizationId,omitempty"`
	Models                []string `json:"models,omitempty"`
}

type providerAccountAdapter interface {
	ID() string
	Status(context.Context, string) (providerAccountStatus, error)
	Authorize(context.Context, string) (json.RawMessage, error)
	Callback(context.Context, string) (json.RawMessage, error)
	Disconnect(context.Context, string) error
}

type providerAccountService struct {
	adapters map[string]providerAccountAdapter
}

func newProviderAccountService(manager *runtimeProviderManager) *providerAccountService {
	service := &providerAccountService{adapters: map[string]providerAccountAdapter{}}
	service.add(&kiloProviderAccountAdapter{manager: manager})
	return service
}

func (s *providerAccountService) add(adapter providerAccountAdapter) {
	if s == nil || adapter == nil {
		return
	}
	id := strings.TrimSpace(adapter.ID())
	if id == "" {
		return
	}
	s.adapters[id] = adapter
}

func (s *providerAccountService) adapter(id string) (providerAccountAdapter, bool) {
	if s == nil {
		return nil, false
	}
	adapter, ok := s.adapters[strings.TrimSpace(id)]
	return adapter, ok
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
		if status.AuthModes == nil {
			status.AuthModes = []string{}
		}
		if status.Models == nil {
			status.Models = []string{}
		}
		result = append(result, status)
	}
	return result, nil
}

type kiloProviderAccountAdapter struct {
	manager *runtimeProviderManager
}

func (a *kiloProviderAccountAdapter) ID() string { return runtimeHostedProviderID }

func (a *kiloProviderAccountAdapter) Status(ctx context.Context, directory string) (providerAccountStatus, error) {
	status := providerAccountStatus{
		ID:                    runtimeHostedProviderID,
		Name:                  "Kilo",
		Description:           "Use your Kilo account, credits, and hosted model catalog.",
		Available:             a != nil && a.manager != nil && a.manager.compatibilityAvailable(),
		Connected:             false,
		AuthModes:             []string{"account"},
		RequiresCompatibility: true,
		Models:                []string{},
	}
	if !status.Available {
		return status, nil
	}

	raw, err := a.manager.requestRaw(ctx, http.MethodGet, "/kilo/auth-status", a.manager.runtimeQuery(directory), nil)
	if err != nil {
		return providerAccountStatus{}, err
	}
	var upstream struct {
		Authenticated  bool   `json:"authenticated"`
		Type           string `json:"type"`
		OrganizationID string `json:"organizationId"`
	}
	if err := json.Unmarshal(unwrapRuntimePayload(raw), &upstream); err != nil {
		return providerAccountStatus{}, err
	}
	status.Connected = upstream.Authenticated
	status.AccountType = upstream.Type
	status.OrganizationID = upstream.OrganizationID
	status.Models = append([]string{}, runtimeHostedPreferredModels...)
	return status, nil
}

func (a *kiloProviderAccountAdapter) Authorize(ctx context.Context, directory string) (json.RawMessage, error) {
	if a == nil || a.manager == nil || !a.manager.compatibilityAvailable() {
		return nil, errRuntimeUnavailable
	}
	return a.manager.requestRaw(ctx, http.MethodPost, "/provider/kilo/oauth/authorize", a.manager.runtimeQuery(directory), map[string]any{"method": 0})
}

func (a *kiloProviderAccountAdapter) Callback(ctx context.Context, directory string) (json.RawMessage, error) {
	if a == nil || a.manager == nil || !a.manager.compatibilityAvailable() {
		return nil, errRuntimeUnavailable
	}
	return a.manager.requestRaw(ctx, http.MethodPost, "/provider/kilo/oauth/callback", a.manager.runtimeQuery(directory), map[string]any{"method": 0})
}

func (a *kiloProviderAccountAdapter) Disconnect(ctx context.Context, _ string) error {
	if a == nil || a.manager == nil || !a.manager.compatibilityAvailable() {
		return errRuntimeUnavailable
	}
	_, err := a.manager.requestRaw(ctx, http.MethodDelete, "/auth/kilo", url.Values{}, nil)
	return err
}

func writeProviderAccountError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errRuntimeUnavailable):
		writeJSON(w, http.StatusServiceUnavailable, jsonError{Error: err.Error()})
	default:
		writeProviderManagerError(w, err)
	}
}

func registerProviderAccountRoutes(mux *http.ServeMux, service *providerAccountService) {
	mux.HandleFunc("GET /local/provider-accounts", func(w http.ResponseWriter, r *http.Request) {
		accounts, err := service.list(r.Context(), r.URL.Query().Get("directory"))
		if err != nil {
			writeProviderAccountError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, accounts)
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
		writeJSON(w, http.StatusOK, status)
	})

	mux.HandleFunc("POST /local/provider-accounts/{id}/authorize", func(w http.ResponseWriter, r *http.Request) {
		adapter, ok := service.adapter(r.PathValue("id"))
		if !ok {
			writeJSON(w, http.StatusNotFound, jsonError{Error: "provider account adapter not found"})
			return
		}
		raw, err := adapter.Authorize(r.Context(), r.URL.Query().Get("directory"))
		if err != nil {
			writeProviderAccountError(w, err)
			return
		}
		writeUnwrappedJSON(w, raw)
	})

	mux.HandleFunc("POST /local/provider-accounts/{id}/callback", func(w http.ResponseWriter, r *http.Request) {
		adapter, ok := service.adapter(r.PathValue("id"))
		if !ok {
			writeJSON(w, http.StatusNotFound, jsonError{Error: "provider account adapter not found"})
			return
		}
		raw, err := adapter.Callback(r.Context(), r.URL.Query().Get("directory"))
		if err != nil {
			writeProviderAccountError(w, err)
			return
		}
		writeUnwrappedJSON(w, raw)
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
		writeJSON(w, http.StatusOK, map[string]any{"disconnected": true, "id": adapter.ID()})
	})
}
