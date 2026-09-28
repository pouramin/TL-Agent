import { K } from "./kernel";

(() => {
  "use strict";

  if (!K || K.__providerAccountsUiInstalled) return;
  K.__providerAccountsUiInstalled = true;

  const clean = (value: any) => String(value ?? "").trim();
  const parseDeviceCode = (input: any) => clean(input).match(/code:\s*([A-Z0-9-]+)/i)?.[1]?.toUpperCase()
    || clean(input).match(/\b[A-Z0-9]{4,}(?:-[A-Z0-9]{3,})+\b/i)?.[0]?.toUpperCase()
    || "";

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
        <span>Sign in with a provider account when an official supported integration is available.</span>
      </div>
    </div>
    <div id="providerAccountList" class="provider-account-list"></div>
    <div class="provider-section-divider"><span>API providers</span></div>
  `;
  providerList.before(section);

  const style = document.createElement("style");
  style.id = "tl-provider-accounts-ui-style";
  style.textContent = `
    .provider-account-section{display:grid;gap:9px;margin:2px 0 12px}
    .provider-subsection-head{display:flex;align-items:flex-start;justify-content:space-between;gap:12px}
    .provider-subsection-head strong,.provider-subsection-head span{display:block}
    .provider-subsection-head strong{font-size:11px}
    .provider-subsection-head span{margin-top:3px;color:var(--muted);font-size:9px;line-height:1.45}
    .provider-account-list{display:grid;gap:8px}
    .provider-account-item{display:grid;grid-template-columns:minmax(0,1fr) auto;gap:12px;align-items:center;padding:11px 12px;border:1px solid var(--line);border-radius:10px;background:var(--panel)}
    .provider-account-title{display:flex;align-items:center;gap:7px}
    .provider-account-title strong{font-size:11px}
    .provider-account-badge{display:inline-flex;align-items:center;height:18px;padding:0 6px;border:1px solid var(--line);border-radius:999px;color:var(--muted);font-size:8px}
    .provider-account-meta{margin-top:4px;color:var(--muted);font-size:9px;line-height:1.45}
    .provider-account-actions{display:flex;gap:6px;align-items:center}
    .provider-account-unavailable{opacity:.72}
    .provider-section-divider{display:flex;align-items:center;gap:10px;margin:3px 0 1px;color:var(--muted);font-size:8px;font-weight:700;letter-spacing:.08em;text-transform:uppercase}
    .provider-section-divider::before,.provider-section-divider::after{content:"";height:1px;background:var(--line);flex:1}
    @media(max-width:760px){.provider-account-item{grid-template-columns:1fr}.provider-account-actions{justify-content:flex-end}}
  `;
  document.head.appendChild(style);

  const accountList = document.getElementById("providerAccountList")!;
  let loading = false;

  const statusText = (account: TLStudioProviderAccount) => {
    if (!account.available) return "Unavailable";
    if (account.state === "connecting") return "Connecting";
    if (account.state === "expired") return "Expired";
    if (account.state === "needs_reauthentication") return "Needs reauthentication";
    if (account.state === "error") return account.error ? `Error · ${account.error}` : "Error";
    if (account.connected) {
      const details = [account.accountLabel, account.accountType, account.organizationId].filter(Boolean).join(" · ");
      return details ? `Connected · ${details}` : "Connected";
    }
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
      const row = document.createElement("div");
      row.className = `provider-account-item${account.available ? "" : " provider-account-unavailable"}`;
      row.dataset.providerAccountId = account.id;

      const copy = document.createElement("div");
      const title = document.createElement("div");
      title.className = "provider-account-title";
      const dot = document.createElement("span");
      dot.className = `provider-status-dot${account.connected ? " ok" : ""}`;
      const name = document.createElement("strong");
      name.textContent = account.name || account.id;
      const badge = document.createElement("span");
      badge.className = "provider-account-badge";
      badge.textContent = "Account";
      title.append(dot, name, badge);

      const meta = document.createElement("div");
      meta.className = "provider-account-meta";
      const description = clean(account.description);
      meta.textContent = [statusText(account), description].filter(Boolean).join(" · ");
      copy.append(title, meta);

      const actions = document.createElement("div");
      actions.className = "provider-account-actions";
      if (account.connected) {
        const disconnect = document.createElement("button");
        disconnect.type = "button";
        disconnect.className = "ghost small provider-delete";
        disconnect.dataset.providerAccountAction = "disconnect";
        disconnect.textContent = "Sign out";
        disconnect.addEventListener("click", () => { void disconnectAccount(account); });
        actions.appendChild(disconnect);
      } else {
        const connect = document.createElement("button");
        connect.type = "button";
        connect.className = "primary small";
        connect.dataset.providerAccountAction = "connect";
        connect.textContent = `Sign in with ${account.name || account.id}`;
        connect.disabled = !account.available;
        if (!account.available) connect.title = "This account integration needs its optional compatibility adapter.";
        connect.addEventListener("click", () => { void connectAccount(account); });
        actions.appendChild(connect);
      }

      row.append(copy, actions);
      accountList.appendChild(row);
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

  K.__providerAccountsUi = { load, render, open, connectAccount, disconnectAccount };
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
