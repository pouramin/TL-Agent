# TL Studio Work Continuity

This file is a durable operating instruction for future TL Studio development sessions.

## Source of truth

- Repository: `https://github.com/pouramin/TL-Studio`
- The repository state is the final source of truth.
- Before continuing substantial work, inspect current `dev` and reconcile this file, `README.md`, `README.fa_IR.md`, `docs/ARCHITECTURE.md`, `docs/KILO_API_CONTRACT.md`, and `VERSION` with the actual code.
- Do not restart or redesign the project from scratch.

## Branch and release discipline

- `main` is stable production only and is promoted to the stable `v0.3.0` line after Phase 2 validation.
- `dev` is the next private alpha line; the current development target is `0.4.0-alpha.5`.
- Feature/fix branches start from `dev`.
- Experimental work must not be merged into `main`.
- Private alpha builds use the GitHub Actions Preview Build artifact flow.
- Stable releases are promoted only after automated gates and hands-on validation.

## Current development state

Current stable baseline after Phase 2 promotion:

`0.3.0`

Current private development target:

`0.4.0-alpha.5`

The alpha.5 TypeSafe Jev milestone is merged into `dev`.

Merged PR:

`#97 — Add TypeSafe Jev support`

Squash merge commit:

`92e7297db240eecf1dc73b061bfea814df6d265b`

The alpha.5 work adds Jev Router through TL Studio's existing generic OpenRouter-compatible provider path and adds a separate, default-off Decision Engine abstraction for direct structured Jev decisions. It does not replace or bypass the Native Agent, Tool Registry, Permission Engine, Tool Executor, provider registry, credential vault, or Kilo compatibility boundaries.

The alpha.4 provider-discovery and bundled-plugin foundation remains merged into `dev`.

Merged PR:

`#95 — Add provider discovery and bundled plugin foundation`

Squash merge commit:

`679ae8663a984be2ba7a95a93d35268fd1acbc75`

The generic Plugin/MCP architecture from PR #93 remains the foundation. The alpha.4 work extends it rather than replacing it: provider model discovery is TL Studio-owned, and bundled/default plugins differ from user-added plugins only at configuration/release/executable-resolution boundaries. Both Plugin origins still enter the same MCP → Tool Registry → Permission → Native Tool Executor → Native Agent path.

Graphify remains the first real external integration used to validate generic MCP behavior. It is **not** bundled in alpha.4 because its current Python/runtime/dependency distribution would add disproportionate release complexity.

Phase 2 native execution milestone was squash-merged through PR:

`#90 — Phase 2: own Agent loop and tool execution`

Phase 2 merge commit:

`1c3fcd05280e3e80d03a1b949f81ae627820b6fd`

Phase 2 was promoted only after the final Preview Build passed and the Windows preview was hands-on validated. Re-read current `main`, `dev`, Release, and CI state from GitHub when resuming.

## Phase 2 ownership — complete

TL Studio now owns the product-facing semantics and supported native execution path for:

- Browser IDE/workspace
- Monaco editor
- file management
- project search
- local terminal/process manager
- preview
- provider/model definitions
- custom provider registry
- custom provider credential vault
- Tool Registry semantic metadata
- executable native core coding tools
- permission policy and native permission enforcement
- session read model
- session create/update/delete/run/abort semantics
- semantic session persistence
- explicit session execution ownership (`native` vs `compatibility`)
- interactive question semantics
- semantic live-event projection
- direct model execution for supported custom providers
- the TL Studio-native model/tool/model Agent loop
- cancellation and loop guards
- project/session presentation and recovery UX

The Browser remains on TL Studio semantic contracts and does not depend directly on raw Kilo Agent/tool/question mutation routes.

## Native Agent runtime

Supported TL Studio-managed custom providers run through:

```text
Browser
→ TL Studio semantic session command
→ TL Studio native Agent loop
→ TL Studio direct model client
→ normalized model/tool call
→ TL Studio permission policy
→ TL Studio Tool Executor
→ TL Studio-owned local subsystem
→ semantic tool result
→ next model turn
→ final assistant response
→ TL Studio persistence + live events
```

