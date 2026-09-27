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
    .jev-config-body{display:grid;gap:12px;padding:16px 18px 18px}.jev-config-section{display:grid;gap:8px;padding:12px;border:1px solid var(--line);border-radius:10px;background:var(--panel-2)}
    .jev-config-row{display:flex;align-items:center;justify-content:space-between;gap:12px}.jev-config-copy{min-width:0}.jev-config-copy strong{display:block;font-size:10px}.jev-config-copy span{display:block;margin-top:3px;color:var(--muted);font-size:9px;line-height:1.45}
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
        <p>Jev Router uses the normal OpenRouter model path. Direct Jev decisions are separate and optional.</p>
      </div>
      <button id="jevConfigClose" type="button" class="ghost small">Close</button>
    </div>
    <div class="jev-config-body">
      <section class="jev-config-section">
        <div class="jev-config-row">
          <div class="jev-config-copy">
            <strong>Jev Router</strong>
            <span>Generative router model: typesafe/jev-router</span>
          </div>
          <button id="jevRouterSetupButton" type="button" class="primary small">Set up Router</button>
        </div>
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
  const routerStatus = document.getElementById("jevRouterStatus") as HTMLElement;
  const decisionSelect = document.getElementById("jevDecisionEngineSelect") as HTMLSelectElement;
  const decisionStatus = document.getElementById("jevDecisionStatus") as HTMLElement;
  const closeButton = document.getElementById("jevConfigClose") as HTMLButtonElement;
  const doneButton = document.getElementById("jevConfigDone") as HTMLButtonElement;
  const providersUI = K.__providersUi;

  let currentProvider: TLStudioDynamicRecord | null = null;
  let currentRouterReady = false;

  const setText = (element: HTMLElement, message = "", kind = "") => {
    element.textContent = message;
    element.className = `jev-config-status${kind ? ` ${kind}` : ""}`;
  };

  const providerHasRouter = (provider: TLStudioDynamicRecord | null) =>
    !!provider && Array.isArray(provider.models) &&
    provider.models.some((model: TLStudioDynamicRecord) => clean(model?.id) === JEV_ROUTER_MODEL);

  const selectOpenRouterProvider = (
    providers: TLStudioDynamicRecord[],
    decision: TLStudioDynamicRecord,
  ) => {
    const candidates = providers.filter((provider) => isOpenRouter(provider?.baseURL));
    return candidates.find(providerHasRouter)
      || candidates.find((provider) => clean(provider?.id) === clean(decision?.providerID))
      || candidates[0]
      || null;
  };

  const nextProviderID = (providers: TLStudioDynamicRecord[]) => {
    const used = new Set(providers.map((provider) => clean(provider?.id)));
    if (!used.has("openrouter")) return "openrouter";
    if (!used.has("typesafe-openrouter")) return "typesafe-openrouter";
    let suffix = 2;
    while (used.has(`typesafe-openrouter-${suffix}`)) suffix++;
    return `typesafe-openrouter-${suffix}`;
  };

  const providerFormValue = (provider: TLStudioDynamicRecord) => {
    const first = Array.isArray(provider?.models) && provider.models.length ? provider.models[0] : {};
    return {
      providerID: provider.id,
      name: provider.name || provider.id,
      protocol: provider.protocol || "openai-compatible",
      baseURL: provider.baseURL || OPENROUTER_BASE_URL,
      modelID: first.id || "",
      modelName: first.name || first.id || "",
      contextLimit: first.contextLimit || "",
      outputLimit: first.outputLimit || "",
      toolCall: first.toolCall !== false,
      reasoning: first.reasoning === true,
    };
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
      const credentialConnected = !!currentProvider && (
        K.state.connectedProviders?.has?.(currentProvider.id)
        || (decision?.providerConfigured === true && clean(decision?.providerID) === clean(currentProvider.id))
      );
      currentRouterReady = hasRouter && credentialConnected;

      compactControl.classList.toggle("active", currentRouterReady);
      compactControlLabel.textContent = currentRouterReady ? "Active JEV" : "Configure JEV";
      compactStatus.textContent = currentRouterReady
        ? `Jev Router ready via ${currentProvider?.name || currentProvider?.id || "OpenRouter"}`
        : hasRouter
          ? "Jev Router is saved, but the OpenRouter credential is not connected."
          : currentProvider
            ? "OpenRouter is configured. Jev Router is not selected yet."
            : "Not configured";

      setupButton.textContent = hasRouter ? "Manage Provider" : (currentProvider ? "Add Jev Router" : "Set up Router");
      setText(routerStatus, currentRouterReady
        ? `Ready. TL Studio will use ${JEV_ROUTER_MODEL} through ${currentProvider?.name || currentProvider?.id}.`
        : hasRouter
          ? "Jev Router is saved, but its OpenRouter credential is not connected."
          : currentProvider
            ? `OpenRouter provider “${currentProvider.name || currentProvider.id}” is available. Add Jev Router to its selected models.`
            : "Configure an OpenRouter provider and select Jev Router. The same OpenRouter credential is reused.",
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
      compactControl.classList.remove("active");
      compactControlLabel.textContent = "Configure JEV";
      compactStatus.textContent = "Could not read Jev status";
      setText(routerStatus, error instanceof Error ? error.message : String(error), "error");
    }
  };

  const openDialog = async () => {
    await refreshStatus();
    if (!dialog.open) dialog.showModal();
  };

  const openExistingProvider = async (provider: TLStudioDynamicRecord, discoverRouter: boolean) => {
    dialog.close();
    providersUI.discoverySelection?.reset?.();
    providersUI.discoverySelection?.setAssumeUnknownTools?.(false);
    providersUI.openProvider?.(providerFormValue(provider), provider.id);
    if (!discoverRouter) return;
    const found = await providersUI.discoverySelection?.discoverModel?.(JEV_ROUTER_MODEL);
    if (!found) {
      compactStatus.textContent = "Jev Router was not returned by OpenRouter.";
    }
  };

  const openNewProvider = async () => {
    const config = await K.api.providers.config();
    const providers = Array.isArray(config?.providers) ? config.providers : [];
    dialog.close();
    providersUI.discoverySelection?.reset?.();
    providersUI.discoverySelection?.setAssumeUnknownTools?.(false);
    providersUI.openProvider?.({
      providerID: nextProviderID(providers),
      name: "TypeSafe via OpenRouter",
      protocol: "openai-compatible",
      baseURL: OPENROUTER_BASE_URL,
      modelID: "",
      modelName: "",
      contextLimit: "",
      outputLimit: "",
      toolCall: true,
      reasoning: false,
    });
    (document.getElementById("providerApiKeyInput") as HTMLInputElement | null)?.focus();
  };

  const setupRouter = async () => {
    setupButton.disabled = true;
    try {
      await refreshStatus();
      if (currentProvider) {
        await openExistingProvider(currentProvider, !providerHasRouter(currentProvider));
      } else {
        await openNewProvider();
      }
    } catch (error) {
      setText(routerStatus, `Could not prepare Jev Router: ${error instanceof Error ? error.message : String(error)}`, "error");
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
