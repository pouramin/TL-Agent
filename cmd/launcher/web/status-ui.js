(() => {
  "use strict";
  const K = window.KLU;
  const baseRenderMessages = K.renderMessages;

  const normalizeCancellation = () => {
    const rows = K.els.conversation?.querySelectorAll(".message.error") || [];
    for (const row of rows) {
      const error = row.querySelector(".message-error-text");
      const text = String(error?.textContent || "").trim();
      if (!/\b(aborted|cancelled|canceled|interrupted)\b/i.test(text)) continue;
      row.classList.remove("error");
      row.classList.add("cancelled");
      if (error) {
        error.classList.remove("message-error-text");
        error.classList.add("message-cancelled-text");
        error.textContent = "Cancelled by user";
      }
    }
  };

  const normalizeRetryableToolErrors = () => {
    const cards = K.els.conversation?.querySelectorAll('.activity-card[data-status="failed"]') || [];
    const running = !!K.state.session && (K.state.sending || K.isSessionRunning(K.state.session.id));
    for (const card of cards) {
      const text = String(card.textContent || "");
      const invalidArguments = /invalid arguments/i.test(text)
        && /is missing and is required/i.test(text);
      if (!invalidArguments) continue;

      card.dataset.status = "retrying";
      if (running) card.open = false;

      const status = card.querySelector(".activity-status");
      if (status) status.textContent = running ? "retrying" : "invalid args";

      const meta = card.querySelector(".activity-meta");
      if (meta && !meta.textContent.trim()) {
        meta.textContent = running ? "agent is correcting the tool call" : "tool call was rejected before execution";
      }
    }
  };

  K.renderMessages = (...args) => {
    const result = baseRenderMessages(...args);
    normalizeCancellation();
    normalizeRetryableToolErrors();
    return result;
  };
})();
