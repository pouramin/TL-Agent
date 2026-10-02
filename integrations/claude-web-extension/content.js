"use strict";

const isTLStudioLoopback = () =>
  location.protocol === "http:" &&
  (location.hostname === "127.0.0.1" || location.hostname === "localhost");

if (isTLStudioLoopback()) {
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
    });
  });

  try {
    chrome.runtime.sendMessage({ type: "tlstudio-ensure-polling" });
  } catch {}
}
