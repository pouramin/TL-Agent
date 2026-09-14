(() => {
  "use strict";
  const K = window.KLU;

  K.refreshAll = async () => {
    K.showError("");
    try {
      await Promise.all([K.checkBackend(), K.loadLocalStatus(), K.loadAgentsAndModels(), K.loadSessions(), K.loadActiveSessions()]);
      if (K.state.session) {
        const fresh = K.state.sessions.find((item) => item.id === K.state.session.id); if (fresh) K.state.session = fresh;
        await Promise.all([K.loadMessages(), K.loadAttention()]); K.renderMessages(); K.renderSessionHeader();
      }
      K.renderSessions();
    } catch (err) { K.showError(err.message || String(err)); }
  };

  const wire = () => {
    const e = K.els;
    e.pickProject.addEventListener("click", K.pickProject); e.emptyPickProject.addEventListener("click", K.pickProject); e.manualProject.addEventListener("click", K.openManualProject);
    e.pathForm.addEventListener("submit", K.setManualProject); e.newSession.addEventListener("click", K.newSession); e.emptyNewSession.addEventListener("click", K.newSession);
    e.sendButton.addEventListener("click", K.sendPrompt); e.refreshButton.addEventListener("click", K.refreshAll); e.accountButton.addEventListener("click", K.signInKilo);
    e.authCancel.addEventListener("click", K.cancelAuth); e.authOpen.addEventListener("click", () => { if (K.state.authURL) window.open(K.state.authURL, "_blank", "noopener,noreferrer"); });
    e.authCode.addEventListener("click", K.copyAuthCode); e.agentSelect.addEventListener("change", K.switchAgent); e.modelSelect.addEventListener("change", K.switchModel);
    e.prompt.addEventListener("input", K.resizePrompt); e.prompt.addEventListener("keydown", (event) => { if (event.key === "Enter" && !event.shiftKey) { event.preventDefault(); K.sendPrompt(); } });
  };

  const init = async () => {
    wire(); K.resizePrompt();
    try {
      await K.loadLocalStatus(); if (!await K.checkBackend()) return;
      await Promise.all([K.loadAgentsAndModels(), K.loadSessions(), K.loadActiveSessions()]); K.renderSessions();
    } catch (err) { K.showError(err.message || String(err)); }
  };
  init();
})();
