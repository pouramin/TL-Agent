# Kilo API Contract

This document pins the browser client in this repository to the public Kilo Protocol v2 contract shipped with **Kilo Code v7.6.2**.

- Upstream repository: `Kilo-Org/kilocode`
- Upstream tag: `v7.6.2`
- Upstream commit: `3d04228b6a642acb3daf68a649269618b6018250`
- Bundled runtime version: see `/KILO_VERSION`

## Rule

The UI and launcher must integrate against the public Protocol v2 routes and schemas for the pinned Kilo version. Internal storage structures and undocumented compatibility shapes are not part of this contract.

The only non-`/api/*` routes intentionally used are Kilo's official Provider HttpApi routes for provider connection state and OAuth. Those routes are used by Kilo's own current clients and are isolated inside the browser adapter.

If Kilo is upgraded, this document and the adapter tests must be reviewed before the bundled runtime version is changed.

## Canonical public routes used by TL-Agent

### Health and location

- `GET /api/health`
- `GET /api/location`

`/api/health` must return `{ "healthy": true }` when the v2 API is ready.

### Agents

- `GET /api/agent`
- Response payload: location-wrapped `Agent.Info[]`
- `Agent.Info` fields used by the UI:
  - `id: string`
  - `model?: Model.Ref`
  - `description?: string`
  - `mode: "subagent" | "primary" | "all"`
  - `hidden: boolean`
  - `color?`
  - `permissions`

The client must not rely on legacy `name` or `displayName` fields.

### Models

- `GET /api/model`
- Response payload: location-wrapped `Model.Info[]`
- `Model.Info` fields used by the UI:
  - `id`
  - `providerID`
  - `name`
  - `capabilities`
  - `variants`
  - `time.released`
  - `cost`
  - `status`
  - `enabled`
  - `limit`

A selected model is represented as a `Model.Ref`:

```json
{
  "id": "model-id",
  "providerID": "provider-id",
  "variant": "optional-variant"
}
```

### Providers

Protocol v2 provider metadata:

- `GET /api/provider`
- `GET /api/provider/:providerID`
- Response type: location-wrapped `Provider.Info`

Provider data is used for availability/configuration display only. Model enumeration comes from `/api/model`, not from provider-internal model maps.

Kilo's official Provider HttpApi is used only where Protocol v2 does not currently expose equivalent state:

- `GET /provider` — connected-provider and provider-default state
- `POST /provider/kilo/oauth/authorize` — start Kilo device authorization
- `POST /provider/kilo/oauth/callback` — wait for/complete Kilo device authorization

These calls must remain isolated in `kilo-api.js`.

### Sessions

- `GET /api/session?order=desc&limit=...`
- `POST /api/session`
- `GET /api/session/active`
- `GET /api/session/:sessionID`
- `POST /api/session/:sessionID/agent`
- `POST /api/session/:sessionID/model`
- `POST /api/session/:sessionID/prompt`
- `POST /api/session/:sessionID/wait`
- `POST /api/session/:sessionID/interrupt`
- `GET /api/session/:sessionID/context`
- `GET /api/session/:sessionID/history`
- `GET /api/session/:sessionID/event`
- `GET /api/session/:sessionID/message/:messageID`

`Session.Info` fields used by the UI:

- `id`
- `projectID`
- `agent?`
- `model?`
- `cost`
- `tokens`
- `time.created`
- `time.updated`
- `title`
- `location`
- `subpath?`
- `revert?`

### Prompt admission

`POST /api/session/:sessionID/prompt`

Payload:

```json
{
  "prompt": {
    "text": "...",
    "files": [],
    "agents": []
  },
  "delivery": "steer",
  "resume": true
}
```

`delivery` is optional and is one of:

- `steer`
- `queue`

The response contains a durable admission receipt (`SessionInput.Admitted`). Prompt admission and provider execution are separate concepts; the UI must not treat the HTTP response as the assistant reply.

### Session messages

- `GET /api/session/:sessionID/message?order=asc&limit=...`
- Response: `{ data: SessionMessage.Message[], cursor: { previous?, next? } }`

This is the canonical projected conversation model for non-streaming/reconnect rendering.

Supported message variants in v7.6.2:

