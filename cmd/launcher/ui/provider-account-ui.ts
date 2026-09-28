import { K } from "./kernel";

(() => {
  "use strict";

  if (!K || K.__providerAccountsUiInstalled) return;
  K.__providerAccountsUiInstalled = true;

  const clean = (value: any) => String(value ?? "").trim();
  const settingsDialog = document.getElementById("settingsDialog") as HTMLDialogElement | null;
  const panel = settingsDialog?.querySelector<HTMLElement>('[data-settings-panel="providers"]');
  const providerList = document.getElementById("providerList");
  if (!settingsDialog || !panel || !providerList) return;

  const section = document.createElement("section");
  section.className = "provider-account-section";
  section.innerHTML = `
    <div class="provider-subsection-head">
      <div>
        <strong>Account providers</strong>
        <span>Connect an account or configure a supported provider.</span>
      </div>
    </div>
    <div id="providerAccountList" class="provider-account-grid"></div>
    <div class="provider-section-divider"><span>API providers</span></div>
  `;
  providerList.before(section);

  const setupDialog = document.createElement("dialog");
  setupDialog.id = "providerAccountSetupDialog";
  setupDialog.innerHTML = `
    <form id="providerAccountSetupForm" class="dialog-card provider-account-setup-dialog">
      <div class="provider-dialog-head">
        <div>
          <h2 id="providerAccountSetupTitle">Provider setup</h2>
          <p id="providerAccountSetupDescription"></p>
        </div>
        <button id="providerAccountSetupClose" class="icon-button" type="button" aria-label="Close provider setup">×</button>
      </div>
      <div id="providerAccountSetupNotice" class="provider-notice hidden" role="status"></div>
      <div id="providerAccountSetupFields" class="provider-account-setup-fields"></div>
      <div class="provider-security-note">These fields contain non-secret provider setup only. OAuth tokens and API credentials stay in the TL Studio credential vault.</div>
      <div class="dialog-actions provider-form-actions">
        <button id="providerAccountSetupCancel" class="ghost" type="button">Cancel</button>
        <button id="providerAccountSetupSave" class="primary" type="submit">Save setup</button>
      </div>
    </form>
  `;
  document.body.appendChild(setupDialog);

  const style = document.createElement("style");
  style.id = "tl-provider-accounts-ui-style";
  style.textContent = `
    .provider-account-section{display:grid;gap:9px;margin:2px 0 12px}
    .provider-subsection-head{display:flex;align-items:flex-start;justify-content:space-between;gap:12px}
    .provider-subsection-head strong,.provider-subsection-head span{display:block}
    .provider-subsection-head strong{font-size:11px}
    .provider-subsection-head span{margin-top:3px;color:var(--muted);font-size:9px;line-height:1.45}
    .provider-account-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:8px}
    .provider-account-card{position:relative;display:flex;min-width:0;min-height:148px;flex-direction:column;align-items:center;justify-content:flex-start;gap:7px;padding:13px 10px 10px;border:1px solid var(--line);border-radius:12px;background:var(--panel);text-align:center}
    .provider-account-card.provider-account-unavailable{opacity:.66}
    .provider-account-card.provider-account-connected{border-color:color-mix(in srgb,var(--accent) 42%,var(--line))}
    .provider-account-logo{display:grid;width:42px;height:42px;place-items:center;border:1px solid var(--line);border-radius:12px;background:color-mix(in srgb,var(--panel) 72%,var(--text) 4%);color:var(--text)}
    .provider-account-logo svg{display:block;width:25px;height:25px}
    .provider-account-logo-fallback{font-size:15px;font-weight:800;line-height:1}
    .provider-account-name{max-width:100%;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:10px;font-weight:720}
    .provider-account-state{display:flex;min-height:14px;align-items:center;gap:5px;color:var(--muted);font-size:8px;line-height:1.2}
    .provider-account-state .provider-status-dot{width:6px;height:6px}
    .provider-account-card-actions{display:flex;width:100%;margin-top:auto;align-items:center;justify-content:center;gap:5px}
    .provider-account-card-actions .primary,.provider-account-card-actions .ghost{min-height:28px;padding:0 9px;font-size:8px}
    .provider-account-signin{min-width:74px}
    .provider-account-setup-button{position:absolute;top:8px;right:8px;width:25px;height:25px;padding:0;border-radius:8px;font-size:12px;line-height:1}
    .provider-account-details{max-width:100%;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:var(--muted);font-size:8px}
    .provider-account-setup-dialog{width:min(620px,calc(100vw - 36px));padding:20px}
    .provider-account-setup-fields{display:grid;gap:10px}.provider-account-setup-field>span{display:block;margin-bottom:5px;color:var(--muted);font-size:var(--tl-ui-xs);font-weight:650}.provider-account-setup-field input{box-sizing:border-box;width:100%;height:34px}.provider-account-setup-field small{display:block;margin-top:5px;color:var(--muted);font-size:var(--tl-ui-xs);line-height:1.45}
    .provider-section-divider{display:flex;align-items:center;gap:10px;margin:3px 0 1px;color:var(--muted);font-size:8px;font-weight:700;letter-spacing:.08em;text-transform:uppercase}
    .provider-section-divider::before,.provider-section-divider::after{content:"";height:1px;background:var(--line);flex:1}
    @media(max-width:900px){.provider-account-grid{grid-template-columns:repeat(2,minmax(0,1fr))}}
    @media(max-width:620px){.provider-account-grid{grid-template-columns:1fr}}
  `;
  document.head.appendChild(style);

  const accountList = document.getElementById("providerAccountList")!;
  let loading = false;

  const setupForm = document.getElementById("providerAccountSetupForm") as HTMLFormElement;
  const setupTitle = document.getElementById("providerAccountSetupTitle") as HTMLElement;
  const setupDescription = document.getElementById("providerAccountSetupDescription") as HTMLElement;
  const setupFields = document.getElementById("providerAccountSetupFields") as HTMLElement;
  const setupNotice = document.getElementById("providerAccountSetupNotice") as HTMLElement;
  const setupClose = document.getElementById("providerAccountSetupClose") as HTMLButtonElement;
  const setupCancel = document.getElementById("providerAccountSetupCancel") as HTMLButtonElement;
  const setupSave = document.getElementById("providerAccountSetupSave") as HTMLButtonElement;
  let setupProviderID = "";
  let setupSaving = false;

  const setSetupNotice = (message = "", error = false) => {
    setupNotice.textContent = message;
    setupNotice.classList.toggle("hidden", !message);
    setupNotice.classList.toggle("error", !!message && error);
  };

  const closeSetup = () => {
    setupProviderID = "";
    setupFields.textContent = "";
    setSetupNotice();
    if (setupDialog.open) setupDialog.close();
  };

  const configureAccount = async (account: TLStudioProviderAccount) => {
    if (!account.setup?.configurable || setupSaving) return;
    setSetupNotice();
    try {
      const setup = await K.api.providerAccounts.setup(account.id);
      setupProviderID = account.id;
      setupTitle.textContent = clean(setup.title) || `Configure ${account.name || account.id}`;
      setupDescription.textContent = clean(setup.description);
      setupFields.textContent = "";
      for (const field of Array.isArray(setup.fields) ? setup.fields : []) {
        const label = document.createElement("label");
        label.className = "provider-account-setup-field";
        const title = document.createElement("span");
        title.textContent = field.label || field.id;
        const input = document.createElement("input");
        input.type = "text";
        input.dataset.providerSetupField = field.id;
        input.value = field.value || "";
        input.placeholder = field.placeholder || "";
        input.required = field.required === true;
        input.readOnly = field.readOnly === true;
        input.autocomplete = "off";
        input.spellcheck = false;
        label.append(title, input);
        if (field.description) {
          const help = document.createElement("small");
          help.textContent = field.description;
          label.appendChild(help);
        }
        setupFields.appendChild(label);
      }
      if (!setupDialog.open) setupDialog.showModal();
      requestAnimationFrame(() => setupFields.querySelector<HTMLInputElement>("input:not([readonly])")?.focus({ preventScroll: true }));
    } catch (error) {
      K.showError(error instanceof Error ? error.message : String(error));
    }
  };

  const providerAccountLogo = (account: TLStudioProviderAccount) => {
    const icons: Record<string, string> = {
      chatgpt: `<svg viewBox="0 0 32 32" aria-hidden="true"><path d="M16 4.2a6.1 6.1 0 0 1 5.8 4.2 6.1 6.1 0 0 1 3.6 10.8 6.1 6.1 0 0 1-9.4 7.1 6.1 6.1 0 0 1-9.4-7.1A6.1 6.1 0 0 1 10.2 8.4 6.1 6.1 0 0 1 16 4.2Z" fill="none" stroke="currentColor" stroke-width="2.2"/><path d="m10.1 11.5 5.9-3.4 5.9 3.4v6.8L16 21.7l-5.9-3.4Z" fill="none" stroke="currentColor" stroke-width="1.8"/></svg>`,
      claude: `<svg viewBox="0 0 32 32" aria-hidden="true"><path d="M16 4v24M4 16h24M7.5 7.5l17 17M24.5 7.5l-17 17" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round"/></svg>`,
      gemini: `<svg viewBox="0 0 32 32" aria-hidden="true"><path d="M16 3.5c1.2 7.3 5.2 11.3 12.5 12.5C21.2 17.2 17.2 21.2 16 28.5 14.8 21.2 10.8 17.2 3.5 16 10.8 14.8 14.8 10.8 16 3.5Z" fill="currentColor"/></svg>`,
      "github-copilot": `<svg viewBox="0 0 32 32" aria-hidden="true"><path d="M7 12.5C7 8.9 10 6 13.5 6h5C22 6 25 8.9 25 12.5V21c0 2.8-2.2 5-5 5h-8c-2.8 0-5-2.2-5-5Z" fill="none" stroke="currentColor" stroke-width="2.2"/><circle cx="12" cy="15" r="1.8" fill="currentColor"/><circle cx="20" cy="15" r="1.8" fill="currentColor"/><path d="M12 21c2.7 1.7 5.3 1.7 8 0" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>`,
      huggingface: `<svg viewBox="0 0 32 32" aria-hidden="true"><circle cx="11" cy="13" r="2.2" fill="currentColor"/><circle cx="21" cy="13" r="2.2" fill="currentColor"/><path d="M8.5 18.5c1.8 4.2 4.3 6 7.5 6s5.7-1.8 7.5-6" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round"/><path d="M6 11.5 3.8 9.8M26 11.5l2.2-1.7" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"/></svg>`,
      openrouter: `<svg viewBox="0 0 32 32" aria-hidden="true"><path d="M5 16h20M19 10l6 6-6 6M11 9 6 13M11 23l-5-4" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"/></svg>`,
    };
    return icons[account.id] || `<span class="provider-account-logo-fallback">${clean(account.name || account.id).slice(0, 1).toUpperCase() || "?"}</span>`;
  };

  const providerAccountDetails = (account: TLStudioProviderAccount) =>
    [clean(account.description), clean(account.billingNote)].filter(Boolean).join(" · ");

  const statusText = (account: TLStudioProviderAccount) => {
    if (!account.available) return "Unavailable";
    if (account.state === "connecting") return "Connecting";
    if (account.state === "expired") return "Expired";
    if (account.state === "needs_reauthentication") return "Needs reauthentication";
    if (account.state === "error") return "Error";
    if (account.connected) return "Connected";
    return "Not connected";
  };

  const syncConnectedProviders = () => {
    for (const account of K.state.providerAccounts) {
      if (account.connected) K.state.connectedProviders.add(account.id);
      else K.state.connectedProviders.delete(account.id);
    }
  };

  const renderAccountButton = () => {
    const connected = K.state.providerAccounts.filter((account) => account.connected).length;
    K.els.accountButton.textContent = "Account";
    K.els.accountButton.classList.toggle("signed-in", connected > 0);
    K.els.accountButton.title = connected
      ? `${connected} provider account${connected === 1 ? "" : "s"} connected — open Providers`
      : "Open provider account settings";
  };

  const render = () => {
    accountList.textContent = "";
    if (!K.state.providerAccounts.length) {
      const empty = document.createElement("div");
      empty.className = "provider-empty";
      empty.textContent = "No account-based provider integrations are available in this build.";
      accountList.appendChild(empty);
      renderAccountButton();
      return;
    }

    for (const account of K.state.providerAccounts) {
      const card = document.createElement("div");
      card.className = `provider-account-card${account.available ? "" : " provider-account-unavailable"}${account.connected ? " provider-account-connected" : ""}`;
      card.dataset.providerAccountId = account.id;
      const details = providerAccountDetails(account);
      if (details) card.title = details;

      if (account.setup?.configurable) {
        const setup = document.createElement("button");
        setup.type = "button";
        setup.className = "ghost provider-account-setup-button";
        setup.dataset.providerAccountAction = "setup";
        setup.textContent = "⚙";
        setup.title = account.setup.label || "Configure provider account setup";
        setup.setAttribute("aria-label", setup.title);
        setup.addEventListener("click", () => { void configureAccount(account); });
        card.appendChild(setup);
      }

      const logo = document.createElement("div");
      logo.className = "provider-account-logo";
      logo.innerHTML = providerAccountLogo(account);
      logo.setAttribute("aria-hidden", "true");

      const name = document.createElement("div");
      name.className = "provider-account-name";
      name.textContent = account.name || account.id;

      const state = document.createElement("div");
      state.className = "provider-account-state";
      const dot = document.createElement("span");
      dot.className = `provider-status-dot${account.connected ? " ok" : ""}`;
      const stateLabel = document.createElement("span");
      stateLabel.textContent = statusText(account);
      state.append(dot, stateLabel);

      const accountDetails = document.createElement("div");
      accountDetails.className = "provider-account-details";
      accountDetails.textContent = account.connected
        ? [account.accountLabel, account.accountType, account.organizationId].filter(Boolean).join(" · ")
        : "";
      if (!accountDetails.textContent) accountDetails.setAttribute("aria-hidden", "true");

      const actions = document.createElement("div");
      actions.className = "provider-account-card-actions";
      if (account.connected) {
        const reconnect = document.createElement("button");
        reconnect.type = "button";
        reconnect.className = "ghost small";
        reconnect.dataset.providerAccountAction = "reconnect";
        reconnect.textContent = "Reconnect";
        reconnect.disabled = !account.available;
        reconnect.addEventListener("click", () => { void connectAccount(account); });

        const disconnect = document.createElement("button");
        disconnect.type = "button";
        disconnect.className = "ghost small provider-delete";
        disconnect.dataset.providerAccountAction = "disconnect";
        disconnect.textContent = "Sign out";
        disconnect.addEventListener("click", () => { void disconnectAccount(account); });
        actions.append(reconnect, disconnect);
      } else {
        const connect = document.createElement("button");
        connect.type = "button";
        connect.className = "primary small provider-account-signin";
        connect.dataset.providerAccountAction = "connect";
        connect.textContent = account.available ? "Sign in" : "Unavailable";
        connect.disabled = !account.available;
        connect.title = account.available
          ? `Sign in with ${account.name || account.id}`
          : clean(account.error) || clean(account.description) || "This account integration is unavailable.";
        connect.addEventListener("click", () => { void connectAccount(account); });
        actions.appendChild(connect);
      }

      card.append(logo, name, state, accountDetails, actions);
      accountList.appendChild(card);
    }
    renderAccountButton();
  };

  const load = async () => {
    if (loading) return K.state.providerAccounts;
    loading = true;
    try {
      const accounts = await K.api.providerAccounts.list();
      K.state.providerAccounts = Array.isArray(accounts) ? accounts : [];
      syncConnectedProviders();
      render();
      return K.state.providerAccounts;
    } catch (error) {
      K.state.providerAccounts = [];
      render();
      throw error;
    } finally {
      loading = false;
    }
  };

  const refreshProviderSurfaces = async () => {
    await K.loadCatalog();
    await load();
    K.__providersUi?.reload?.();
    K.renderModels?.();
    K.renderSessionHeader?.();
  };

  const wait = (ms: number, signal: AbortSignal) => new Promise<void>((resolve, reject) => {
    const timer = window.setTimeout(resolve, ms);
    signal.addEventListener("abort", () => {
      window.clearTimeout(timer);
      reject(new DOMException("Provider login cancelled", "AbortError"));
    }, { once: true });
  });

  const connectAccount = async (account: TLStudioProviderAccount) => {
    if (!account.available) return;
    K.showError("");
    K.state.authController?.abort();
    const controller = new AbortController();
    K.state.authController = controller;
    K.state.authURL = "";
    K.state.authProviderID = account.id;
    K.state.authLoginID = "";

    const title = K.els.authDialog.querySelector("h2");
    if (title) title.textContent = `Sign in with ${account.name || account.id}`;
    K.els.authInstructions.textContent = "Starting secure provider authorization…";
    K.els.authCode.textContent = "";
    K.els.authCodeWrap.classList.add("hidden");
    K.els.authOpen.disabled = true;
    K.els.authDialog.showModal();

    try {
      const login = await K.api.providerAccounts.beginLogin(account.id);
      K.state.authLoginID = clean(login.loginId);
      K.state.authURL = clean(login.authorizationUrl || login.verificationUrl);
      K.els.authInstructions.textContent = clean(login.instructions) || "Authorization is ready. Open the sign-in page to continue.";
      const code = clean(login.userCode);
      if (code) {
        K.els.authCode.textContent = code;
        K.els.authCodeWrap.classList.remove("hidden");
      }
      K.els.authOpen.disabled = !K.state.authURL;

      const expiresAt = login.expiresAt ? Date.parse(login.expiresAt) : 0;
      const interval = Math.max(1, Number(login.pollIntervalSeconds) || 2) * 1000;
      while (!controller.signal.aborted) {
        if (expiresAt && Date.now() >= expiresAt) throw new Error("Provider sign-in expired. Start again.");
        const status = await K.api.providerAccounts.pollLogin(account.id, login.loginId, controller.signal);
        if (status.state === "connected" || status.connected) {
          K.state.authController = null;
          K.state.authProviderID = "";
          K.state.authLoginID = "";
          K.els.authInstructions.textContent = "Signed in successfully.";
          await refreshProviderSurfaces();
          window.setTimeout(() => { if (K.els.authDialog.open) K.els.authDialog.close(); }, 650);
          return;
        }
        if (status.state === "expired") throw new Error("Provider sign-in expired. Start again.");
        if (status.state === "needs_reauthentication") throw new Error("Provider requires authentication again.");
        if (status.state === "error") throw new Error(status.error || "Provider sign-in failed.");
        await wait(interval, controller.signal);
      }
    } catch (error) {
      if ((error as any)?.name !== "AbortError") {
        K.els.authInstructions.textContent = `Sign-in failed: ${error instanceof Error ? error.message : String(error)}`;
      }
      try { await load(); } catch {}
    } finally {
      if (K.state.authController === controller) K.state.authController = null;
    }
  };

  setupForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    if (!setupProviderID || setupSaving) return;
    setupSaving = true;
    setupSave.disabled = true;
    setSetupNotice();
    try {
      const values: Record<string, string> = {};
      for (const input of setupFields.querySelectorAll<HTMLInputElement>("[data-provider-setup-field]")) {
        values[clean(input.dataset.providerSetupField)] = clean(input.value);
      }
      await K.api.providerAccounts.configureSetup(setupProviderID, values);
      closeSetup();
      await refreshProviderSurfaces();
    } catch (error) {
      setSetupNotice(error instanceof Error ? error.message : String(error), true);
    } finally {
      setupSaving = false;
      setupSave.disabled = false;
    }
  });
  setupClose.addEventListener("click", closeSetup);
  setupCancel.addEventListener("click", closeSetup);
  setupDialog.addEventListener("close", () => {
    if (!setupSaving) {
      setupProviderID = "";
      setupFields.textContent = "";
      setSetupNotice();
    }
  });

  const disconnectAccount = async (account: TLStudioProviderAccount) => {
    if (!account.connected) return;
    if (!window.confirm(`Sign out of ${account.name || account.id} on this computer?`)) return;
    K.showError("");
    try {
      await K.api.providerAccounts.disconnect(account.id);
      if (K.state.session?.model?.providerID === account.id) K.state.session.model = undefined;
      if (K.els.modelSelect) K.els.modelSelect.value = "";
      await refreshProviderSurfaces();
    } catch (error) {
      K.showError(error instanceof Error ? error.message : String(error));
      try { await load(); } catch {}
    }
  };

  const open = async () => {
    if (typeof K.activateSettingsSection === "function") K.activateSettingsSection("providers");
    if (!settingsDialog.open) settingsDialog.showModal();
    await Promise.allSettled([
      K.__providersUi?.reload?.(),
      load(),
    ]);
  };

  K.__providerAccountsUi = { load, render, open, connectAccount, configureAccount, disconnectAccount };
  K.openProviderAccounts = open;

  const previousRenderAccount = K.renderAccount;
  K.renderAccount = () => {
    if (K.state.providerAccounts.length) {
      renderAccountButton();
      return;
    }
    previousRenderAccount?.();
  };

  window.addEventListener("tlstudio:providers-changed", () => { void load(); });
  void load().catch(() => {});
})();
