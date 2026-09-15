[English](./README.md) | [فارسی](./README.fa_IR.md)

<p align="center">
  <img src="./media/tunnellab-logo.jpg" width="300" alt="TunnelLab">
</p>

<h1 align="center">TL Agent</h1>

<p align="center">
  A local, standalone coding-agent workspace powered by Kilo Code — no IDE required.
</p>

<p align="center">
  <a href="https://github.com/pouramin/TL-Agent/releases"><img src="https://img.shields.io/github/v/release/pouramin/TL-Agent?include_prereleases&sort=semver" alt="Release"></a>
  <a href="https://github.com/pouramin/TL-Agent/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/pouramin/TL-Agent/ci.yml?branch=main&label=CI" alt="CI"></a>
  <a href="https://github.com/pouramin/TL-Agent/releases"><img src="https://img.shields.io/github/downloads/pouramin/TL-Agent/total" alt="Downloads"></a>
  <a href="./LICENSE"><img src="https://img.shields.io/github/license/pouramin/TL-Agent" alt="License"></a>
  <a href="https://github.com/Kilo-Org/kilocode"><img src="https://img.shields.io/badge/runtime-Kilo%20Code-c9ff62" alt="Kilo Code"></a>
</p>

**TL Agent** runs the Kilo coding-agent runtime on your own computer and gives it a dedicated browser workspace. You can work with local projects, coding agents, models, tools, permissions, sessions, file changes, and project files without opening VS Code, JetBrains, Cursor, Docker, or a hosted TL Agent backend.

> [!IMPORTANT]
> TL Agent is an independent open-source project. It uses Kilo Code as its local agent runtime but is not an official Kilo Code product.

## Quick Start

### One-command launch

If Node.js/npm is already installed, launch TL Agent from the project directory you want to work on:

```bash
npx --yes github:pouramin/TL-Agent
```

The lightweight npx launcher detects your OS and architecture, downloads the matching TL Agent release from GitHub, verifies its SHA-256 checksum, caches it locally, and starts TL Agent with the current directory as the project.

### Portable release

No Node.js is required for the normal portable build. Download the archive for your platform from **[GitHub Releases](https://github.com/pouramin/TL-Agent/releases)**, extract it, and run:

```text
Windows:  tl-agent.exe
Linux:    ./tl-agent
macOS:    ./tl-agent
```

Release archives already bundle the pinned Kilo runtime, so users do not need to install Kilo separately.

## Features

- **Standalone local workspace** — coding-agent UI in your browser without an IDE.
- **Production Kilo coding API** — uses the same production coding path used by the official Kilo client.
- **Local project picker** — choose a project folder directly from the operating-system folder picker.
- **Code agent + model selection** — switch agents and available Kilo/provider models from the composer.
- **Kilo account flow** — device sign-in and sign-out for Kilo-hosted models.
- **Live agent activity** — SSE updates with compact Reasoning and Tool cards.
- **Permissions & questions** — handle `once`, `always`, `reject`, and interactive agent questions.
- **Stop / Abort** — interrupt an active run from the UI.
- **Session management** — create, rename, and delete sessions per project.
- **Changes panel** — inspect changed files, addition/deletion counts, and patches.
- **Project file explorer** — read-only local file navigation and preview inside TL Agent.
- **Appearance settings** — System, Dark, and Light themes plus interface font-size controls.
- **Local-first security model** — loopback-only server, random per-run backend password, origin checks, and restrictive CSP.
- **No TL Agent telemetry or cloud service** — model traffic goes to the provider configured in Kilo, not through TL Agent infrastructure.

## How it works

```text
Browser UI
    │ localhost only
    ▼
TL Agent launcher (Go)
    │ authenticated local reverse proxy
    ▼
kilo serve
    │
    ├─ code agent / sessions / tools
    ├─ permissions / questions / SSE events
    ├─ project files / terminal commands
    └─ configured AI providers
```

TL Agent deliberately keeps the Kilo runtime separate from its own UI/launcher layer. The selected project stays on the user's computer, and TL Agent does not proxy model traffic through project-owned infrastructure.

## Supported builds

| Platform | Architecture |
| --- | --- |
| Windows | x64 |
| Linux | x64, ARM64 |
| macOS | Intel x64, Apple Silicon ARM64 |

The Kilo runtime version is pinned in [`KILO_VERSION`](./KILO_VERSION).

## Release package

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

## Runtime contract & testing

TL Agent is pinned to a known Kilo runtime contract rather than guessing response shapes at runtime. The current compatibility target is documented in [`docs/KILO_API_CONTRACT.md`](./docs/KILO_API_CONTRACT.md).

CI downloads the real pinned Kilo binary and validates:

- project routing,
- production agent/provider/session endpoints,
- the `code` agent,
- async prompt execution,
- SSE events,
- permission handling,
- a real Kilo write-tool flow against a local fake LLM,
- and actual file creation on disk.

## Zero-infrastructure rule

TL Agent is intentionally designed so the maintainer does not need to pay for a VPS, application hosting, database, API gateway, model inference, or telemetry backend. Source, issues, CI, release definitions, and downloadable builds live on GitHub.

Any paid AI usage is between the user and the provider configured in Kilo.

## Security model

The launcher:

1. binds the UI to loopback only (`127.0.0.1`, `localhost`, or `::1`),
2. starts Kilo on loopback with a random per-run password,
3. keeps that password server-side,
4. routes the selected project directory locally,
5. rejects cross-origin browser requests, and
6. serves the UI with a restrictive Content Security Policy.

Kilo is a coding agent that can read/write files and execute commands when permissions allow it. Only run TL Agent on projects and machines you trust.

## Status

TL Agent is currently an **early alpha**. The core path has been validated in CI and on a real Windows machine:

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

## License & attribution

TL Agent launcher/UI code is MIT licensed. Kilo Code is also MIT licensed and remains a separate upstream project. Release archives that bundle Kilo retain its license notice; see [`THIRD_PARTY_NOTICES.md`](./THIRD_PARTY_NOTICES.md).

Built under the **TunnelLab** identity.
