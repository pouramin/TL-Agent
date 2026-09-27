package main

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

var errRuntimeUnavailable = errors.New("optional compatibility runtime is unavailable")

type runtimeCredentials struct {
	Username string
	Password string
}

type runtimeEngine interface {
	ID() string
	FindBinary(override string) (string, error)
	Command(ctx context.Context, binary string, port int, credentials runtimeCredentials) *exec.Cmd
	PrepareRequest(req *http.Request, project string, credentials runtimeCredentials)
}

type runtimeBackend struct {
	state       *appState
	target      *url.URL
	credentials runtimeCredentials
	engine      runtimeEngine
	client      *http.Client
}

func newRuntimeBackend(state *appState, backendURL string, credentials runtimeCredentials, engine runtimeEngine) (*runtimeBackend, error) {
	if engine == nil {
		engine = defaultRuntimeEngine()
	}
	var target *url.URL
	if strings.TrimSpace(backendURL) != "" {
		parsed, err := url.Parse(backendURL)
		if err != nil {
			return nil, err
		}
		if parsed.Scheme == "" || parsed.Host == "" {
			return nil, errors.New("runtime backend URL must be an absolute URL")
		}
		target = parsed
	}
	return &runtimeBackend{
		state:       state,
		target:      target,
		credentials: credentials,
		engine:      engine,
		client:      &http.Client{},
	}, nil
}

func (b *runtimeBackend) available() bool {
	return b != nil && b.target != nil
}

func (b *runtimeBackend) persistenceRuntimeID() string {
	if b == nil || !b.available() || b.engine == nil {
		return "native"
	}
	return b.engine.ID()
}

func (b *runtimeBackend) newRequest(
	ctx context.Context,
	method string,
	route string,
	directory string,
	query url.Values,
	body io.Reader,
) (*http.Request, error) {
	if !b.available() {
		return nil, errRuntimeUnavailable
	}
	target := *b.target
	target.Path = route
	if query != nil {
		target.RawQuery = query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, target.String(), body)
	if err != nil {
		return nil, err
	}
	b.engine.PrepareRequest(req, directory, b.credentials)
	return req, nil
}

func (b *runtimeBackend) reverseProxy() http.Handler {
	if !b.available() {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusServiceUnavailable, jsonError{Error: errRuntimeUnavailable.Error()})
		})
	}
	proxy := httputil.NewSingleHostReverseProxy(b.target)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.URL.Path = strings.TrimPrefix(req.URL.Path, "/runtime")
		if req.URL.Path == "" {
			req.URL.Path = "/"
		}
		req.Host = b.target.Host
		project := ""
		if b.state != nil {
			project = b.state.projectPath()
		}
		b.engine.PrepareRequest(req, project, b.credentials)
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		writeJSON(w, http.StatusBadGateway, jsonError{Error: "Runtime backend unavailable: " + err.Error()})
	}
	return proxy
}

func brandRuntimeConsoleLine(line string) string {
	const upstreamStartup = "kilo server listening on "
	lower := strings.ToLower(line)
	if index := strings.Index(lower, upstreamStartup); index >= 0 {
		return line[:index] + "TL Studio runtime listening on " + line[index+len(upstreamStartup):]
	}
	return line
}

func relayRuntimeOutput(reader io.ReadCloser, writer io.Writer) {
	defer reader.Close()
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 16*1024), 1<<20)
	for scanner.Scan() {
		_, _ = io.WriteString(writer, brandRuntimeConsoleLine(scanner.Text())+"\n")
	}
}

func startRuntime(
	ctx context.Context,
	engine runtimeEngine,
	binary string,
	port int,
	credentials runtimeCredentials,
) (*exec.Cmd, error) {
	cmd := engine.Command(ctx, binary, port, credentials)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = windowsHideProcess()
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go relayRuntimeOutput(stdout, os.Stdout)
	go relayRuntimeOutput(stderr, os.Stderr)
	return cmd, nil
}
