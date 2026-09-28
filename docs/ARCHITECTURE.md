# TL Studio architecture

## Product boundary

TL Studio is a local-first application implemented as one mandatory Go process plus embedded Browser assets.

The Go launcher owns the workspace, filesystem, Search, process/Terminal execution, Preview, Provider Registry, credential vault, direct model clients, Native Agent, sessions, questions, permissions, events, Tool Registry, Tool Executor, Plugins/MCP, and Provider Account adapters.

Normal startup initializes TL Studio services, starts the loopback local server, and opens the Browser UI.

## Browser boundary

The Browser uses TL Studio-owned semantic APIs on the same local control origin.

Core paths include:

~~~text
/local/status
/local/health
/local/providers
/local/provider-accounts
/local/sessions
/local/questions
/local/permissions
/local/events
/local/plugins
/local/tools
~~~

Browser source is TypeScript under cmd/launcher/ui. browser.ts is the ordered entry point and esbuild creates the embedded Browser bundle. Monaco is bundled locally.

## Native Agent

The execution path is:

~~~text
Browser
→ local Session run
→ TL Studio Session domain
→ Native Agent
→ selected Provider
→ model response
→ TL Studio Tool Executor when requested
→ Provider continuation
→ semantic transcript and final answer
~~~

TL Studio executes only protocols and capabilities it implements directly. Unsupported capability is returned explicitly.

## Sessions

All active sessions are TL Studio sessions. The Session domain owns IDs, create/rename/delete, run/abort, status, messages, activity and usage, file changes, and persistence.

## Interactive questions

Interactive questions are native Agent semantics:

~~~text
Native Agent
→ interaction.question
→ Question Manager
→ /local/questions
→ Browser
→ reply or reject
→ Agent resumes
~~~

Questions are scoped by session and support choices, multiple selection, custom text, rejection, and context cancellation.

## Permissions

The Permission Engine is authoritative for native Tool execution. It owns pending requests, allow-once and reject decisions, project-scoped remembered allow rules, sensitive-action restrictions, policy matching, and enforcement.

## Events

The local Event Bus is the semantic event source:

~~~text
Native Agent / Tool / Session / Question / Permission / Workspace
→ TL Studio Event Bus
→ /local/events
→ Browser
~~~

Persisted semantic Session data remains transcript authority.

## Providers and credentials

The Provider Registry is authoritative for Provider configuration and Model catalogs.

Supported direct model protocols on the current development line include:

- OpenAI-compatible Chat Completions;
- OpenAI Responses;
- Anthropic Messages;
- Google Gemini generateContent.

Credentials are stored separately in the TL Studio credential vault. Manual API credentials and account-backed credentials use separate vault slots.

## Provider Account domain

ProviderAccountAdapter and /local/provider-accounts define the generic account-backed integration boundary.

The lifecycle covers:

- status;
- begin/complete login;
- polling where required;
- cancellation;
- refresh;
- credential resolution;
- model discovery;
- disconnect.

An optional setup contract exposes only non-secret configuration fields.

Browser code receives semantic account state and safe login instructions. OAuth authorization codes, PKCE verifiers, access tokens, refresh tokens, API keys, cookies, and client secrets stay on the Go side.

Current 0.6 development adapters:

- OpenRouter — official OAuth + PKCE and account-backed Model discovery;
- Hugging Face — public-client OAuth + PKCE, refresh lifecycle, Inference Providers discovery;
- Google / Gemini — installed-app OAuth + PKCE, refresh/revocation, native Gemini transport.

Gemini sign-in uses a temporary HTTP listener bound only to 127.0.0.1 on a random port. The callback validates OAuth state and shuts down after completion, cancellation, or expiry.

ChatGPT/Codex, Claude account login, and GitHub Copilot remain explicit deferred account boundaries while their documented third-party contracts do not fit the current native execution model.

Private or undocumented Provider auth flows are not reverse-engineered.

## Tools and Plugins/MCP

The Native Tool Executor resolves TL Studio Tool Registry descriptors and enforces project boundaries and permissions before execution.

Core Tools cover workspace files, Search, process execution, and interactive questions. Plugins/MCP can contribute Tools through the same Agent-facing execution model.

## Terminal and Preview

Processes are project-scoped. Command stop terminates process trees where supported.

Preview is isolated from the TL Studio control origin and accepts loopback Preview URLs only.

## Security boundaries

The control UI binds to loopback. Non-loopback Host values are rejected and Browser Origin must match the local control origin.

Filesystem APIs are project-boundary checked. Traversal and symlink escapes are rejected. External Providers, repositories, prompts, and MCP/plugin processes are separate trust boundaries.

## Release architecture

A normal package contains tl-studio or tl-studio.exe, TL Studio licenses/notices, and bundled TL Studio plugins where configured.

Stable builds are published from main. Development Preview Builds are produced separately from dev.

## Regression proof

CI is expected to prove:

1. Browser strict TypeScript/build succeeds.
2. Go tests and vet succeed.
3. Supported platform cross-compiles succeed.
4. Standalone TL Studio starts normally.
5. Native Session lifecycle works.
6. Provider → Native Agent → Tool → filesystem → Provider continuation → final answer works.
7. Native Agent question → pending question → answer → resumed Agent works.
8. Native permissions and remembered rules work.
9. Real Browser smoke succeeds.
10. Provider Registry, credentials, and model catalog work.
11. Account-provider flows are covered through mocked contract tests without real user credentials.
12. Review and release packaging succeeds.
