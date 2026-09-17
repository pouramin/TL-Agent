(() => {
  "use strict";

  const K = window.KLU;
  if (!K || K.__providersUiInstalled) return;
  K.__providersUiInstalled = true;

  const PACKAGES = Object.freeze({
    "openai-compatible": "@ai-sdk/openai-compatible",
    "openai-responses": "@ai-sdk/openai",
    "anthropic-messages": "@ai-sdk/anthropic",
  });
  const PACKAGE_PROTOCOL = Object.fromEntries(Object.entries(PACKAGES).map(([protocol, npm]) => [npm, protocol]));
  const PROVIDER_ID = /^[a-z0-9][a-z0-9-_]*$/;
  const SUPPORTED_PACKAGES = new Set(Object.values(PACKAGES));

  const clean = (value) => String(value ?? "").trim();
  const positiveInt = (value) => {
    const raw = clean(value);
    if (!raw) return undefined;
    const number = Number(raw);
    return Number.isInteger(number) && number > 0 ? number : NaN;
  };
  const safeURL = (value) => {
    try {
      const url = new URL(clean(value));
      if (!/^https?:$/.test(url.protocol)) return "";
      return url.toString().replace(/\/$/, "");
    } catch { return ""; }
  };

  const validateDraft = (draft) => {
    const id = clean(draft.providerID);
    if (!PROVIDER_ID.test(id)) return "Provider ID must use lowercase letters, numbers, dashes, or underscores.";
    if (!clean(draft.name)) return "Display name is required.";
    if (!PACKAGES[draft.protocol]) return "Choose a supported provider API.";
    if (!safeURL(draft.baseURL)) return "Enter a valid http(s) Base URL.";
    if (!clean(draft.modelID)) return "Model ID is required.";
    const context = positiveInt(draft.contextLimit);
    const output = positiveInt(draft.outputLimit);
    if (Number.isNaN(context)) return "Context limit must be a positive whole number.";
    if (Number.isNaN(output)) return "Output limit must be a positive whole number.";
    return "";
  };

  const buildProviderConfig = (draft, existing = {}) => {
    const modelID = clean(draft.modelID);
    const context = positiveInt(draft.contextLimit);
    const output = positiveInt(draft.outputLimit);
    const previousOptions = existing?.options && typeof existing.options === "object" ? existing.options : {};
    const previousModels = existing?.models && typeof existing.models === "object" ? existing.models : {};
    const previousModel = previousModels[modelID] && typeof previousModels[modelID] === "object" ? previousModels[modelID] : {};
    const limit = {
      ...(previousModel.limit && typeof previousModel.limit === "object" ? previousModel.limit : {}),
      ...(context ? { context } : {}),
      ...(output ? { output } : {}),
    };
    if (!context) delete limit.context;
    if (!output) delete limit.output;

    return {
      ...existing,
      name: clean(draft.name),
      npm: PACKAGES[draft.protocol],
      options: {
        ...previousOptions,
        baseURL: safeURL(draft.baseURL),
      },
      models: {
        ...previousModels,
        [modelID]: {
          ...previousModel,
          name: clean(draft.modelName) || modelID,
          tool_call: draft.toolCall !== false,
          reasoning: draft.reasoning === true,
          ...(Object.keys(limit).length ? { limit } : {}),
        },
      },
    };
  };

  const customProviderEntries = (overlay) => {
    const providers = overlay?.effective?.provider;
    if (!providers || typeof providers !== "object") return [];
    return Object.entries(providers)
      .filter(([, config]) => config && typeof config === "object" && SUPPORTED_PACKAGES.has(config.npm))
      .map(([id, config]) => ({ id, config }))
      .sort((a, b) => String(a.config.name || a.id).localeCompare(String(b.config.name || b.id)));
  };

  K.__providersUi = {
    PACKAGES,
    validateDraft,
    buildProviderConfig,
    customProviderEntries,
  };

  const settingsDialog = document.getElementById("settingsDialog");
  const settingsNav = settingsDialog?.querySelector(".settings-nav");
  const settingsContent = settingsDialog?.querySelector(".settings-content");
  if (!settingsDialog || !settingsNav || !settingsContent) return;

  const navButton = document.createElement("button");
  navButton.className = "settings-nav-item";
  navButton.type = "button";
  navButton.dataset.settingsSection = "providers";
  navButton.innerHTML = '<span class="settings-nav-icon" aria-hidden="true">◇</span><span>Providers</span>';
  const aboutButton = settingsNav.querySelector('[data-settings-section="about"]');
  settingsNav.insertBefore(navButton, aboutButton || null);

  const panel = document.createElement("section");
  panel.className = "settings-panel hidden providers-settings-panel";
  panel.dataset.settingsPanel = "providers";
  panel.innerHTML = `
    <div class="settings-panel-head providers-panel-head">
      <div>
        <h3>Providers</h3>
        <p>Add OpenAI-compatible, OpenAI Responses, or Anthropic-compatible endpoints to TL Agent. Models saved here appear in TL Agent's model selector.</p>
      </div>
      <button id="providerAddButton" class="primary provider-add-button" type="button">Add provider</button>
    </div>
    <div id="providerNotice" class="provider-notice hidden" role="status"></div>
    <div id="providerList" class="provider-list"></div>
    <form id="providerForm" class="provider-form hidden">
      <div class="provider-form-head">
        <div><strong id="providerFormTitle">Add provider</strong><span>Configuration is saved globally by TL Agent and is available across projects.</span></div>
        <button id="providerFormCancelTop" class="icon-button" type="button" aria-label="Close provider form">×</button>
      </div>
      <div class="provider-form-grid">
        <label><span>Provider ID</span><input id="providerIdInput" autocomplete="off" spellcheck="false" placeholder="my-provider" /></label>
        <label><span>Display name</span><input id="providerNameInput" autocomplete="off" placeholder="My Provider" /></label>
        <label><span>Provider API</span><select id="providerProtocolSelect"><option value="openai-compatible">OpenAI Compatible</option><option value="openai-responses">OpenAI Responses</option><option value="anthropic-messages">Anthropic Messages</option></select></label>
        <label class="provider-field-wide"><span>Base URL</span><input id="providerBaseUrlInput" autocomplete="off" spellcheck="false" placeholder="https://api.example.com/v1" /></label>
        <label class="provider-field-wide"><span>API key</span><input id="providerApiKeyInput" type="password" autocomplete="new-password" spellcheck="false" placeholder="Leave blank to keep an existing key" /></label>
        <label><span>Model ID</span><input id="providerModelIdInput" autocomplete="off" spellcheck="false" placeholder="model-id" /></label>
        <label><span>Model name</span><input id="providerModelNameInput" autocomplete="off" placeholder="Model name" /></label>
        <label><span>Context limit</span><input id="providerContextInput" inputmode="numeric" autocomplete="off" placeholder="Optional" /></label>
        <label><span>Max output</span><input id="providerOutputInput" inputmode="numeric" autocomplete="off" placeholder="Optional" /></label>
      </div>
      <div class="provider-toggles">
        <label><input id="providerToolCallInput" type="checkbox" checked /> <span>Tool calling</span></label>
        <label><input id="providerReasoningInput" type="checkbox" /> <span>Reasoning</span></label>
      </div>
      <div class="provider-security-note">API keys are stored by the local runtime credential store. TL Agent does not save provider keys in browser storage or in the provider config file.</div>
      <div class="provider-limit-note">For custom models, set context/output limits when you know them. The current runtime disables automatic context compaction when a custom model has no known context limit.</div>
      <div class="dialog-actions provider-form-actions"><button id="providerFormCancel" class="ghost" type="button">Cancel</button><button id="providerFormSave" class="primary" type="submit">Save provider</button></div>
    </form>
  `;
  const aboutPanel = settingsContent.querySelector('[data-settings-panel="about"]');
  settingsContent.insertBefore(panel, aboutPanel || null);

  const style = document.createElement("style");
  style.id = "tl-providers-ui-style";
  style.textContent = `
    .providers-panel-head{display:flex;align-items:flex-start;justify-content:space-between;gap:16px}.provider-add-button{flex:none}
    .provider-notice{margin:-8px 0 14px;padding:9px 10px;border:1px solid var(--line);border-radius:8px;background:var(--panel-2);font-size:10px;line-height:1.45}.provider-notice.error{border-color:color-mix(in srgb,var(--danger),var(--line) 55%);color:var(--danger)}
    .provider-list{display:grid;gap:8px}.provider-empty{padding:24px 12px;border:1px dashed var(--line);border-radius:10px;color:var(--muted);font-size:10px;text-align:center}.provider-item{display:grid;grid-template-columns:minmax(0,1fr) auto;gap:12px;align-items:center;padding:11px 12px;border:1px solid var(--line);border-radius:10px;background:var(--panel)}.provider-item-title{display:flex;align-items:center;gap:7px}.provider-item-title strong{font-size:11px}.provider-status-dot{width:7px;height:7px;border-radius:50%;background:var(--muted-2)}.provider-status-dot.ok{background:var(--accent)}.provider-item-meta{margin-top:4px;color:var(--muted);font-size:9px;line-height:1.45}.provider-item-actions{display:flex;gap:6px}.provider-delete{color:var(--danger)}
    .provider-form{margin-top:14px;padding:14px;border:1px solid var(--line);border-radius:11px;background:var(--panel-2)}.provider-form-head{display:flex;align-items:flex-start;justify-content:space-between;gap:12px;margin-bottom:13px}.provider-form-head strong,.provider-form-head span{display:block}.provider-form-head strong{font-size:12px}.provider-form-head span{margin-top:3px;color:var(--muted);font-size:9px}.provider-form-grid{display:grid;grid-template-columns:1fr 1fr;gap:10px}.provider-form-grid label>span{display:block;margin:0 0 5px;color:var(--muted);font-size:9px;font-weight:650}.provider-form-grid input,.provider-form-grid select{box-sizing:border-box;width:100%;height:34px}.provider-field-wide{grid-column:1/-1}.provider-toggles{display:flex;gap:18px;margin-top:12px;color:var(--text);font-size:10px}.provider-toggles label{display:flex;align-items:center;gap:5px}.provider-security-note,.provider-limit-note{margin-top:11px;color:var(--muted);font-size:9px;line-height:1.5}.provider-security-note{color:color-mix(in srgb,var(--accent),var(--text) 45%)}.provider-form-actions{padding:13px 0 0}.providers-settings-panel.busy{opacity:.72;pointer-events:none}
    @media(max-width:760px){.providers-panel-head{display:block}.provider-add-button{margin-top:10px}.provider-form-grid{grid-template-columns:1fr}.provider-field-wide{grid-column:auto}.provider-item{grid-template-columns:1fr}.provider-item-actions{justify-content:flex-end}}
  `;
  document.head.appendChild(style);

  const $ = (id) => document.getElementById(id);
  const els = {
    add: $("providerAddButton"), notice: $("providerNotice"), list: $("providerList"), form: $("providerForm"),
    title: $("providerFormTitle"), cancel: $("providerFormCancel"), cancelTop: $("providerFormCancelTop"), save: $("providerFormSave"),
    id: $("providerIdInput"), name: $("providerNameInput"), protocol: $("providerProtocolSelect"), baseURL: $("providerBaseUrlInput"), apiKey: $("providerApiKeyInput"),
    modelID: $("providerModelIdInput"), modelName: $("providerModelNameInput"), context: $("providerContextInput"), output: $("providerOutputInput"),
    toolCall: $("providerToolCallInput"), reasoning: $("providerReasoningInput"),
  };

  let overlay = null;
  let editingID = "";
  let saving = false;

  const setBusy = (value) => {
    saving = value;
    panel.classList.toggle("busy", value);
    if (els.save) els.save.disabled = value;
  };
  const notice = (message = "", error = false) => {
    els.notice.textContent = message;
    els.notice.classList.toggle("hidden", !message);
    els.notice.classList.toggle("error", !!message && error);
  };
  const activate = () => {
    for (const button of settingsDialog.querySelectorAll("[data-settings-section]")) {
      const active = button.dataset.settingsSection === "providers";
      button.classList.toggle("active", active);
      if (active) button.setAttribute("aria-current", "page"); else button.removeAttribute("aria-current");
    }
    for (const item of settingsDialog.querySelectorAll("[data-settings-panel]")) item.classList.toggle("hidden", item.dataset.settingsPanel !== "providers");
  };

  const draft = () => ({
    providerID: els.id.value,
    name: els.name.value,
    protocol: els.protocol.value,
    baseURL: els.baseURL.value,
    apiKey: els.apiKey.value,
    modelID: els.modelID.value,
    modelName: els.modelName.value,
    contextLimit: els.context.value,
    outputLimit: els.output.value,
    toolCall: els.toolCall.checked,
    reasoning: els.reasoning.checked,
  });

  const clearForm = () => {
    editingID = "";
    els.form.reset();
    els.protocol.value = "openai-compatible";
    els.toolCall.checked = true;
    els.reasoning.checked = false;
    els.id.disabled = false;
    els.title.textContent = "Add provider";
    els.form.classList.add("hidden");
  };

  const fillForm = (value, existingID = "") => {
    editingID = existingID;
    els.id.value = value.providerID || "";
    els.name.value = value.name || "";
    els.protocol.value = value.protocol || "openai-compatible";
    els.baseURL.value = value.baseURL || "";
    els.apiKey.value = "";
    els.modelID.value = value.modelID || "";
    els.modelName.value = value.modelName || "";
    els.context.value = value.contextLimit || "";
    els.output.value = value.outputLimit || "";
    els.toolCall.checked = value.toolCall !== false;
    els.reasoning.checked = value.reasoning === true;
    els.id.disabled = !!existingID;
    els.title.textContent = existingID ? `Edit ${value.name || existingID}` : "Add provider";
    els.form.classList.remove("hidden");
    els.form.scrollIntoView?.({ block: "nearest" });
  };

  const modelCount = (cfg) => Object.keys(cfg?.models && typeof cfg.models === "object" ? cfg.models : {}).length;
  const renderList = () => {
    els.list.textContent = "";
    const entries = customProviderEntries(overlay);
    if (!entries.length) {
      const empty = document.createElement("div");
      empty.className = "provider-empty";
      empty.textContent = "No custom providers yet. Add any compatible endpoint to get started.";
      els.list.appendChild(empty);
      return;
    }
    for (const entry of entries) {
      const row = document.createElement("div");
      row.className = "provider-item";
      const copy = document.createElement("div");
      const title = document.createElement("div");
      title.className = "provider-item-title";
      const dot = document.createElement("span");
      const loaded = K.state.providers.some((provider) => provider?.id === entry.id);
      dot.className = `provider-status-dot${loaded ? " ok" : ""}`;
      const strong = document.createElement("strong");
      strong.textContent = entry.config.name || entry.id;
      title.append(dot, strong);
      const meta = document.createElement("div");
      meta.className = "provider-item-meta";
      const protocol = PACKAGE_PROTOCOL[entry.config.npm] || entry.config.npm;
      const keyed = K.state.connectedProviders.has(entry.id);
      meta.textContent = `${entry.id} · ${protocol} · ${modelCount(entry.config)} model${modelCount(entry.config) === 1 ? "" : "s"} · ${keyed ? "credentials connected" : "no stored key"}`;
      copy.append(title, meta);

      const actions = document.createElement("div");
      actions.className = "provider-item-actions";
      const edit = document.createElement("button");
      edit.type = "button";
      edit.className = "ghost small";
      edit.textContent = "Edit";
      edit.addEventListener("click", () => editEntry(entry));
      const remove = document.createElement("button");
      remove.type = "button";
      remove.className = "ghost small provider-delete";
      remove.textContent = "Delete";
      remove.addEventListener("click", () => deleteEntry(entry));
      actions.append(edit, remove);
      row.append(copy, actions);
      els.list.appendChild(row);
    }
  };

  const load = async () => {
    notice("");
    try {
      overlay = await K.api.config.overlay({ scope: "global" });
      renderList();
    } catch (error) {
      notice(`Could not load provider settings: ${error.message || String(error)}`, true);
    }
  };

  const editEntry = (entry) => {
    const cfg = entry.config || {};
    const models = cfg.models && typeof cfg.models === "object" ? cfg.models : {};
    const first = Object.entries(models)[0] || ["", {}];
    const modelID = first[0];
    const model = first[1] || {};
    fillForm({
      providerID: entry.id,
      name: cfg.name || entry.id,
      protocol: PACKAGE_PROTOCOL[cfg.npm] || "openai-compatible",
      baseURL: cfg.options?.baseURL || "",
      modelID,
      modelName: model.name || modelID,
      contextLimit: model.limit?.context || "",
      outputLimit: model.limit?.output || "",
      toolCall: model.tool_call !== false,
      reasoning: model.reasoning === true,
    }, entry.id);
  };

  const save = async (event) => {
    event?.preventDefault();
    if (saving) return;
    const value = draft();
    const error = validateDraft(value);
    if (error) return notice(error, true);

    setBusy(true);
    notice("");
    try {
      overlay = await K.api.config.overlay({ scope: "global" });
      const effective = overlay?.effective && typeof overlay.effective === "object" ? overlay.effective : {};
      const providers = { ...(effective.provider && typeof effective.provider === "object" ? effective.provider : {}) };
      const existing = providers[editingID || clean(value.providerID)] || {};
      const id = clean(value.providerID);
      providers[id] = buildProviderConfig(value, existing);
      const disabled = Array.isArray(effective.disabled_providers) ? effective.disabled_providers.filter((item) => item !== id) : [];

      await K.api.config.update({ scope: "global", set: { provider: providers, disabled_providers: disabled } });
      if (clean(value.apiKey)) await K.api.auth.setApiKey(id, clean(value.apiKey));
      await K.api.runtime.dispose();
      await K.loadCatalog();
      overlay = await K.api.config.overlay({ scope: "global" });
      renderList();
      clearForm();
      const loaded = K.state.models.some((model) => model.providerID === id && model.id === clean(value.modelID));
      notice(loaded
        ? `${value.name} saved. ${clean(value.modelID)} is now available in the model selector.`
        : `${value.name} was saved, but the runtime did not load ${clean(value.modelID)}. Check the endpoint, protocol, and model ID.`, !loaded);
    } catch (err) {
      notice(`Could not save provider: ${err.message || String(err)}`, true);
    } finally {
      setBusy(false);
    }
  };

  const deleteEntry = async (entry) => {
    if (saving) return;
    if (!window.confirm(`Delete provider “${entry.config.name || entry.id}”? Stored credentials for this provider will also be removed.`)) return;
    setBusy(true);
    notice("");
    try {
      overlay = await K.api.config.overlay({ scope: "global" });
      const effective = overlay?.effective && typeof overlay.effective === "object" ? overlay.effective : {};
      const providers = { ...(effective.provider && typeof effective.provider === "object" ? effective.provider : {}) };
      providers[entry.id] = null;
      await K.api.config.update({ scope: "global", set: { provider: providers } });
      await K.api.auth.remove(entry.id).catch(() => {});
      await K.api.runtime.dispose();
      await K.loadCatalog();
      overlay = await K.api.config.overlay({ scope: "global" });
      renderList();
      clearForm();
      notice(`${entry.config.name || entry.id} removed.`);
    } catch (error) {
      notice(`Could not delete provider: ${error.message || String(error)}`, true);
    } finally {
      setBusy(false);
    }
  };

  navButton.addEventListener("click", () => { activate(); load(); });
  els.add.addEventListener("click", () => { notice(""); fillForm({ toolCall: true }); });
  els.cancel.addEventListener("click", clearForm);
  els.cancelTop.addEventListener("click", clearForm);
  els.form.addEventListener("submit", save);
})();