Current direct provider protocols:

- OpenAI-compatible Chat Completions
- OpenAI Responses
- Anthropic Messages compatible

Current native executable coding tools:

- `files.read`
- `files.list`
- `files.write`
- `files.edit`
- `search.content`
- `terminal.command`

These reuse existing TL Studio project-boundary, symlink, search, process, cancellation, and permission infrastructure.

Unknown native tools are rejected.

## Kilo compatibility path

Pinned Kilo version remains:

`7.6.2`

Kilo remains bundled as the currently tested third-party compatibility engine.

It is still used for:

- hosted Kilo authentication/models
- legacy/runtime-configured providers that are not resolvable from the TL Studio provider registry + credential vault
- runtime-only capabilities not yet represented by TL Studio native handlers
- compatibility/session behavior covered by existing real-Kilo regression tests

Kilo is no longer the mandatory owner of the Agent loop or core coding tool execution for supported TL Studio-managed custom providers.

Private Kilo details such as `x-kilo-directory`, `prompt_async`, raw Kilo session envelopes, auth routes, question routes, and Kilo-specific tool IDs must remain isolated in compatibility/adapter code.

## Native execution safety

The native path includes:

- strict TL Studio tool IDs and schemas
- structured argument validation/results/errors
- project path confinement
- path traversal and symlink protections
- separate read/write/execute permission classes
- project-scoped remembered non-sensitive permission rules
- no remembered approval for sensitive terminal execution
- timeout/context cancellation
- process-tree stop behavior through the existing process manager
- maximum Agent iterations
- maximum tool rounds
- maximum tools per round
- repeated-identical-tool-call guard
- explicit execution ownership in persisted sessions
- no API secrets in session history

## Automated proof of independence

The repository contains deterministic native tests proving a coding task can complete without Kilo performing the Agent loop or tool execution.

The strongest proof starts a fake OpenAI-compatible HTTP model server and exercises:

```text
user request
→ native model request
→ model emits files.write
→ TL Studio Tool Executor writes a real fixture file
→ structured tool result returns to the model
→ second model request
→ final assistant response
→ semantic session persistence
```

Additional tests cover provider translation/normalization, path traversal rejection, unknown tools, native permission behavior, terminal cancellation, and prevention of Kilo-private protocol leakage into native runtime files.

## CI and validation status at Phase 2 promotion

Before PR #90 was squash-merged, the final `0.3.0-alpha.23` head passed:

- Browser strict TypeScript check
- Browser build
- generated-JavaScript cleanliness
- local Monaco bundle verification
- Go tests
- Go vet
- Browser JavaScript syntax
- npm quick-launcher verification
- TL Studio runtime-boundary enforcement
- Python test syntax
- supported platform cross-compiles
- real bundled-runtime product contract
- real bundled-runtime prompt/write-tool E2E
- custom-provider compatibility contract
- npm package contract

The old Kilo prompt/write E2E remains green through capability-based compatibility fallback.

## Important architectural decisions

1. Native execution is selected only when the requested custom provider/model can be resolved from TL Studio-owned provider definitions and credentials.
2. Hosted Kilo and unresolved legacy runtime providers fall back to the Kilo compatibility adapter.
3. Session persistence explicitly records the execution owner so native semantic history is not accidentally overwritten by compatibility runtime reads.
4. The existing semantic live-event channel is shared by native and compatibility execution; no second Browser event architecture was introduced.
5. The existing local file/search/process systems are reused; native tools do not duplicate those subsystems.
6. Kilo remains bundled for compatibility. Do not claim it is fully removable yet.

## Plugin / MCP hands-on fixes — alpha.3

Windows hands-on validation of `0.4.0-alpha.2` found two UX inconsistencies in Settings → Plugins:

