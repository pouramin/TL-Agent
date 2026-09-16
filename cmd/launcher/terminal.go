package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

const terminalOutputLimit = 1 << 20

type processManager struct {
	mu        sync.RWMutex
	projectFn func() string
	processes map[string]*managedProcess
}

type managedProcess struct {
	mu        sync.RWMutex
	id        string
	command   string
	cwd       string
	startedAt time.Time
	endedAt   *time.Time
	exitCode  *int
	running   bool
	output    []byte
	cmd       *exec.Cmd
}

type processSnapshot struct {
	ID        string     `json:"id"`
	Command   string     `json:"command"`
	CWD       string     `json:"cwd"`
	StartedAt time.Time  `json:"startedAt"`
	EndedAt   *time.Time `json:"endedAt,omitempty"`
	ExitCode  *int       `json:"exitCode,omitempty"`
	Running   bool       `json:"running"`
	Output    string     `json:"output"`
}

func newProcessManager(projectFn func() string) *processManager {
	return &processManager{projectFn: projectFn, processes: make(map[string]*managedProcess)}
}

func (m *processManager) start(command string) (*managedProcess, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil, errors.New("command is required")
	}
	cwd := m.projectFn()
	if cwd == "" {
		return nil, errors.New("open a project before running commands")
	}
	cmd := shellCommand(command)
	configureManagedCommand(cmd)
	cmd.Dir = cwd
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("capture stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("capture stderr: %w", err)
	}
	id, err := randomSecret(8)
	if err != nil {
		return nil, err
	}
	p := &managedProcess{id: id, command: command, cwd: cwd, startedAt: time.Now().UTC(), running: true, cmd: cmd}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start command: %w", err)
	}
	m.mu.Lock()
	m.processes[id] = p
	m.mu.Unlock()
	go p.copyOutput(stdout)
	go p.copyOutput(stderr)
	go p.wait()
	return p, nil
}

func shellCommand(command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("cmd.exe", "/d", "/s", "/c", command)
	}
	return exec.Command("/bin/sh", "-lc", command)
}

func (p *managedProcess) copyOutput(reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	buffer := make([]byte, 0, 64*1024)
	scanner.Buffer(buffer, 1024*1024)
	for scanner.Scan() {
		p.appendOutput(scanner.Text() + "\n")
	}
	if err := scanner.Err(); err != nil {
		p.appendOutput("[TL Agent] output error: " + err.Error() + "\n")
	}
}

func (p *managedProcess) appendOutput(text string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.output = append(p.output, text...)
	if len(p.output) > terminalOutputLimit {
		p.output = append([]byte(nil), p.output[len(p.output)-terminalOutputLimit:]...)
	}
}

func (p *managedProcess) wait() {
	err := p.cmd.Wait()
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		} else {
			code = -1
			p.appendOutput("[TL Agent] process error: " + err.Error() + "\n")
		}
	}
	now := time.Now().UTC()
	p.mu.Lock()
	p.running = false
	p.exitCode = &code
	p.endedAt = &now
	p.mu.Unlock()
}

func (p *managedProcess) snapshot() processSnapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return processSnapshot{ID: p.id, Command: p.command, CWD: p.cwd, StartedAt: p.startedAt, EndedAt: p.endedAt, ExitCode: p.exitCode, Running: p.running, Output: string(p.output)}
}

func (m *processManager) get(id string) (*managedProcess, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.processes[id]
	return p, ok
}

func (m *processManager) stop(id string) error {
	p, ok := m.get(id)
	if !ok {
		return errors.New("process not found")
	}
	p.mu.RLock()
	running := p.running
	cmd := p.cmd
	p.mu.RUnlock()
	if running {
		if err := terminateManagedProcess(cmd); err != nil {
			return fmt.Errorf("stop process: %w", err)
		}
	}
	return nil
}

func registerLocalProcessRoutes(mux *http.ServeMux, state *appState) {
	manager := newProcessManager(state.projectPath)
	mux.HandleFunc("POST /local/process", func(w http.ResponseWriter, r *http.Request) {
		var body struct { Command string `json:"command"` }
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
		if err := decoder.Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, jsonError{Error: "invalid JSON body"})
			return
		}
		process, err := manager.start(body.Command)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, jsonError{Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, process.snapshot())
	})
	mux.HandleFunc("GET /local/process/{id}", func(w http.ResponseWriter, r *http.Request) {
		process, ok := manager.get(r.PathValue("id"))
		if !ok {
			writeJSON(w, http.StatusNotFound, jsonError{Error: "process not found"})
			return
		}
		writeJSON(w, http.StatusOK, process.snapshot())
	})
	mux.HandleFunc("DELETE /local/process/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := manager.stop(r.PathValue("id")); err != nil {
			writeJSON(w, http.StatusNotFound, jsonError{Error: err.Error()})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
