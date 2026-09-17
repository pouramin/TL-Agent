package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectLivePreviewStaticProject(t *testing.T) {
	project := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, "index.html"), []byte("<h1>hello</h1>"), 0o600); err != nil {
		t.Fatal(err)
	}
	info := detectLivePreview(project)
	if !info.Supported || info.Mode != "static" || info.Entry != "index.html" {
		t.Fatalf("unexpected static preview detection: %#v", info)
	}
}

func TestDetectLivePreviewDevServer(t *testing.T) {
	project := t.TempDir()
	manifest := `{"scripts":{"dev":"vite"},"devDependencies":{"vite":"^7.0.0"}}`
	if err := os.WriteFile(filepath.Join(project, "package.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "pnpm-lock.yaml"), []byte("lockfileVersion: '9.0'"), 0o600); err != nil {
		t.Fatal(err)
	}
	info := detectLivePreview(project)
	if !info.Supported || info.Mode != "dev-server" {
		t.Fatalf("unexpected dev preview detection: %#v", info)
	}
	if info.Framework != "Vite" || info.PackageManager != "pnpm" || info.Command != "pnpm dev" {
		t.Fatalf("unexpected dev preview metadata: %#v", info)
	}
}

func TestDetectLivePreviewRejectsNetworkExposedScript(t *testing.T) {
	project := t.TempDir()
	manifest := `{"scripts":{"dev":"vite --host 0.0.0.0"},"devDependencies":{"vite":"^7.0.0"}}`
	if err := os.WriteFile(filepath.Join(project, "package.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	info := detectLivePreview(project)
	if info.Supported || info.Mode != "dev-server" || !strings.Contains(strings.ToLower(info.Reason), "non-loopback") {
		t.Fatalf("network-exposed script should not auto-run: %#v", info)
	}
}

func TestPreviewHostServesProjectAndBlocksSensitiveFiles(t *testing.T) {
	project := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, "index.html"), []byte("<h1>preview-ok</h1>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".env"), []byte("SECRET=do-not-serve"), 0o600); err != nil {
		t.Fatal(err)
	}
	host := newPreviewHost(&appState{project: project})

	rootReq := httptest.NewRequest(http.MethodGet, "http://preview/", nil)
	rootRes := httptest.NewRecorder()
	host.serveProject(rootRes, rootReq)
	if rootRes.Code != http.StatusOK || !strings.Contains(rootRes.Body.String(), "preview-ok") {
		t.Fatalf("static preview failed: status=%d body=%q", rootRes.Code, rootRes.Body.String())
	}
	if rootRes.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("static preview must disable cache: %q", rootRes.Header().Get("Cache-Control"))
	}

	secretReq := httptest.NewRequest(http.MethodGet, "http://preview/.env", nil)
	secretRes := httptest.NewRecorder()
	host.serveProject(secretRes, secretReq)
	if secretRes.Code != http.StatusForbidden {
		t.Fatalf("sensitive preview path status=%d", secretRes.Code)
	}
}

func TestPreviewBridgeAllowsOnlyLoopbackTargets(t *testing.T) {
	state := &appState{project: t.TempDir()}
	mux := http.NewServeMux()
	registerLocalPreviewRoutes(mux, state)

	external := httptest.NewRequest(http.MethodGet, "/local/preview/bridge?url=https%3A%2F%2Fexample.com", nil)
	externalRes := httptest.NewRecorder()
	mux.ServeHTTP(externalRes, external)
	if externalRes.Code != http.StatusBadRequest {
		t.Fatalf("external preview target status=%d", externalRes.Code)
	}

	local := httptest.NewRequest(http.MethodGet, "/local/preview/bridge?url=http%3A%2F%2F127.0.0.1%3A5173%2F", nil)
	localRes := httptest.NewRecorder()
	mux.ServeHTTP(localRes, local)
	if localRes.Code != http.StatusOK {
		t.Fatalf("loopback preview target status=%d body=%s", localRes.Code, localRes.Body.String())
	}
	if !strings.Contains(localRes.Header().Get("Content-Security-Policy"), "frame-src http://127.0.0.1:5173") {
		t.Fatalf("bridge CSP does not pin the target origin: %q", localRes.Header().Get("Content-Security-Policy"))
	}
	if localRes.Header().Get("X-Frame-Options") != "SAMEORIGIN" {
		t.Fatalf("bridge frame policy=%q", localRes.Header().Get("X-Frame-Options"))
	}
	if !strings.Contains(localRes.Body.String(), `iframe src="http://127.0.0.1:5173/"`) {
		t.Fatalf("bridge target missing: %s", localRes.Body.String())
	}
}

func TestPreviewInfoStartsIsolatedStaticOrigin(t *testing.T) {
	project := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, "index.html"), []byte("isolated-preview"), 0o600); err != nil {
		t.Fatal(err)
	}
	state := &appState{project: project}
	mux := http.NewServeMux()
	registerLocalPreviewRoutes(mux, state)

	req := httptest.NewRequest(http.MethodGet, "/local/preview/info", nil)
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("preview info status=%d body=%s", res.Code, res.Body.String())
	}
	var info livePreviewInfo
	if err := json.Unmarshal(res.Body.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	if !info.Supported || info.Mode != "static" || !strings.HasPrefix(info.URL, "http://127.0.0.1:") {
		t.Fatalf("unexpected preview info: %#v", info)
	}
	response, err := http.Get(info.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), "isolated-preview") {
		t.Fatalf("isolated server status=%d body=%q", response.StatusCode, body)
	}
}

func TestLivePreviewVersionChangesWithProjectFiles(t *testing.T) {
	project := t.TempDir()
	path := filepath.Join(project, "index.html")
	if err := os.WriteFile(path, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := livePreviewVersion(project)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("two-two"), 0o600); err != nil {
		t.Fatal(err)
	}
	after, err := livePreviewVersion(project)
	if err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Fatalf("preview version did not change: %q", before)
	}
}
