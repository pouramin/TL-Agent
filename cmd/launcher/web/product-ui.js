(() => {
  "use strict";
  const K = window.KLU;
  const $ = (id) => document.getElementById(id);

  const ui = {
    settingsButton: $("settingsButton"),
    settingsDialog: $("settingsDialog"),
    settingsClose: $("settingsClose"),
    settingsCloseIcon: $("settingsCloseIcon"),
    settingsNavItems: [...document.querySelectorAll("[data-settings-section]")],
    settingsPanels: [...document.querySelectorAll("[data-settings-panel]")],
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
  const kiloConnected = () => K.state.kiloAuth?.authenticated ?? K.state.connectedProviders.has("kilo");

  const readSetting = (key, fallback) => {
    try { return window.localStorage.getItem(key) || fallback; }
    catch { return fallback; }
  };
  const writeSetting = (key, value) => {
    try { window.localStorage.setItem(key, value); }
    catch {}
  };

  const normalizePath = (value) => {
    let path = String(value || "").replace(/[\\/]+$/, "").replace(/\\/g, "/");
    if (K.state.local?.platform === "windows") path = path.toLowerCase();
    return path;
  };
  const samePath = (a, b) => normalizePath(a) === normalizePath(b);
  const sessionDirectory = (session) => session?.directory || session?.path || "";

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

  // Kilo's product server scopes session listing to a directory. TL Agent keeps
  // only a small persistent history of project paths, then asks Kilo for the
  // authoritative root sessions in every known project and merges the results.
  // Session content itself never lives in TL Agent's history file.
  const scopedLoadSessions = K.loadSessions;
  K.loadSessions = async () => {
    let history;
    try {
      history = await K.request("/local/projects");
    } catch (error) {
      console.warn("[TL Agent] Recent-project history unavailable; falling back to current project", error);
      return scopedLoadSessions();
    }

    const projects = Array.isArray(history?.projects) ? history.projects.filter(Boolean) : [];
    if (!projects.length && K.state.local?.project) projects.push(K.state.local.project);
    const results = await Promise.allSettled(
      projects.map((directory) => K.api.sessions.list({ limit: 50, directory })),
    );

    const merged = new Map();
    results.forEach((result, index) => {
      const directory = projects[index];
      if (result.status !== "fulfilled") {
        console.warn(`[TL Agent] Could not read sessions for ${directory}`, result.reason);
        return;
      }
      for (const raw of Array.isArray(result.value?.data) ? result.value.data : []) {
        if (!raw?.id) continue;
        const session = { ...raw, directory: raw.directory || directory };
        const previous = merged.get(session.id);
        const updated = Number(session.time?.updated || session.time?.created || 0);
        const previousUpdated = Number(previous?.time?.updated || previous?.time?.created || 0);
        if (!previous || updated >= previousUpdated) merged.set(session.id, session);
      }
    });

    const sessions = [...merged.values()]
      .sort((a, b) => Number(b?.time?.updated || b?.time?.created || 0) - Number(a?.time?.updated || a?.time?.created || 0))
      .slice(0, 150);
    K.state.sessions = sessions;
    K.renderSessions();
    return sessions;
  };

  const selectSessionInCurrentProject = K.selectSession;
  K.selectSession = async (session) => {
    if (!session?.id) return;
    const directory = sessionDirectory(session);
    const current = K.state.local?.project || "";
    if (directory && !samePath(directory, current)) {
      K.showError("");
      try {
        K.state.local = await K.request("/local/project", {
          method: "POST",
          body: JSON.stringify({ path: directory }),
        });
        await K.afterProjectChange();
        session = K.state.sessions.find((item) => item.id === session.id) || session;
      } catch (error) {
        K.showError(`Could not switch to this session's project: ${error.message || String(error)}`);
        return;
      }
    }
    return selectSessionInCurrentProject(session);
  };

  K.renderSessions = () => {
    const list = K.els.sessions;
    if (!list) return;
    list.textContent = "";
    if (!K.state.sessions.length) {
      const empty = document.createElement("div");
      empty.className = "sidebar-empty";
      empty.textContent = "No sessions yet.";
      list.appendChild(empty);
      return;
    }

    for (const session of K.state.sessions) {
      const row = document.createElement("div");
      const open = document.createElement("button");
      const remove = document.createElement("button");
      const title = document.createElement("strong");
      const meta = document.createElement("span");
      const directory = sessionDirectory(session);
      const project = directory ? K.basename(directory) : "Unknown project";
      const age = K.relativeTime(session.time?.updated || session.time?.created);

      row.className = "session-row";
      open.className = `session-item session-main${K.state.session?.id === session.id ? " active" : ""}`;
      open.type = "button";
      open.title = directory || session.title || "Session";
      title.textContent = session.title || "Untitled session";
      meta.textContent = `${project} · ${session.agent || "default"}${age ? ` · ${age}` : ""}`;
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
    const directory = sessionDirectory(session) || undefined;
    try {
      // Abort defensively even when the session belongs to a different project;
      // active-session status is scoped to the currently open directory.
      await K.api.sessions.abort(session.id, { scope: "tree", directory }).catch(() => {});
      await K.api.sessions.remove(session.id, { directory });
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
    const connected = kiloConnected();
    const button = K.els.accountButton;
    if (!button) return originalRenderAccount?.();
    button.textContent = "Account";
    button.classList.toggle("signed-in", connected);
    button.title = connected ? "Kilo account connected — open account settings" : "Connect a Kilo account";
  };

  const renderAccountDialog = () => {
    const connected = kiloConnected();
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
    try {
      await K.refreshKiloAuthStatus?.();
    } catch (error) {
      K.showError(`Unable to verify Kilo account state: ${error.message || String(error)}`);
    }
    renderAccountDialog();
    ui.accountDialog?.showModal();
  };

  const startSignIn = async () => {
    if (ui.accountDialog?.open) ui.accountDialog.close();
    await originalSignInKilo();
  };

  const signOut = async () => {
    if (!kiloConnected()) return;
    if (!window.confirm("Sign out of the Kilo account on this computer?")) return;
    ui.accountSignOut.disabled = true;
    K.showError("");
    try {
      K.state.authController?.abort();
      K.state.authController = null;

      await K.api.oauth.disconnectKilo();
      K.applyKiloAuthStatus?.({ authenticated: false });

      await K.api.runtime.dispose();
      await K.loadCatalog();
      const status = await K.refreshKiloAuthStatus();
      if (status.authenticated) throw new Error("Kilo still reports an authenticated account after sign-out.");

      if (K.state.session?.model?.providerID === "kilo") K.state.session.model = undefined;
      if (K.els.modelSelect) K.els.modelSelect.value = "";
      K.renderSessionHeader?.();
      renderAccountDialog();
      if (ui.accountDialog?.open) ui.accountDialog.close();
    } catch (error) {
      K.showError(error.message || String(error));
      try { await K.refreshKiloAuthStatus?.(); } catch {}
      renderAccountDialog();
    } finally {
      ui.accountSignOut.disabled = false;
    }
  };

  const activateSettingsSection = (name = "general") => {
    for (const button of ui.settingsNavItems) {
      const active = button.dataset.settingsSection === name;
      button.classList.toggle("active", active);
      if (active) button.setAttribute("aria-current", "page");
      else button.removeAttribute("aria-current");
    }
    for (const panel of ui.settingsPanels) {
      panel.classList.toggle("hidden", panel.dataset.settingsPanel !== name);
    }
  };

  const openSettings = () => {
    applyAppearance(readSetting(THEME_KEY, "system"));
    applyFontSize(readSetting(FONT_KEY, "default"));
    activateSettingsSection("general");
    ui.settingsDialog?.showModal();
  };

  const closeSettings = () => ui.settingsDialog?.close();

  ui.settingsButton?.addEventListener("click", openSettings);
  ui.settingsClose?.addEventListener("click", closeSettings);
  ui.settingsCloseIcon?.addEventListener("click", closeSettings);
  for (const button of ui.settingsNavItems) {
    button.addEventListener("click", () => activateSettingsSection(button.dataset.settingsSection));
  }
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
