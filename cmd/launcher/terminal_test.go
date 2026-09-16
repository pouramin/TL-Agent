package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"
)

func terminalTestCommand() string {
	if runtime.GOOS == "windows" {
		return "echo terminal-ok"
	}
	return "printf terminal-ok"
}

func TestLocalProcessRoutesRunAndReadOutput(t *testing.T) {
	project := t.TempDir()
	state := &appState{project: project}
	mux := http.NewServeMux()
	registerLocalProcessRoutes(mux, state)
	server := httptest.NewServer(mux)
	defer server.Close()

	body := `{"command":` + strconvQuote(terminalTestCommand()) + `}`
	response, err := http.Post(server.URL+"/local/process", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		data, _ := io.ReadAll(response.Body)
		t.Fatalf("start status=%d body=%s", response.StatusCode, data)
	}
	var started processSnapshot
	if err := json.NewDecoder(response.Body).Decode(&started); err != nil {
		t.Fatal(err)
	}
	if started.ID == "" {
		t.Fatal("missing process id")
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		res, err := http.Get(server.URL + "/local/process/" + started.ID)
		if err != nil {
			t.Fatal(err)
		}
		var snap processSnapshot
		err = json.NewDecoder(res.Body).Decode(&snap)
		res.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if !snap.Running {
			if !strings.Contains(snap.Output, "terminal-ok") {
				t.Fatalf("output=%q", snap.Output)
			}
			if snap.ExitCode == nil || *snap.ExitCode != 0 {
				t.Fatalf("exit code=%v", snap.ExitCode)
			}
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("process did not finish")
}

func TestLocalProcessRoutesRequireProjectAndCommand(t *testing.T) {
	state := &appState{}
	mux := http.NewServeMux()
	registerLocalProcessRoutes(mux, state)
	server := httptest.NewServer(mux)
	defer server.Close()

	for _, body := range []string{`{"command":""}`, `{"command":"echo no-project"}`} {
		response, err := http.Post(server.URL+"/local/process", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusBadRequest {
			t.Fatalf("status=%d for %s", response.StatusCode, body)
		}
	}
}

func TestLocalProcessUnknownID(t *testing.T) {
	state := &appState{project: t.TempDir()}
	mux := http.NewServeMux()
	registerLocalProcessRoutes(mux, state)
	server := httptest.NewServer(mux)
	defer server.Close()

	response, err := http.Get(server.URL + "/local/process/missing")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d", response.StatusCode)
	}
}

func strconvQuote(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
