# Architecture

## Components

### Launcher

A single Go binary using only the Go standard library. It owns the lifecycle of the Kilo child process, chooses ephemeral ports, serves embedded static UI assets, and reverse-proxies `/kilo/*` to the authenticated Kilo server.

### Kilo runtime

The launcher executes:

```text
kilo serve --hostname 127.0.0.1 --port <ephemeral-port>
```

with a random `KILO_SERVER_PASSWORD`. The runtime remains responsible for sessions, agents, tools, filesystem access, provider integrations, and model execution.

### Browser UI

Static HTML/CSS/JavaScript is embedded in the launcher at build time. The browser only talks to the launcher origin. It never receives the Kilo server password.

## Request flow

```text
Browser
  │  GET/POST http://127.0.0.1:<ui-port>/kilo/...
  ▼
Launcher reverse proxy
  │  strips /kilo
  │  injects Basic Auth
  │  injects x-kilo-directory
  ▼
Kilo HTTP API on 127.0.0.1:<backend-port>
```

## Session MVP

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

The first MVP polls active session state. A later version can consume Kilo's event stream/SSE for lower-latency incremental updates.

## No cloud control plane

There is intentionally no application server belonging to this project. The only external requests are made by Kilo or the browser for services the user explicitly configures (for example an AI provider or Kilo login), and optional future update checks against GitHub Releases.
