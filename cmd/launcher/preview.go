package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"html"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const previewPackageJSONLimit = 1 << 20

type livePreviewInfo struct {
	Supported      bool   `json:"supported"`
	Mode           string `json:"mode"`
	Framework      string `json:"framework,omitempty"`
	PackageManager string `json:"packageManager,omitempty"`
	Script         string `json:"script,omitempty"`
	Command        string `json:"command,omitempty"`
	Entry          string `json:"entry,omitempty"`
	URL            string `json:"url,omitempty"`
	Reason         string `json:"reason,omitempty"`
}

type previewPackageJSON struct {
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

type previewHost struct {
	state    *appState
	once     sync.Once
	baseURL  string
	startErr error
}

func newPreviewHost(state *appState) *previewHost {
	return &previewHost{state: state}
}

func (h *previewHost) ensure() (string, error) {
	h.once.Do(func() {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			h.startErr = err
			return
		}
		h.baseURL = "http://" + listener.Addr().String() + "/"
		server := &http.Server{Handler: http.HandlerFunc(h.serveProject)}
		go func() {
			_ = server.Serve(listener)
		}()
	})
	return h.baseURL, h.startErr
}

func (h *previewHost) serveProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	requested := strings.TrimPrefix(r.URL.Path, "/")
	if requested == "" {
		requested = "index.html"
	}
	if previewSensitivePath(requested) {
		http.Error(w, "preview path is protected", http.StatusForbidden)
		return
	}
	target, rel, err := resolveProjectEntry(h.state.projectPath(), requested)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	info, err := os.Stat(target)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if info.IsDir() {
		indexRel := slashJoin(rel, "index.html")
		if previewSensitivePath(indexRel) {
			http.Error(w, "preview path is protected", http.StatusForbidden)
			return
		}
		target, _, err = resolveProjectEntry(h.state.projectPath(), indexRel)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		info, err = os.Stat(target)
		if err != nil || !info.Mode().IsRegular() {
			http.NotFound(w, r)
			return
		}
	}
	if !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	file, err := os.Open(target)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, filepath.Base(target), info.ModTime(), file)
}

func registerLocalPreviewRoutes(mux *http.ServeMux, state *appState) {
	host := newPreviewHost(state)
	mux.HandleFunc("GET /local/preview/info", func(w http.ResponseWriter, _ *http.Request) {
		info := detectLivePreview(state.projectPath())
		if info.Supported && info.Mode == "static" {
			previewURL, err := host.ensure()
			if err != nil {
				info.Supported = false
				info.Reason = "Could not start the isolated static preview server: " + err.Error()
			} else {
				info.URL = previewURL
			}
		}
		writeJSON(w, http.StatusOK, info)
	})
	mux.HandleFunc("GET /local/preview/version", func(w http.ResponseWriter, _ *http.Request) {
		version, err := livePreviewVersion(state.projectPath())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, jsonError{Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"version": version})
	})
	mux.HandleFunc("GET /local/preview/bridge", func(w http.ResponseWriter, r *http.Request) {
		target, origin, err := validatePreviewTarget(r.URL.Query().Get("url"))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, jsonError{Error: err.Error()})
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; frame-src "+origin+"; frame-ancestors 'self'")
		_, _ = fmt.Fprintf(w, `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><style>html,body,iframe{width:100%%;height:100%%;margin:0;border:0;background:#fff;overflow:hidden}</style></head><body><iframe src="%s" title="TL Agent live preview"></iframe></body></html>`, html.EscapeString(target))
	})
}

func detectLivePreview(project string) livePreviewInfo {
	root, err := canonicalProjectRoot(project)
	if err != nil {
		return livePreviewInfo{Reason: err.Error()}
	}
	indexPath := filepath.Join(root, "index.html")
	_, indexErr := os.Stat(indexPath)
	hasIndex := indexErr == nil

	packagePath := filepath.Join(root, "package.json")
	packageInfo, packageErr := os.Stat(packagePath)
	if packageErr == nil && packageInfo.Mode().IsRegular() {
		if packageInfo.Size() > previewPackageJSONLimit {
			return livePreviewInfo{Reason: "package.json is too large to inspect safely"}
		}
		data, err := os.ReadFile(packagePath)
		if err == nil {
			var manifest previewPackageJSON
			if json.Unmarshal(data, &manifest) == nil {
				for _, script := range []string{"dev", "start", "serve"} {
					commandBody := strings.TrimSpace(manifest.Scripts[script])
					if commandBody == "" {
						continue
					}
					if previewScriptMayExposeNetwork(commandBody) {
						return livePreviewInfo{
							Mode:      "dev-server",
							Framework: detectPreviewFramework(manifest),
							Script:    script,
							Reason:    "The preview script appears to bind a non-loopback host. TL Agent will not auto-run a network-exposed preview command.",
						}
					}
					manager := detectPreviewPackageManager(root)
					return livePreviewInfo{
						Supported:      true,
						Mode:           "dev-server",
						Framework:      detectPreviewFramework(manifest),
						PackageManager: manager,
						Script:         script,
						Command:        previewPackageCommand(manager, script),
					}
				}
			}
		}
	}

	if hasIndex {
		return livePreviewInfo{Supported: true, Mode: "static", Framework: "Static HTML", Entry: "index.html"}
	}
	return livePreviewInfo{Reason: "No index.html or supported package.json dev/start/serve script was found."}
}