- the unsaved editor's **Test Connection** incorrectly treated Graphify graph readiness as part of MCP connectivity, while the saved-card test correctly tested only the MCP server;
- closing Settings after a completed plugin flow could leave the editor open with stale form values when Settings was reopened.

The `0.4.0-alpha.3` fix separates MCP connection testing from integration readiness, keeps Graphify graph readiness as an enable/execution precondition, and resets/hides the Plugin editor after save and whenever the Settings dialog closes.

## Plugin / MCP milestone — implemented on feature branch

The `0.4.0-alpha.3` milestone adds a TL Studio-owned generic Plugin system with first-class MCP support.

Current architecture:

```text
Settings → Plugins
        ↓
TL Studio Plugin Manager
        ↓
MCP Client Manager
        ↓
discovered MCP tools
        ↓
TL Studio Tool Registry
        ↓
Permission Engine
        ↓
Native Tool Executor
        ↓
Native Agent
```

Implemented behavior:

- persisted project/global Plugin definitions owned by TL Studio
- initial Plugin type `mcp`
- initial MCP transport `stdio`
- transport client interface designed so Streamable HTTP can be added without changing the Agent architecture
- user-configurable command, argument vector, working directory, scope, and environment-variable names
- secret environment values stored through the existing TL Studio credential vault and omitted from normal Plugin API responses
- explicit user confirmation before enabling a local Plugin command
- MCP initialize handshake, dynamic tool discovery, optional resource discovery, structured tool calls/results, process error reporting, reconnect, stop, timeout, and cancellation signaling
- namespaced tool IDs in the form `mcp.<plugin-id>.<tool-name>`
- discovered MCP schemas normalized into native model tool definitions
- MCP tools merged into the existing TL Studio Tool Registry
- all MCP Agent calls routed through the existing native Permission Engine
- conservative permission classification for unknown MCP capabilities
- enabled Plugin tools exposed to the existing native Agent loop; the Agent remains the tool-selection decision maker
- Plugin disable/remove/project switch/application shutdown stops relevant MCP child processes
- Settings → Plugins list/add/configure/remove/enable/disable/Test Connection UI
- Graphify convenience detection for `graphify` and `graphify-mcp`
- Graphify Build/Rebuild using the fixed local `graphify extract . --code-only` command after explicit confirmation
- Graphify Open Graph reuses TL Studio Preview for `graphify-out/graph.html`
- Graphify MCP tools are never hardcoded; they are discovered through MCP like any other plugin

Deterministic automated coverage uses a fake local stdio MCP server and does not require Graphify in CI. Coverage proves Plugin persistence/scoping, disabled vs enabled process behavior, MCP initialization, tool/resource discovery, namespacing/schema normalization, native Agent tool availability, tool-result round-trip, permission authorization, cancellation, process failure status, secret redaction, disable/stop behavior, and preservation of the built-in native tool set.

The implementation has also passed the existing real bundled-runtime product contract, real prompt/write E2E, strict Browser TypeScript/build checks, Go test/vet, and custom-provider compatibility gates on the feature PR during development.

Final PR gates were green before merge:

- strict Browser TypeScript check and Browser build
- Go tests and vet, including deterministic fake stdio MCP coverage
- supported launcher cross-compiles
- npm package contract
- real bundled-runtime product contract
- real bundled-runtime prompt/write-tool E2E
- real bundled-runtime custom-provider contract

The milestone was squash-merged into `dev` only. Stable `main` remains on `v0.3.0`.

Post-merge private Windows Preview Build:

- workflow run: `35735591863`
- result: `success`
- head: `5636e1311f74a02e255fd4353bcdb07935df81f4`
- artifact: `TL-Studio-0.4.0-alpha.3-Windows-x64-Preview`
- inner product ZIP SHA-256: `b3dff1738edc3411906ae70582e86265052ec61db07bd23662423671d8b70a3a`
- checksum was independently recomputed after downloading the Actions artifact and matched `SHA256SUMS.txt`.

