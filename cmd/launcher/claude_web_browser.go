package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const claudeWebURL = "https://claude.ai/"

type claudeWebProbe struct {
	Connected        bool   `json:"connected"`
	Status           int    `json:"status,omitempty"`
	OrganizationID   string `json:"organizationId,omitempty"`
	OrganizationName string `json:"organizationName,omitempty"`
	Error            string `json:"error,omitempty"`
}

type claudeWebCompletionResult struct {
	OK     bool   `json:"ok"`
	Status int    `json:"status,omitempty"`
	Text   string `json:"text,omitempty"`
	Error  string `json:"error,omitempty"`
}

type claudeWebTransport interface {
	Available() error
	OpenLogin(context.Context) error
	Probe(context.Context) (claudeWebProbe, error)
	Complete(context.Context, string) (string, error)
	Close(context.Context) error
}

type claudeWebBrowserTransport struct {
	mu         sync.Mutex
	command    *exec.Cmd
	profileDir string
}

func newClaudeWebBrowserTransport() *claudeWebBrowserTransport {
	return &claudeWebBrowserTransport{profileDir: claudeWebProfileDirectory()}
}

func (t *claudeWebBrowserTransport) Available() error {
	_, err := resolveClaudeWebBrowserExecutable()
	return err
}

func claudeWebProfileDirectory() string {
	return filepath.Join(tlStudioStateDirectory(), "claude-web-browser")
}

func claudeWebBrowserExecutableCandidates() []string {
	values := []string{}
	if override := strings.TrimSpace(os.Getenv("TL_STUDIO_CLAUDE_WEB_BROWSER")); override != "" {
		values = append(values, override)
	}
	if config, err := loadClaudeWebConfig(); err == nil && strings.TrimSpace(config.BrowserExecutable) != "" {
		values = append(values, strings.TrimSpace(config.BrowserExecutable))
	}
	switch runtime.GOOS {
	case "windows":
		for _, root := range []string{
			os.Getenv("PROGRAMFILES"),
			os.Getenv("PROGRAMFILES(X86)"),
			os.Getenv("LOCALAPPDATA"),
		} {
			root = strings.TrimSpace(root)
			if root == "" {
				continue
			}
			values = append(values,
				filepath.Join(root, "Microsoft", "Edge", "Application", "msedge.exe"),
				filepath.Join(root, "Google", "Chrome", "Application", "chrome.exe"),
			)
		}
		values = append(values, "msedge", "chrome")
	case "darwin":
		values = append(values,
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"google-chrome",
			"chromium",
		)
	default:
		values = append(values, "google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "microsoft-edge", "microsoft-edge-stable")
	}
	return values
}

func resolveClaudeWebBrowserExecutable() (string, error) {
	seen := map[string]bool{}
	for _, candidate := range claudeWebBrowserExecutableCandidates() {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" || seen[candidate] {
			continue
		}
		seen[candidate] = true
		if path, ok := resolveExecutableCandidate(candidate); ok {
			return path, nil
		}
	}
	return "", errors.New("Chrome, Edge, or Chromium was not found for Claude Web browser sign-in")
}

func (t *claudeWebBrowserTransport) OpenLogin(ctx context.Context) error {
	if err := t.closeBrowser(ctx); err != nil {
		return err
	}
	_, err := t.ensureBrowser(ctx, true)
	return err
}

func (t *claudeWebBrowserTransport) Probe(ctx context.Context) (claudeWebProbe, error) {
	if _, err := t.ensureBrowser(ctx, false); err != nil {
		return claudeWebProbe{}, err
	}
	value, err := t.evaluate(ctx, `(async () => {
  try {
    const response = await fetch("/api/organizations", {credentials: "include"});
    const text = await response.text();
    if (!response.ok) {
      return JSON.stringify({connected:false,status:response.status,error:text.slice(0,500)});
    }
    let payload;
    try { payload = JSON.parse(text); } catch (_) {
      return JSON.stringify({connected:false,status:response.status,error:"Claude Web returned invalid organization JSON"});
    }
    const org = Array.isArray(payload) && payload.length ? payload[0] : null;
    return JSON.stringify({
      connected: !!(org && org.uuid),
      status: response.status,
      organizationId: org && org.uuid ? String(org.uuid) : "",
      organizationName: org && org.name ? String(org.name) : ""
    });
  } catch (error) {
    return JSON.stringify({connected:false,error:String(error)});
  }
})()`)
	if err != nil {
		return claudeWebProbe{}, err
	}
	var probe claudeWebProbe
	if err := json.Unmarshal([]byte(value), &probe); err != nil {
		return claudeWebProbe{}, fmt.Errorf("decode Claude Web login probe: %w", err)
	}
	return probe, nil
}

