import { K } from "./kernel";

(() => {
  "use strict";

  if (!K || K.__pluginsInstalled) return;
  K.__pluginsInstalled = true;
  K.state.plugins = [];
  let savedPlugins: TLStudioPluginView[] = [];
  let pluginCatalog: TLStudioPluginCatalogEntry[] = [];

  const panel = document.querySelector<HTMLElement>('[data-settings-panel="plugins"]');
  const settingsDialog = document.getElementById("settingsDialog") as HTMLDialogElement | null;
  const list = document.getElementById("pluginList");
  const editor = document.getElementById("pluginEditor");
  const pluginDialog = document.getElementById("pluginDialog") as HTMLDialogElement | null;
  const pluginDialogTitle = document.getElementById("pluginDialogTitle");
  const pluginDialogClose = document.getElementById("pluginDialogClose") as HTMLButtonElement | null;
  const addButton = document.getElementById("pluginAddButton") as HTMLButtonElement | null;
  const editID = document.getElementById("pluginEditID") as HTMLInputElement | null;
  const nameInput = document.getElementById("pluginNameInput") as HTMLInputElement | null;
  const descriptionInput = document.getElementById("pluginDescriptionInput") as HTMLInputElement | null;
  const commandInput = document.getElementById("pluginCommandInput") as HTMLInputElement | null;
  const argsInput = document.getElementById("pluginArgsInput") as HTMLTextAreaElement | null;
  const transportSelect = document.getElementById("pluginTransportSelect") as HTMLSelectElement | null;
  const scopeSelect = document.getElementById("pluginScopeSelect") as HTMLSelectElement | null;
  const cwdInput = document.getElementById("pluginCwdInput") as HTMLInputElement | null;
  const envInput = document.getElementById("pluginEnvInput") as HTMLTextAreaElement | null;
  const status = document.getElementById("pluginEditorStatus");
  const cancelButton = document.getElementById("pluginCancelButton") as HTMLButtonElement | null;
  const testButton = document.getElementById("pluginTestButton") as HTMLButtonElement | null;
  const saveButton = document.getElementById("pluginSaveButton") as HTMLButtonElement | null;
  const jevDirectDialog = document.getElementById("jevDirectDialog") as HTMLDialogElement | null;
  const jevDirectClose = document.getElementById("jevDirectClose") as HTMLButtonElement | null;
  const jevDirectCancel = document.getElementById("jevDirectCancel") as HTMLButtonElement | null;
  const jevDirectSave = document.getElementById("jevDirectSave") as HTMLButtonElement | null;
  const jevDirectApiKey = document.getElementById("jevDirectApiKey") as HTMLInputElement | null;
  const jevDirectStatus = document.getElementById("jevDirectStatus");
  const layaRoutingDialog = document.getElementById("layaRoutingDialog") as HTMLDialogElement | null;
  const layaRoutingClose = document.getElementById("layaRoutingClose") as HTMLButtonElement | null;
  const layaRoutingCancel = document.getElementById("layaRoutingCancel") as HTMLButtonElement | null;
  const layaRoutingSave = document.getElementById("layaRoutingSave") as HTMLButtonElement | null;
  const layaRoutingProfile = document.getElementById("layaRoutingProfile") as HTMLSelectElement | null;
  const layaRoutingModels = document.getElementById("layaRoutingModels");
  const layaRoutingPoolSummary = document.getElementById("layaRoutingPoolSummary");
  const layaRoutingStatus = document.getElementById("layaRoutingStatus");
  const layaRoutingPreviewInput = document.getElementById("layaRoutingPreviewInput") as HTMLTextAreaElement | null;
  const layaRoutingPreviewButton = document.getElementById("layaRoutingPreviewButton") as HTMLButtonElement | null;
  const layaRoutingPreviewResult = document.getElementById("layaRoutingPreviewResult");
  let layaRoutingSnapshot: TLStudioLayaRouterStatus | null = null;
  let layaRoutingDraftModels: TLStudioLayaRouterModel[] = [];
  if (!panel || !list || !editor || !pluginDialog || !addButton || !nameInput || !commandInput || !argsInput || !transportSelect || !scopeSelect || !cwdInput || !envInput || !editID) return;

  const setStatus = (message = "", kind = "") => {
    if (!status) return;
    status.textContent = message;
    status.className = `plugin-editor-status${kind ? ` ${kind}` : ""}`;
  };

  const parseArguments = () => String(argsInput.value || "")
    .split(/\r?\n/)
    .map((value) => value.trim())
    .filter(Boolean);

  const parseEnvironment = () => {
    const result: Record<string, string> = {};
    for (const raw of String(envInput.value || "").split(/\r?\n/)) {
      const line = raw.trim();
      if (!line) continue;
      const index = line.indexOf("=");
      const key = (index < 0 ? line : line.slice(0, index)).trim();
      const value = index < 0 ? "" : line.slice(index + 1);
      if (key) result[key] = value;
    }
    return result;
  };

  const formPlugin = () => ({
    id: editID.value || undefined,
    name: nameInput.value.trim(),
    description: descriptionInput?.value.trim() || "",
    type: "mcp",
    enabled: editID.value
      ? !!K.state.plugins.find((item) => item.id === editID.value)?.enabled
      : false,
    scope: scopeSelect.value || "project",
    transport: transportSelect.value || "stdio",
    command: commandInput.value.trim(),
    arguments: parseArguments(),
    workingDirectory: cwdInput.value.trim(),
  });

  const resetEditor = () => {
    editID.value = "";
    nameInput.value = "";
    if (descriptionInput) descriptionInput.value = "";
    commandInput.value = "";
    argsInput.value = "";
    transportSelect.value = "stdio";
    scopeSelect.value = "project";
    cwdInput.value = "";
    envInput.value = "";
    setStatus();
  };

  const closeEditor = () => {
    if (pluginDialog.open) pluginDialog.close();
    resetEditor();
  };

  const openEditor = (plugin?: TLStudioPluginView) => {
    editID.value = plugin?.id || "";
    nameInput.value = plugin?.name || "";
    if (descriptionInput) descriptionInput.value = plugin?.description || "";
    commandInput.value = plugin?.command || "";
    argsInput.value = (plugin?.arguments || []).join("\n");
    transportSelect.value = plugin?.transport || "stdio";
    scopeSelect.value = plugin?.scope || "project";
    cwdInput.value = plugin?.workingDirectory || "";
    envInput.value = (plugin?.environment || []).map((item) => `${item.name}=`).join("\n");
    setStatus(plugin?.environment?.length ? "Secret environment values are preserved unless you replace or remove their variable names." : "");
    if (pluginDialogTitle) pluginDialogTitle.textContent = plugin ? "Configure plugin" : "Add plugin";
    if (!pluginDialog.open) pluginDialog.showModal();
    requestAnimationFrame(() => nameInput.focus({ preventScroll: true }));
  };

  const statusClass = (value: string) => "plugin-status-" + String(value || "unknown").toLowerCase().replace(/[^a-z0-9]+/g, "-");

  const jevDirectConfigured = (plugin?: TLStudioPluginView | null) =>
    String(plugin?.integration?.details?.apiKeyConfigured || "").toLowerCase() === "true";

  const setJevDirectStatus = (message = "", kind = "") => {
    if (!jevDirectStatus) return;
    jevDirectStatus.textContent = message;
    jevDirectStatus.className = `plugin-editor-status${kind ? ` ${kind}` : ""}`;
  };

  const openJevDirectDialog = (plugin?: TLStudioPluginView | null) => {
    if (!jevDirectDialog || !jevDirectApiKey) return;
    jevDirectApiKey.value = "";
    setJevDirectStatus(
      jevDirectConfigured(plugin)
        ? "A TypeSafe API key is already stored. Leave the field blank to keep it, or enter a new key to replace it."
        : "Enter a TypeSafe API key to enable the direct JEV router.",
    );
    if (!jevDirectDialog.open) jevDirectDialog.showModal();
    requestAnimationFrame(() => jevDirectApiKey.focus({ preventScroll: true }));
  };

  const closeJevDirectDialog = () => {
    if (jevDirectDialog?.open) jevDirectDialog.close();
    if (jevDirectApiKey) jevDirectApiKey.value = "";
    setJevDirectStatus();
  };

  const pluginConfigForUpdate = (plugin: TLStudioPluginView) => ({
    id: plugin.id,
    name: plugin.name,
    description: plugin.description || "",
    type: plugin.type,
    enabled: plugin.enabled,
    scope: plugin.scope,
    project: plugin.project || "",
    transport: plugin.transport || "",
    command: plugin.command || "",
    arguments: plugin.arguments || [],
    workingDirectory: plugin.workingDirectory || "",
    environment: plugin.environment || [],
    metadata: plugin.metadata || {},
  });

  const actionButton = (label: string, action: string, pluginID: string, className = "ghost small") => {
    const button = document.createElement("button");
    button.type = "button";
    button.className = className;
    button.textContent = label;
    button.dataset.pluginAction = action;
    button.dataset.pluginId = pluginID;
    return button;
  };

  const renderCard = (plugin: TLStudioPluginView) => {
    const catalogPreset = pluginCatalog.find((preset) => preset.id === plugin.id);
    if (catalogPreset) return renderInstalledCatalogCard(plugin, catalogPreset);
    const bundled = plugin.origin === "bundled";
    const card = document.createElement("article");
    card.className = `plugin-card${bundled ? " plugin-card-bundled" : ""}`;
    card.dataset.pluginId = plugin.id;

    const head = document.createElement("div");
    head.className = "plugin-card-head";
    const copy = document.createElement("div");
    copy.className = "plugin-card-copy";
    const title = document.createElement("strong");
    title.textContent = plugin.name;
    const description = document.createElement("span");
    description.textContent = plugin.description || `${plugin.type.toUpperCase()} · ${plugin.transport}`;
    copy.append(title, description);

    const state = document.createElement("span");
    state.className = `plugin-status ${statusClass(plugin.status)}`;
    state.textContent = plugin.status || (plugin.enabled ? "Starting" : "Disabled");
    head.append(copy, state);

    const meta = document.createElement("div");
    meta.className = "plugin-meta";
    const scope = plugin.scope === "global" ? "All projects" : "Current project";
    meta.textContent = bundled
      ? `Included with TL Studio${plugin.version ? ` · v${plugin.version}` : ""} · ${plugin.discoveredTools || 0} tools`
      : `${plugin.transport} · ${scope} · ${plugin.discoveredTools || 0} tools`;

    const command = document.createElement("code");
    command.className = "plugin-command";
    command.textContent = [plugin.command, ...(plugin.arguments || [])].join(" ");

    card.append(head, meta);
    if (!bundled) card.appendChild(command);

    if (plugin.error) {
      const error = document.createElement("div");
      error.className = "plugin-error";
      error.textContent = plugin.error;
      card.appendChild(error);
    }

    if (plugin.integration?.summary) {
      const integration = document.createElement("div");
      integration.className = "plugin-integration-meta";
      integration.textContent = plugin.integration.summary;
      card.appendChild(integration);
    }

    const actions = document.createElement("div");
    actions.className = "plugin-card-actions";
    actions.append(
      actionButton(plugin.enabled ? "Disable" : "Enable", "toggle", plugin.id, plugin.enabled ? "ghost small" : "primary small"),
      actionButton("Test Connection", "test", plugin.id),
    );
    if (!bundled) {
      actions.append(
        actionButton("Configure", "configure", plugin.id),
        actionButton("Remove", "remove", plugin.id, "ghost small danger-text"),
      );
    }
    for (const integrationAction of plugin.integration?.actions || []) {
      const extra = actionButton(integrationAction.label, "integration", plugin.id);
      extra.dataset.integrationActionId = integrationAction.id;
      actions.insertBefore(extra, actions.children[1] || null);
    }
    card.appendChild(actions);
    return card;
  };

  const renderCatalogCard = (preset: TLStudioPluginCatalogEntry) => {
    const card = document.createElement("article");
    card.className = "plugin-catalog-card";
    card.dataset.pluginId = preset.id;

    const logo = document.createElement("div");
    logo.className = "plugin-catalog-logo";
    if (preset.icon) {
      const image = document.createElement("img");
      image.src = preset.icon;
      image.alt = "";
      logo.appendChild(image);
    } else {
      logo.textContent = (preset.name || "?").slice(0, 1).toUpperCase();
    }

    const name = document.createElement("strong");
    name.className = "plugin-catalog-name";
    name.textContent = preset.name;

    const state = document.createElement("span");
    state.className = "plugin-catalog-state";
    state.textContent = "Available";

    const category = document.createElement("span");
    category.className = "plugin-catalog-category";
    category.textContent = preset.category || "MCP integration";

    const description = document.createElement("p");
    description.className = "plugin-catalog-description";
    description.textContent = preset.description || "Curated MCP integration";

    const scope = document.createElement("span");
    scope.className = "plugin-catalog-scope";
    scope.textContent = preset.scope === "global" ? "All projects" : "Current project";

    const actions = document.createElement("div");
    actions.className = "plugin-catalog-actions";
    actions.append(actionButton("Add", "catalog-add", preset.id, "primary small"));

    card.append(logo, name, state, category, description, scope, actions);
    return card;
  };

  const renderInstalledCatalogCard = (plugin: TLStudioPluginView, preset: TLStudioPluginCatalogEntry) => {
    const card = document.createElement("article");
    card.className = "plugin-catalog-card plugin-catalog-installed";
    card.dataset.pluginId = plugin.id;

    const logo = document.createElement("div");
    logo.className = "plugin-catalog-logo";
    if (preset.icon) {
      const image = document.createElement("img");
      image.src = preset.icon;
      image.alt = "";
      logo.appendChild(image);
    } else {
      logo.textContent = (preset.name || "?").slice(0, 1).toUpperCase();
    }

    const name = document.createElement("strong");
    name.className = "plugin-catalog-name";
    name.textContent = plugin.name || preset.name;

    const state = document.createElement("span");
    state.className = `plugin-status ${statusClass(plugin.status)}`;
    state.textContent = plugin.status || (plugin.enabled ? "Starting" : "Disabled");

    const category = document.createElement("span");
    category.className = "plugin-catalog-category";
    category.textContent = preset.category || "MCP integration";

    const description = document.createElement("p");
    description.className = "plugin-catalog-description";
    description.textContent = plugin.description || preset.description || "Curated MCP integration";

    const scope = document.createElement("span");
    scope.className = "plugin-catalog-scope";
    const scopeText = plugin.scope === "global" ? "All projects" : "Current project";
    scope.textContent = `${scopeText} · ${plugin.discoveredTools || 0} tools`;

    const actions = document.createElement("div");
    actions.className = "plugin-catalog-actions";
    if (preset.id === "jev-direct") {
      actions.append(actionButton("Configure", "jev-direct-config", plugin.id, "ghost small"));
      if (jevDirectConfigured(plugin)) {
        actions.append(actionButton(plugin.enabled ? "Disable" : "Enable", "toggle", plugin.id, plugin.enabled ? "ghost small" : "primary small"));
      }
    } else {
      actions.append(
        actionButton(plugin.enabled ? "Disable" : "Enable", "toggle", plugin.id, plugin.enabled ? "ghost small" : "primary small"),
        actionButton("Test", "test", plugin.id),
      );
    }
    if (preset.id === "laya" && plugin.enabled) {
      actions.append(actionButton("Routing", "laya-routing", plugin.id, "ghost small"));
    }
    for (const integrationAction of plugin.integration?.actions || []) {
      const extra = actionButton(integrationAction.label, "integration", plugin.id);
      extra.dataset.integrationActionId = integrationAction.id;
      actions.appendChild(extra);
    }
    actions.append(actionButton("Remove", "remove", plugin.id, "ghost small danger-text"));

    card.append(logo, name, state, category, description, scope, actions);
    if (plugin.error) {
      const error = document.createElement("div");
      error.className = "plugin-error plugin-catalog-error";
      error.textContent = plugin.error;
      card.appendChild(error);
    }
    return card;
  };

  const renderSavedCard = (plugin: TLStudioPluginView) => {
    const card = document.createElement("article");
    card.className = "plugin-card plugin-card-saved";
    card.dataset.pluginId = plugin.id;

    const head = document.createElement("div");
    head.className = "plugin-card-head";
    const copy = document.createElement("div");
    copy.className = "plugin-card-copy";
    const title = document.createElement("strong");
    title.textContent = plugin.name;
    const description = document.createElement("span");
    description.textContent = plugin.description || "Saved MCP plugin";
    copy.append(title, description);

    const state = document.createElement("span");
    state.className = "plugin-status";
    state.textContent = "Saved";
    head.append(copy, state);

    const meta = document.createElement("div");
    meta.className = "plugin-meta plugin-saved-project";
    meta.textContent = `Saved for: ${plugin.project || "another project"}`;
    meta.title = plugin.project || "";

    const command = document.createElement("code");
    command.className = "plugin-command";
    command.textContent = [plugin.command, ...(plugin.arguments || [])].join(" ");

    const actions = document.createElement("div");
    actions.className = "plugin-card-actions";
    actions.append(actionButton("Use in current project", "attach", plugin.id, "primary small"));

    card.append(head, meta, command, actions);
    return card;
  };

  const appendSection = <T,>(
    titleText: string,
    subtitleText: string,
    plugins: T[],
    emptyTitle: string,
    emptyText: string,
    renderer: (plugin: T) => HTMLElement,
  ) => {
    const section = document.createElement("section");
    section.className = "plugin-section";
    const head = document.createElement("div");
    head.className = "plugin-section-head";
    const title = document.createElement("strong");
    title.textContent = titleText;
    const subtitle = document.createElement("span");
    subtitle.textContent = subtitleText;
    head.append(title, subtitle);
    section.appendChild(head);
    if (plugins.length) {
      const body = document.createElement("div");
      body.className = renderer === renderCatalogCard ? "plugin-catalog-grid" : "plugin-section-list";
      for (const plugin of plugins) body.appendChild(renderer(plugin));
      section.appendChild(body);
    } else {
      const empty = document.createElement("div");
      empty.className = "plugin-empty";
      const strong = document.createElement("strong");
      strong.textContent = emptyTitle;
      const span = document.createElement("span");
      span.textContent = emptyText;
      empty.append(strong, span);
      section.appendChild(empty);
    }
    list.appendChild(section);
  };

  const render = () => {
    list.textContent = "";
    const bundled = K.state.plugins.filter((plugin) => plugin.origin === "bundled");
    const userAdded = K.state.plugins.filter((plugin) => plugin.origin !== "bundled");
    const activePluginIDs = new Set(K.state.plugins.map((plugin) => plugin.id));
    const available = pluginCatalog.filter((preset) => !activePluginIDs.has(preset.id));

    appendSection(
      "Available integrations",
      "Curated integrations install and enable themselves when you choose Add.",
      available,
      "All curated integrations are added",
      "You can still add any compatible stdio MCP server with Add plugin.",
      renderCatalogCard,
    );

    if (bundled.length) {
      appendSection(
        "Included with TL Studio",
        "Version-pinned plugins shipped inside this TL Studio package.",
        bundled,
        "",
        "",
        renderCard,
      );
    }

    appendSection(
      "Added by you",
      "Installed integrations and external MCP servers configured for this project or globally.",
      userAdded,
      "No plugins added yet",
      "Choose an integration above or add any compatible stdio MCP server.",
      renderCard,
    );

    if (savedPlugins.length) {
      appendSection(
        "Saved for another project",
        "These project-scoped plugins still exist, but belong to a different project.",
        savedPlugins,
        "",
        "",
        renderSavedCard,
      );
    }
  };

  const load = async () => {
    try {
      const [current, saved, catalog] = await Promise.all([
        K.api.plugins.list(),
        K.api.plugins.saved(),
        K.api.plugins.catalog(),
      ]);
      K.state.plugins = current;
      savedPlugins = saved;
      pluginCatalog = catalog;
      render();
      return K.state.plugins;
    } catch (error) {
      list.textContent = `Could not load plugins: ${(error as Error).message || String(error)}`;
      return [];
    }
  };

  const busy = (button: HTMLButtonElement | null, value: boolean, text?: string) => {
    if (!button) return;
    if (value) {
      button.dataset.previousText = button.textContent || "";
      button.disabled = true;
      if (text) button.textContent = text;
    } else {
      button.disabled = false;
      if (button.dataset.previousText) button.textContent = button.dataset.previousText;
      delete button.dataset.previousText;
    }
  };

  const setLayaRoutingStatus = (message = "", kind = "") => {
    if (!layaRoutingStatus) return;
    layaRoutingStatus.textContent = message;
    layaRoutingStatus.className = `plugin-editor-status${kind ? ` ${kind}` : ""}`;
  };

  const scoreSelect = (className: string, value: number, label: string) => {
    const select = document.createElement("select");
    select.className = className;
    select.setAttribute("aria-label", label);
    for (let score = 1; score <= 5; score++) {
      const option = document.createElement("option");
      option.value = String(score);
      option.textContent = String(score);
      option.selected = score === Math.max(1, Math.min(5, Number(value) || 3));
      select.appendChild(option);
    }
    return select;
  };

  const layaModelKey = (model: Pick<TLStudioLayaRouterModel, "providerID" | "modelID">) =>
    `${model.providerID}\u0000${model.modelID}`;

  const cloneLayaModels = (models: TLStudioLayaRouterModel[] = []) =>
    models.map((model) => ({ ...model }));

  const layaGroupRank = (group: string) => {
    switch (group) {
      case "free": return 0;
      case "included": return 1;
      case "budget": return 2;
      case "standard": return 3;
      case "premium": return 4;
      default: return 3;
    }
  };

  const currentLayaProfile = () => layaRoutingProfile?.value || "balanced";

  const visibleLayaRoutingModels = () => {
    const profile = currentLayaProfile();
    let models = cloneLayaModels(layaRoutingDraftModels);
    if (profile === "free") models = models.filter((model) => model.group === "free");

    models.sort((a, b) => {
      const readiness = (model: TLStudioLayaRouterModel) =>
        (model.enabled ? 0 : 2) + (model.ready ? 0 : 1);
      const readyDiff = readiness(a) - readiness(b);
      if (readyDiff !== 0) return readyDiff;

      if (profile === "cost") {
        const groupDiff = layaGroupRank(a.group) - layaGroupRank(b.group);
        if (groupDiff !== 0) return groupDiff;
        if (a.quality !== b.quality) return b.quality - a.quality;
        if (a.speed !== b.speed) return b.speed - a.speed;
      } else if (profile === "quality" || profile === "free") {
        if (a.quality !== b.quality) return b.quality - a.quality;
        if (a.speed !== b.speed) return b.speed - a.speed;
        const groupDiff = layaGroupRank(a.group) - layaGroupRank(b.group);
        if (groupDiff !== 0) return groupDiff;
      } else if (profile === "speed") {
        if (a.speed !== b.speed) return b.speed - a.speed;
        if (a.quality !== b.quality) return b.quality - a.quality;
        const groupDiff = layaGroupRank(a.group) - layaGroupRank(b.group);
        if (groupDiff !== 0) return groupDiff;
      }

      return `${a.providerName || a.providerID} ${a.modelName || a.modelID}`
        .localeCompare(`${b.providerName || b.providerID} ${b.modelName || b.modelID}`);
    });
    return models;
  };

  const updateLayaPoolSummary = (visible: TLStudioLayaRouterModel[]) => {
    if (!layaRoutingPoolSummary) return;
    const total = layaRoutingDraftModels.length;
    const ready = visible.filter((model) => model.enabled && model.ready).length;
    const profile = currentLayaProfile();
    const prefix = profile === "free"
      ? "Free only"
      : profile === "cost"
        ? "Lowest cost order"
        : profile === "quality"
          ? "Best quality order"
          : profile === "speed"
            ? "Fastest order"
            : "Balanced pool";
    layaRoutingPoolSummary.textContent =
      `${prefix} · showing ${visible.length} of ${total} models · ${ready} ready`;
  };

  const patchLayaDraftModel = (
    providerID: string,
    modelID: string,
    patch: Partial<TLStudioLayaRouterModel>,
  ) => {
    const key = `${providerID}\u0000${modelID}`;
    const model = layaRoutingDraftModels.find((item) => layaModelKey(item) === key);
    if (model) Object.assign(model, patch);
  };

  const renderLayaRoutingModels = () => {
    if (!layaRoutingModels) return;
    layaRoutingModels.textContent = "";
    const visible = visibleLayaRoutingModels();
    updateLayaPoolSummary(visible);

    if (!layaRoutingDraftModels.length) {
      const empty = document.createElement("div");
      empty.className = "plugin-empty";
      empty.textContent = layaRoutingSnapshot?.message || "No tool-capable models are configured in TL Studio yet.";
      layaRoutingModels.appendChild(empty);
      return;
    }
    if (!visible.length) {
      const empty = document.createElement("div");
      empty.className = "plugin-empty";
      empty.textContent = currentLayaProfile() === "free"
        ? "No models are currently classified as Free. Switch to another profile to change a model's cost group."
        : "No models match this routing profile.";
      layaRoutingModels.appendChild(empty);
      return;
    }

    for (const model of visible) {
      const row = document.createElement("div");
      row.className = "laya-routing-model-row";
      row.dataset.providerId = model.providerID;
      row.dataset.modelId = model.modelID;

      const enabled = document.createElement("input");
      enabled.type = "checkbox";
      enabled.className = "laya-routing-model-enabled";
      enabled.checked = model.enabled;
      enabled.setAttribute("aria-label", `Use ${model.modelName || model.modelID} in Laya Router`);
      enabled.addEventListener("change", () => {
        patchLayaDraftModel(model.providerID, model.modelID, { enabled: enabled.checked });
        updateLayaPoolSummary(visibleLayaRoutingModels());
      });

      const copy = document.createElement("div");
      copy.className = "laya-routing-model-copy";
      const name = document.createElement("strong");
      name.textContent = model.modelName || model.modelID;
      const meta = document.createElement("span");
      meta.textContent = `${model.providerName || model.providerID} · ${model.availability || (model.ready ? "Ready" : model.connected ? "Unavailable" : "Not connected")}`;
      copy.append(name, meta);

      const group = document.createElement("select");
      group.className = "laya-routing-model-group";
      group.setAttribute("aria-label", `Cost group for ${model.modelName || model.modelID}`);
      for (const [value, label] of [
        ["free", "Free"],
        ["included", "Included quota"],
        ["budget", "Budget"],
        ["standard", "Standard"],
        ["premium", "Premium"],
      ]) {
        const option = document.createElement("option");
        option.value = value;
        option.textContent = label;
        option.selected = value === model.group;
        group.appendChild(option);
      }
      group.addEventListener("change", () => {
        patchLayaDraftModel(model.providerID, model.modelID, { group: group.value });
        renderLayaRoutingModels();
      });

      const quality = scoreSelect("laya-routing-model-quality", model.quality, `Quality for ${model.modelName || model.modelID}`);
      quality.addEventListener("change", () => {
        patchLayaDraftModel(model.providerID, model.modelID, { quality: Number(quality.value || 3) });
        if (currentLayaProfile() === "quality" || currentLayaProfile() === "free") renderLayaRoutingModels();
      });
      const speed = scoreSelect("laya-routing-model-speed", model.speed, `Speed for ${model.modelName || model.modelID}`);
      speed.addEventListener("change", () => {
        patchLayaDraftModel(model.providerID, model.modelID, { speed: Number(speed.value || 3) });
        if (currentLayaProfile() === "speed") renderLayaRoutingModels();
      });

      const connection = document.createElement("span");
      connection.className = `laya-routing-connection ${model.ready ? "connected" : ""}`;
      connection.textContent = model.ready ? "Ready" : (model.availability || (model.connected ? "Unavailable" : "Offline"));
      connection.title = model.availability || "";

      row.append(enabled, copy, group, quality, speed, connection);
      layaRoutingModels.appendChild(row);
    }
  };

  const collectLayaRoutingPolicy = () => ({
    version: 1,
    profile: currentLayaProfile(),
    models: layaRoutingDraftModels.map((model) => ({
      providerID: model.providerID,
      modelID: model.modelID,
      enabled: model.enabled,
      group: model.group || "standard",
      quality: Number(model.quality || 3),
      speed: Number(model.speed || 3),
    })),
  });

  const closeLayaRouting = () => {
    if (layaRoutingDialog?.open) layaRoutingDialog.close();
    layaRoutingSnapshot = null;
    layaRoutingDraftModels = [];
    if (layaRoutingPreviewResult) layaRoutingPreviewResult.textContent = "";
    setLayaRoutingStatus();
  };

  const openLayaRouting = async () => {
    if (!layaRoutingDialog || !layaRoutingProfile || !layaRoutingModels) return;
    if (!layaRoutingDialog.open) layaRoutingDialog.showModal();
    layaRoutingModels.textContent = "Loading model pool…";
    setLayaRoutingStatus();
    if (layaRoutingPreviewResult) layaRoutingPreviewResult.textContent = "";
    try {
      const status = await K.api.layaRouter.status();
      layaRoutingSnapshot = status;
      layaRoutingDraftModels = cloneLayaModels(status.models || []);
      layaRoutingProfile.value = status.profile || "balanced";
      renderLayaRoutingModels();
      if (status.message) setLayaRoutingStatus(status.message, status.available ? "" : "error");
    } catch (error) {
      layaRoutingModels.textContent = "";
      setLayaRoutingStatus((error as Error).message || String(error), "error");
    }
  };

  jevDirectClose?.addEventListener("click", closeJevDirectDialog);
  jevDirectCancel?.addEventListener("click", closeJevDirectDialog);
  jevDirectDialog?.addEventListener("click", (event) => {
    if (event.target === jevDirectDialog) closeJevDirectDialog();
  });
  jevDirectSave?.addEventListener("click", async () => {
    const plugin = K.state.plugins.find((item) => item.id === "jev-direct");
    if (!plugin || !jevDirectApiKey) return;
    const key = jevDirectApiKey.value.trim();
    if (!key && !jevDirectConfigured(plugin)) {
      setJevDirectStatus("Enter a TypeSafe API key.", "error");
      return;
    }
    busy(jevDirectSave, true, "Saving…");
    setJevDirectStatus("Saving the TypeSafe credential and enabling JEV Direct…");
    try {
      await K.api.plugins.update(plugin.id, pluginConfigForUpdate(plugin), { TYPESAFE_API_KEY: key });
      await K.api.plugins.setEnabled(plugin.id, true);
      await load();
      await K.loadCatalog?.().catch(() => {});
      setJevDirectStatus("JEV Direct Router is enabled.", "success");
      closeJevDirectDialog();
    } catch (error) {
      setJevDirectStatus((error as Error).message || String(error), "error");
    } finally {
      busy(jevDirectSave, false);
    }
  });

  layaRoutingClose?.addEventListener("click", closeLayaRouting);
  layaRoutingCancel?.addEventListener("click", closeLayaRouting);
  layaRoutingProfile?.addEventListener("change", () => {
    renderLayaRoutingModels();
    if (layaRoutingPreviewResult) layaRoutingPreviewResult.textContent = "";
    setLayaRoutingStatus();
  });
  layaRoutingSave?.addEventListener("click", async () => {
    busy(layaRoutingSave, true, "Saving…");
    setLayaRoutingStatus("Saving routing policy…");
    try {
      const status = await K.api.layaRouter.configure(collectLayaRoutingPolicy());
      layaRoutingSnapshot = status;
      layaRoutingDraftModels = cloneLayaModels(status.models || []);
      layaRoutingProfile!.value = status.profile || currentLayaProfile();
      renderLayaRoutingModels();
      setLayaRoutingStatus("Routing policy saved.", "success");
      await K.loadCatalog?.().catch(() => {});
      closeLayaRouting();
    } catch (error) {
      setLayaRoutingStatus((error as Error).message || String(error), "error");
    } finally {
      busy(layaRoutingSave, false);
    }
  });
  layaRoutingPreviewButton?.addEventListener("click", async () => {
    const prompt = layaRoutingPreviewInput?.value.trim() || "";
    if (!prompt) {
      if (layaRoutingPreviewResult) layaRoutingPreviewResult.textContent = "Enter a request to preview.";
      return;
    }
    busy(layaRoutingPreviewButton, true, "Analyzing…");
    if (layaRoutingPreviewResult) layaRoutingPreviewResult.textContent = "Laya is classifying the request…";
    try {
      const policy = collectLayaRoutingPolicy();
      const result = await K.api.layaRouter.preview({ prompt, ...policy });
      if (layaRoutingPreviewResult) {
        const analysis = result.analysis || ({} as TLStudioLayaRoutePreview["analysis"]);
        layaRoutingPreviewResult.textContent =
          `${result.modelName || result.modelID} · ${result.providerName || result.providerID} · ${result.group} · difficulty ${Number(analysis.difficulty || 0).toFixed(2)}/3 · ${analysis.domain || "unknown"}`;
      }
      setLayaRoutingStatus(result.reason || "", "success");
    } catch (error) {
      if (layaRoutingPreviewResult) layaRoutingPreviewResult.textContent = "";
      setLayaRoutingStatus((error as Error).message || String(error), "error");
    } finally {
      busy(layaRoutingPreviewButton, false);
    }
  });

  addButton.addEventListener("click", () => openEditor());
  cancelButton?.addEventListener("click", closeEditor);
  pluginDialogClose?.addEventListener("click", closeEditor);
  pluginDialog.addEventListener("close", resetEditor);

  testButton?.addEventListener("click", async () => {
    const plugin = formPlugin();
    const environment = parseEnvironment();
    if (!plugin.name || !plugin.command) {
      setStatus("Name and Command are required.", "error");
      return;
    }
    busy(testButton, true, "Testing…");
    setStatus("Starting MCP server and discovering tools…");
    try {
      const result = await K.api.plugins.testConfig(plugin, environment);
      setStatus(`Connected. Discovered ${result.discoveredTools || 0} tools and ${result.resources || 0} resources.`, "success");
    } catch (error) {
      setStatus((error as Error).message || String(error), "error");
    } finally {
      busy(testButton, false);
    }
  });

  saveButton?.addEventListener("click", async () => {
    const plugin = formPlugin();
    const environment = parseEnvironment();
    if (!plugin.name || !plugin.command) {
      setStatus("Name and Command are required.", "error");
      return;
    }
    busy(saveButton, true, "Saving…");
    try {
      if (editID.value) {
        await K.api.plugins.update(editID.value, plugin, environment);
      } else {
        await K.api.plugins.create(plugin, environment);
      }
      closeEditor();
      await load();
      await K.loadToolRegistry?.().catch(() => {});
    } catch (error) {
      setStatus((error as Error).message || String(error), "error");
    } finally {
      busy(saveButton, false);
    }
  });

  list.addEventListener("click", async (event) => {
    const button = (event.target as Element | null)?.closest<HTMLButtonElement>("button[data-plugin-action]");
    if (!button) return;
    const action = button.dataset.pluginAction;
    if (action === "catalog-add") {
      const preset = pluginCatalog.find((item) => item.id === button.dataset.pluginId);
      if (!preset) return;
      busy(button, true, "Installing…");
      K.showError("");
      try {
        await K.api.plugins.installCatalog(preset.id);
        await load();
        await K.loadToolRegistry?.().catch(() => {});
        await K.loadCatalog?.().catch(() => {});
        if (preset.id === "jev-direct") {
          openJevDirectDialog(K.state.plugins.find((item) => item.id === "jev-direct"));
        }
      } catch (error) {
        K.showError((error as Error).message || String(error));
        busy(button, false);
      }
      return;
    }
    const plugin = (action === "attach" ? savedPlugins : K.state.plugins).find((item) => item.id === button.dataset.pluginId);
    if (!plugin) return;

    if (action === "laya-routing") {
      await openLayaRouting();
      return;
    }
    if (action === "jev-direct-config") {
      openJevDirectDialog(plugin);
      return;
    }

    if (action === "attach") {
      const sourceProject = String(plugin.project || "");
      if (!sourceProject) return;
      if (!window.confirm(`Use “${plugin.name}” in the current project?\n\nIts saved project scope will move from:\n${sourceProject}`)) return;
      busy(button, true, "Moving…");
      try {
        await K.api.plugins.attach(plugin.id, sourceProject);
        await load();
        await K.loadToolRegistry?.().catch(() => {});
      } catch (error) {
        K.showError((error as Error).message || String(error));
      } finally {
        busy(button, false);
      }
      return;
    }
    if (action === "configure") {
      openEditor(plugin);
      return;
    }
    if (action === "remove") {
      if (!window.confirm(`Remove plugin “${plugin.name}”? Its saved secret environment values will also be removed.`)) return;
      busy(button, true, "Removing…");
      try {
        await K.api.plugins.remove(plugin.id);
        if (editID.value === plugin.id) closeEditor();
        await load();
        await K.loadToolRegistry?.().catch(() => {});
        await K.loadCatalog?.().catch(() => {});
      } catch (error) {
        K.showError((error as Error).message || String(error));
      } finally {
        busy(button, false);
      }
      return;
    }
    if (action === "toggle") {
      const enabling = !plugin.enabled;
      if (enabling) {
        if (plugin.type === "router") {
          if (!window.confirm(`Enable “${plugin.name}”?\n\nTL Studio will use this router to choose among your connected Agent models.`)) return;
        } else {
          const exact = [plugin.command, ...(plugin.arguments || [])].join(" ");
          if (!window.confirm(`Enable “${plugin.name}”?\n\nTL Studio will start this local MCP command when the plugin is needed:\n${exact}`)) return;
        }
      }
      busy(button, true, enabling ? "Enabling…" : "Disabling…");
      try {
        await K.api.plugins.setEnabled(plugin.id, enabling);
        await load();
        await K.loadToolRegistry?.().catch(() => {});
        await K.loadCatalog?.().catch(() => {});
      } catch (error) {
        K.showError((error as Error).message || String(error));
      } finally {
        busy(button, false);
      }
      return;
    }
    if (action === "test") {
      busy(button, true, "Testing…");
      try {
        const result = await K.api.plugins.test(plugin.id);
        K.showError("");
        await load();
        window.alert(`${plugin.name}: connected successfully. ${result.discoveredTools || 0} tools discovered.`);
      } catch (error) {
        K.showError((error as Error).message || String(error));
        await load();
      } finally {
        busy(button, false);
      }
      return;
    }
    if (action === "integration") {
      const actionID = String(button.dataset.integrationActionId || "");
      const descriptor = (plugin.integration?.actions || []).find((item) => item.id === actionID);
      if (!descriptor) return;
      if (descriptor.kind === "preview") {
        const path = String(descriptor.target || "");
        if (!path) return;
        (document.getElementById("settingsDialog") as HTMLDialogElement | null)?.close();
        K.preview?.open?.();
        await K.preview?.selectEntry?.(path);
        return;
      }
      if (descriptor.requiresConfirmation && !window.confirm(descriptor.confirmation || `Run “${descriptor.label}”?`)) return;
      busy(button, true, `${descriptor.label}…`);
      try {
        const result = await K.api.plugins.action(plugin.id, descriptor.id, !!descriptor.requiresConfirmation);
        await load();
        const output = String(result?.result?.output || "").trim();
        if (output) console.info(`[TL Studio] Plugin action ${descriptor.id}\n${output}`);
      } catch (error) {
        K.showError((error as Error).message || String(error));
        await load();
      } finally {
        busy(button, false);
      }
    }
  });

  document.querySelector('[data-settings-section="plugins"]')?.addEventListener("click", () => load());
  document.getElementById("settingsButton")?.addEventListener("click", () => {
    if (!panel.classList.contains("hidden")) load();
  });

  settingsDialog?.addEventListener("close", () => {
    if (pluginDialog.open) pluginDialog.close();
    closeJevDirectDialog();
    closeLayaRouting();
  });

  const baseAfterProjectChange = K.afterProjectChange;
  if (typeof baseAfterProjectChange === "function") {
    K.afterProjectChange = async (...args: any[]) => {
      const result = await baseAfterProjectChange(...args);
      closeEditor();
      closeJevDirectDialog();
      closeLayaRouting();
      await load();
      await K.loadCatalog?.().catch(() => {});
      return result;
    };
  }

  load();
})();
