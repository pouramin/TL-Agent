# Kilo API Contract

This repository bundles **Kilo Code v7.6.2** and intentionally follows the same product-facing HTTP API used by Kilo's official VS Code client for local coding sessions.

- Upstream repository: `Kilo-Org/kilocode`
- Upstream tag: `v7.6.2`
- Upstream commit: `3d04228b6a642acb3daf68a649269618b6018250`
- Bundled runtime version: see `/KILO_VERSION`
- Browser adapter: `cmd/launcher/web/kilo-api.js`

## Why this contract exists

`@kilocode/sdk/v2/client` in Kilo v7.6.2 exposes more than one API generation at the same time:

1. the **product/current HttpApi**, used by Kilo's official VS Code client for coding;
2. the experimental **Protocol v2 `/api/*`** surface.

The package name alone therefore does not mean every `/api/*` route is the product runtime used by the official UI.

TL-Agent originally migrated its main chat path to the experimental Protocol v2 surface. Real Windows testing exposed the mismatch: `/api/agent` returned the core `build` and `plan` agents, while the product agent layer in Kilo patches `build` into `code` and adds Kilo's product agents. The experimental Session runner also has documented parity gaps in v7.6.2.

**Rule:** TL-Agent's primary coding path must follow the same generated SDK methods actually invoked by the official Kilo client for the pinned version. Experimental `/api/*` routes are not used for the primary chat runtime unless upstream itself migrates the official client and runtime parity is verified.

## Project routing

Every project-scoped product request carries the selected project explicitly as the `directory` query parameter. The launcher additionally injects `x-kilo-directory` at the reverse-proxy boundary.

This dual routing is deliberate:

- it matches how the official SDK/client supplies `directory`;
- it prevents a server CWD from silently becoming the active project;
- changing the project in TL-Agent must also change agents, providers, sessions, messages, permissions, questions, events, and tool filesystem operations.

`GET /path?directory=...` is the runtime routing assertion used by CI.

## Product routes used by TL-Agent

### Runtime health and path

- `GET /global/health`
- `GET /path?directory=...`

### Agents

- `GET /agent?directory=...`

The product agent response uses fields such as:

- `name`
- `displayName?`
- `description?`
- `mode`
- `hidden`
- `permission`
- `model?`

Kilo v7.6.2 constructs base agents including `build` and `plan`, then its Kilo product patch renames `build` to **`code`** and adds/patches product agents. The contract test therefore requires a visible `code` agent and rejects an exposed raw `build` agent on the product route.

TL-Agent normalizes `name` as the stable selection value and uses `displayName` when available for display.

### Providers and models

- `GET /provider?directory=...`

The product provider response is:

```json
{
  "all": [],
  "default": {},
  "connected": [],
  "failed": []
}
```

Models are enumerated from each provider's `models` collection because this is the provider surface used by the official product client in v7.6.2.

Kilo's authenticated free default remains:

```text
providerID: kilo
modelID: kilo-auto/free
```

TL-Agent keeps an internal browser selection as `{ providerID, id, variant? }` and converts it at the adapter boundary to the product SDK model reference `{ providerID, modelID }`.

### Sessions

- `GET /session?directory=...`
- `POST /session?directory=...`
- `GET /session/status?directory=...`
- `GET /session/:sessionID?directory=...`
- `GET /session/:sessionID/message?directory=...`
- `POST /session/:sessionID/prompt_async?directory=...`
- `POST /session/:sessionID/abort?directory=...`

The official VS Code client in Kilo v7.6.2 sends normal coding messages through `client.session.promptAsync(...)`. TL-Agent mirrors that path.

A text prompt is sent as:

```json
{
  "agent": "code",
  "model": {
    "providerID": "kilo",
    "modelID": "kilo-auto/free"
  },
  "parts": [
    {
      "type": "text",
      "text": "..."
    }
  ]
}
```

Agent and model are sent explicitly with each prompt. TL-Agent does not depend on the experimental `/api/session/:id/agent`, `/model`, or `/prompt` routes for the product chat path.

### Messages

Production Session messages are returned as:

```json
[
  {
    "info": { "role": "user" },
    "parts": []
  },
  {
    "info": { "role": "assistant" },
    "parts": []
  }
]
```

This `info + parts` envelope is the canonical message representation for the product route in v7.6.2.

Relevant Part variants include:

- `text`
- `reasoning`
- `tool`
- `subtask`
- file-related parts