The implementation/build milestone is therefore complete. The remaining product-validation step is hands-on Windows testing of `0.4.0-alpha.3`, especially Settings → Plugins, arbitrary stdio MCP add/test/enable/disable, and Graphify build/query/open behavior. Do not start the separate runtime-independence phase until this hands-on milestone is validated.

## Model discovery + bundled plugin foundation — alpha.4

The `0.4.0-alpha.4` feature line adds two product-level capabilities without changing Native Agent architecture.

### Provider model discovery

Implemented:

- TL Studio-owned `POST /runtime/providers/discover` draft/discovery contract
- generic OpenAI-compatible/OpenAI Responses discovery through `<baseURL>/models`
- provider-specific Anthropic Messages discovery with `x-api-key`, Anthropic version header, and pagination
- normalized discovered-model metadata for ID/name, context/output limits, and optional tool/reasoning/vision capability signals when the upstream actually provides them
- explicit unknown capability state in discovery rather than guessing from model names
- pre-save **Test / Discover Models**
- searchable multi-model selection with bounded Browser rendering for large catalogs
- explicit user control over whether models with unknown tool capability should be treated as tool-capable when saved
- manual Model ID entry retained as fallback
- configured models preserved when a later provider refresh stops returning them
- saved-provider last-good catalog cache with stale warnings
- 401/403 authentication failures never hidden by stale-cache fallback
- discovery cache contains no API keys and is removed with the provider
- response-size, model-count, timeout, URL, and cross-origin redirect guards

Discovered catalogs are not automatically copied into `providers.json`; only selected/configured models become execution definitions. Automatic background polling and giant hardcoded provider/model catalogs remain intentionally out of scope.

### Bundled/default plugin foundation

Implemented:

- generic Plugin origin distinction: `bundled` vs `user`
- bundled metadata from an embedded versioned `cmd/launcher/bundled_plugins.json`
- package-relative bundled executable resolution under `plugins/<id>/bin`
- separate bundled enable/disable state; bundled identities cannot be removed or shadowed by user config
- Settings grouping: **Included with TL Studio** and **Added by you**
- no separate Agent/runtime executor for bundled tools
- bundled and user-added MCP tools share dynamic discovery, Tool Registry, Permission Engine, Native Tool Executor, and Native Agent
- project switch restarts all MCP clients so global clients do not retain the previous project working directory
- bundled child processes receive a reduced ordinary environment rather than inheriting unrelated launcher secrets
- stable-release and Windows Preview workflows call the same bundled-plugin staging script
- staging manifest requires all supported TL Studio targets, HTTPS artifacts, exact SHA-256, pinned version, upstream metadata, and a repository-retained license for each third-party bundled plugin
- deterministic release-staging regression coverage for raw/ZIP/tar.gz artifact member handling

The alpha.4 bundled-plugin manifest is intentionally empty. No third-party executable is added merely to populate Settings. A real bundled candidate should be added only after its platform artifacts, dependency footprint, license, update model, and user value pass these gates.

### Validation on feature branch

The feature PR has passed the existing **CI** and **Custom Provider Contract** workflows repeatedly during implementation, including after the initial model-discovery/UI work and after the generic bundled-plugin release foundation. Re-check the current PR head before merge.

Merge validation completed:

- final feature-head **CI**: success
- final **Custom Provider Contract**: success
- final **npm Package Contract**: success
- PR #95 marked ready and squash-merged into `dev`
- `dev` version: `0.4.0-alpha.4`
- stable `main` version remains `0.3.0`

Post-merge Windows Preview Build:

- workflow: **Preview Build**
- run: `36129998956`
- result: `success`
- head: `679ae8663a984be2ba7a95a93d35268fd1acbc75`
- artifact: `TL-Studio-0.4.0-alpha.4-Windows-x64-Preview`
- artifact ID: `10861861706`
- inner product ZIP SHA-256 from workflow: `75051d6454c11af165443a9cc0594002e05068f3a3e095e2f09a9c24062c5cca`
- the inner product ZIP checksum was independently recomputed after downloading the Actions artifact and matched `SHA256SUMS.txt`

