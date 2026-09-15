# TL Agent

> A local, standalone coding-agent workspace powered by [Kilo Code](https://github.com/Kilo-Org/kilocode).

TL Agent runs the Kilo coding-agent runtime on your own computer and gives it a dedicated browser interface, without requiring VS Code, JetBrains, Cursor, Docker, a hosted backend, or a project-owned server.

```text
Browser UI
    │ localhost only
    ▼
TL Agent launcher (Go)
    │ authenticated reverse proxy
    ▼
kilo serve
    │
    ├─ code agent / sessions / tools
    ├─ project files / terminal commands
    └─ configured AI providers
```

## What works today

- Standalone local browser workspace; no IDE required.
- Bundles and starts the pinned Kilo runtime automatically.
- Uses the same production Kilo HTTP API path used by the official Kilo client for coding.
- Project-folder selection with per-project session routing.
- Kilo device login and provider/model selection.
- Production `code` agent flow using `prompt_async`.
- Live session events over Kilo SSE, with low-frequency reconciliation only as a fallback/watchdog.
- Compact Reasoning and Tool activity cards.
- Permission requests with `once`, `always`, and `reject`.
- Kilo question prompts.
- Session changed-files panel with per-file diff stats and patch previews.
- Stop/Abort for an active coding run.
- Session rename and delete controls.
- Random per-run backend password kept out of browser JavaScript.
- Loopback-only binding and cross-origin protection.
- No TL Agent telemetry and no TL Agent cloud service.

## Why

Kilo already provides the difficult runtime pieces: the coding agent, tools, sessions, provider integrations, MCP support, project filesystem access, and headless server. TL Agent adds a focused desktop-like browser workspace and a small launcher around that runtime.

The project deliberately does **not** proxy model traffic through infrastructure owned by TL Agent. Model traffic is handled by the provider configured in Kilo.

## Download

Release packages bundle the launcher and the official pinned Kilo binary:

```text
tl-agent/
├─ tl-agent[.exe]
├─ bin/
│  └─ kilo[.exe]
├─ LICENSE
├─ THIRD_PARTY_NOTICES.md
└─ third_party/
   └─ KILO_LICENSE.txt
```

Download one archive, extract it, and run `tl-agent` (`tl-agent.exe` on Windows). No separate Kilo, Node.js, npm, IDE, Docker, or database installation is required.

Current release builds target:

- Windows x64
- Linux x64
- Linux ARM64
- macOS Intel
- macOS Apple Silicon

The Kilo version used by TL Agent is pinned in [`KILO_VERSION`](./KILO_VERSION).

## Run from source

Development requirements:

- Go 1.23+
- A Kilo binary in `PATH`, beside the launcher in `./bin/kilo`, or supplied with `--kilo`

```bash
go run ./cmd/launcher
```

Open a specific project:

```bash
go run ./cmd/launcher --project /path/to/project
```

Use a specific Kilo binary:

```bash
go run ./cmd/launcher --kilo /path/to/kilo
```

Use `--no-browser` to suppress automatic browser launch.

## Runtime contract

TL Agent is intentionally pinned to a known Kilo runtime contract instead of guessing response shapes at runtime. The current compatibility target is documented in [`docs/KILO_API_CONTRACT.md`](./docs/KILO_API_CONTRACT.md).

CI downloads the real pinned Kilo binary and gates changes against:

- project routing,
- production agent/provider/session endpoints,
- the `code` agent,
- async prompt execution,
- SSE events,
- permission handling,
- a real Kilo write-tool flow against a local fake LLM,
- and actual file creation on disk.

## Zero-infrastructure rule

TL Agent is designed so the maintainer does not need to pay for:

- VPS or application hosting
- a domain
- a database
- an API gateway
- model inference on behalf of users
- telemetry infrastructure

Source, issues, CI/release definitions, and downloadable builds live on GitHub. Runtime state and project access stay on the user's machine. Any paid AI usage is between the user and the provider configured in Kilo.

## Security model

The launcher:

1. binds its UI only to loopback (`127.0.0.1`, `localhost`, or `::1`),
2. starts Kilo on `127.0.0.1` with a random per-run password,
3. keeps that password server-side,
4. routes the selected project directory to Kilo locally,
5. rejects cross-origin browser requests, and
6. serves the UI with a restrictive Content Security Policy.

Kilo is a coding agent that can read/write files and execute commands when permissions allow it. Only run TL Agent on projects and machines you trust.

## Status

Early alpha. The core MVP has been validated both in CI and on a real Windows machine through the complete path:

```text
TL Agent UI
→ Kilo production coding API
→ code agent
→ model
→ tool call
→ permission
→ local file write
→ final assistant response
```

The next work is product polish rather than proving the core architecture.

## License and attribution

TL Agent launcher/UI code is MIT licensed. Kilo Code is also MIT licensed and remains a separate upstream project. Release archives that bundle Kilo retain its license notice; see [`THIRD_PARTY_NOTICES.md`](./THIRD_PARTY_NOTICES.md).

TL Agent is an independent project and is not presented as an official Kilo Code product.
