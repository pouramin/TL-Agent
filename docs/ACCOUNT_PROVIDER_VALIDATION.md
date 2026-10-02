# Account Provider manual validation

This checklist is the manual release gate for the TL Studio 0.6 account-provider milestone.

Automated CI already covers the provider-account HTTP contract, credential separation, OAuth state/PKCE behavior with local fixtures, refresh/revocation logic, model discovery, Native Agent execution, Browser smoke, Windows packaging, and the absence of a Kilo runtime. The checks below cover the parts that require real provider accounts and consent screens.

## Review build

Use the Windows x64 review package produced by the latest validated 0.6 pull request.

Run:

    .\tl-studio.exe

Normal TL Studio startup must not require a runtime flag or a second Agent runtime. ChatGPT validation may lazily launch OpenAI's official Codex CLI only after the ChatGPT account integration is used.

## Hugging Face

No provider-specific setup should be required before sign-in.

1. Open Settings → Providers.
2. Find Hugging Face under Account providers.
3. Choose Sign in with Hugging Face.
4. Complete the Hugging Face consent flow in the system browser.
5. Return to TL Studio and confirm the account shows Connected.
6. Confirm Hugging Face models appear in the normal model selector.
7. Select a tool-capable model and run a simple prompt that reads a project file.
8. Restart TL Studio and confirm the connection and discovered provider survive restart.
9. Sign out and confirm the account-managed provider disappears.
10. If a manual Hugging Face API credential was configured before account sign-in, confirm it is still available after sign-out.

Expected security boundary:

- OAuth access/refresh tokens stay in the TL Studio credential vault.
- providers.json contains provider/model metadata only.
- Browser storage and Browser API responses contain no OAuth tokens.

## Google / Gemini

### Google Cloud prerequisites

Use a Google Cloud project intended for the test.

1. Enable the Google Generative Language API.
2. Configure the Google Auth Platform / OAuth consent screen.
3. For a test configuration, add the Google account used for validation as a test user.
4. Create an OAuth client with application type Desktop app.
5. Copy the Desktop OAuth Client ID.
6. Note the Google Cloud Project ID.

The Desktop OAuth Client ID is an identifier, not a client secret. Do not enter or store a Google client secret in TL Studio.

### TL Studio setup and sign-in

1. Open Settings → Providers.
2. Find Google / Gemini.
3. Choose Set up.
4. Enter the Google Cloud Project ID.
5. Enter the Desktop OAuth Client ID.
6. Save setup.
7. Confirm the account changes from setup-required to available for sign-in.
8. Choose Sign in with Google / Gemini.
9. Complete consent in the system browser.
10. Confirm the callback returns to a temporary URL in the form:

       http://127.0.0.1:<random-port>

   It must not use a path on the TL Studio control server.
11. Return to TL Studio and confirm the account shows Connected.
12. Confirm Gemini generateContent models appear in the normal model selector.
13. Select a Gemini model and run:
    - a plain text prompt;
    - a prompt that causes a native project-file tool call;
    - a continuation after that tool call.
14. Restart TL Studio and confirm setup/account state persists.
15. Sign out and confirm Google token revocation is attempted and the account-managed Gemini provider is removed.

Expected local state:

- google-gemini.json contains only the non-secret Client ID and Project ID.
- OAuth access/refresh tokens stay in the TL Studio credential vault.
- providers.json may contain the non-secret Project ID and model metadata, but no tokens.

Environment variables remain development overrides:

    TL_STUDIO_GOOGLE_CLIENT_ID
    TL_STUDIO_GOOGLE_PROJECT_ID

A production build should eventually ship TL Studio's own registered Desktop OAuth Client ID so normal users do not need to supply an OAuth client ID themselves.

## OpenRouter

1. Choose Sign in with OpenRouter.
2. Complete its documented OAuth + PKCE flow.
3. Confirm models are discovered through the normal provider path.
4. Run a Native Agent prompt with a tool-capable model.
5. Sign out and confirm any pre-existing manual OpenRouter API credential remains intact.

## ChatGPT / Codex

ChatGPT account support uses OpenAI's official Codex login and model surfaces.

### Manual Windows status — 2026-09-29

Core runtime validation passed on the current 0.6 development preview after PR #155.

Verified with a real ChatGPT-plan account on Windows:

- browser authorization completed successfully;
- account state returned to TL Studio as Connected;
- ChatGPT-plan models were discovered and selectable;
- the selected model ID was routed correctly and reported correctly by the bridge;
- simple warm responses completed in roughly 3–5 seconds after the persistent Codex app-server optimization;
- project-file read completed through TL Studio Tools;
- multi-file read and project Search completed through TL Studio Tools;
- a write operation triggered TL Studio Permission handling and modified only the requested content;
- a read → write → read continuation completed correctly through the TL Studio model → Tool → model loop;
- Terminal execution completed through the TL Studio Tool path.

The full ChatGPT/Codex Windows checklist is now closed.

Additional validation completed:

- after fully closing and reopening TL Studio, the ChatGPT account remained connected and the discovered models were still usable;
- after signing out, fully closing, and reopening TL Studio, the ChatGPT account remained signed out and required a fresh sign-in before use.


Prerequisite:

- either `codex` from the official OpenAI Codex CLI is available on PATH;
- or `npx` is available so TL Studio can run `npx @openai/codex`;
- or an explicit official Codex executable path is configured from Provider Settings.

Validation:

