package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
)

type providerAccountStatus struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Description    string   `json:"description,omitempty"`
	Available      bool     `json:"available"`
	Connected      bool     `json:"connected"`
	AuthModes      []string `json:"authModes"`
	AccountType    string   `json:"accountType,omitempty"`
	OrganizationID string   `json:"organizationId,omitempty"`
	Models         []string `json:"models,omitempty"`
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

func newProviderAccountService(adapters ...providerAccountAdapter) *providerAccountService {
	service := &providerAccountService{adapters: map[string]providerAccountAdapter{}}
	for _, adapter := range adapters {
		if adapter == nil { continue }
		id := strings.TrimSpace(adapter.ID())
		if id != "" { service.adapters[id] = adapter }
	}
	return service
}

func (s *providerAccountService) adapter(id string) (providerAccountAdapter, bool) {
	if s == nil { return nil, false }
	adapter, ok := s.adapters[strings.TrimSpace(id)]
	return adapter, ok
}

func (s *providerAccountService) list(ctx context.Context, directory string) ([]providerAccountStatus, error) {
	if s == nil { return []providerAccountStatus{}, nil }
	ids := make([]string, 0, len(s.adapters))
	for id := range s.adapters { ids = append(ids, id) }
	sort.Strings(ids)
	result := make([]providerAccountStatus, 0, len(ids))
	for _, id := range ids {
		status, err := s.adapters[id].Status(ctx, directory)
		if err != nil { return nil, err }
		status.ID = id
		if status.AuthModes == nil { status.AuthModes = []string{"account"} }
		result = append(result, status)
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

func registerProviderAccountRoutes(mux *http.ServeMux, service *providerAccountService) {
	mux.HandleFunc("GET /local/provider-accounts", func(w http.ResponseWriter, r *http.Request) {
		items, err := service.list(r.Context(), r.URL.Query().Get("directory"))
		if err != nil { writeProviderAccountError(w, err); return }
		writeJSON(w, http.StatusOK, items)
	})

	mux.HandleFunc("GET /local/provider-accounts/{id}", func(w http.ResponseWriter, r *http.Request) {
		adapter, ok := service.adapter(r.PathValue("id"))
		if !ok { writeJSON(w, http.StatusNotFound, jsonError{Error: "provider account adapter not found"}); return }
		status, err := adapter.Status(r.Context(), r.URL.Query().Get("directory"))
		if err != nil { writeProviderAccountError(w, err); return }
		status.ID = strings.TrimSpace(adapter.ID())
		if status.AuthModes == nil { status.AuthModes = []string{"account"} }
		writeJSON(w, http.StatusOK, status)
	})

	mux.HandleFunc("POST /local/provider-accounts/{id}/authorize", func(w http.ResponseWriter, r *http.Request) {
		adapter, ok := service.adapter(r.PathValue("id"))
		if !ok { writeJSON(w, http.StatusNotFound, jsonError{Error: "provider account adapter not found"}); return }
		raw, err := adapter.Authorize(r.Context(), r.URL.Query().Get("directory"))
		if err != nil { writeProviderAccountError(w, err); return }
		writeRawProviderAccountJSON(w, raw)
	})

	mux.HandleFunc("POST /local/provider-accounts/{id}/callback", func(w http.ResponseWriter, r *http.Request) {
		adapter, ok := service.adapter(r.PathValue("id"))
		if !ok { writeJSON(w, http.StatusNotFound, jsonError{Error: "provider account adapter not found"}); return }
		raw, err := adapter.Callback(r.Context(), r.URL.Query().Get("directory"))
		if err != nil { writeProviderAccountError(w, err); return }
		writeRawProviderAccountJSON(w, raw)
	})

	mux.HandleFunc("DELETE /local/provider-accounts/{id}", func(w http.ResponseWriter, r *http.Request) {
		adapter, ok := service.adapter(r.PathValue("id"))
		if !ok { writeJSON(w, http.StatusNotFound, jsonError{Error: "provider account adapter not found"}); return }
		if err := adapter.Disconnect(r.Context(), r.URL.Query().Get("directory")); err != nil {
			writeProviderAccountError(w, err); return
		}
		writeJSON(w, http.StatusOK, map[string]any{"disconnected": true})
	})
}

func writeRawProviderAccountJSON(w http.ResponseWriter, raw json.RawMessage) {
	if len(raw) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{})
		return
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		writeJSON(w, http.StatusBadGateway, jsonError{Error: "provider account adapter returned invalid JSON"})
		return
	}
	writeJSON(w, http.StatusOK, value)
}