func (t *claudeWebBrowserTransport) Complete(ctx context.Context, prompt string) (string, error) {
	if _, err := t.ensureBrowser(ctx, false); err != nil {
		return "", err
	}
	promptJSON, err := json.Marshal(prompt)
	if err != nil {
		return "", err
	}
	expression := fmt.Sprintf(`(async () => {
  const prompt = %s;
  let orgId = "";
  let chatId = "";
  try {
    const orgResponse = await fetch("/api/organizations", {credentials:"include"});
    const orgText = await orgResponse.text();
    if (!orgResponse.ok) {
      return JSON.stringify({ok:false,status:orgResponse.status,error:orgText.slice(0,1000)});
    }
    const organizations = JSON.parse(orgText);
    const org = Array.isArray(organizations) && organizations.length ? organizations[0] : null;
    orgId = org && org.uuid ? String(org.uuid) : "";
    if (!orgId) {
      return JSON.stringify({ok:false,error:"Claude Web account is not signed in"});
    }

    const createResponse = await fetch("/api/organizations/" + encodeURIComponent(orgId) + "/chat_conversations", {
      method:"POST",
      credentials:"include",
      headers:{"Content-Type":"application/json"},
      body:JSON.stringify({name:""})
    });
    const createText = await createResponse.text();
    if (!createResponse.ok) {
      return JSON.stringify({ok:false,status:createResponse.status,error:createText.slice(0,1000)});
    }
    const created = JSON.parse(createText);
    chatId = created && created.uuid ? String(created.uuid) : "";
    if (!chatId) {
      return JSON.stringify({ok:false,error:"Claude Web did not return a conversation id"});
    }

    const timezone = (() => {
      try { return Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC"; }
      catch (_) { return "UTC"; }
    })();

    const completionResponse = await fetch(
      "/api/organizations/" + encodeURIComponent(orgId) + "/chat_conversations/" + encodeURIComponent(chatId) + "/completion",
      {
        method:"POST",
        credentials:"include",
        headers:{
          "Accept":"text/event-stream",
          "Content-Type":"application/json"
        },
        body:JSON.stringify({attachments:[],files:[],prompt,timezone})
      }
    );
    if (!completionResponse.ok) {
      const detail = await completionResponse.text();
      return JSON.stringify({ok:false,status:completionResponse.status,error:detail.slice(0,1500)});
    }
    if (!completionResponse.body) {
      return JSON.stringify({ok:false,error:"Claude Web completion did not provide a response body"});
    }

    const reader = completionResponse.body.getReader();
    const decoder = new TextDecoder();
    let buffer = "";
    let output = "";
    let done = false;
    while (!done) {
      const item = await reader.read();
      done = item.done;
      if (item.value) buffer += decoder.decode(item.value, {stream:!done});
      const lines = buffer.split("\n");
      buffer = lines.pop() || "";
      for (const rawLine of lines) {
        const line = rawLine.trim();
        if (!line.startsWith("data:")) continue;
        const payloadText = line.slice(5).trim();
        if (!payloadText || payloadText === "[DONE]") continue;
        let event;
        try { event = JSON.parse(payloadText); } catch (_) { continue; }
        if (event.type === "completion" && typeof event.completion === "string") {
          output += event.completion;
        } else if (event.type === "content_block_delta" && event.delta && typeof event.delta.text === "string") {
          output += event.delta.text;
        } else if (event.type === "error") {
          const message = event.message || (event.error && event.error.message) || JSON.stringify(event);
          return JSON.stringify({ok:false,error:String(message).slice(0,1500)});
        }
      }
    }
    if (buffer.trim().startsWith("data:")) {
      const payloadText = buffer.trim().slice(5).trim();
      try {
        const event = JSON.parse(payloadText);
        if (event.type === "completion" && typeof event.completion === "string") output += event.completion;
        if (event.type === "content_block_delta" && event.delta && typeof event.delta.text === "string") output += event.delta.text;
      } catch (_) {}
    }
    return JSON.stringify({ok:true,status:completionResponse.status,text:output});
  } catch (error) {
    return JSON.stringify({ok:false,error:String(error)});
  } finally {
    if (orgId && chatId) {
      try {
        await fetch(
          "/api/organizations/" + encodeURIComponent(orgId) + "/chat_conversations/" + encodeURIComponent(chatId),
          {method:"DELETE",credentials:"include"}
        );
      } catch (_) {}
    }
  }
})()`, string(promptJSON))
	value, err := t.evaluateWithTimeout(ctx, expression, 5*time.Minute)
	if err != nil {
		return "", err
	}
	var result claudeWebCompletionResult
	if err := json.Unmarshal([]byte(value), &result); err != nil {
		return "", fmt.Errorf("decode Claude Web completion bridge: %w", err)
	}
	if !result.OK {
		detail := strings.TrimSpace(result.Error)
		if detail == "" {
			detail = "Claude Web request failed"
		}
		if result.Status != 0 {
			return "", fmt.Errorf("Claude Web request failed with HTTP %d: %s", result.Status, detail)
		}
		return "", errors.New(detail)
	}
	if strings.TrimSpace(result.Text) == "" {
		return "", errors.New("Claude Web returned an empty response")
	}
	return result.Text, nil
}