1. Open Settings → Providers.
2. Find ChatGPT / Codex under Account providers.
3. Confirm the entry is enabled rather than marked Deferred.
4. If Codex is not auto-detected, choose Codex setup and configure the official Codex executable path.
5. Choose Sign in with ChatGPT / Codex.
6. Confirm the system browser opens the authorization URL returned by the official Codex App Server.
7. Complete ChatGPT authorization.
8. Return to TL Studio and confirm the account shows Connected.
9. Confirm the connected account label/plan metadata is shown when available.
10. Confirm ChatGPT-plan models discovered through Codex appear in the normal TL Studio model selector.
11. Run a plain prompt and confirm a normal assistant response.
12. Run a prompt that requires a TL Studio project-file Tool call.
13. Approve/reject the TL Studio Permission prompt as appropriate and confirm Tool execution happens through TL Studio rather than Codex modifying the project directly.
14. Confirm the continuation after the Tool result completes normally.
15. ✅ Restart TL Studio and confirm the ChatGPT account remains usable through the isolated Codex auth store.
16. ✅ Refresh/discover models again and confirm the model catalog still resolves.
17. ✅ Sign out and confirm the account-managed ChatGPT provider disappears from the TL Studio model selector and remains signed out after restart.

Expected security/product boundary:

- TL Studio does not read, copy, serialize, or expose raw ChatGPT OAuth tokens.
- Codex auth material stays inside the official Codex client's TL Studio-specific isolated `CODEX_HOME`.
- Browser state contains only semantic account/login state, not raw ChatGPT credentials.
- The Codex model bridge runs from an empty temporary working directory.
- User/project Codex config and rules are ignored for the bridge turn.
- The bridge uses a read-only sandbox, approval policy `never`, and web search disabled.
- Project mutations, Tool execution, Permission decisions, Session persistence, and the outer model → Tool → model loop remain TL Studio-owned.
- No private OAuth client, ChatGPT browser cookie, private session token, or undocumented ChatGPT backend endpoint is used.

If ChatGPT login succeeds but model execution fails, record separately whether the failure is:
- Codex executable resolution;
- account entitlement/model availability;
- Codex App Server login/account state;
- model bridge structured output;
- TL Studio Tool-call conversion or continuation.

Do not substitute an OpenAI API key for this test; the purpose is specifically to validate ChatGPT-plan account access.

## Claude / Claude Code

Claude account support uses Anthropic's official Claude Code browser-login and non-interactive model surfaces.

Automated coverage verifies:

- the official Claude Code CLI resolves on Windows;
- account login/status/logout use the isolated TL Studio Claude config directory;
- manual Anthropic API configuration remains independent from account-backed usage;
- alternate API/cloud credential environment sources are scrubbed from the account helper;
- account-backed models use the separate `claude-account` runtime provider;
- structured Tool requests return to the TL Studio Tool/Permission loop rather than Claude Code executing project tools;
- restart and logout persistence are covered by the fake-CLI regression fixture.

Prerequisite:

- either the official `claude` executable is available on PATH;
- or `npx` is available so TL Studio can run `npx @anthropic-ai/claude-code`;
- or an explicit official Claude Code executable path is configured from Provider Settings.

Real Windows validation:

1. Open Settings → Providers and find Claude.
2. Confirm the card offers both Sign in and API configuration.
3. Choose Sign in and confirm the official Claude Code browser authorization opens.
4. Complete authorization with an eligible Claude.ai subscription.
5. Return to TL Studio and confirm the card shows Account connected.
6. Confirm `Claude / Account` appears in the normal model selector with Sonnet, Opus, and Haiku aliases.
7. Run a plain prompt and record response latency.
8. Run a prompt requiring a TL Studio project-file Tool call and confirm Permission/Tool execution remains TL Studio-owned.
9. If an Anthropic API key is also configured, confirm both account and API provider paths remain selectable independently.
10. Restart TL Studio and confirm the account remains connected and usable.
11. Sign out, restart again, and confirm account-backed models remain signed out while any manual Anthropic API configuration is preserved.

Expected security/product boundary:

- TL Studio does not read or serialize the raw Claude.ai OAuth credential.
- Claude Code stores account auth under the TL Studio-specific `CLAUDE_CONFIG_DIR`.
- inherited API-key, bearer-token, profile, Bedrock, Vertex, Foundry, and external OAuth-token environment sources are not passed into the account helper;
- Claude Code built-in tools and MCP tools are disabled for TL Studio model turns;
- the bridge working directory is an empty temporary directory;
- project mutations, Tool execution, Permission decisions, Session persistence, and the outer model → Tool → model loop remain TL Studio-owned.

## Deferred account integrations

The following entry should be visible but unavailable, with an explicit reason:

- GitHub Copilot

It must not expose a working Sign in action or create runtime credentials.

## Release decision

Do not promote 0.6 to stable main solely because automated CI is green.

Before stable promotion, record:

- review build tested;
- Hugging Face real sign-in result;
- Google / Gemini real sign-in result;
- model discovery result for each connected account provider;
- ChatGPT real account login and plan-backed model result — passed on Windows on 2026-09-29;
- Claude.ai real account login and subscription-backed model result;
- at least one Native Agent model/tool/model round trip for each provider intended to be declared usable — ChatGPT passed on Windows on 2026-09-29; Claude pending real-account validation;
- sign-out/restart behavior — ChatGPT passed on Windows on 2026-09-29;
- any provider-specific limitation that must be documented.

If a real provider rejects a documented flow, keep the integration on dev and record the exact provider error before changing architecture or authentication behavior.
