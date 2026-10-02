# TL Studio architecture

## Product boundary

TL Studio is a local-first application with one mandatory Go product process plus embedded Browser assets.

The Go launcher owns the workspace, filesystem, Search, process/Terminal execution, Preview, provider registry, credential management, direct model clients, Native Agent, sessions, questions, permissions, events, Tool Registry, Tool Executor, Plugins/MCP, and Provider Account adapters.

Explicit integrations may use an optional provider-owned helper process when that is the provider's documented third-party surface. The ChatGPT account path is one such integration: TL Studio launches the official OpenAI Codex CLI only for ChatGPT authentication/model transport while retaining ownership of TL Studio Sessions, permissions, tools, persistence, and the outer Agent loop.

## Startup

Normal startup initializes TL Studio services, starts the loopback local server, and opens the Browser UI.

Provider helper processes are lazy and optional; they start only when their integration is used.

## Browser boundary

The Browser uses TL Studio-owned semantic APIs on the same local control origin. Core paths include /local/status, /local/health, /local/providers, /local/provider-accounts, /local/sessions, /local/questions, /local/permissions, /local/events, /local/plugins, and /local/tools.

Browser source is TypeScript under cmd/launcher/ui. browser.ts is the ordered entry point and esbuild creates the embedded Browser bundle. Generated JavaScript is not tracked. Monaco is bundled locally.

## Native Agent

The execution path is:

Browser
→ local Session run
→ TL Studio Session domain
→ Native Agent
→ selected Provider
→ model response
→ TL Studio Tool Executor when requested
→ Provider continuation
→ semantic transcript and final answer

The runtime resolves only providers and models that TL Studio can execute directly. Unsupported protocols or capabilities produce explicit errors instead of being routed to another engine.

## Sessions

All active sessions are TL Studio sessions. The Session domain owns IDs, create/rename/delete, run/abort, status, messages, activity and usage, file changes, and persistence.

## Interactive questions

Interactive questions are native Agent semantics:

Native Agent
→ interaction.question
→ Question Manager
→ /local/questions
→ Browser
→ reply or reject
→ Question Manager
→ Agent resumes

Questions are scoped by session. Multiple choice, multiple selection, custom text, rejection, and context cancellation are supported. Pending/resolved changes emit semantic attention.changed events.

## Permissions

The Permission Engine is authoritative for native tool execution. It owns pending requests, allow-once and reject decisions, project-scoped remembered allow rules, sensitive-action restrictions, policy matching, enforcement, and semantic attention events.

The Browser uses only /local/permissions*.

## Events

The local Event Bus is the single semantic event source:

Native Agent / Tool / Session / Question / Permission / Workspace
→ TL Studio Event Bus
→ /local/events
→ Browser

The SSE stream is a responsiveness signal. Persisted semantic session data remains transcript authority.

## Providers and credentials

The provider registry is authoritative for model connection definitions and model catalogs.

Supported direct model protocols currently include OpenAI-compatible Chat Completions, OpenAI Responses, Anthropic Messages, and Google Gemini generateContent. ChatGPT-plan access uses the separate `codex-chatgpt` provider transport described below.

Manual API credentials and account credentials managed by TL Studio are stored separately in the TL Studio credential vault and do not appear in providers.json or Browser storage. A connected account may temporarily take precedence without overwriting the user's manual API key.

For ChatGPT, OpenAI's official Codex client owns the OAuth token persistence and refresh lifecycle inside an isolated TL Studio-specific `CODEX_HOME`. TL Studio does not read, copy, serialize, or expose those raw ChatGPT tokens.

Unsupported models remain explicit unsupported capabilities. JEV/OpenRouter uses the same native provider and Agent boundaries.

## Provider Account domain

ProviderAccountAdapter and /local/provider-accounts* define the generic account-backed integration boundary. The lifecycle covers typed login challenges, polling/completion, cancellation, refresh, status, model discovery, and logout. An optional Provider Account setup contract exposes only non-secret configuration fields through /local/provider-accounts/{id}/setup.

Browser code receives only semantic account state and safe login instructions. OAuth authorization codes, PKCE verifiers, access tokens, refresh tokens, API keys, cookies, and client secrets never enter Browser state. Provider credentials are either stored in the TL Studio credential vault or, for the ChatGPT integration, managed by the official Codex client in TL Studio's isolated `CODEX_HOME`.

The Provider Settings UI does not equate "no account OAuth" with "unavailable". The compact cards use account sign-in only for account-oriented integrations (ChatGPT/Codex and the reserved GitHub Copilot slot). Claude/Anthropic, Google/Gemini, Hugging Face, and OpenRouter route to API-credential presets that reuse the normal discovery-first Provider editor. The presets use the documented service endpoints and keep API credentials in the normal TL Studio credential vault.

OpenRouter is the first concrete account adapter in the 0.6 development line. It uses OpenRouter's documented OAuth + PKCE flow, exchanges the authorization code server-side for a user-controlled OpenRouter key, stores that key in the account credential slot, discovers models through the normal provider discovery path, and then uses the existing Native Agent/provider execution path.

