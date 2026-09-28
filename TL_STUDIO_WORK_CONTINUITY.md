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

None.

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

Normal startup initializes TL Studio services and the loopback local server directly. There is no compatibility-runtime discovery, port allocation, credential creation, subprocess, reverse proxy, or fallback engine.

The old flags --runtime-bin and --native-only are removed because native execution is the normal and only core execution mode.

## Session architecture

Browser
→ /local/sessions*
→ TL Studio Session domain
→ Native Agent
→ selected Provider

Session create, rename, delete, run, abort, status, transcript, activity, changes, and persistence are TL Studio-owned.

Old live sidecar-only alpha state is not preserved through a permanent compatibility dependency.

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

Provider configuration is authoritative in TL Studio and is never synchronized into a local compatibility runtime.

Direct native model protocols currently include OpenAI-compatible Chat Completions, OpenAI Responses, Anthropic Messages, and Google Gemini generateContent. Unsupported capabilities return explicit errors; there is no fallback engine.

The generic ProviderAccountAdapter architecture now has a typed account lifecycle: begin login, OAuth callback completion or polling, cancellation, refresh, status, model discovery, and logout. Browser state is semantic only and does not carry provider access tokens, refresh tokens, authorization codes, PKCE verifiers, API keys, cookies, or client secrets.

Manual API credentials and account credentials use separate credential-vault slots. An account credential takes precedence for native model execution while connected, but signing out exposes the preserved manual API credential instead of deleting it.

OpenRouter is the first concrete 0.6 account adapter. It uses the documented OpenRouter OAuth + PKCE flow, performs code exchange server-side, stores the resulting account key only in the TL Studio credential vault, discovers models through the normal provider discovery layer, and exposes those models through the existing Native Agent/model selector path.

Hugging Face is merged on dev through PR #123. It uses TL Studio's public CIMD identity at https://pouramin.dev/.well-known/oauth-cimd, Authorization Code + PKCE, loopback callbacks, account-vault access/refresh token persistence, automatic refresh in the Provider Account adapter, and the official OpenAI-compatible Inference Providers router. The provider-specific credential lifecycle remains behind ProviderAccountAdapter; the Native Agent receives only the resolved runtime credential.

Google/Gemini is merged on dev through PR #124. The implementation uses the official installed-app OAuth flow with PKCE, refresh and revocation, a Google Cloud quota project, native Gemini generateContent transport, model discovery, and preservation of Gemini thought signatures across tool-call continuation. No client secret is embedded. PR #127 adds a generic non-secret Provider Account setup surface so the 0.6 alpha can persist the Desktop OAuth Client ID and Google Cloud Project ID from Provider Settings; TL_STUDIO_GOOGLE_CLIENT_ID and TL_STUDIO_GOOGLE_PROJECT_ID remain development overrides. OAuth access/refresh tokens still live only in the credential vault. PR #128 is merged on dev. Each Google OAuth login now uses a dedicated temporary 127.0.0.1 listener on a random port so redirect_uri follows Google's documented Desktop loopback form instead of using a path on TL Studio's control server.

Custom Provider setup is simplified so a new provider normally requires only an API address, OpenAI-compatible or Anthropic-compatible protocol choice, and credential. Provider IDs are generated internally and model discovery runs automatically on save; manual model entry remains an explicit fallback. PR #126 fixed edit-mode behavior so hidden legacy Model ID fields cannot silently bypass a fresh discovery run when connection settings change.

ChatGPT/Codex account support remains a required 0.6 product goal. Current official OpenAI Codex app-server documentation exposes ChatGPT account login, device-code login, model listing, and Codex thread/turn execution, but no documented raw model transport suitable for preserving TL Studio's Native Agent loop was identified. The OpenAI internal chatgptAuthTokens route is explicitly internal-only and must not be used. TL Studio must not copy OpenCode's private Codex OAuth client or undocumented ChatGPT backend endpoints.

PR #125 exposes ChatGPT/Codex, Claude, and GitHub Copilot as explicit unavailable account adapters in Provider Settings while keeping them out of runtime credential resolution. Claude remains deferred because no documented arbitrary third-party consumer OAuth client contract was found. Copilot remains deferred because the documented model-access path is coupled to the Copilot SDK/runtime.

Current official Kilo documentation exposes the Gateway to external clients through API-key based public endpoints. No documented public third-party OAuth/device contract suitable for TL Studio account login was identified for this milestone, so Sign in with Kilo is removed rather than reverse-engineering a private flow.

Kilo Gateway may still be configured as a normal external API provider by a user. That does not create a runtime dependency.

## Browser contract

Browser code uses TL Studio-owned /local/* APIs. The generic /runtime/* reverse proxy and hosted aliases are removed.

The Browser must not reintroduce implementation-runtime routes.

## Packaging

Preview and Release workflows build only TL Studio plus still-supported bundled plugins and notices.

They do not download, stage, or package Kilo.

CI explicitly fails if kilo or kilo.exe is found in a review or release staging directory.

KILO_VERSION, Kilo license payload, Kilo API contract documentation, and Kilo-specific contract scripts have been removed from this development line.

## Regression proof

Required evidence before review:

- strict Browser TypeScript and Browser build
- go test ./...
- go vet ./...
- supported platform cross-compiles
- standalone normal startup with no sidecar
- native Product Contract
- native Custom Provider contract
- Native Agent model/tool/model filesystem E2E
- Native Agent interactive Question E2E
- Native Permission tests
- real Browser smoke
- no /runtime/* Browser dependency
- no Kilo executable in Windows review package
- SHA256 for the Windows review package

## Manual Windows validation

The final review package must run as:

.\tl-studio.exe

No --native-only flag should be required.

Manual review should verify:

- package contains no kilo.exe
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

## Stable-history note

Older commits and PRs used Kilo as a bundled or optional compatibility runtime. Those historical milestones explain how TL Studio reached the current architecture, but they are not the current product contract. Current architecture is defined by the repository source, README files, and docs/ARCHITECTURE.md on the active development head.

## Current manual validation gate

Automated validation is green through PR #130. The next release gate remains real provider-account validation rather than more mocked OAuth work.

Use docs/ACCOUNT_PROVIDER_VALIDATION.md.

A Windows x64 review package is produced by CI and should be used for real Hugging Face and Google/Gemini sign-in tests. Do not promote dev to stable main until the intended real account integrations have passed this checklist or their remaining limitations have been explicitly accepted and documented.

## Resume protocol

1. Inspect current dev and main; repository state is the source of truth.
2. Reconcile this checkpoint with README.md, README.fa_IR.md, docs/ARCHITECTURE.md, VERSION, open PRs, and CI.
3. Keep product development on dev.
4. Promote dev to main only for a validated stable release.
5. After a stable promotion, align dev with the new main baseline before beginning the next milestone.