Remaining product validation:

- hands-on Windows validation of Provider discovery, multi-model selection/refresh/manual fallback, bundled-vs-user Plugins grouping, and MCP enable/disable lifecycle
- do not promote alpha.4 work to stable `main` before those hands-on checks pass

## TypeSafe Jev milestone — alpha.5

The `0.4.0-alpha.5` development line adds TypeSafe Jev in two deliberately separate layers.

### Jev Router

The normal generative integration uses the exact OpenRouter model ID:

```text
typesafe/jev-router
```

Implementation behavior:

- reuses the existing generic `openai-compatible` provider path rather than adding a dedicated Agent backend
- reuses an already configured official OpenRouter provider and its TL Studio-owned credential when present
- otherwise pre-fills the existing provider form for `https://openrouter.ai/api/v1`
- discovers Jev Router from the provider's live model catalog
- stores `kind: "router"` as TL Studio model metadata
- keeps Router selection in the normal model selector
- preserves the current native streaming, system-prompt, conversation-history, and tool-definition path
- reads tool/reasoning support from provider metadata when published and does not infer capability from the model name
- disables the Jev setup flow's optimistic "assume unknown tools" behavior by default
- records the provider-returned routed model as semantic model activity when the response actually supplies it
- never hard-codes the underlying models Jev Router may select
- never falls back automatically to a paid direct Jev model

As of 2026-09-26, OpenRouter lists Jev Router with zero prompt/completion token pricing. Treat this as mutable upstream metadata, not a permanent product guarantee.

### Optional direct Decision Engine

Direct Jev System One decisions do not use the normal chat-model abstraction. TL Studio now owns a small provider-independent `decisionEngine` interface with a Jev/OpenRouter implementation.

Launcher routes:

```text
GET  /local/decision-engine
PUT  /local/decision-engine
POST /local/decision-engine/evaluate
```

Current behavior:

- default state is `off`
- explicit Jev enablement is required
- enabling reuses an existing official OpenRouter credential; no duplicate API key entry
- default direct model alias is `~typesafe/jev-latest`
- `typesafe/jev-1.13` is also accepted as an explicit direct model in the backend config
- direct calls target OpenRouter's separate `/api/alpha/decisions` endpoint
- Choice, Score, and Noul answers are normalized with probabilities/confidence/legend/usage when returned
- validation, credential, auth, rate-limit, timeout, cancellation, network, upstream, and invalid-response failures are typed
- Decision API credentials are not allowed to cross origin on redirects
- no automatic model routing, tool routing, permission scoring, Agent continuation, or output-verification hook is enabled yet
- deterministic TL Studio permission/security rules remain authoritative; Jev is never a security boundary

Selecting Jev Router does not invoke the Decisions API and cannot silently substitute `typesafe/jev-1.13` or `~typesafe/jev-latest`.

### UI

Settings → Providers now contains a compact TypeSafe Jev card with:

```text
Set up Jev Router
Decision Engine: Off / Jev via OpenRouter (paid)
```

No separate large settings section or unfinished feature toggles were added.

### Automated validation

Final feature head before merge:

`d22f648365bfff4f3e042da1e7c261d71c1cdc5e`

Passed:

- CI
- strict Browser TypeScript check
- Browser build
- generated Browser JavaScript cleanliness
- local Monaco bundle verification
- Go tests
- Go vet
- Browser JavaScript syntax
- npm quick-launcher verification
- TL Studio runtime-boundary enforcement
- Python syntax checks
- bundled-plugin manifest/staging validation
- supported-platform cross-compiles
- real bundled-runtime product contract
- real bundled-runtime prompt/write-tool E2E
- Custom Provider Contract
- npm Package Contract

The final implementation uses mocked/fixture upstream responses and does not require a live OpenRouter key or spend real Jev Decision API credits during automated tests.