func (t *claudeWebBrowserTransport) Close(ctx context.Context) error {
	return t.closeBrowser(ctx)
}

func (t *claudeWebBrowserTransport) closeBrowser(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.closeBrowserLocked(ctx)
}

func (t *claudeWebBrowserTransport) closeBrowserLocked(ctx context.Context) error {
	if port, browserWS, ok := claudeWebActivePort(t.profileDir); ok {
		_ = port
		closeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		_, _ = cdpCall(closeCtx, browserWS, "Browser.close", map[string]any{})
		cancel()
	}
	if t.command != nil && t.command.Process != nil {
		_ = t.command.Process.Kill()
	}
	t.command = nil
	return nil
}

func (t *claudeWebBrowserTransport) ensureBrowser(ctx context.Context, visible bool) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if port, _, ok := claudeWebActivePort(t.profileDir); ok {
		return port, nil
	}
	executable, err := resolveClaudeWebBrowserExecutable()
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(t.profileDir, 0o700); err != nil {
		return 0, fmt.Errorf("create Claude Web browser profile: %w", err)
	}
	_ = os.Remove(filepath.Join(t.profileDir, "DevToolsActivePort"))

	args := []string{
		"--user-data-dir=" + t.profileDir,
		"--remote-debugging-address=127.0.0.1",
		"--remote-debugging-port=0",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-sync",
	}
	if visible {
		args = append(args, "--new-window")
	} else {
		args = append(args, "--headless=new", "--disable-gpu")
	}
	args = append(args, claudeWebURL)

	cmd := exec.Command(executable, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("start Claude Web browser: %w", err)
	}
	t.command = cmd
	go func(command *exec.Cmd) {
		_ = command.Wait()
		t.mu.Lock()
		if t.command == command {
			t.command = nil
		}
		t.mu.Unlock()
	}(cmd)

	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		if port, _, ok := claudeWebActivePort(t.profileDir); ok {
			return port, nil
		}
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	_ = cmd.Process.Kill()
	return 0, errors.New("Claude Web browser did not expose its local DevTools endpoint")
}

