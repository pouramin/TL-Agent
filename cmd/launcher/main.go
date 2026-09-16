package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"embed"
)

var version = "dev"

//go:embed web/*
var webFS embed.FS

type appState struct {
	mu          sync.RWMutex
	project     string
	kiloPath    string
	backendURL  string
	frontendURL string
}

func (s *appState) snapshot() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return map[string]any{
		"version":     version,
		"project":     s.project,
		"kiloPath":    s.kiloPath,
		"backendURL":  s.backendURL,
		"frontendURL": s.frontendURL,
		"platform":    runtime.GOOS,
		"arch":        runtime.GOARCH,
	}
}

func (s *appState) setProject(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.project = path
}

func (s *appState) projectPath() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.project
}

type jsonError struct {
	Error string `json:"error"`
}

func main() {
	var projectArg string
	var noBrowser bool
	var listenAddr string
	var kiloOverride string
	flag.StringVar(&projectArg, "project", "", "project directory to open")
	flag.BoolVar(&noBrowser, "no-browser", false, "do not open the browser automatically")
	flag.StringVar(&listenAddr, "listen", "127.0.0.1", "frontend listen address")
	flag.StringVar(&kiloOverride, "kilo", "", "path to the kilo binary")
	flag.Parse()
	if !isLoopbackHost(listenAddr) {
		log.Fatalf("listen: %q is not a loopback address; this UI intentionally binds only to localhost", listenAddr)
	}

	if projectArg == "" && flag.NArg() > 0 {
		projectArg = flag.Arg(0)
	}
	project, err := normalizeProject(projectArg)
	if err != nil {
		log.Fatalf("project: %v", err)
	}

	kiloPath, err := findKiloBinary(kiloOverride)
	if err != nil {
		log.Fatal(err)
	}

	backendPort, err := freePort("127.0.0.1")
	if err != nil {
		log.Fatalf("find backend port: %v", err)
	}
	frontendPort, err := freePort(listenAddr)
	if err != nil {
		log.Fatalf("find frontend port: %v", err)
	}

	username := "kilo"
	password, err := randomSecret(24)
	if err != nil {
		log.Fatalf("create server password: %v", err)
	}

	backendURL := fmt.Sprintf("http://127.0.0.1:%d", backendPort)
	frontendURL := "http://" + net.JoinHostPort(listenAddr, fmt.Sprint(frontendPort))
	state := &appState{
		project:     project,
		kiloPath:    kiloPath,
		backendURL:  backendURL,
		frontendURL: frontendURL,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	kiloCmd, err := startKilo(ctx, kiloPath, backendPort, username, password)
	if err != nil {
		log.Fatalf("start kilo: %v", err)
	}
	defer stopProcess(kiloCmd)

	if err := waitForPort(ctx, "127.0.0.1", backendPort, 12*time.Second); err != nil {
		stopProcess(kiloCmd)
		log.Fatalf("kilo backend did not start: %v", err)
	}

	server, err := newServer(state, backendURL, username, password)
	if err != nil {
		stopProcess(kiloCmd)
		log.Fatalf("create local server: %v", err)
	}

	httpServer := &http.Server{
		Addr:              net.JoinHostPort(listenAddr, fmt.Sprint(frontendPort)),
		Handler:           server,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer shutdownCancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	fmt.Printf("TL Agent %s\n", version)
	fmt.Printf("  Project: %s\n", project)
	fmt.Printf("  Local:   %s\n", frontendURL)
	fmt.Printf("  Backend: %s\n", backendURL)

	if !noBrowser {
		go func() {
			time.Sleep(250 * time.Millisecond)
			if err := openBrowser(frontendURL); err != nil {
				log.Printf("open browser: %v", err)
			}
		}()
	}

	err = httpServer.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("local server: %v", err)
	}
}

func newServer(state *appState, backendURL, username, password string) (http.Handler, error) {
	target, err := url.Parse(backendURL)
	if err != nil {
		return nil, err
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.URL.Path = strings.TrimPrefix(req.URL.Path, "/kilo")
		if req.URL.Path == "" {
			req.URL.Path = "/"
		}
		req.Host = target.Host
		req.SetBasicAuth(username, password)
		if project := state.projectPath(); project != "" {
			req.Header.Set("x-kilo-directory", strings.ReplaceAll(url.QueryEscape(project), "+", "%20"))
		}
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		writeJSON(w, http.StatusBadGateway, jsonError{Error: "Kilo backend unavailable: " + err.Error()})
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /local/status", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, state.snapshot())
	})
	mux.HandleFunc("POST /local/project", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, jsonError{Error: "invalid JSON body"})
			return
		}
		project, err := normalizeProject(body.Path)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, jsonError{Error: err.Error()})
			return
		}
		state.setProject(project)
		writeJSON(w, http.StatusOK, state.snapshot())
	})
	mux.HandleFunc("POST /local/pick-directory", func(w http.ResponseWriter, _ *http.Request) {
		path, err := pickDirectory(state.projectPath())
		if err != nil {
			writeJSON(w, http.StatusNotImplemented, jsonError{Error: err.Error()})
			return
		}
		if path == "" {
			writeJSON(w, http.StatusOK, state.snapshot())
			return
		}
		project, err := normalizeProject(path)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, jsonError{Error: err.Error()})
			return
		}
		state.setProject(project)
		writeJSON(w, http.StatusOK, state.snapshot())
	})
	registerLocalFileRoutes(mux, state)
	registerLocalProcessRoutes(mux, state)
	mux.Handle("/kilo/", proxy)
	mux.Handle("/kilo", proxy)

	assets, err := fs.Sub(webFS, "web")
	if err != nil {
		return nil, err
	}
	indexHTML, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		return nil, fmt.Errorf("read embedded index.html: %w", err)
	}
	fileServer := http.FileServer(http.FS(assets))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusOK)
			if r.Method == http.MethodGet {
				_, _ = w.Write(indexHTML)
			}
			return
		}
		fileServer.ServeHTTP(w, r)
	})
	return localOnly(securityHeaders(mux)), nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

func localOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil {
			host = r.Host
		}
		if !isLoopbackHost(host) {
			http.Error(w, "localhost access only", http.StatusForbidden)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			parsed, parseErr := url.Parse(origin)
			if parseErr != nil || !isLoopbackHost(parsed.Hostname()) || !strings.EqualFold(parsed.Hostname(), host) {
				http.Error(w, "invalid origin", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self'; img-src 'self' data:; connect-src 'self'; font-src 'self' data:")
		next.ServeHTTP(w, r)
	})
}

func normalizeProject(input string) (string, error) {
	if strings.TrimSpace(input) == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return wd, nil
	}
	abs, err := filepath.Abs(input)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("not a directory: %s", abs)
	}
	return abs, nil
}

func findKiloBinary(override string) (string, error) {
	candidates := []string{}
	if strings.TrimSpace(override) != "" {
		candidates = append(candidates, override)
	}
	if env := strings.TrimSpace(os.Getenv("KILO_BIN")); env != "" {
		candidates = append(candidates, env)
	}
	if exe, err := os.Executable(); err == nil {
		name := "kilo"
		if runtime.GOOS == "windows" {
			name = "kilo.exe"
		}
		candidates = append(candidates,
			filepath.Join(filepath.Dir(exe), "bin", name),
			filepath.Join(filepath.Dir(exe), name),
		)
	}
	candidates = append(candidates, "kilo")
	for _, candidate := range candidates {
		if resolved, err := exec.LookPath(candidate); err == nil {
			return resolved, nil
		}
		if filepath.IsAbs(candidate) || strings.ContainsRune(candidate, os.PathSeparator) {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate, nil
			}
		}
	}
	return "", errors.New("TL Agent runtime not found")
}

func startKilo(ctx context.Context, binary string, port int, username, password string) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, binary, "serve", "--hostname", "127.0.0.1", "--port", fmt.Sprint(port))
	cmd.Env = append(os.Environ(), "KILO_SERVER_USERNAME="+username, "KILO_SERVER_PASSWORD="+password)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func stopProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Signal(os.Interrupt)
	done := make(chan struct{})
	go func() {
		_, _ = cmd.Process.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		_ = cmd.Process.Kill()
	}
}

func freePort(host string) (int, error) {
	listener, err := net.Listen("tcp", net.JoinHostPort(host, "0"))
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func randomSecret(bytes int) (string, error) {
	buffer := make([]byte, bytes)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

func waitForPort(ctx context.Context, host string, port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	address := net.JoinHostPort(host, fmt.Sprint(port))
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", address, 150*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return fmt.Errorf("timed out waiting for %s", address)
}

func openBrowser(target string) error {
	var command string
	var args []string
	switch runtime.GOOS {
	case "windows":
		command = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", target}
	case "darwin":
		command = "open"
		args = []string{target}
	default:
		command = "xdg-open"
		args = []string{target}
	}
	return exec.Command(command, args...).Start()
}

func pickDirectory(current string) (string, error) {
	switch runtime.GOOS {
	case "windows":
		script := fmt.Sprintf(`Add-Type -AssemblyName System.Windows.Forms; $dialog = New-Object System.Windows.Forms.FolderBrowserDialog; $dialog.Description = 'Choose a project folder'; $dialog.ShowNewFolderButton = $true; %s if ($dialog.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) { [Console]::OutputEncoding = [System.Text.Encoding]::UTF8; Write-Output $dialog.SelectedPath }`, powershellInitialDirectory(current))
		output, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-STA", "-Command", script).Output()
		if err != nil {
			return "", fmt.Errorf("folder picker: %w", err)
		}
		return strings.TrimSpace(string(output)), nil
	case "darwin":
		output, err := exec.Command("osascript", "-e", `POSIX path of (choose folder with prompt "Choose a project folder")`).Output()
		if err != nil {
			return "", fmt.Errorf("folder picker: %w", err)
		}
		return strings.TrimSpace(string(output)), nil
	default:
		if path, err := exec.LookPath("zenity"); err == nil {
			output, runErr := exec.Command(path, "--file-selection", "--directory", "--title=Choose a project folder").Output()
			if runErr != nil {
				return "", fmt.Errorf("folder picker: %w", runErr)
			}
			return strings.TrimSpace(string(output)), nil
		}
		return "", errors.New("native folder picker is unavailable on this system; enter the path manually")
	}
}

func powershellInitialDirectory(current string) string {
	if strings.TrimSpace(current) == "" {
		return ""
	}
	escaped := strings.ReplaceAll(current, "'", "''")
	return fmt.Sprintf("$dialog.SelectedPath = '%s';", escaped)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
