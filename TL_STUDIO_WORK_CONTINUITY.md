# TL Studio work continuity

Last updated: 2026-09-28

## Source of truth

The actual repository state on dev is the source of truth. Before substantial work, inspect current dev and reconcile this file with README.md, README.fa_IR.md, docs/ARCHITECTURE.md, VERSION, open PRs, and CI.

Stable main contains only validated stable releases. Ongoing development continues on dev.

## Current milestone

Development version:

0.6.0-alpha.1

Stable base:

v0.5.0 on main

Development status:

The fully native runtime milestone is complete and released as stable v0.5.0. The active 0.6 milestone is account-based providers and simplified custom-provider setup.

Current validation state:

Real Windows provider-account validation is in progress on current dev.

Current open development PR:

#143 — Enable ChatGPT account login through official Codex (in progress).

Completed 0.6 account-provider / validation UX PRs:

#130 — Compact provider account settings into logo cards
#129 — Document the 0.6 real account validation gate
#128 — Use a dedicated Gemini desktop OAuth loopback
#127 — Add in-app setup for account providers
#126 — Keep custom provider setup discovery-first
#125 — Expose deferred account-provider boundaries
#124 — Google Gemini account provider for 0.6
#123 — Hugging Face account provider for 0.6
#122 — Account provider foundation for 0.6

Completed milestone PR:

#115 — Remove the Kilo runtime dependency completely

Stable promotion rule:

Promote dev to main only after the next milestone is fully validated and its version has been finalized as stable. Stable main must not carry alpha or beta version labels.

## Architecture state

TL Studio core execution is fully native.

Owned by TL Studio:

- Workspace and Monaco editor
- Files and Search
- Terminal/process foundation
- Preview
- Plugin/MCP system
- Provider registry and discovery
- Credential vault
- Custom Providers
- Direct native model clients
- Native Agent loop
- Native Tool Registry and Tool Executor
- Native Permissions
- Native semantic Session persistence and execution
- Native interactive Questions
- Native local Event Bus
- generic Provider Account domain
- JEV/OpenRouter integration

Normal startup initializes TL Studio services and the loopback local server directly.

Optional provider/plugin integrations may launch their own explicit helper processes only when that integration is used. The ChatGPT account path is provider-specific and uses the official OpenAI Codex CLI; it does not replace TL Studio Session, Permission, Tool, persistence, or Native Agent ownership.

Native execution is the normal core execution mode.

## Session architecture

Browser
→ /local/sessions*
→ TL Studio Session domain
→ Native Agent
→ selected Provider

Session create, rename, delete, run, abort, status, transcript, activity, changes, and persistence are TL Studio-owned.

## Question architecture

Native Agent
→ interaction.question
→ TL Studio Question Manager
→ /local/questions
→ Browser
→ reply/reject
→ Question Manager
→ Native Agent resumes

The Question Manager supports multiple choice, custom text, rejection, session scoping, cancellation, and semantic attention events.

## Permissions and events

Permissions are fully TL Studio-owned. Pending requests, one-time approvals, remembered project rules, rejection, and enforcement use /local/permissions*.

The local Event Bus is authoritative:

Native Agent / Tool / Session / Question / Permission / Workspace
→ local event bus
→ /local/events
→ Browser

There is no external runtime event stream.

## Providers

Provider configuration and model catalogs are authoritative in TL Studio.

Direct native model protocols currently include OpenAI-compatible Chat Completions, OpenAI Responses, Anthropic Messages, and Google Gemini generateContent. ChatGPT-plan access uses the explicit `codex-chatgpt` provider bridge. Unsupported capabilities return explicit errors.

The generic ProviderAccountAdapter architecture now has a typed account lifecycle: begin login, OAuth callback completion or polling, cancellation, refresh, status, model discovery, and logout. Browser state is semantic only and does not carry provider access tokens, refresh tokens, authorization codes, PKCE verifiers, API keys, cookies, or client secrets.

Manual API credentials and TL Studio-managed account credentials use separate credential-vault slots. An account credential takes precedence for native model execution while connected, but signing out exposes the preserved manual API credential instead of deleting it.

ChatGPT is the exception to token persistence ownership: OpenAI's official Codex client manages ChatGPT OAuth/refresh material inside an isolated TL Studio-specific `CODEX_HOME`. TL Studio never reads or serializes the raw ChatGPT tokens.

OpenRouter is the first concrete 0.6 account adapter. It uses the documented OpenRouter OAuth + PKCE flow, performs code exchange server-side, stores the resulting account key only in the TL Studio credential vault, discovers models through the normal provider discovery layer, and exposes those models through the existing Native Agent/model selector path.

Hugging Face is merged on dev through PR #123. It uses TL Studio's public CIMD identity at https://pouramin.dev/.well-known/oauth-cimd, Authorization Code + PKCE, loopback callbacks, account-vault access/refresh token persistence, automatic refresh in the Provider Account adapter, and the official OpenAI-compatible Inference Providers router. The provider-specific credential lifecycle remains behind ProviderAccountAdapter; the Native Agent receives only the resolved runtime credential.

