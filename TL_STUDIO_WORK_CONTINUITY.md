# TL Studio work continuity

Last updated: 2026-09-28

## Source of truth

The actual repository state on dev is the source of truth. Before substantial work, inspect current dev and reconcile this file with README.md, README.fa_IR.md, docs/ARCHITECTURE.md, VERSION, open PRs, and CI.

Stable main contains only validated stable releases. Ongoing development continues on dev.

## Current milestone

Release version:

0.5.0

Release status:

The fully native runtime milestone has completed automated CI and hands-on Windows validation. The current task is promotion of this validated development line to stable main as v0.5.0.

Completed milestone PR:

#115 — Remove the Kilo runtime dependency completely

Stable promotion rule:

Promote dev to main only after release checks are green. Stable main must not carry alpha or beta version labels.

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

Direct native model protocols currently include OpenAI-compatible Chat Completions, OpenAI Responses, and Anthropic Messages. Unsupported capabilities return explicit errors; there is no fallback engine.

The generic ProviderAccountAdapter architecture remains. If no documented account integration is implemented, /local/provider-accounts returns an empty list.

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

## Resume protocol

1. Inspect current dev and main; repository state is the source of truth.
2. Reconcile this checkpoint with README.md, README.fa_IR.md, docs/ARCHITECTURE.md, VERSION, open PRs, and CI.
3. Keep product development on dev.
4. Promote dev to main only for a validated stable release.
5. After a stable promotion, align dev with the new main baseline before beginning the next milestone.
