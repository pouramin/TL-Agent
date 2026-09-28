# Account Provider manual validation

This checklist is the manual release gate for the TL Studio 0.6 account-provider milestone.

Automated CI already covers the provider-account HTTP contract, credential separation, OAuth state/PKCE behavior with local fixtures, refresh/revocation logic, model discovery, Native Agent execution, Browser smoke, Windows packaging, and the absence of a Kilo runtime. The checks below cover the parts that require real provider accounts and consent screens.

## Review build

Use the Windows x64 review package produced by the latest validated 0.6 pull request.

Run:

    .\tl-studio.exe

Normal startup must not require a runtime flag or a second executable.

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

## Deferred account integrations

The following entries should be visible but unavailable, with an explicit reason:

- ChatGPT / Codex
- Claude
- GitHub Copilot

They must not expose a working Sign in action, create runtime credentials, or introduce another Agent runtime.

## Release decision

Do not promote 0.6 to stable main solely because automated CI is green.

Before stable promotion, record:

- review build tested;
- Hugging Face real sign-in result;
- Google / Gemini real sign-in result;
- model discovery result for each connected account provider;
- at least one Native Agent model/tool/model round trip for each provider intended to be declared usable;
- sign-out/restart behavior;
- any provider-specific limitation that must be documented.

If a real provider rejects a documented flow, keep the integration on dev and record the exact provider error before changing architecture or authentication behavior.