- `agent-switched`
- `model-switched`
- `user`
- `synthetic`
- `system`
- `shell`
- `assistant`
- `compaction`

Assistant message content is stored in `assistant.content[]`, with these variants:

- `text`
  - `id`
  - `text`
- `reasoning`
  - `id`
  - `text`
  - optional provider metadata/time
- `tool`
  - `id`
  - `name`
  - `state`
  - optional provider metadata
  - time information

Tool states:

- `pending`
- `running`
- `completed`
- `error`

Do not use the internal/legacy `info + parts` representation in the browser adapter.

### Streaming and durable events

Global server event stream:

- `GET /api/event` (SSE)

Per-session durable replay/tail stream:

- `GET /api/session/:sessionID/event?after=...` (SSE)

The Session v2 event family contains, among others:

- `session.next.agent.switched`
- `session.next.model.switched`
- `session.next.prompt.admitted`
- `session.next.prompted`
- `session.next.context.updated`
- `session.next.synthetic`
- `session.next.shell.started`
- `session.next.shell.ended`
- `session.next.step.started`
- `session.next.step.ended`
- `session.next.step.failed`
- `session.next.text.started`
- `session.next.text.delta` (live-only)
- `session.next.text.ended`
- `session.next.reasoning.started`
- `session.next.reasoning.delta` (live-only)
- `session.next.reasoning.ended`
- `session.next.tool.input.started`
- `session.next.tool.input.delta` (live-only)
- `session.next.tool.input.ended`
- `session.next.tool.called`
- `session.next.tool.progress`
- `session.next.tool.success`
- `session.next.tool.failed`
- `session.next.retried`
- compaction events
- revert events

For reconnect safety, projected messages are the source of truth. Live SSE deltas are presentation events and must not be treated as durable history.

### Permissions

- `GET /api/session/:sessionID/permission`
- `GET /api/session/:sessionID/permission/:requestID`
- `POST /api/session/:sessionID/permission/:requestID/reply`

Permission reply payload:

```json
{
  "reply": "once"
}
```

Allowed replies:

- `once`
- `always`
- `reject`

A pending `Permission.Request` includes:

- `id`
- `sessionID`
- `action`
- `resources[]`
- optional `save[]`
- optional `metadata`
- optional tool `source`

### Questions

- `GET /api/session/:sessionID/question`
- `POST /api/session/:sessionID/question/:requestID/reply`
- `POST /api/session/:sessionID/question/:requestID/reject`

A question request may contain multiple questions. Each question can expose:

- `header`
- `question`
- `options[]`
- `multiple?`
- `custom?`

Reply shape:

```json
{
  "answers": [
    ["Selected label"],
    ["One", "Two"]
  ]
}
```

The outer array follows question order; each inner array contains selected/custom answer strings for that question.

## Kilo account OAuth

The official Kilo clients use provider OAuth for provider `kilo`, method `0`:

1. authorize
2. display verification URL and device code/instructions
3. wait on callback
4. refresh Kilo/profile/provider state

TL-Agent matches this client flow through the adapter and does not infer OAuth completion from model responses.

## Browser adapter policy

All browser-side Kilo calls must go through `cmd/launcher/web/kilo-api.js`. UI modules must not construct Kilo endpoint paths directly.

The adapter owns:

- response envelope handling
- query construction
- model refs
- session CRUD and switching
- prompt admission
- message pagination
- SSE connection/reconnection
- permission/question replies
- provider connection state
- Kilo OAuth calls

This isolates future Kilo upgrades to one integration boundary.

## Contract enforcement

`scripts/check-kilo-v2-contract.py` is run by CI against the actual pinned Kilo binary downloaded from the official Kilo GitHub release. It verifies the core runtime response shapes used by the UI.

A Protocol v2 refactor must not be merged if the real-runtime contract job fails.

## Upgrade checklist

Before changing `/KILO_VERSION`:

1. compare upstream `packages/protocol/src/groups/*` against this contract;
2. compare `packages/schema/src/session*.ts`, `agent.ts`, `model.ts`, `provider.ts`, `permission.ts`, and `question.ts`;
3. review `specs/v2/schema-changelog.md`;
4. update adapter tests and fixtures;
5. run the real-runtime contract and smoke tests;
6. only then publish a release with the new Kilo binary.
