package main

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestProviderAccountLocalAPINeverReturnsStoredTokens(t *testing.T) {
	t.Setenv("TL_STUDIO_STATE_DIR", t.TempDir())

	state := &appState{frontendURL: "http://127.0.0.1:32125"}
	manager := newProviderManager(state)
	adapter := newHuggingFaceAccountAdapter(state, manager)
	adapter.clientID = "public-test-client"

	credential := providerAccountStoredCredential{
		AccessToken:  "leak-check-access",
		RefreshToken: "leak-check-refresh",
		ExpiresAt:    time.Now().UTC().Add(time.Hour).Format(time.RFC3339),
		AccountLabel: "user@example.test",
		AccountType:  "PRO",
		Entitlement:  "Inference Providers",
	}
	if err := adapter.saveCredential(credential); err != nil {
		t.Fatal(err)
	}

	service := newProviderAccountService(adapter)
	mux := http.NewServeMux()
	registerProviderAccountRoutes(mux, service)
	server := httptest.NewServer(mux)
	defer server.Close()

	var logs bytes.Buffer
	previousWriter := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(previousWriter)

	for _, endpoint := range []string{
		"/local/provider-accounts",
		"/local/provider-accounts/huggingface",
	} {
		res, err := http.Get(server.URL + endpoint)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != http.StatusOK {
			t.Fatalf("%s returned status %d: %s", endpoint, res.StatusCode, body)
		}
		for _, secret := range []string{credential.AccessToken, credential.RefreshToken} {
			if strings.Contains(string(body), secret) {
				t.Fatalf("%s leaked stored provider token", endpoint)
			}
		}
	}

	for _, secret := range []string{credential.AccessToken, credential.RefreshToken} {
		if strings.Contains(logs.String(), secret) {
			t.Fatal("provider account operation leaked a token to normal logs")
		}
	}

	if _, err := adapter.RuntimeCredential(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestProviderAccountBrowserContractHasNoCredentialFields(t *testing.T) {
	sources := strings.Join([]string{
		readBrowserSource(t, "global.d.ts"),
		readBrowserSource(t, "runtime-api.ts"),
		readBrowserSource(t, "provider-account-ui.ts"),
	}, "\n")

	for _, forbidden := range []string{
		"accessToken",
		"refreshToken",
		"codeVerifier",
		"clientSecret",
	} {
		if strings.Contains(sources, forbidden) {
			t.Fatalf("Browser Provider Account contract must not expose credential field %q", forbidden)
		}
	}
}


func TestProviderAccountUIKeepsAccountAndAPIConfigurationSeparate(t *testing.T) {
	source := readBrowserSource(t, "provider-account-ui.ts")
	for _, required := range []string{
		"Account connections",
		"API configuration",
		"Sign in with",
		"Reconnect",
		"Sign out",
		"unsupportedReason",
		"authorizationUrl",
		"verificationUrl",
		"userCode",
		"providerAccounts.complete",
		"providerAccounts.cancel",
		"cancelActiveLogin",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("Provider Account UI contract missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"localStorage",
		"sessionStorage",
		"parseDeviceCode",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("Provider Account UI must not use %q for account authentication", forbidden)
		}
	}
}
