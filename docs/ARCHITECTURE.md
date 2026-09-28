# TL Studio architecture

## Product boundary

TL Studio is a local-first application implemented as one mandatory Go process plus embedded Browser assets.

The Go launcher owns the workspace, filesystem, Search, process/Terminal execution, Preview, provider registry, credential vault, direct model clients, Native Agent, sessions, questions, permissions, events, Tool Registry, Tool Executor, Plugins/MCP, and generic Provider Account adapters.

There is no local compatibility runtime below TL Studio. Normal startup initializes TL Studio services, starts the loopback local server, and opens the Browser UI.

## Startup

Startup does not discover a Kilo binary, allocate a compatibility-runtime port, create sidecar credentials, start a second Agent process, create a reverse proxy, or silently fall back to another execution engine.

Native execution is the only core execution mode.

## Browser boundary

The Browser uses TL Studio-owned semantic APIs on the same local control origin. Core paths include /local/status, /local/health, /local/providers, /local/provider-accounts, /local/sessions, /local/questions, /local/permissions, /local/events, /local/plugins, and /local/tools.

The Browser has no implementation-runtime route dependency and there is no generic /runtime/* reverse proxy.

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

Old live sidecar-only alpha state is intentionally not kept alive through a permanent compatibility dependency.

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

The provider registry is authoritative. Provider configuration and credentials are never mirrored into another local runtime.

Supported direct model protocols currently include OpenAI-compatible Chat Completions, OpenAI Responses, and Anthropic Messages.

Credentials are stored separately in the TL Studio credential vault. They do not appear in providers.json or Browser storage. Manual API credentials and account-backed credentials use separate vault slots; an account connection may temporarily take precedence without overwriting the user's manual API key.

Unsupported models remain explicit unsupported capabilities; there is no fallback engine. JEV/OpenRouter uses the same native provider and Agent boundaries.

An external service such as Kilo Gateway may be configured only as a normal documented HTTPS/API-key provider, exactly like any other external API. No local Kilo binary or private OAuth protocol is involved.

## Provider Account domain

ProviderAccountAdapter and /local/provider-accounts* define the generic account-backed integration boundary. The lifecycle covers typed login challenges, polling/completion, cancellation, refresh, status, model discovery, and logout.

Browser code receives only semantic account state and safe login instructions. OAuth authorization codes, PKCE verifiers, access tokens, refresh tokens, API keys, cookies, and client secrets stay on the Go side and credential material is persisted only through the TL Studio credential vault.

OpenRouter is the first concrete account adapter in the 0.6 development line. It uses OpenRouter's documented OAuth + PKCE flow, exchanges the authorization code server-side for a user-controlled OpenRouter key, stores that key in the account credential slot, discovers models through the normal provider discovery path, and then uses the existing Native Agent/provider execution path.

Hugging Face is the second concrete account adapter. TL Studio identifies itself as a public native OAuth client through a CIMD document hosted at https://pouramin.dev/.well-known/oauth-cimd, uses Authorization Code + PKCE with a loopback callback, stores access and refresh tokens only in the account vault slot, refreshes expiring tokens inside the Provider Account adapter, and syncs the official Inference Providers model catalog into the existing Provider Registry. The Native Agent resolves a fresh account credential through the adapter without containing Hugging Face-specific authentication logic.

Private or undocumented provider OAuth flows are not reverse-engineered. Account integrations must not introduce a second Agent runtime or compatibility engine.

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

CI fails if kilo or kilo.exe appears in a review or release package.

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
11. Windows review and release packages contain no Kilo executable.
12. Production Browser source and workflows contain no compatibility-runtime path.