Hugging Face is the second concrete account adapter. TL Studio identifies itself as a public native OAuth client through a CIMD document hosted at https://pouramin.dev/.well-known/oauth-cimd, uses Authorization Code + PKCE with a loopback callback, stores access and refresh tokens only in the account vault slot, refreshes expiring tokens inside the Provider Account adapter, and syncs the official Inference Providers model catalog into the existing Provider Registry. The Native Agent resolves a fresh account credential through the adapter without containing Hugging Face-specific authentication logic.

Google/Gemini is implemented as an installed-app OAuth adapter using Authorization Code + PKCE, refresh tokens, and token revocation. Each login starts a temporary HTTP listener bound only to 127.0.0.1 on a random port and uses the bare loopback origin as redirect_uri, matching Google's documented desktop-app loopback pattern; the listener validates OAuth state and shuts down after completion, cancellation, or expiry. Gemini model calls use the native generateContent transport and keep Google-specific thought-signature continuation state inside the model protocol boundary. The non-secret Google Cloud Project ID is sent as x-goog-user-project for quota/billing and is persisted in local account setup (and the account-managed provider definition after discovery); OAuth tokens remain only in the credential vault. During the 0.6 alpha the Desktop OAuth Client ID can also be supplied through the non-secret setup surface until a production TL Studio OAuth client is registered and shipped.

ChatGPT/Codex is implemented through OpenAI's documented Codex App Server and CLI surface. Account login uses Codex-managed ChatGPT OAuth; TL Studio opens the returned authorization URL, observes the official login-completed notification, reads semantic account metadata, and synchronizes `model/list` into an account-managed Provider definition. The official Codex authentication store is isolated under TL Studio's own `CODEX_HOME`.

For ChatGPT-plan model turns, `codex-chatgpt` invokes the official Codex CLI as an ephemeral provider bridge with user/project Codex configuration ignored, a read-only sandbox, approval policy `never`, and web search disabled. TL Studio supplies only the current model-turn conversation plus TL Studio Tool schemas and requires structured output containing either assistant text or TL Studio Tool calls. Codex does not receive the user's project directory as its working directory. TL Studio then executes requested Tools through its own Tool Executor and Permission Engine and continues the outer model → tool → model loop.

The bridge is deliberately provider-specific rather than a replacement Agent runtime: TL Studio remains authoritative for Session lifecycle, Tool execution, permissions, project mutations, persistence, and semantic events. If the official `codex` executable is not on PATH, the alpha can launch `npx @openai/codex`; Provider Settings also accepts an explicit Codex executable path.

Account providers whose authentication or model-transport contract does not currently fit this product boundary remain visible as explicit unavailable adapters rather than disappearing from Settings. Claude account login is explicitly deferred: Anthropic's current Agent SDK documentation states that, unless previously approved, third-party developers may not offer claude.ai login or subscription rate limits in their products and should use API-key authentication instead (verified 2026-10-02: https://code.claude.com/docs/en/agent-sdk/overview). The fact that Claude Code itself has official browser-login commands does not make that consumer login an approved third-party product integration. GitHub Copilot remains deferred while its documented model integration requires the Copilot SDK/runtime.

Private or undocumented provider OAuth flows are not reverse-engineered. Deferred boundary adapters are never registered as runtime credential sources.

## Tools and Plugins/MCP

The Native Tool Executor resolves TL Studio Tool Registry descriptors and enforces project boundaries and permissions before execution.

Core tools cover workspace files, Search, and process execution. Plugins/MCP can contribute tools through the same Agent-facing execution model.

## Terminal and Preview

Processes are project-scoped. Command stop terminates process trees where supported. The current Terminal foundation is a command runner rather than a full PTY.

Preview is isolated from the TL Studio control origin and accepts loopback Preview URLs only.

## Security boundaries

The control UI binds to loopback. Non-loopback Host values are rejected and Browser Origin must match the local control origin.

Filesystem APIs are project-boundary checked. Traversal and symlink escapes are rejected. External providers, repositories, prompts, and MCP/plugin processes are untrusted inputs.

## Release architecture

A normal package contains tl-studio or tl-studio.exe, TL Studio licenses/notices, and bundled TL Studio plugins where configured. It does not contain a coding-runtime sidecar.

CI validates review and release package contents before publication.

## Regression proof

CI is expected to prove:

1. Browser strict TypeScript/build succeeds.
2. Go tests and vet succeed.
3. Supported platform cross-compiles succeed.
4. Standalone TL Studio starts normally with no sidecar.
5. Native Session lifecycle works.
6. Fake Provider → Native Agent → native tool → filesystem → Provider continuation → final answer works.
7. Native Agent question → pending question → answer → resumed Agent works.
8. Native permissions and remembered rules work.
9. Real Browser smoke succeeds.
10. Provider registry, credentials, and catalog work without another runtime.
11. Windows review and release packages pass package-content validation.
12. Production Browser source and workflows use the intended TL Studio product contracts.
