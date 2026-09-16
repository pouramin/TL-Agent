# Architecture

## Components

### Launcher

A single Go binary using only the Go standard library. It owns the lifecycle of the Kilo child process, chooses ephemeral ports, serves embedded static UI assets, exposes the project-scoped local workspace API, and reverse-proxies `/kilo/*` to the authenticated Kilo server.

The launcher is also the filesystem trust boundary for browser IDE operations. Browser requests never receive arbitrary host filesystem access: file reads and mutations are resolved relative to the selected project, traversal and symlink escapes are rejected, and workspace mutations protect Git metadata.

### Kilo runtime

The launcher executes:

```text
kilo serve --hostname 127.0.0.1 --port <ephemeral-port>
```

with a random `KILO_SERVER_PASSWORD`. The runtime remains responsible for sessions, agents, tools, provider integrations, model execution, and filesystem operations initiated by the agent.

Kilo is behind TL Agent's product boundary. The browser never connects to it directly and never receives its server password.

### Browser UI

Static HTML/CSS/JavaScript is embedded in the launcher at build time. The browser only talks to the launcher origin.

The browser IDE foundation keeps one in-memory buffer per open editor tab. User-initiated reads, saves, creates, renames, and deletes go through launcher-owned `/local/*` routes. File previews include a SHA-256 revision token; normal saves send that token back so an external edit by the agent, Git, or another process cannot be silently overwritten. A deliberate force-save is a separate explicit action after a conflict.

Runtime file/session events trigger workspace reconciliation. Clean open buffers follow disk changes automatically, while dirty buffers are preserved and marked when the disk version changes or disappears.

## Request flow

```text
Browser
  │
  ├── /local/*  ───────────────► launcher project filesystem boundary
  │
  └── /kilo/*
          │
          ▼
      launcher reverse proxy
          │  strips /kilo
          │  injects Basic Auth
          │  injects x-kilo-directory
          ▼
      Kilo HTTP API on 127.0.0.1:<backend-port>
```

## Session and agent surface

The UI uses Kilo's current APIs for:

- sessions
- messages/prompts
- active-session state
- agent switching
- model switching
- provider/model discovery
- Kilo provider OAuth/device authorization
- session permissions
- session questions
- event/SSE-driven progress and file-change reconciliation

## Editor asset strategy

The browser IDE foundation intentionally remains dependency-free at runtime and keeps the existing direct embedded-static-assets build. It does not load editor code, fonts, workers, or other assets from a CDN.

The first foundation milestone uses TL Agent's embedded editor surface so the filesystem, tab, save, dirty-state, and conflict contracts can land without introducing a separate frontend supply chain. Monaco can be added later only as a locally bundled/vendored editor implementation, with its workers and assets shipped inside the same application and without weakening the Content Security Policy or local-first boundary.

## Local-only security boundary

The public UI binds to loopback only. Requests are rejected when the Host is not loopback, and browser requests with an Origin must match the same local origin. The Kilo child process also binds to `127.0.0.1` and is protected with a random per-launch password known only to the launcher.

Project file mutation routes reject paths outside the selected project, project-root mutation, symlink-parent escapes, and protected Git metadata. Direct symlink writes are not treated as editable regular files.

## No cloud control plane

There is intentionally no application server belonging to this project. The only external requests are made by Kilo or the browser for services the user explicitly configures (for example an AI provider or Kilo login), and optional future update checks against GitHub Releases.
