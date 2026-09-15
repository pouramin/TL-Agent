(() => {
  "use strict";
  const K = window.KLU;
  const $ = (id) => document.getElementById(id);

  const ui = {
    settingsButton: $("settingsButton"),
    settingsDialog: $("settingsDialog"),
    settingsClose: $("settingsClose"),
    appearanceSelect: $("appearanceSelect"),
    fontSizeSelect: $("fontSizeSelect"),
    accountDialog: $("accountDialog"),
    accountStatus: $("accountStatus"),
    accountSignIn: $("accountSignIn"),
    accountSignOut: $("accountSignOut"),
    accountClose: $("accountClose"),
  };

  const THEME_KEY = "tl-agent.appearance";
  const FONT_KEY = "tl-agent.font-size";
  const systemTheme = window.matchMedia?.("(prefers-color-scheme: light)");

  const readSetting = (key, fallback) => {
    try { return window.localStorage.getItem(key) || fallback; }
    catch { return fallback; }
  };
  const writeSetting = (key, value) => {
    try { window.localStorage.setItem(key, value); }
    catch {}
  };

  const applyAppearance = (value) => {
    const preference = ["system", "dark", "light"].includes(value) ? value : "system";
    const resolved = preference === "system" ? (systemTheme?.matches ? "light" : "dark") : preference;
    document.documentElement.dataset.theme = preference;
    document.documentElement.dataset.resolvedTheme = resolved;
    document.documentElement.style.colorScheme = resolved;
    if (ui.appearanceSelect) ui.appearanceSelect.value = preference;
  };

  const applyFontSize = (value) => {
    const size = ["small", "default", "large"].includes(value) ? value : "default";
    document.documentElement.dataset.fontSize = size;
    if (ui.fontSizeSelect) ui.fontSizeSelect.value = size;
  };

  applyAppearance(readSetting(THEME_KEY, "system"));
  applyFontSize(readSetting(FONT_KEY, "default"));
  systemTheme?.addEventListener?.("change", () => {
    if (readSetting(THEME_KEY, "system") === "system") applyAppearance("system");
  });

  const originalRenderSessions = K.renderSessions;
  K.renderSessions = () => {
    const list = K.els.sessions;
    if (!list) return originalRenderSessions?.();
    list.textContent = "";
    if (!K.state.sessions.length) {
      const empty = document.createElement("div");
      empty.className = "sidebar-empty";
      empty.textContent = "No sessions in this project yet.";
      list.appendChild(empty);
      return;
    }

    for (const session of K.state.sessions) {
      const row = document.createElement("div");
      const open = document.createElement("button");
      const remove = document.createElement("button");
      const title = document.createElement("strong");
      const meta = document.createElement("span");

      row.className = "session-row";
      open.className = `session-item session-main${K.state.session?.id === session.id ? " active" : ""}`;
      open.type = "button";
      title.textContent = session.title || "Untitled session";
      meta.textContent = `${session.agent || "default"} · ${K.relativeTime(session.time?.updated || session.time?.created)}`;
      open.append(title, meta);
      open.addEventListener("click", () => K.selectSession(session));

      remove.className = "session-quick-delete";
      remove.type = "button";
      remove.textContent = "×";
      remove.title = `Delete ${session.title || "session"}`;
      remove.setAttribute("aria-label", `Delete ${session.title || "session"}`);
      remove.addEventListener("click", async (event) => {
        event.stopPropagation();
        await K.deleteSessionFromSidebar(session);
      });

      row.append(open, remove);
      list.appendChild(row);
    }
  };

  K.deleteSessionFromSidebar = async (session) => {
    if (!session?.id) return;
    if (!window.confirm(`Delete “${session.title || "Untitled session"}” permanently?`)) return;
    try {
      const selectedSending = K.state.session?.id === session.id && K.state.sending;
      if (K.isSessionRunning(session.id) || selectedSending) {
        await K.api.sessions.abort(session.id, { scope: "tree" }).catch(() => {});
      }
      await K.api.sessions.remove(session.id);
      if (K.state.session?.id === session.id) {
        K.state.changes = [];
        K.newSession();
      }
      await K.loadSessions();
      K.renderChanges?.();
      K.refreshWorkspaceControls?.();
    } catch (error) {
      K.showError(error.message || String(error));
    }
  };

  const originalRenderAccount = K.renderAccount;
  K.renderAccount = () => {
    const connected = K.state.connectedProviders.has("kilo");
    const button = K.els.accountButton;
    if (!button) return originalRenderAccount?.();
    button.textContent = "Account";
    button.classList.toggle("signed-in", connected);
    button.title = connected ? "Kilo account connected — open account settings" : "Connect a Kilo account";
  };

  const renderAccountDialog = () => {
    const connected = K.state.connectedProviders.has("kilo");
    if (ui.accountStatus) {
      ui.accountStatus.textContent = connected
        ? "Your Kilo account is connected on this computer."
        : "No Kilo account is connected. Sign in to use Kilo-hosted models.";
    }
    ui.accountSignIn?.classList.toggle("hidden", connected);
    ui.accountSignOut?.classList.toggle("hidden", !connected);
  };

  const originalSignInKilo = K.signInKilo;
  K.signInKilo = async () => {
    renderAccountDialog();
    ui.accountDialog?.showModal();
  };

  const startSignIn = async () => {
    if (ui.accountDialog?.open) ui.accountDialog.close();
    await originalSignInKilo();
  };

  const signOut = async () => {
    if (!K.state.connectedProviders.has("kilo")) return;
    if (!window.confirm("Sign out of the Kilo account on this computer?")) return;
    ui.accountSignOut.disabled = true;
    try {
      await K.api.oauth.disconnectKilo();
      if (K.state.session?.model?.providerID === "kilo") K.state.session.model = undefined;
      if (K.els.modelSelect) K.els.modelSelect.value = "";
      await K.loadCatalog();
      K.renderSessionHeader?.();
      renderAccountDialog();
    } catch (error) {
      K.showError(error.message || String(error));
    } finally {
      ui.accountSignOut.disabled = false;
    }
  };

  const openSettings = () => {
    applyAppearance(readSetting(THEME_KEY, "system"));
    applyFontSize(readSetting(FONT_KEY, "default"));
    ui.settingsDialog?.showModal();
  };

  ui.settingsButton?.addEventListener("click", openSettings);
  ui.settingsClose?.addEventListener("click", () => ui.settingsDialog.close());
  ui.appearanceSelect?.addEventListener("change", () => {
    writeSetting(THEME_KEY, ui.appearanceSelect.value);
    applyAppearance(ui.appearanceSelect.value);
  });
  ui.fontSizeSelect?.addEventListener("change", () => {
    writeSetting(FONT_KEY, ui.fontSizeSelect.value);
    applyFontSize(ui.fontSizeSelect.value);
  });
  ui.accountSignIn?.addEventListener("click", startSignIn);
  ui.accountSignOut?.addEventListener("click", signOut);
  ui.accountClose?.addEventListener("click", () => ui.accountDialog.close());
})();