func detectPreviewPackageManager(root string) string {
	for _, candidate := range []struct {
		name string
		file string
	}{
		{"pnpm", "pnpm-lock.yaml"},
		{"yarn", "yarn.lock"},
		{"bun", "bun.lock"},
		{"bun", "bun.lockb"},
	} {
		if info, err := os.Stat(filepath.Join(root, candidate.file)); err == nil && !info.IsDir() {
			return candidate.name
		}
	}
	return "npm"
}

func previewPackageCommand(manager, script string) string {
	switch manager {
	case "pnpm":
		return "pnpm " + script
	case "yarn":
		return "yarn " + script
	case "bun":
		return "bun run " + script
	default:
		return "npm run " + script
	}
}

func detectPreviewFramework(manifest previewPackageJSON) string {
	deps := make(map[string]string, len(manifest.Dependencies)+len(manifest.DevDependencies))
	for name, version := range manifest.Dependencies {
		deps[name] = version
	}
	for name, version := range manifest.DevDependencies {
		deps[name] = version
	}
	for _, candidate := range []struct {
		pkg   string
		label string
	}{
		{"next", "Next.js"},
		{"astro", "Astro"},
		{"vite", "Vite"},
		{"@angular/core", "Angular"},
		{"react-scripts", "Create React App"},
		{"parcel", "Parcel"},
		{"nuxt", "Nuxt"},
		{"@sveltejs/kit", "SvelteKit"},
	} {
		if _, ok := deps[candidate.pkg]; ok {
			return candidate.label
		}
	}
	return "Web project"
}

func previewScriptMayExposeNetwork(script string) bool {
	value := strings.ToLower(strings.TrimSpace(script))
	for _, marker := range []string{
		"--host=0.0.0.0", "--host 0.0.0.0",
		"--hostname=0.0.0.0", "--hostname 0.0.0.0",
		"host=0.0.0.0", "--host=::", "--host ::",
		"--hostname=::", "--hostname ::",
	} {
		if strings.Contains(value, marker) {
			return true
		}
	}
	fields := strings.Fields(value)
	for index, field := range fields {
		if field == "--host" && (index+1 >= len(fields) || strings.HasPrefix(fields[index+1], "-")) {
			return true
		}
	}
	return false
}

func validatePreviewTarget(raw string) (string, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", fmt.Errorf("preview URL is required")
	}
	target, err := url.Parse(raw)
	if err != nil || target.Host == "" {
		return "", "", fmt.Errorf("invalid preview URL")
	}
	if target.Scheme != "http" && target.Scheme != "https" {
		return "", "", fmt.Errorf("preview URL must use http or https")
	}
	if !isLoopbackHost(target.Hostname()) {
		return "", "", fmt.Errorf("preview URL must point to localhost")
	}
	origin := target.Scheme + "://" + target.Host
	return target.String(), origin, nil
}

func previewSensitivePath(requested string) bool {
	clean := strings.Trim(strings.ReplaceAll(requested, "\\", "/"), "/")
	for _, part := range strings.Split(clean, "/") {
		lower := strings.ToLower(part)
		if lower == ".git" || strings.HasPrefix(lower, ".env") || lower == ".npmrc" || lower == ".yarnrc" {
			return true
		}
	}
	base := strings.ToLower(filepath.Base(clean))
	if base == "id_rsa" || base == "id_ed25519" || base == "credentials.json" {
		return true
	}
	switch strings.ToLower(filepath.Ext(base)) {
	case ".pem", ".key", ".p12", ".pfx":
		return true
	}
	return false
}

func livePreviewVersion(project string) (string, error) {
	root, err := canonicalProjectRoot(project)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	count := 0
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if path == root {
			return nil
		}
		name := strings.ToLower(entry.Name())
		if entry.IsDir() {
			switch name {
			case ".git", "node_modules", ".next", ".cache", ".turbo", "dist", "build", "coverage":
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() || previewSensitivePath(path) {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		_, _ = fmt.Fprintf(hash, "%s\x00%d\x00%d\n", filepath.ToSlash(rel), info.Size(), info.ModTime().UnixNano())
		count++
		if count >= 3000 {
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sum := hash.Sum(nil)
	return fmt.Sprintf("%x", sum[:12]), nil
}
