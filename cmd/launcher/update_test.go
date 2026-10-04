package main

import "testing"

func TestCompareTLStudioSemver(t *testing.T) {
	cases := []struct{ a, b string; want int }{
		{"v0.5.0", "v0.5.0", 0},
		{"v0.5.1", "v0.5.0", 1},
		{"v0.6.0-alpha.2", "v0.6.0-alpha.1", 1},
		{"v0.6.0-beta.1", "v0.6.0-alpha.9", 1},
		{"v0.6.0", "v0.6.0-rc.2", 1},
	}
	for _, tc := range cases {
		a, ok := parseTLStudioSemver(tc.a); if !ok { t.Fatalf("parse %q", tc.a) }
		b, ok := parseTLStudioSemver(tc.b); if !ok { t.Fatalf("parse %q", tc.b) }
		got := compareTLStudioSemver(a, b)
		if got < 0 { got = -1 } else if got > 0 { got = 1 }
		if got != tc.want { t.Fatalf("compare %s to %s = %d, want %d", tc.a, tc.b, got, tc.want) }
	}
}

func TestSelectTLStudioUpdateUsesCompatibleVerifiedRelease(t *testing.T) {
	releases := []tlStudioRelease{
		{TagName: "v0.6.0", HTMLURL: "https://example.test/v0.6.0", Assets: []tlStudioReleaseAsset{
			{Name: "tl-studio-v0.6.0-windows-x64.zip", BrowserDownloadURL: "https://example.test/windows.zip"},
			{Name: "SHA256SUMS.txt", BrowserDownloadURL: "https://example.test/sums"},
		}},
		{TagName: "v0.7.0", HTMLURL: "https://example.test/v0.7.0", Assets: []tlStudioReleaseAsset{
			{Name: "tl-studio-v0.7.0-linux-x64.tar.gz", BrowserDownloadURL: "https://example.test/linux.tar.gz"},
			{Name: "SHA256SUMS.txt", BrowserDownloadURL: "https://example.test/sums-070"},
		}},
	}
	candidate, ok := selectTLStudioUpdate("v0.5.0", releases, "windows", "amd64")
	if !ok { t.Fatal("expected compatible Windows update") }
	if candidate.LatestVersion != "v0.6.0" || candidate.AssetURL != "https://example.test/windows.zip" || candidate.SumsURL == "" {
		t.Fatalf("unexpected candidate: %#v", candidate)
	}
}

func TestStableBuildDoesNotSelectPrerelease(t *testing.T) {
	releases := []tlStudioRelease{{TagName: "v0.6.0-beta.1", Prerelease: true, Assets: []tlStudioReleaseAsset{
		{Name: "tl-studio-v0.6.0-beta.1-windows-x64.zip", BrowserDownloadURL: "https://example.test/beta.zip"},
		{Name: "SHA256SUMS.txt", BrowserDownloadURL: "https://example.test/sums"},
	}}}
	if _, ok := selectTLStudioUpdate("v0.5.0", releases, "windows", "amd64"); ok {
		t.Fatal("stable build must not auto-select prerelease update")
	}
	if _, ok := selectTLStudioUpdate("v0.6.0-alpha.1", releases, "windows", "amd64"); !ok {
		t.Fatal("prerelease build should accept newer prerelease")
	}
}
