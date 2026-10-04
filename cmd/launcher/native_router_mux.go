package main

import (
	"context"
	"errors"
	"strings"
)

type nativeRequestRouterMux struct {
	routers []nativeRequestRouter
}

func newNativeRequestRouterMux(routers ...nativeRequestRouter) *nativeRequestRouterMux {
	filtered := make([]nativeRequestRouter, 0, len(routers))
	for _, router := range routers {
		if router != nil {
			filtered = append(filtered, router)
		}
	}
	return &nativeRequestRouterMux{routers: filtered}
}

func (m *nativeRequestRouterMux) Handles(model *sessionModelRef) bool {
	if m == nil {
		return false
	}
	for _, router := range m.routers {
		if router.Handles(model) {
			return true
		}
	}
	return false
}

func (m *nativeRequestRouterMux) Route(ctx context.Context, project, prompt string) (nativeRouteSelection, error) {
	if m == nil || len(m.routers) == 0 {
		return nativeRouteSelection{}, errors.New("request router is unavailable")
	}
	return m.routers[0].Route(ctx, project, prompt)
}

func (m *nativeRequestRouterMux) RouteFor(ctx context.Context, project, prompt string, target *sessionModelRef) (nativeRouteSelection, error) {
	if m == nil {
		return nativeRouteSelection{}, errors.New("request router is unavailable")
	}
	for _, router := range m.routers {
		if target != nil && router.Handles(target) {
			return router.Route(ctx, project, prompt)
		}
	}
	return nativeRouteSelection{}, errors.New("selected request router is unavailable")
}

func (m *nativeRequestRouterMux) Fallback(
	ctx context.Context,
	project string,
	prompt string,
	previous nativeRouteSelection,
	failure error,
) (nativeRouteSelection, bool, error) {
	if m == nil {
		return nativeRouteSelection{}, false, nil
	}
	routerRef := &sessionModelRef{
		ProviderID: strings.TrimSpace(previous.RouterProviderID),
		ID: strings.TrimSpace(previous.RouterModelID),
	}
	for _, router := range m.routers {
		if routerRef.ProviderID != "" && routerRef.ID != "" && !router.Handles(routerRef) {
			continue
		}
		fallback, ok := router.(nativeRequestFallbackRouter)
		if !ok {
			continue
		}
		return fallback.Fallback(ctx, project, prompt, previous, failure)
	}
	return nativeRouteSelection{}, false, nil
}