### Post-merge Windows Preview Build

Workflow run:

`36253274892`

Result:

`success`

Head:

`92e7297db240eecf1dc73b061bfea814df6d265b`

Artifact:

`TL-Studio-0.4.0-alpha.5-Windows-x64-Preview`

Artifact ID:

`10910140359`

GitHub artifact ZIP digest:

`sha256:602ab05eb8b01c328bc1ded540ce7e645e9aa442dbbb876ffe7129b8f7f52885`

All Windows Preview workflow stages passed, including Browser build, `go test ./...`, Windows launcher compilation, bundled Kilo staging, bundled-plugin staging, packaging, and artifact upload.

Remaining product validation:

- hands-on Windows test of Jev Router setup against a real user-configured OpenRouter account
- confirm Jev Router appears in the normal model selector after live discovery
- confirm normal chat/Agent behavior for the capabilities actually published by OpenRouter
- confirm provider-returned routed-model metadata is displayed only when OpenRouter supplies it
- optionally test direct paid Decision Engine manually only if the user explicitly wants to spend OpenRouter credits
- do not promote alpha.5 work to stable `main` before hands-on validation

## Alpha.5 hands-on fixes after Windows validation

Hands-on Windows testing exposed several product-surface bugs in the first alpha.5 preview. They were fixed in:

`#99 — Fix provider catalog leakage and compact Jev settings`

Squash merge commit:

`212ce1a68b8de3f291264b55e564a84be3411dd2`

Fixes now merged into `dev`:

- runtime-only provider catalogs no longer appear as if the user configured them in TL Studio
- the normal model selector now contains only:
  - Kilo's preferred hosted free route (`kilo-auto/free`)
  - TL Studio-managed providers saved in `providers.json`
- a managed provider contributes only the exact models the user selected/saved; unselected runtime catalog models are excluded
- OpenRouter model discovery now validates the supplied bearer key through the official `GET /api/v1/key` endpoint before accepting the model catalog
- an invalid OpenRouter key fails before `/models`; public catalog behavior can no longer make a bad key look valid
- `Settings → Plugins` always renders both `Included with TL Studio` and `Added by you`, with explicit empty states when a group has no entries
- the large permanent Jev card was replaced with a compact JEV status/control row
- clicking the compact JEV control opens a focused Jev configuration dialog
- Jev status refreshes after Provider Save/Delete and recognizes an already-saved `typesafe/jev-router`
- no paid Kilo hosted models are intentionally exposed in the normal selector

Validation for PR #99:

- CI: success
- Custom Provider Contract: success
- TypeScript: success
- Browser build: success
- Go tests: success
- Go vet: success
- runtime product contract: success
- runtime prompt/write E2E: success
- supported-platform cross-compiles: success

Post-merge Windows Preview Build:

`36314019242`

Head:

`212ce1a68b8de3f291264b55e564a84be3411dd2`

Result:

`success`

Artifact:

`TL-Studio-0.4.0-alpha.5-Windows-x64-Preview`

Artifact ID:

`10929718502`

GitHub artifact digest:

`sha256:8251aa431bb1f588efa9a77135849a2f365fc969320f654a141aa2febd921613`

Inner product ZIP SHA256:

`86a98d4a9ef970fb434b59433d1baacbb6d69d72130ff0955ffbcb0229a9bbaa`

Hands-on validation should continue from this preview, not the earlier alpha.5 artifact. Re-test provider persistence/visibility, selected-model filtering, invalid OpenRouter credential handling, both plugin groups, compact Jev setup/status, and the normal Agent/tool/permission path.

## Alpha.5 provider-registry resilience fix

A second hands-on Windows validation pass exposed a deeper ownership bug: TL Studio's product-owned provider registry was still blocked by Kilo/OpenCode compatibility synchronization. A runtime HTTP 400 could therefore make saved providers disappear from Settings, remove Kilo Auto Free from the model selector, make Jev status unreadable, and prevent adding any provider.

