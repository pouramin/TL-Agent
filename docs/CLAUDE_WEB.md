[English](./CLAUDE_WEB.md) · [فارسی](./CLAUDE_WEB.fa_IR.md)

# Claude Web (Free/Pro)

TL Studio can use the Claude session already signed in to your normal Chrome profile through the **TL Studio Claude Web Bridge**.

## Requirement

The bridge extension must be installed in the same Chrome profile that is signed in to <code>claude.ai</code>.

[Install TL Studio Claude Web Bridge from the Chrome Web Store](https://chromewebstore.google.com/detail/cpellhbmfdhcgkblnmnppndmeiigmjcg)

The Store listing is intentionally unlisted. You do not need Developer mode for the production extension.

## Setup

1. Open **Settings → Providers** in TL Studio.
2. On the **Claude** card, choose **Web**.
3. If the bridge is not installed, TL Studio opens the Chrome Web Store install page automatically.
4. Install the extension in the current Chrome profile.
5. Make sure that same Chrome profile is signed in to <code>claude.ai</code>.
6. Return to TL Studio and choose **Web** again.
7. When the card shows **Web connected**, Claude Web models are available in the model selector.

## Pairing lifecycle

The bridge uses a per-pair random token and accepts requests only from TL Studio's localhost/127.0.0.1 origin. Chrome Manifest V3 may suspend the extension service worker between turns. If that clears the extension's in-memory pairing, TL Studio automatically re-pairs with the existing local token and retries the interrupted transport command once. A normal service-worker wake-up should therefore not require the user to sign in again.

## Security boundary

- TL Studio does not read, export, serialize, or store Claude cookies or browser credentials.
- Authenticated Claude requests run inside the normal <code>claude.ai</code> page context with browser-owned credentials.
- The bridge accepts only inference transport commands (<code>probe</code> and <code>complete</code>).
- The extension has no TL Studio filesystem, Terminal, Permission, Session, Tool, or project endpoint access.
- TL Studio remains the owner of the Agent loop, tools, permissions, files, terminal, sessions, and persistence.

## Troubleshooting

**The bridge is not installed**

Choose **Claude → Web**. TL Studio opens the Store page automatically. Install the extension, return to TL Studio, and choose **Web** again.

**Claude is not signed in**

Open <code>claude.ai</code> in the same Chrome profile and sign in, then reconnect **Web** in TL Studio.

**The extension service worker restarted**

No manual action should be required. TL Studio automatically re-pairs and retries once. If the request still fails, reconnect **Claude → Web** from Provider Settings.

**Review/development build**

The bundled unpacked extension remains available for development testing, but normal users should use the Chrome Web Store build.
