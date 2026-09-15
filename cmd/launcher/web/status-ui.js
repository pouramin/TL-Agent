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

  K.renderMessages = (...args) => {
    const result = baseRenderMessages(...args);
    normalizeCancellation();
    return result;
  };
})();