func claudeWebActivePort(profileDir string) (int, string, bool) {
	data, err := os.ReadFile(filepath.Join(profileDir, "DevToolsActivePort"))
	if err != nil {
		return 0, "", false
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 2 {
		return 0, "", false
	}
	port, err := strconv.Atoi(strings.TrimSpace(lines[0]))
	if err != nil || port <= 0 || port > 65535 {
		return 0, "", false
	}
	browserPath := strings.TrimSpace(lines[1])
	if browserPath == "" {
		return 0, "", false
	}
	endpoint := fmt.Sprintf("http://127.0.0.1:%d/json/version", port)
	client := &http.Client{Timeout: 500 * time.Millisecond}
	response, err := client.Get(endpoint)
	if err != nil {
		return 0, "", false
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return 0, "", false
	}
	var version struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&version); err != nil {
		return 0, "", false
	}
	browserWS := strings.TrimSpace(version.WebSocketDebuggerURL)
	if browserWS == "" {
		browserWS = fmt.Sprintf("ws://127.0.0.1:%d%s", port, browserPath)
	}
	return port, browserWS, true
}

func (t *claudeWebBrowserTransport) evaluate(ctx context.Context, expression string) (string, error) {
	return t.evaluateWithTimeout(ctx, expression, 30*time.Second)
}

