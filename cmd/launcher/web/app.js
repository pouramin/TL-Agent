(() => {
  "use strict";
  const K = window.KLU;

  K.refreshAll = async () => {
    K.showError("");
    try {
      await Promise.all([K.checkBackend(), K.loadLocalStatus(), K.loadCatalog(), K.loadSessions(), K.loadActiveSessions()]);
      if (K.state.session) {
        const fresh = K.state.sessions.find((item) => item.id === K.state.session.id);
        if (fresh) K.state.session = fresh;
        await Promise.all([K.loadMessages(), K.loadAttention()]);
        K.renderMessages();
        K.renderSessionHeader();
        K.syncSelectors();
      }
      K.renderSessions();
    } catch (err) { K.showError(err.message || String(err)); }
  };

  const wire = () => {
    const e = K.els;
    e.pickProject.addEventListener("click", K.pickProject);
    e.emptyPickProject.addEventListener("click", K.pickProject);
    e.manualProject.textContent = "Browse…";
    e.manualProject.title = "Choose a project folder";
    e.manualProject.addEventListener("click", K.pickProject);
    e.pathForm.addEventListener("submit", K.setManualProject);
    e.newSession.addEventListener("click", K.newSession);
    e.emptyNewSession.addEventListener("click", K.newSession);
    e.sendButton.addEventListener("click", K.sendPrompt);
    e.refreshButton.addEventListener("click", K.refreshAll);
    e.accountButton.addEventListener("click", K.signInKilo);
    e.authCancel.addEventListener("click", K.cancelAuth);
    e.authOpen.addEventListener("click", () => {
      if (K.state.authURL) window.open(K.state.authURL, "_blank", "noopener,noreferrer");
    });
    e.authCode.addEventListener("click", K.copyAuthCode);
    e.agentSelect.addEventListener("change", K.switchAgent);
    e.modelSelect.addEventListener("change", K.switchModel);
    e.prompt.addEventListener("input", K.resizePrompt);
    e.prompt.addEventListener("keydown", (event) => {
      if (event.key === "Enter" && !event.shiftKey) {
        event.preventDefault();
        K.sendPrompt();
      }
    });
    window.addEventListener("beforeunload", K.stopEvents);
  };

  const init = async () => {
    wire();
    K.resizePrompt();
    try {
      await K.loadLocalStatus();
      if (!await K.checkBackend()) return;
      await Promise.all([K.loadCatalog(), K.loadSessions(), K.loadActiveSessions()]);
      K.renderSessions();
      K.startEvents();
    } catch (err) { K.showError(err.message || String(err)); }
  };

  const loadDiagnostics = () => new Promise((resolve) => {
    if (K.__diagnosticsUiInstalled) return resolve();
    const script = document.createElement("script");
    script.src = "/diagnostics-ui.js";
    script.async = false;
    script.onload = resolve;
    script.onerror = () => {
      console.warn("[TL Agent] Session diagnostics UI failed to load");
      resolve();
    };
    document.head.appendChild(script);
  });

  loadDiagnostics().finally(init);
})();
