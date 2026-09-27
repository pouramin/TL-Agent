import { K } from "./kernel";

(() => {
  "use strict";

  if (!K || K.__jevUiInstalled) return;
  K.__jevUiInstalled = true;

  const JEV_ROUTER_MODEL = "typesafe/jev-router";
  const OPENROUTER_BASE_URL = "https://openrouter.ai/api/v1";
  const panel = document.querySelector<HTMLElement>('[data-settings-panel="providers"]');
  const providerList = document.getElementById("providerList");
  if (!panel || !providerList || !K.__providersUi) return;

  const clean = (value: unknown) => String(value ?? "").trim();
  const isOpenRouter = (value: unknown) => {
    try {
      const url = new URL(clean(value));
      return url.protocol === "https:"
        && url.hostname.toLowerCase() === "openrouter.ai"
        && url.pathname.replace(/\/+$/, "") === "/api/v1";
    } catch {
      return false;
    }
  };

  const style = document.createElement("style");
  style.id = "tl-jev-ui-style";
  style.textContent = `
    .jev-compact-row{display:flex;align-items:center;justify-content:space-between;gap:14px;margin-top:12px;padding:10px 12px;border:1px solid var(--line);border-radius:10px;background:var(--panel-2)}
    .jev-compact-copy{min-width:0}.jev-compact-copy strong{display:block;font-size:11px}.jev-compact-copy span{display:block;margin-top:3px;color:var(--muted);font-size:9px;line-height:1.4}
    .jev-compact-control{flex:none;display:flex;align-items:center;gap:8px;padding:4px 8px 4px 5px;border:1px solid var(--line);border-radius:999px;background:var(--panel);color:var(--muted);cursor:pointer;font:inherit;font-size:9px;font-weight:700}
    .jev-compact-control:hover{border-color:color-mix(in srgb,var(--accent),var(--line) 55%);color:var(--text)}
    .jev-switch-track{position:relative;width:28px;height:16px;border-radius:999px;background:var(--panel-3);border:1px solid var(--line);transition:.15s ease}
    .jev-switch-knob{position:absolute;top:2px;left:2px;width:10px;height:10px;border-radius:50%;background:var(--muted-2);transition:.15s ease}
    .jev-compact-control.active .jev-switch-track{background:color-mix(in srgb,var(--accent),transparent 76%);border-color:color-mix(in srgb,var(--accent),var(--line) 55%)}
    .jev-compact-control.active .jev-switch-knob{left:14px;background:var(--accent)}
    .jev-config-dialog{width:min(520px,calc(100vw - 32px));padding:0;border:1px solid var(--line);border-radius:12px;background:var(--panel);color:var(--text);box-shadow:var(--shadow)}
    .jev-config-dialog::backdrop{background:rgba(0,0,0,.58)}
    .jev-config-head{display:flex;align-items:flex-start;justify-content:space-between;gap:16px;padding:16px 18px;border-bottom:1px solid var(--line-soft)}
    .jev-config-head h3{margin:0;font-size:14px}.jev-config-head p{margin:4px 0 0;color:var(--muted);font-size:9px;line-height:1.45}
    .jev-config-body{display:grid;gap:12px;padding:16px 18px 18px}.jev-config-section{display:grid;gap:10px;padding:12px;border:1px solid var(--line);border-radius:10px;background:var(--panel-2)}
    .jev-config-row{display:flex;align-items:center;justify-content:space-between;gap:12px}.jev-config-copy{min-width:0}.jev-config-copy strong{display:block;font-size:10px}.jev-config-copy span{display:block;margin-top:3px;color:var(--muted);font-size:9px;line-height:1.45}
    .jev-key-field{display:grid;gap:5px}.jev-key-field span{color:var(--muted);font-size:9px}.jev-key-field input{width:100%;height:34px}.jev-api-key{-webkit-text-security:disc}
    .jev-config-status{min-height:12px;color:var(--muted);font-size:9px;line-height:1.45}.jev-config-status.ok{color:color-mix(in srgb,var(--accent),var(--text) 35%)}.jev-config-status.error{color:var(--danger)}
    .jev-config-row select{min-width:190px;height:32px}.jev-config-actions{display:flex;justify-content:flex-end;gap:8px}
    @media(max-width:760px){.jev-compact-row{align-items:flex-start}.jev-config-row{display:grid;align-items:stretch}.jev-config-row select,.jev-config-row button{width:100%}}
  `;
  document.head.appendChild(style);

  const compact = document.createElement("section");
  compact.className = "jev-compact-row";
  compact.innerHTML = `
    <div class="jev-compact-copy">
      <strong>JEV</strong>
      <span id="jevCompactStatus">Checking TypeSafe Jev…</span>
    </div>
    <button id="jevCompactControl" type="button" class="jev-compact-control" aria-haspopup="dialog">
      <span class="jev-switch-track" aria-hidden="true"><span class="jev-switch-knob"></span></span>
      <span id="jevCompactControlLabel">Configure</span>
    </button>
  `;
  providerList.insertAdjacentElement("afterend", compact);

  const dialog = document.createElement("dialog");
  dialog.className = "jev-config-dialog";
  dialog.id = "jevConfigDialog";
  dialog.innerHTML = `
    <div class="jev-config-head">
      <div>
        <h3>TypeSafe Jev</h3>
        <p>Jev Router is configured automatically through OpenRouter. Direct Jev decisions remain a separate optional capability.</p>
      </div>
      <button id="jevConfigClose" type="button" class="ghost small">Close</button>
    </div>
    <div class="jev-config-body">
      <section class="jev-config-section">
        <div class="jev-config-row">
          <div class="jev-config-copy">
            <strong>Jev Router</strong>
            <span>TL Studio automatically discovers and saves typesafe/jev-router. No model selection is required.</span>
          </div>
          <button id="jevRouterSetupButton" type="button" class="primary small">Activate JEV</button>
        </div>
        <label id="jevApiKeyField" class="jev-key-field">
          <span>OpenRouter API key</span>
          <input id="jevOpenRouterKeyInput" class="jev-api-key" type="text" autocomplete="off" spellcheck="false" autocapitalize="off" data-form-type="other" data-lpignore="true" data-1p-ignore placeholder="Enter OpenRouter API key" />
        </label>
        <div id="jevRouterStatus" class="jev-config-status"></div>
      </section>
      <section class="jev-config-section">
        <div class="jev-config-row">
          <div class="jev-config-copy">
            <strong>Decision Engine</strong>
            <span>Off by default. Direct Jev Decisions API calls are paid and never run automatically.</span>
          </div>
          <select id="jevDecisionEngineSelect">
            <option value="off">Off</option>
            <option value="jev">Jev via OpenRouter (paid)</option>
          </select>
        </div>
        <div id="jevDecisionStatus" class="jev-config-status"></div>
      </section>
      <div class="jev-config-actions">
        <button id="jevConfigDone" type="button" class="primary small">Done</button>
      </div>
    </div>
  `;
  document.body.appendChild(dialog);

  const compactStatus = document.getElementById("jevCompactStatus") as HTMLElement;
  const compactControl = document.getElementById("jevCompactControl") as HTMLButtonElement;
  const compactControlLabel = document.getElementById("jevCompactControlLabel") as HTMLElement;
  const setupButton = document.getElementById("jevRouterSetupButton") as HTMLButtonElement;
  const apiKeyField = document.getElementById("jevApiKeyField") as HTMLElement;
  const apiKeyInput = document.getElementById("jevOpenRouterKeyInput") as HTMLInputElement;
  const routerStatus = document.getElementById("jevRouterStatus") as HTMLElement;
  const decisionSelect = document.getElementById("jevDecisionEngineSelect") as HTMLSelectElement;
  const decisionStatus = document.getElementById("jevDecisionStatus") as HTMLElement;
  const closeButton = document.getElementById("jevConfigClose") as HTMLButtonElement;
  const doneButton = document.getElementById("jevConfigDone") as HTMLButtonElement;
  const providersUI = K.__providersUi;

  let currentProvider: TLStudioDynamicRecord | null = null;
  let currentRouterReady = false;
  let currentCredentialConnected = false;

  const setText = (element: HTMLElement, message = "", kind = "") => {
    element.textContent = message;
    element.className = `jev-config-status${kind ? ` ${kind}` : ""}`;
  };

  const providerHasRouter = (provider: TLStudioDynamicRecord | null) =>
    !!provider && Array.isArray(provider.models) &&
    provider.models.some((model: TLStudioDynamicRecord) => clean(model?.id) === JEV_ROUTER_MODEL);

  const isJevOnlyProvider = (provider: TLStudioDynamicRecord | null) =>
    !!provider && isOpenRouter(provider?.baseURL) &&
    Array.isArray(provider.models) &&
    provider.models.length === 1 &&
    clean(provider.models[0]?.id) === JEV_ROUTER_MODEL;

  const jevManagedBy = (provider: TLStudioDynamicRecord | null) =>
    clean(provider?.managedBy) === "jev" || isJevOnlyProvider(provider);

  const selectOpenRouterProvider = (
    providers: TLStudioDynamicRecord[],
    decision: TLStudioDynamicRecord,
  ) => {
    const candidates = providers.filter((provider) => isOpenRouter(provider?.baseURL));
    return candidates.find(providerHasRouter)
      || candidates.find((provider) => clean(provider?.id) === clean(decision?.providerID))
      || candidates.find((provider) => clean(provider?.id) === "openrouter")
      || candidates[0]
      || null;
  };

  const nextProviderID = (providers: TLStudioDynamicRecord[]) => {
    const used = new Set(providers.map((provider) => clean(provider?.id)));
    if (!used.has("openrouter")) return "openrouter";
    let suffix = 2;
    while (used.has(`openrouter-${suffix}`)) suffix++;
    return `openrouter-${suffix}`;
  };

  const configuredJevModel = (model: TLStudioDynamicRecord) => ({
    id: JEV_ROUTER_MODEL,
    name: clean(model?.name) || "Jev Router",
    kind: "router",
    toolCall: model?.toolCall === true,
    reasoning: model?.reasoning === true,
    ...(Number(model?.contextLimit) > 0 ? { contextLimit: Number(model.contextLimit) } : {}),
    ...(Number(model?.outputLimit) > 0 ? { outputLimit: Number(model.outputLimit) } : {}),
  });

  const mergeJevModel = (provider: TLStudioDynamicRecord | null, discovered: TLStudioDynamicRecord) => {
    const existing = Array.isArray(provider?.models) ? provider.models : [];
    return [
      ...existing.filter((model: TLStudioDynamicRecord) => clean(model?.id) !== JEV_ROUTER_MODEL),
      configuredJevModel(discovered),
    ];
  };

  const refreshStatus = async () => {
    try {
      const [config, decision] = await Promise.all([
        K.api.providers.config(),
        K.api.decisionEngine.status(),
      ]);
      const providers = Array.isArray(config?.providers) ? config.providers : [];
      currentProvider = selectOpenRouterProvider(providers, decision || {});
      const hasRouter = providerHasRouter(currentProvider);
      currentCredentialConnected = !!currentProvider && (
        K.state.connectedProviders?.has?.(currentProvider.id)
        || (decision?.providerConfigured === true && clean(decision?.providerID) === clean(currentProvider.id))
      );
      currentRouterReady = hasRouter && currentCredentialConnected;

      compactControl.classList.toggle("active", currentRouterReady);
      compactControlLabel.textContent = currentRouterReady ? "Active JEV" : "Configure JEV";
      compactStatus.textContent = currentRouterReady
        ? `Jev Router ready via ${currentProvider?.name || currentProvider?.id || "OpenRouter"}`
        : hasRouter
          ? "Jev Router is saved, but the OpenRouter credential is not connected."
          : currentProvider
            ? "OpenRouter is configured. Activate JEV to add the router automatically."
            : "Not configured";

      apiKeyField.classList.toggle("hidden", currentCredentialConnected);
      setupButton.textContent = currentRouterReady ? "Refresh JEV" : "Activate JEV";
      setText(routerStatus, currentRouterReady
        ? `Ready. TL Studio uses ${JEV_ROUTER_MODEL} through ${currentProvider?.name || currentProvider?.id}.`
        : currentCredentialConnected
          ? "OpenRouter credential found. Click Activate JEV; TL Studio will discover and save Jev Router automatically."
          : "Enter an OpenRouter API key. TL Studio will validate it, find Jev Router, and save it automatically.",
        currentRouterReady ? "ok" : "");

      decisionSelect.value = decision?.engine === "jev" ? "jev" : "off";
      if (decision?.engine === "jev") {
        setText(decisionStatus,
          `Enabled via ${decision?.providerName || decision?.providerID || "OpenRouter"}. No automatic Agent or permission decisions are active.`,
          "ok");
      } else if (decision?.providerConfigured) {
        setText(decisionStatus, "Off. The existing OpenRouter credential can be reused if you explicitly enable paid Jev decisions.");
      } else {
        setText(decisionStatus, "Off. Configure a valid OpenRouter credential before enabling paid Jev decisions.");
      }
    } catch (error) {
      currentProvider = null;
      currentRouterReady = false;
      currentCredentialConnected = false;
      compactControl.classList.remove("active");
      compactControlLabel.textContent = "Configure JEV";
      compactStatus.textContent = "Could not read Jev status";
      apiKeyField.classList.remove("hidden");
      setText(routerStatus, error instanceof Error ? error.message : String(error), "error");
    }
  };

  const openDialog = async () => {
    apiKeyInput.value = "";
    await refreshStatus();
    if (!dialog.open) dialog.showModal();
  };

  const setupRouter = async () => {
    if (setupButton.disabled) return;
    setupButton.disabled = true;
    setText(routerStatus, "Checking OpenRouter and locating Jev Router…");
    try {
      const config = await K.api.providers.config();
      const providers = Array.isArray(config?.providers) ? config.providers : [];
      const key = clean(apiKeyInput.value);

      await refreshStatus();
      if (!currentCredentialConnected && !key) {
        throw new Error("Enter your OpenRouter API key.");
      }

      const providerID = clean(currentProvider?.id) || nextProviderID(providers);
      const protocol = clean(currentProvider?.protocol) || "openai-compatible";
      const discovery = await K.api.providers.discover({
        ...(currentProvider ? { providerID } : {}),
        protocol,
        baseURL: OPENROUTER_BASE_URL,
        ...(key ? { apiKey: key } : {}),
      });
      const models = Array.isArray(discovery?.models) ? discovery.models : [];
      const jev = models.find((model: TLStudioDynamicRecord) => clean(model?.id) === JEV_ROUTER_MODEL);
      if (!jev) {
        throw new Error("Jev Router is not available for this OpenRouter account.");
      }

      const provider = {
        id: providerID,
        name: clean(currentProvider?.name) || "OpenRouter",
        protocol,
        baseURL: OPENROUTER_BASE_URL,
        ...(jevManagedBy(currentProvider) || !currentProvider ? { managedBy: "jev" } : {}),
        models: mergeJevModel(currentProvider, jev),
      };
      await K.api.providers.upsert(providerID, {
        provider,
        ...(key ? { apiKey: key } : {}),
      });

      apiKeyInput.value = "";
      await K.loadCatalog();
      await providersUI.reload?.();
      window.dispatchEvent(new CustomEvent("tlstudio:providers-changed"));
      await refreshStatus();
      setText(routerStatus, "Jev Router is active. No manual model selection was required.", "ok");
    } catch (error) {
      setText(routerStatus, `Could not activate Jev Router: ${error instanceof Error ? error.message : String(error)}`, "error");
    } finally {
      setupButton.disabled = false;
    }
  };

  const changeDecisionEngine = async () => {
    const next = decisionSelect.value === "jev" ? "jev" : "off";
    if (next === "jev") {
      const approved = window.confirm(
        "Enable Jev Decision Engine? Direct Jev Decision API calls are paid through your OpenRouter account. "
        + "TL Studio will reuse your existing OpenRouter credential. This does not enable automatic routing or permission decisions."
      );
      if (!approved) {
        await refreshStatus();
        return;
      }
    }
    decisionSelect.disabled = true;
    try {
      await K.api.decisionEngine.configure(next);
      await refreshStatus();
    } catch (error) {
      setText(decisionStatus, `Could not update Decision Engine: ${error instanceof Error ? error.message : String(error)}`, "error");
      await refreshStatus();
    } finally {
      decisionSelect.disabled = false;
    }
  };

  compactControl.addEventListener("click", () => { void openDialog(); });
  setupButton.addEventListener("click", () => { void setupRouter(); });
  decisionSelect.addEventListener("change", () => { void changeDecisionEngine(); });
  closeButton.addEventListener("click", () => dialog.close());
  doneButton.addEventListener("click", () => dialog.close());
  dialog.addEventListener("click", (event) => {
    if (event.target === dialog) dialog.close();
  });

  window.addEventListener("tlstudio:providers-changed", () => { void refreshStatus(); });
  const providersNav = document.querySelector<HTMLElement>('[data-settings-section="providers"]');
  providersNav?.addEventListener("click", () => { void refreshStatus(); });
  void refreshStatus();
})();