func (t *claudeWebBrowserTransport) evaluateWithTimeout(ctx context.Context, expression string, timeout time.Duration) (string, error) {
	port, err := t.ensureBrowser(ctx, false)
	if err != nil {
		return "", err
	}
	pageWS, err := claudeWebPageWebSocket(ctx, port)
	if err != nil {
		return "", err
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	raw, err := cdpCall(callCtx, pageWS, "Runtime.evaluate", map[string]any{
		"expression":    expression,
		"awaitPromise":  true,
		"returnByValue": true,
	})
	if err != nil {
		return "", err
	}
	var evaluated struct {
		Result struct {
			Type        string          `json:"type"`
			Value       json.RawMessage `json:"value"`
			Description string          `json:"description,omitempty"`
		} `json:"result"`
		ExceptionDetails json.RawMessage `json:"exceptionDetails,omitempty"`
	}
	if err := json.Unmarshal(raw, &evaluated); err != nil {
		return "", fmt.Errorf("decode DevTools evaluation result: %w", err)
	}
	if len(evaluated.ExceptionDetails) > 0 && string(evaluated.ExceptionDetails) != "null" {
		return "", fmt.Errorf("Claude Web browser evaluation failed: %s", string(evaluated.ExceptionDetails))
	}
	if evaluated.Result.Type != "string" {
		if evaluated.Result.Description != "" {
			return "", errors.New(evaluated.Result.Description)
		}
		return "", fmt.Errorf("Claude Web browser returned DevTools type %q", evaluated.Result.Type)
	}
	var value string
	if err := json.Unmarshal(evaluated.Result.Value, &value); err != nil {
		return "", fmt.Errorf("decode Claude Web browser value: %w", err)
	}
	return value, nil
}

func claudeWebPageWebSocket(ctx context.Context, port int) (string, error) {
	client := &http.Client{Timeout: 2 * time.Second}
	listURL := fmt.Sprintf("http://127.0.0.1:%d/json/list", port)
	readTargets := func() ([]struct {
		Type                 string `json:"type"`
		URL                  string `json:"url"`
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}, error) {
		response, err := client.Get(listURL)
		if err != nil {
			return nil, err
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("DevTools target list returned HTTP %d", response.StatusCode)
		}
		var targets []struct {
			Type                 string `json:"type"`
			URL                  string `json:"url"`
			WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
		}
		if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&targets); err != nil {
			return nil, err
		}
		return targets, nil
	}

	targets, err := readTargets()
	if err != nil {
		return "", err
	}
	fallbackPageWS := ""
	for _, target := range targets {
		if target.Type != "page" || target.WebSocketDebuggerURL == "" {
			continue
		}
		if strings.HasPrefix(target.URL, "https://claude.ai") {
			return target.WebSocketDebuggerURL, nil
		}
		if fallbackPageWS == "" {
			// During Google/SSO authentication the dedicated Claude tab can
			// temporarily navigate away from claude.ai. Keep polling that same
			// page instead of spawning extra Claude tabs while login is in flight.
			fallbackPageWS = target.WebSocketDebuggerURL
		}
	}
	if fallbackPageWS != "" {
		return fallbackPageWS, nil
	}
	_, browserWS, ok := claudeWebActivePort(claudeWebProfileDirectory())
	if !ok {
		return "", errors.New("Claude Web browser DevTools endpoint is unavailable")
	}
	if _, err := cdpCall(ctx, browserWS, "Target.createTarget", map[string]any{"url": claudeWebURL}); err != nil {
		return "", err
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		targets, err = readTargets()
		if err == nil {
			for _, target := range targets {
				if target.Type == "page" && strings.HasPrefix(target.URL, "https://claude.ai") && target.WebSocketDebuggerURL != "" {
					return target.WebSocketDebuggerURL, nil
				}
			}
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return "", errors.New("Claude Web page target was not created")
}

type cdpResponse struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func cdpCall(ctx context.Context, websocketURL, method string, params map[string]any) (json.RawMessage, error) {
	conn, reader, err := openLocalWebSocket(ctx, websocketURL)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	request := map[string]any{"id": 1, "method": method, "params": params}
	encoded, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	if err := writeWebSocketFrame(conn, 0x1, encoded); err != nil {
		return nil, err
	}
	for {
		opcode, payload, err := readWebSocketMessage(reader, conn)
		if err != nil {
			return nil, err
		}
		switch opcode {
		case 0x8:
			return nil, errors.New("DevTools WebSocket closed before returning a response")
		case 0x1:
			var response cdpResponse
			if json.Unmarshal(payload, &response) != nil || response.ID != 1 {
				continue
			}
			if response.Error != nil {
				return nil, fmt.Errorf("DevTools %s failed (%d): %s", method, response.Error.Code, response.Error.Message)
			}
			return response.Result, nil
		}
	}
}

func openLocalWebSocket(ctx context.Context, rawURL string) (net.Conn, *bufio.Reader, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, nil, err
	}
	if parsed.Scheme != "ws" {
		return nil, nil, errors.New("only local ws:// DevTools endpoints are supported")
	}
	host := parsed.Hostname()
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return nil, nil, errors.New("refusing non-loopback DevTools WebSocket endpoint")
	}
	port := parsed.Port()
	if port == "" {
		port = "80"
	}
	address := net.JoinHostPort(host, port)
	conn, err := (&net.Dialer{Timeout: 3 * time.Second}).DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, nil, err
	}

	keyBytes := make([]byte, 16)
	if _, err := rand.Read(keyBytes); err != nil {
		_ = conn.Close()
		return nil, nil, err
	}
	key := base64.StdEncoding.EncodeToString(keyBytes)
	path := parsed.EscapedPath()
	if path == "" {
		path = "/"
	}
	if parsed.RawQuery != "" {
		path += "?" + parsed.RawQuery
	}
	request := fmt.Sprintf(
		"GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n",
		path, parsed.Host, key,
	)
	if _, err := io.WriteString(conn, request); err != nil {
		_ = conn.Close()
		return nil, nil, err
	}
	reader := bufio.NewReader(conn)
	httpRequest := &http.Request{Method: http.MethodGet}
	response, err := http.ReadResponse(reader, httpRequest)
	if err != nil {
		_ = conn.Close()
		return nil, nil, err
	}
	if response.StatusCode != http.StatusSwitchingProtocols {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("DevTools WebSocket upgrade returned HTTP %d", response.StatusCode)
	}
	hash := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	expected := base64.StdEncoding.EncodeToString(hash[:])
	if strings.TrimSpace(response.Header.Get("Sec-WebSocket-Accept")) != expected {
		_ = conn.Close()
		return nil, nil, errors.New("DevTools WebSocket handshake validation failed")
	}
	return conn, reader, nil
}

