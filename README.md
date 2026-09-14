# Kilo Local UI

> Working title. A zero-infrastructure, local browser interface for [Kilo Code](https://github.com/Kilo-Org/kilocode).

Kilo Local UI launches the Kilo coding-agent runtime on your own computer, exposes it only on loopback, and opens a standalone browser interface. You do **not** need VS Code, JetBrains, Cursor, Docker, a hosted backend, a database, or a server owned by this project.

## Why

Kilo already provides the hard part: the local agent runtime, tools, sessions, model/provider integrations, MCP, and a headless HTTP API through `kilo serve`. This project adds a small local launcher and a browser GUI on top of that runtime.

```text
Browser UI
    │
    │ localhost only
    ▼
Kilo Local UI launcher (Go)
    │
    │ authenticated reverse proxy
    ▼
kilo serve
    │
    ├─ agents / sessions / tools
    ├─ project files / terminal commands
    └─ configured AI providers
```

## Current MVP

- Runs independently of any IDE.
- Starts `kilo serve` automatically on a random loopback port.
- Generates a random Kilo server password on every launch.
- Keeps Kilo credentials out of browser JavaScript by proxying requests through the launcher.
- Opens a local browser UI automatically.
- Lets you choose a local project folder.
- Lists and creates Kilo sessions.
- Sends prompts and displays user/assistant/tool activity.
- Switches Kilo agents and models.
- Supports Kilo device-login from the browser UI.
- Handles Kilo permission requests (`once`, `always`, `reject`).
- Handles Kilo question requests.
- Blocks non-loopback binding and cross-origin browser requests.
- Has no project telemetry and no project-owned cloud service.

The UI currently polls session state instead of using Kilo SSE. Streaming events, richer diff/file views, terminal tabs, MCP management, updater UX, and more polished provider management are planned follow-ups.

## Run from source

Requirements for development:

- Go 1.23+
- A Kilo binary, either in `PATH` or at `./bin/kilo` (`bin\\kilo.exe` on Windows)

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

The launcher prints the local URL and opens it automatically. Use `--no-browser` to suppress automatic browser launch.

## Release packages

Release builds are designed to bundle the official Kilo binary next to the launcher:

```text
kilo-local-ui/
├─ kilo-local-ui[.exe]
├─ bin/
│  └─ kilo[.exe]
├─ LICENSE
└─ THIRD_PARTY_NOTICES.md
```

The user downloads one archive and runs the launcher. No separate Kilo, Node.js, npm, IDE, Docker, or local database installation is required.

The release workflow pins the Kilo version in [`KILO_VERSION`](./KILO_VERSION), downloads the corresponding official Kilo release asset, runs tests, cross-compiles the launcher, and packages Windows, Linux, and macOS archives. It only publishes when a `v*` tag is intentionally pushed.

## Zero-infrastructure rule

This project is intentionally designed so the maintainer does not need to pay for:

- VPS or application hosting
- a domain
- a database
- an API gateway
- model inference on behalf of users
- project telemetry infrastructure

Source, issues, CI/release definitions, and downloadable builds live on GitHub. Runtime state and project access stay on the user's machine. Any paid AI model usage is between the user and the provider configured in Kilo.

## Security model

The launcher:

1. binds its UI only to loopback (`127.0.0.1`, `localhost`, or `::1`),
2. starts Kilo on `127.0.0.1` with a random per-run password,
3. keeps that password server-side,
4. injects the selected project directory only when proxying to Kilo,
5. rejects cross-origin browser requests, and
6. serves the UI with a restrictive Content Security Policy.

Kilo is a coding agent that can read/write files and execute commands when permissions allow it. Only run it on projects and machines you trust.

## Status

Early alpha / MVP. The code compiles on Windows x64, Linux x64/ARM64, and macOS Intel/Apple Silicon. Unit/integration tests cover the local proxy boundary. A real Kilo binary is bundled by the release workflow; local development can use an installed Kilo binary.

## License and attribution

The launcher/UI code in this repository is MIT licensed. Kilo Code is also MIT licensed and remains a separate upstream project. Release archives that bundle Kilo must retain its license notice; see [`THIRD_PARTY_NOTICES.md`](./THIRD_PARTY_NOTICES.md).

This project is independent and is not presented as an official Kilo Code product.