Google/Gemini is merged on dev through PR #124. The implementation uses the official installed-app OAuth flow with PKCE, refresh and revocation, a Google Cloud quota project, native Gemini generateContent transport, model discovery, and preservation of Gemini thought signatures across tool-call continuation. No client secret is embedded. PR #127 adds a generic non-secret Provider Account setup surface so the 0.6 alpha can persist the Desktop OAuth Client ID and Google Cloud Project ID from Provider Settings; TL_STUDIO_GOOGLE_CLIENT_ID and TL_STUDIO_GOOGLE_PROJECT_ID remain development overrides. OAuth access/refresh tokens still live only in the credential vault. PR #128 is merged on dev. Each Google OAuth login now uses a dedicated temporary 127.0.0.1 listener on a random port so redirect_uri follows Google's documented Desktop loopback form instead of using a path on TL Studio's control server.

Custom Provider setup is simplified so a new provider normally requires only an API address, OpenAI-compatible or Anthropic-compatible protocol choice, and credential. Provider IDs are generated internally and model discovery runs automatically on save; manual model entry remains an explicit fallback. PR #126 fixed edit-mode behavior so hidden legacy Model ID fields cannot silently bypass a fresh discovery run when connection settings change.

PR #143 enables ChatGPT/Codex account support through OpenAI's documented Codex surface. Login uses `codex app-server` with the official `account/login/start` ChatGPT browser flow, `account/read`, `account/logout`, and `model/list`. TL Studio uses an isolated `CODEX_HOME`, never imports browser cookies/session tokens, never copies a private OAuth client, and never calls undocumented ChatGPT backend endpoints.

For ChatGPT-plan inference, the account-managed Provider uses protocol `codex-chatgpt`. Each model turn launches the official Codex CLI in ephemeral read-only bridge mode with user/project Codex config and rules ignored, approval policy set to never, web search disabled, and an empty temporary working directory. The bridge receives the TL Studio conversation and TL Studio Tool schemas and must return structured assistant text or TL Studio Tool calls. TL Studio remains authoritative for the outer model → tool → model loop, Tool execution, permissions, project mutations, Session persistence, and semantic events.

The ChatGPT adapter auto-detects an installed `codex` executable. When unavailable it can use `npx @openai/codex`; Provider Settings also exposes a non-secret Codex executable override. Tests use a fake Codex executable and no real credentials.

PR #125 originally exposed ChatGPT/Codex, Claude, and GitHub Copilot as unavailable boundaries. After PR #143 only Claude and GitHub Copilot remain deferred. Claude remains deferred because no documented arbitrary third-party consumer OAuth client contract was found. Copilot remains deferred because the documented model-access path is coupled to the Copilot SDK/runtime.

## Browser contract

Browser code uses TL Studio-owned /local/* APIs.

Provider-specific helper processes are not Browser-facing control surfaces; the Browser sees only semantic TL Studio Provider Account state.

## Packaging

Preview and Release workflows build TL Studio plus supported bundled plugins and notices.

The ChatGPT integration does not bundle Codex into the TL Studio package in this alpha. It uses an installed Codex executable or the explicit npx fallback at runtime.

Release and review packages remain subject to package-content validation.

## Regression proof

Required evidence before review:

- strict Browser TypeScript and Browser build
- go test ./...
- go vet ./...
- supported platform cross-compiles
- standalone normal startup
- native Product Contract
- native Custom Provider contract
- Native Agent model/tool/model filesystem E2E
- Native Agent interactive Question E2E
- Native Permission tests
- real Browser smoke
- ChatGPT official-Codex account lifecycle with fake Codex fixture
- ChatGPT structured model/tool bridge regression test
- SHA256 for the Windows review package

## Manual Windows validation

The final review package must run as:

.\tl-studio.exe

Manual review should verify:

- normal startup
- project loading
- Explorer and Monaco
- Search
- Sessions
- Provider setup
- Native Agent response
- native file tool and workspace reconciliation
- interactive Question flow
- Permission approval/reject/remember behavior
- Terminal and Stop
- Preview
- Plugins/MCP
- JEV/OpenRouter where configured
- ChatGPT sign-in through official Codex
- ChatGPT account model discovery
- ChatGPT model response and TL Studio Tool call round-trip

## Current manual validation gate

Automated validation is green through the provider-card/README work. PR #143 adds automated fake-Codex coverage for ChatGPT but real account validation remains a release gate.

Use docs/ACCOUNT_PROVIDER_VALIDATION.md.

A Windows x64 review package is produced by CI and should be used for real OpenRouter, Hugging Face, Google/Gemini, and ChatGPT/Codex sign-in tests. ChatGPT validation must include a real ChatGPT-plan login, model discovery, a plain response, and at least one TL Studio Tool round-trip. Do not promote dev to stable main until the intended real account integrations have passed this checklist or their remaining limitations have been explicitly accepted and documented.

## Resume protocol

1. Inspect current dev and main; repository state is the source of truth.
2. Reconcile this checkpoint with README.md, README.fa_IR.md, docs/ARCHITECTURE.md, VERSION, open PRs, and CI.
3. Keep product development on dev.
4. Promote dev to main only for a validated stable release.
5. After a stable promotion, align dev with the new main baseline before beginning the next milestone.
