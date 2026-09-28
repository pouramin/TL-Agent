package main

import (
	"strings"
	"testing"
)

func TestBrowserProductAPINeverUsesRuntimeProxy(t *testing.T) {
	source := readBrowserSource(t, "runtime-api.ts")
	for _, forbidden := range []string{"/runtime/", "hosted:", "legacySessions:", "Sign in with Kilo"} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("native Browser product API contains removed compatibility marker %q", forbidden)
		}
	}
	for _, required := range []string{"/local/providers/catalog", "/local/providers/config", "/local/sessions", "/local/questions", "/local/permissions", "/local/events"} {
		if !strings.Contains(source, required) {
			t.Fatalf("native Browser product API is missing %q", required)
		}
	}
}
