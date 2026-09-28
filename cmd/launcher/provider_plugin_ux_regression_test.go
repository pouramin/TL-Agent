package main

import (
	"strings"
	"testing"
)

func TestPluginSettingsAlwaysShowBundledAndUserSections(t *testing.T) {
	source := readBrowserSource(t, "plugins.ts")
	for _, required := range []string{
		`Included with TL Studio`,
		`No bundled plugins in this build`,
		`Added by you`,
		`No plugins added yet`,
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("plugins UI must keep section visible when empty; missing %q", required)
		}
	}
	if strings.Contains(source, `if (!plugins.length) return;`) {
		t.Fatal("empty plugin groups must not disappear from Settings")
	}
}

func TestJevSettingsUseCompactControlAndRefreshAfterProviderChanges(t *testing.T) {
	source := readBrowserSource(t, "jev-ui.ts")
	for _, required := range []string{
		`.jev-compact-row`,
		`Active JEV`,
		`Configure JEV`,
		`jevConfigDialog`,
		`tlstudio:providers-changed`,
		`providerHasRouter`,
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("compact Jev settings contract missing %q", required)
		}
	}
	if strings.Contains(source, `.jev-settings-card`) {
		t.Fatal("legacy full-height Jev card should not remain in Providers settings")
	}
}


func TestJevSetupIsAutomaticAndDoesNotOpenGenericProviderForm(t *testing.T) {
	source := readBrowserSource(t, "jev-ui.ts")
	for _, required := range []string{
		`K.api.providers.discover`,
		`K.api.providers.upsert`,
		`typesafe/jev-router`,
		`No manual model selection was required.`,
		`id="jevOpenRouterKeyInput"`,
		`data-form-type="other"`,
		`data-lpignore="true"`,
		`data-1p-ignore`,
		`-webkit-text-security:disc`,
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("one-click Jev setup contract missing %q", required)
		}
	}
	for _, forbidden := range []string{
		`providersUI.openProvider`,
		`discoverySelection?.discoverModel`,
		`TypeSafe via OpenRouter`,
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("Jev setup must not require the generic provider/model UI; found %q", forbidden)
		}
	}
}

func TestManagedJevProviderIsHiddenAndRouterIsTopLevel(t *testing.T) {
	providers := readBrowserSource(t, "providers-ui.ts")
	core := readBrowserSource(t, "core.ts")
	jev := readBrowserSource(t, "jev-ui.ts")

	for _, required := range []string{
		`.filter((provider: any) => !clean(provider?.managedBy))`,
		`managedBy: "jev"`,
	} {
		if !strings.Contains(providers+jev, required) {
			t.Fatalf("managed JEV provider UI contract missing %q", required)
		}
	}
	if !strings.Contains(core, `const routers = K.state.models.filter((model) => model.kind === "router")`) {
		t.Fatal("model selector must separate routers from provider-grouped models")
	}
	if !strings.Contains(core, `select.appendChild(option);`) {
		t.Fatal("router models must be appended directly at the top level")
	}
}

func TestJevCompactSwitchActuallyTogglesPersistedEnablement(t *testing.T) {
	source := readBrowserSource(t, "jev-ui.ts")
	for _, required := range []string{
		`K.api.jevRouter.status()`,
		`K.api.jevRouter.configure(false)`,
		`K.api.jevRouter.configure(true)`,
		`const toggleJev = async () =>`,
		`compactControl.addEventListener("click", () => { void toggleJev(); })`,
		`JEV unavailable`,
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("JEV toggle contract missing %q", required)
		}
	}
	if strings.Contains(source, `compactControl.addEventListener("click", () => { void openDialog(); })`) {
		t.Fatal("compact JEV control must toggle active state instead of always opening configuration")
	}
}

func TestProviderAPIKeyFieldAvoidsPasswordManagerSemantics(t *testing.T) {
	source := readBrowserSource(t, "providers-ui.ts")
	for _, required := range []string{
		`id="providerApiKeyInput"`,
		`type="text"`,
		`autocomplete="off"`,
		`data-form-type="other"`,
		`data-lpignore="true"`,
		`data-1p-ignore`,
		`-webkit-text-security:disc`,
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("provider API key field must avoid password-manager semantics; missing %q", required)
		}
	}
	if strings.Contains(source, `id="providerApiKeyInput" type="password"`) ||
		strings.Contains(source, `autocomplete="new-password"`) {
		t.Fatal("provider API key field must not be presented to the browser as a login password")
	}
}


func TestProviderSettingsUseDedicatedModalEditor(t *testing.T) {
	source := readBrowserSource(t, "providers-ui.ts")
	for _, required := range []string{
		`providerDialog.id = "providerDialog"`,
		`providerDialog.showModal()`,
		`providerDialog.close()`,
		`providerDialog.addEventListener("close", resetFormFields)`,
		`type="submit">Done</button>`,
		`edit.textContent = "Configure"`,
		`.provider-dialog-card{width:min(760px,calc(100vw - 36px))`,
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("provider modal contract missing %q", required)
		}
	}
	if strings.Contains(source, `<form id="providerForm" class="provider-form hidden">`) {
		t.Fatal("provider form must not remain as a hidden inline Settings form")
	}
}
