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