Fixed in:

`#101 — Make provider registry resilient to runtime sync failures`

Squash merge commit:

`9e7f8dec23f123e3816c4b52b95f3638fdcaef9f`

Behavior now:

- `providers.json` is authoritative and remains readable/editable even when runtime provider synchronization fails
- legacy runtime-provider import is best-effort only when no TL Studio registry exists yet
- runtime/Kilo provider synchronization is compatibility-only and cannot brick TL Studio Provider Settings
- provider Save persists TL Studio registry + credential vault first; runtime sync errors are logged but do not reject the save
- provider Delete removes TL Studio registry/vault/cache first; runtime cleanup is best-effort
- Provider catalog can fall back entirely to TL Studio-owned state if runtime `/provider` fails
- `kilo-auto/free` remains available as the only normal Kilo hosted model during runtime catalog failure
- saved custom providers remain in the selector with only their explicitly saved models
- vault-backed provider credentials remain reflected as connected even if Kilo's runtime auth catalog is unavailable
- the Provider API-key field no longer uses `type=password` or `autocomplete=new-password`; it remains visually masked but requests no password-manager storage
- Provider Settings security copy now correctly describes the TL Studio-owned credential vault

Regression coverage deliberately forces HTTP 400 responses for runtime provider sync and catalog reads and proves that provider Settings, Kilo Auto Free, saved custom models, vault credentials, and new Provider saves remain functional.

Validation for PR #101:

- CI: success
- Custom Provider Contract: success
- strict Browser TypeScript/build: success
- Go tests/vet: success
- runtime product contract: success
- runtime prompt/write E2E: success
- supported-platform cross-compiles: success

Post-merge Windows Preview Build:

`36315553592`

Head:

`9e7f8dec23f123e3816c4b52b95f3638fdcaef9f`

Result:

`success`

Artifact:

`TL-Studio-0.4.0-alpha.5-Windows-x64-Preview`

Artifact ID:

`10929939701`

GitHub artifact digest:

`sha256:4bc335499f90203691d96b4d031c4d91e711cd79f6f46de8b8c3ae38347ff582`

Inner product ZIP SHA256:

`5602bfabeb59ed99bc4ed6e416f25b6da12bbe5963bcf749d82111c3a7594ff1`

Hands-on validation should continue from this Preview. Re-test:

- previously saved Provider cards appear without hardcoding
- Model Selector contains Backend default plus Kilo Auto Free and only saved custom models
- API-key field does not trigger Chrome password generation/storage UX
- OpenRouter can be saved manually
- Jev setup can reuse/add OpenRouter without runtime 400 blocking it
- invalid OpenRouter key still fails discovery before model listing

## Next product phase

Phase 2 is complete according to its ownership criteria.

After the Plugin/MCP milestone is validated on `dev`, the previously identified runtime-independence work remains a separate future architectural phase: native session ownership cleanup, runtime-optional startup, and eventually lazy compatibility-engine startup. Do not mix that work into the Plugin/MCP milestone.

## Development behavior

- Continue autonomously through the current milestone.
- Prefer implementation and validation over repeatedly asking whether to continue.
- Preserve existing working behavior unless the milestone explicitly changes it.
- Keep engine-specific implementation details behind TL Studio-owned contracts/adapters.
- Before a session limit, update this file with current branch, version, commit/PR, completed work, tests, remaining work, exact next action, and Preview Build status.

## Communication and bidirectional-text rule

The project owner primarily communicates in Persian.

- Write Persian explanations in an RTL-friendly form.
- Do not start a Persian sentence with an English word, identifier, version, branch name, filename, or technical term.
- Avoid mixing Persian and English fragments in the same sentence when that can break bidirectional rendering.
- Put English technical terms, identifiers, commands, paths, branch names, filenames, versions, and code on their own separate lines whenever practical.
- Code blocks remain left-to-right.
- Keep architecture explanations simple and direct.
