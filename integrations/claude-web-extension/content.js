"use strict";

(() => {
  if (globalThis.__tlStudioClaudeWebBridgeInstalled) return;
  globalThis.__tlStudioClaudeWebBridgeInstalled = true;

  const isTLStudioLoopback = () =>
    location.protocol === "http:" &&
    (location.hostname === "127.0.0.1" || location.hostname === "localhost");

  if (!isTLStudioLoopback()) return;

  window.addEventListener("message", (event) => {
    if (event.source !== window || event.origin !== location.origin) return;
    const message = event.data;
    if (!message || message.type !== "tlstudio-claude-web-pair") return;

    const token = String(message.token || "");
    const origin = String(message.origin || location.origin);
    if (!token || origin !== location.origin) return;

    void chrome.runtime.sendMessage({
      type: "tlstudio-pair",
      token,
      origin
    }).then((response) => {
      window.postMessage({
        type: "tlstudio-claude-web-pair-result",
        token,
        response: response || null
      }, location.origin);
    }).catch((error) => {
      window.postMessage({
        type: "tlstudio-claude-web-pair-result",
        token,
        response: {
          ok: false,
          connected: false,
          error: String(error?.message || error)
        }
      }, location.origin);
    });
  });

  try {
    chrome.runtime.sendMessage({ type: "tlstudio-ensure-polling" });
  } catch {}
})();
