# TL Studio

<p align="center">
  <img src="./media/tl-studio-logo.svg" width="360" alt="TL Studio">
</p>

TL Studio is a local-first browser IDE and coding-agent workspace. The product is one TL Studio application: workspace, editor, Search, Terminal, Preview, sessions, permissions, questions, providers, Plugins/MCP, and the Native Agent all run behind TL Studio-owned local contracts.

Current stable release: **0.5.0**

Development line: **0.6.0-alpha.1**

Stable main remains on v0.5.0 while new work continues on dev.

## Native architecture

TL Studio now runs normally without a compatibility sidecar.

- **Native Agent** — TL Studio owns the model/tool/model loop, cancellation, loop guards, semantic persistence, and live events.
- **Native sessions** — create, rename, delete, run, abort, status, messages, changes, and persistence are TL Studio-owned.
- **Native interactive questions** — an Agent can pause on interaction.question, expose a semantic pending question, receive multiple-choice or custom-text answers, and resume.
- **Native permissions** — pending approvals, one-time decisions, project-scoped remembered rules, rejection, and enforcement are owned by TL Studio.
- **Native events** — /local/events is the authoritative semantic SSE stream.
- **Native providers** — provider definitions, model catalogs, discovery, and credentials are owned by TL Studio. Model calls go directly from TL Studio to configured providers.
- **Provider accounts** — /local/provider-accounts owns documented account-login integrations, PKCE/login state, refresh/logout, account model discovery, and semantic account status without exposing tokens to the Browser.
- **Native tools** — files, Search, Terminal/process execution, workspace reconciliation, and Plugin/MCP tools run through the TL Studio Tool Executor.

There is no Kilo binary requirement, subprocess, local Kilo server, reverse proxy, session adapter, permission fallback, question adapter, event stream, provider synchronization, model fallback, or release payload.

Kilo may still be configured by a user as an ordinary external API provider when using a documented public HTTPS/API-key endpoint. That relationship is the same as any other external provider and does not introduce a local runtime dependency.

## Workspace

The Browser workspace includes:

- file Explorer with create, rename, delete, save, refresh, and optimistic concurrency protection;
- Monaco editor bundled locally, with no CDN dependency;
- project Search;
- project-scoped Terminal/process execution with stop support;
- Live Preview on a separate loopback origin;
- Agent conversation, sessions, usage/activity metadata, changes, permissions, and interactive questions;
- Provider settings and credential vault;
- Plugins/MCP with project/global configuration and discovered tools;
- JEV/OpenRouter integration through the native provider path.

## Provider support

TL Studio currently has direct model clients for OpenAI-compatible Chat Completions, OpenAI Responses, Anthropic Messages, and native Google Gemini GenerateContent.

Provider definitions live in TL Studio local state. API keys and account credentials are stored separately in the TL Studio credential vault and are never written to providers.json or Browser storage. Explicit API keys and account login use the same Provider/Model path from the Native Agent's point of view.

The 0.6 development line includes official account-auth adapters for Hugging Face and Google / Gemini. Hugging Face requires a registered public OAuth client ID. Gemini requires a registered Google Desktop OAuth client and a Google Cloud project with the Gemini API enabled; Gemini consumer subscriptions are not treated as API billing entitlement.

ChatGPT / Codex account support remains a required 0.6 product target, but TL Studio will not copy another application's OAuth client or undocumented ChatGPT backend. Claude remains available through its documented API configuration while consumer-account login is withheld until Anthropic exposes a suitable third-party client contract. GitHub Copilot OAuth is documented, but its current SDK/runtime boundary is not embedded beneath the TL Studio Native Agent.

A model or protocol that the native client cannot execute returns an explicit unsupported-capability error. It is never routed through a hidden compatibility runtime.

JEV remains an optional product-managed router on top of the normal OpenRouter-compatible provider path. It uses the same Native Agent, Tool Executor, permissions, sessions, and event system.

## Local product API

The Browser talks only to TL Studio local semantic endpoints, including:

- /local/status
- /local/health
- /local/path
- /local/agents
- /local/providers/*
- /local/provider-accounts*
- /local/sessions*
- /local/questions*
- /local/permissions*
- /local/events
- /local/plugins*
- /local/tools

The historical /runtime/* reverse-proxy architecture is not part of the product anymore.

## Build from source

Requirements:

- Go 1.23+
- Node.js 18+ for Browser development/build tooling

Build and check Browser assets:

    npm install --ignore-scripts --no-audit --no-fund
    npm run check:web
    npm run build:web

Run tests:

    go test ./...
    go vet ./...

Run TL Studio:

    go run ./cmd/launcher --project /path/to/project

Or build the executable:

    go build -o tl-studio ./cmd/launcher
    ./tl-studio --project /path/to/project

Windows:

    go build -o tl-studio.exe ./cmd/launcher
    .\tl-studio.exe --project C:\path\to\project

Normal execution is native. There is no special native-only flag and no runtime binary to install or select.

## Release packaging

Normal release packages contain the TL Studio executable, TL Studio licenses/notices, and bundled TL Studio plugins where configured. They do not contain kilo or kilo.exe.

CI has hard package assertions that fail if either binary appears in a review or release package. PR validation also starts the standalone TL Studio executable normally, exercises native product contracts, runs a real Browser smoke test, and produces a Windows x64 review ZIP.

## Security

TL Studio is local-first. The control UI binds to loopback only, cross-origin Browser requests are rejected, project filesystem APIs enforce project boundaries, Preview is isolated from the control origin, credentials stay in the TL Studio credential vault, and sensitive tool execution remains permission-gated.

Configured external model providers and MCP servers are separate trust boundaries. Review provider endpoints, prompts, repositories, plugin commands, and requested permissions before allowing sensitive actions.

See SECURITY.md.

## Repository branches

- main — stable production
- dev — active next-version development
- feature branches — isolated milestone work based on dev

Architecture details are in docs/ARCHITECTURE.md.

## License

TL Studio is MIT licensed. See LICENSE and THIRD_PARTY_NOTICES.md for software that is still distributed with TL Studio.
