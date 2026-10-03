package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"strings"
)

func testServerWebSocketFrame(fin bool, opcode byte, payload []byte) []byte {
	first := opcode & 0x0F
	if fin {
		first |= 0x80
	}
	frame := []byte{first}
	switch {
	case len(payload) < 126:
		frame = append(frame, byte(len(payload)))
	case len(payload) <= 0xFFFF:
		frame = append(frame, 126, byte(len(payload)>>8), byte(len(payload)))
	default:
		frame = append(frame, 127)
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(payload)))
		frame = append(frame, size[:]...)
	}
	return append(frame, payload...)
}

func TestClaudeWebSocketMessageReassemblesFragmentsAcrossPing(t *testing.T) {
	var wire bytes.Buffer
	wire.Write(testServerWebSocketFrame(false, 0x1, []byte(`{"id":1,"res`)))
	wire.Write(testServerWebSocketFrame(true, 0x9, []byte("ping")))
	wire.Write(testServerWebSocketFrame(true, 0x0, []byte(`ult":{"ok":true}}`)))

	var pong bytes.Buffer
	opcode, payload, err := readWebSocketMessage(bufio.NewReader(bytes.NewReader(wire.Bytes())), &pong)
	if err != nil {
		t.Fatal(err)
	}
	if opcode != 0x1 {
		t.Fatalf("unexpected reconstructed WebSocket opcode %#x", opcode)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("fragmented message was not reconstructed as JSON: %s (%v)", payload, err)
	}
	if decoded["id"] != float64(1) {
		t.Fatalf("unexpected reconstructed payload: %#v", decoded)
	}
	if pong.Len() == 0 {
		t.Fatal("WebSocket ping was not answered while reconstructing fragmented message")
	}
}

func TestClaudeWebPageTargetTreatsExternalLoginNavigationAsInProgress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/json/list" {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusOK, []map[string]any{{
			"type":                 "page",
			"url":                  "https://accounts.google.com/signin",
			"webSocketDebuggerUrl": "ws://127.0.0.1:54321/devtools/page/google-login",
		}})
	}))
	defer server.Close()

	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := claudeWebPageWebSocket(context.Background(), port, t.TempDir()); !errors.Is(err, errClaudeWebPageNotReady) {
		t.Fatalf("external-login navigation must remain in-progress, got %v", err)
	}
}

func TestClaudeWebTransientBrowserErrorsIncludeDestroyedExecutionContext(t *testing.T) {
	for _, err := range []error{
		errClaudeWebPageNotReady,
		errors.New("DevTools Runtime.evaluate failed (-32000): Cannot find default execution context"),
		errors.New("Execution context was destroyed, most likely because of a navigation"),
		errors.New("Inspected target navigated or closed"),
	} {
		if !isClaudeWebTransientBrowserError(err) {
			t.Fatalf("expected transient Claude Web browser error: %v", err)
		}
	}
	if isClaudeWebTransientBrowserError(errors.New("permission denied")) {
		t.Fatal("unrelated Claude Web browser errors must not be hidden as transient")
	}
}


func TestClaudeWebBrowserTransportUsesNonDefaultCloneProfile(t *testing.T) {
	source := readRepoText(t, "cmd/launcher/claude_web_browser.go")
	for _, required := range []string{
		`--user-data-dir=`,
		`--remote-debugging-address=127.0.0.1`,
		`--remote-debugging-port=0`,
		`--headless=new`,
		`--profile-directory=`,
		`credentials: "include"`,
		`/api/organizations`,
		`/completion`,
		`Browser.close`,
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("Claude Web CDP clone transport missing %q", required)
		}
	}
}