A tool part contains its tool name and state. Tool state can be pending/running/completed/error and includes input/output/error metadata as appropriate.

Projected messages remain the reconnect-safe rendering source of truth.

### Events

- `GET /global/event?directory=...` (SSE)

The global event stream is wrapped with project/location metadata. The event itself is under `payload` and uses product event names such as:

- `message.updated`
- `message.part.updated`
- `session.updated`
- `session.status`
- `session.idle`
- permission/question events
- file edit events

The adapter unwraps the outer global-event envelope before UI dispatch.

TL-Agent uses events for responsive refresh and also polls Session status/messages after a prompt as a conservative fallback. It does not treat an SSE delta as durable conversation history.

### Permissions

- `GET /permission?directory=...`
- `POST /permission/:requestID/reply?directory=...`

Pending permission requests are filtered by `sessionID` in the adapter.

Production `PermissionRequest` fields used by the UI:

- `id`
- `sessionID`
- `permission`
- `patterns[]`
- `metadata`
- `always[]`
- optional tool source

Reply values are exactly:

- `once`
- `always`
- `reject`

### Questions

- `GET /question?directory=...`
- `POST /question/:requestID/reply?directory=...`
- `POST /question/:requestID/reject?directory=...`

Requests are filtered by `sessionID` in the adapter. Questions support option lists, single/multiple selection, optional custom answers, and an optional default single-select label.

### Kilo account OAuth

- `POST /provider/kilo/oauth/authorize`
- `POST /provider/kilo/oauth/callback`

TL-Agent uses Kilo's normal device authorization flow and refreshes product provider state afterward.

## Browser adapter policy

All browser-side Kilo calls must go through:

`cmd/launcher/web/kilo-api.js`

UI modules must not construct Kilo endpoint paths directly. The adapter owns:

- directory routing
- response unwrapping
- model reference conversion (`id` ↔ `modelID`)
- Agent/provider/session endpoints
- `prompt_async`
- Session status/messages
- global SSE envelope unwrapping
- Permission/Question routes
- Kilo OAuth

This makes an upstream Kilo upgrade an explicit adapter/contract change rather than a scattered UI change.

## Automated contract gates

### Product contract

`scripts/check-kilo-product-contract.py`

CI downloads the exact official Kilo binary pinned in `KILO_VERSION`, starts it through the TL-Agent launcher, and verifies:

- product health
- selected-project routing
- product Agent list contains `code`, not raw `build`
- Provider response shape
- production Session creation/list/messages/status
- production message envelope is `info + parts`
- Permission/Question list shapes
- `/global/event` SSE is available

### Prompt + real write-tool E2E

`scripts/check-kilo-prompt-e2e.py`

CI also starts a local fake OpenAI-compatible LLM with no external API key or cost. The project config mirrors Kilo v7.6.2's own `testProviderConfig` format (`provider`, singular).

The E2E must pass this chain:

```text
TL-Agent launcher
→ real bundled Kilo 7.6.2
→ GET /agent returns code
→ production custom provider discovery
→ POST /session
→ POST /session/:id/prompt_async
→ fake LLM requests Kilo's real write tool
→ Kilo writes hello.txt inside the selected project
→ provider receives tool result
→ final assistant reply
→ GET /session/:id/message returns info + parts
→ global product events observed
```

The test asserts both:

```text
hello.txt == KILO_LOCAL_UI_OK
```

and the final assistant marker:

```text
E2E_PRODUCT_OK
```

A release must not be cut if this product-path E2E fails.

## Protocol v2 status

The experimental `/api/*` Protocol v2 remains valuable and may eventually replace more of the current HttpApi. In Kilo v7.6.2 it is **not** the primary TL-Agent coding contract because the official product client has not migrated its main coding flow to that surface and upstream parity documentation still lists incomplete runtime behavior.

Do not mix product and experimental Session models opportunistically. Any future migration must be version-pinned and validated against the official Kilo client plus real-runtime E2E.

## Upgrade checklist

Before changing `KILO_VERSION`:

1. inspect the generated `@kilocode/sdk/v2/client` routes for the new tag;
2. inspect the official VS Code client and identify which SDK methods it actually invokes for Agent, Provider, Session, Prompt, Permission, Question, and events;
3. inspect Kilo's product Agent patching behavior;
4. compare message/part schemas;
5. review Protocol v2 parity only as a separate potential migration;
6. update this document and `kilo-api.js`;
7. run product contract + production Prompt/write-tool E2E against the exact new binary;
8. only then publish a release.