func writeWebSocketFrame(writer io.Writer, opcode byte, payload []byte) error {
	mask := make([]byte, 4)
	if _, err := rand.Read(mask); err != nil {
		return err
	}
	header := []byte{0x80 | (opcode & 0x0F)}
	length := len(payload)
	switch {
	case length < 126:
		header = append(header, 0x80|byte(length))
	case length <= 0xFFFF:
		header = append(header, 0x80|126, byte(length>>8), byte(length))
	default:
		header = append(header, 0x80|127)
		var buffer [8]byte
		binary.BigEndian.PutUint64(buffer[:], uint64(length))
		header = append(header, buffer[:]...)
	}
	header = append(header, mask...)
	masked := make([]byte, len(payload))
	for index := range payload {
		masked[index] = payload[index] ^ mask[index%4]
	}
	if _, err := writer.Write(header); err != nil {
		return err
	}
	_, err := writer.Write(masked)
	return err
}

func readWebSocketMessage(reader *bufio.Reader, writer io.Writer) (byte, []byte, error) {
	var messageOpcode byte
	message := make([]byte, 0, 4096)
	for {
		fin, opcode, payload, err := readWebSocketFrame(reader)
		if err != nil {
			return 0, nil, err
		}
		switch opcode {
		case 0x8:
			return opcode, payload, nil
		case 0x9:
			if writer != nil {
				if err := writeWebSocketFrame(writer, 0xA, payload); err != nil {
					return 0, nil, err
				}
			}
			continue
		case 0xA:
			continue
		case 0x1, 0x2:
			if messageOpcode != 0 {
				return 0, nil, errors.New("DevTools WebSocket started a new data message before the previous message finished")
			}
			messageOpcode = opcode
			message = append(message[:0], payload...)
		case 0x0:
			if messageOpcode == 0 {
				return 0, nil, errors.New("DevTools WebSocket returned an unexpected continuation frame")
			}
			if len(message)+len(payload) > 32<<20 {
				return 0, nil, errors.New("DevTools WebSocket message exceeded 32 MiB")
			}
			message = append(message, payload...)
		default:
			continue
		}
		if len(message) > 32<<20 {
			return 0, nil, errors.New("DevTools WebSocket message exceeded 32 MiB")
		}
		if fin && messageOpcode != 0 {
			return messageOpcode, message, nil
		}
	}
}

func readWebSocketFrame(reader *bufio.Reader) (bool, byte, []byte, error) {
	first, err := reader.ReadByte()
	if err != nil {
		return false, 0, nil, err
	}
	second, err := reader.ReadByte()
	if err != nil {
		return false, 0, nil, err
	}
	fin := first&0x80 != 0
	opcode := first & 0x0F
	masked := second&0x80 != 0
	length := uint64(second & 0x7F)
	switch length {
	case 126:
		var buffer [2]byte
		if _, err := io.ReadFull(reader, buffer[:]); err != nil {
			return false, 0, nil, err
		}
		length = uint64(binary.BigEndian.Uint16(buffer[:]))
	case 127:
		var buffer [8]byte
		if _, err := io.ReadFull(reader, buffer[:]); err != nil {
			return false, 0, nil, err
		}
		length = binary.BigEndian.Uint64(buffer[:])
	}
	if length > 32<<20 {
		return false, 0, nil, errors.New("DevTools WebSocket frame exceeded 32 MiB")
	}
	var mask [4]byte
	if masked {
		if _, err := io.ReadFull(reader, mask[:]); err != nil {
			return false, 0, nil, err
		}
	}
	payload := make([]byte, int(length))
	if _, err := io.ReadFull(reader, payload); err != nil {
		return false, 0, nil, err
	}
	if masked {
		for index := range payload {
			payload[index] ^= mask[index%4]
		}
	}
	return fin, opcode, payload, nil
}

func removeClaudeWebProfileWithRetry(ctx context.Context, profileDir string) error {
	deadline := time.Now().Add(3 * time.Second)
	var lastErr error
	for {
		if err := os.RemoveAll(profileDir); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("clear TL Studio Claude Web browser profile: %w", lastErr)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}
