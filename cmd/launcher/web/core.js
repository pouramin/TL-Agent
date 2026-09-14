(() => {
  "use strict";

  const $ = (id) => document.getElementById(id);
  const K = window.KLU = {
    els: {
      projectName: $("projectName"), projectPath: $("projectPath"), pickProject: $("pickProject"), manualProject: $("manualProject"),
      backendStatus: $("backendStatus"), sessions: $("sessions"), newSession: $("newSession"), emptyNewSession: $("emptyNewSession"),
      emptyPickProject: $("emptyPickProject"), emptyState: $("emptyState"), conversation: $("conversation"), sessionTitle: $("sessionTitle"),
      sessionMeta: $("sessionMeta"), agentSelect: $("agentSelect"), modelSelect: $("modelSelect"), refreshButton: $("refreshButton"),
      accountButton: $("accountButton"), prompt: $("prompt"), sendButton: $("sendButton"), errorBanner: $("errorBanner"), versionLabel: $("versionLabel"),
      pathDialog: $("pathDialog"), pathForm: $("pathForm"), pathInput: $("pathInput"),
      authDialog: $("authDialog"), authInstructions: $("authInstructions"), authCodeWrap: $("authCodeWrap"), authCode: $("authCode"),
      authCancel: $("authCancel"), authOpen: $("authOpen"), attentionDialog: $("attentionDialog"), attentionTitle: $("attentionTitle"),
      attentionBody: $("attentionBody"), attentionActions: $("attentionActions"),
    },
    state: {
      local: null,
      location: null,
      sessions: [],
      session: null,
      messages: [],
      activeSessions: {},
      agents: [],
      models: [],
      providers: [],
      providerDefaults: {},
      connectedProviders: new Set(),
      eventSource: null,
      fallbackPolling: null,
      sending: false,
      revision: 0,
      authController: null,
      authURL: "",
      attentionKey: "",
      live: { text: "", reasoning: "", assistantMessageID: "" },
    },
  };

  K.basename = (path) => {
    if (!path) return "No project";
    const bits = path.replace(/[\\/]+$/, "").split(/[\\/]/);
    return bits[bits.length - 1] || path;
  };

  K.formatTime = (input) => {
    if (!input) return "";
    const date = new Date(input);
    return Number.isNaN(date.getTime()) ? "" : new Intl.DateTimeFormat(undefined, { hour: "2-digit", minute: "2-digit" }).format(date);
  };

  K.relativeTime = (input) => {
    if (!input) return "";
    const ts = new Date(input).getTime();
    if (!Number.isFinite(ts)) return "";
    const diff = Date.now() - ts;
    if (diff < 60_000) return "now";
    if (diff < 3_600_000) return `${Math.max(1, Math.floor(diff / 60_000))}m`;
    if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)}h`;
    return `${Math.floor(diff / 86_400_000)}d`;
  };

  K.showError = (message) => {
    const e = K.els.errorBanner;
    e.textContent = message || "";
    e.classList.toggle("hidden", !message);
  };

  K.request = async (path, options = {}) => {
    const response = await fetch(path, {
      cache: "no-store",
      ...options,
      headers: { ...(options.body ? { "Content-Type": "application/json" } : {}), ...(options.headers || {}) },
    });
    const type = response.headers.get("content-type") || "";
    const payload = type.includes("application/json") ? await response.json().catch(() => null) : await response.text().catch(() => "");
    if (!response.ok) {
      const message = payload && typeof payload === "object"
        ? payload.error || payload.message || payload.data?.message || payload.data?.data?.message || JSON.stringify(payload)
        : String(payload || `${response.status} ${response.statusText}`);
      throw new Error(message);
    }
    return payload;
  };

  K.loadLocalStatus = async () => {
    const { els, state } = K;
    state.local = await K.request("/local/status");
    els.projectName.textContent = K.basename(state.local.project);
    els.projectPath.textContent = state.local.project || "Choose a folder";
    els.projectPath.title = state.local.project || "";
    els.versionLabel.textContent = `v${state.local.version} · ${state.local.platform}/${state.local.arch}`;
  };

  K.checkBackend = async () => {
    try {
      const health = await K.api.health();
      if (health?.healthy !== true) throw new Error("Kilo health check did not report healthy");
      K.state.location = await K.api.location().catch(() => null);
      K.els.backendStatus.className = "status-dot ok";
      K.els.backendStatus.innerHTML = "<i></i> Local";
      return true;
    } catch (err) {
      K.els.backendStatus.className = "status-dot error";
      K.els.backendStatus.innerHTML = "<i></i> Offline";
      K.showError(`Kilo backend: ${err.message}`);
      return false;
    }
  };

  K.modelValue = (model) => model ? `${model.providerID}::${model.id}${model.variant ? `::${model.variant}` : ""}` : "";

  K.preferredKiloModel = () => {
    if (!K.state.connectedProviders.has("kilo")) return undefined;
    const candidates = ["kilo-auto/free", K.state.providerDefaults?.kilo].filter(Boolean);
    for (const id of candidates) {
      const match = K.state.models.find((model) => model.providerID === "kilo" && model.id === id && model.enabled !== false);
      if (match) return { providerID: match.providerID, id: match.id };
    }
    const first = K.state.models.find((model) => model.providerID === "kilo" && model.enabled !== false);
    return first ? { providerID: first.providerID, id: first.id } : undefined;
  };

  K.renderAccount = () => {
    const connected = K.state.connectedProviders.has("kilo");
    K.els.accountButton.textContent = connected ? "Kilo signed in" : "Sign in to Kilo";
    K.els.accountButton.classList.toggle("signed-in", connected);
    K.els.accountButton.title = connected ? "Kilo account is connected" : "Connect a Kilo account for Kilo-hosted models";
  };

  K.renderAgents = () => {
    const select = K.els.agentSelect;
    const current = select.value;
    select.innerHTML = '<option value="">Default</option>';
    for (const agent of K.state.agents) {
      const option = document.createElement("option");
      option.value = agent.id;
      option.textContent = agent.id;
      option.title = agent.description || agent.id;
      select.appendChild(option);
    }
    if ([...select.options].some((o) => o.value === current)) select.value = current;
  };

  K.renderModels = () => {
    const select = K.els.modelSelect;
    const current = select.value;
    select.innerHTML = '<option value="">Backend default</option>';
    const providerNames = new Map(K.state.providers.map((provider) => [provider.id, provider.name || provider.id]));
    const grouped = new Map();
    for (const model of K.state.models) {
      if (model.enabled === false) continue;
      const key = model.providerID;
      if (!grouped.has(key)) grouped.set(key, []);
      grouped.get(key).push(model);
    }
    for (const [providerID, models] of [...grouped.entries()].sort(([a], [b]) => (providerNames.get(a) || a).localeCompare(providerNames.get(b) || b))) {
      const group = document.createElement("optgroup");
      group.label = providerNames.get(providerID) || providerID;
      models.sort((a, b) => (a.name || a.id).localeCompare(b.name || b.id));
      for (const model of models) {
        const option = document.createElement("option");
        option.value = K.modelValue(model);
        option.textContent = model.name || model.id;
        option.title = `${providerID}/${model.id}`;
        group.appendChild(option);
      }
      select.appendChild(group);
    }
    const values = [...select.querySelectorAll("option")].map((o) => o.value);
    if (current && values.includes(current)) {
      select.value = current;
      return;
    }
    const preferred = K.modelValue(K.preferredKiloModel());
    if (preferred && values.includes(preferred)) select.value = preferred;
  };

  K.loadCatalog = async () => {
    const [agents, models, providers, runtime] = await Promise.allSettled([
      K.api.agents(),
      K.api.models(),
      K.api.providers(),
      K.api.providerRuntimeState(),
    ]);

    if (agents.status === "fulfilled") {
      K.state.agents = agents.value.filter((agent) => agent && !agent.hidden && agent.mode !== "subagent" && typeof agent.id === "string");
      K.renderAgents();
    }
    if (models.status === "fulfilled") K.state.models = models.value.filter((model) => model && typeof model.id === "string" && typeof model.providerID === "string");
    if (providers.status === "fulfilled") K.state.providers = providers.value.filter((provider) => provider && typeof provider.id === "string");
    if (runtime.status === "fulfilled") {
      K.state.connectedProviders = runtime.value.connected;
      K.state.providerDefaults = runtime.value.defaults;
    } else {
      K.state.connectedProviders = new Set();
      K.state.providerDefaults = {};
    }
    K.renderModels();
    K.renderAccount();
  };

  K.selectedModel = () => {
    const value = K.els.modelSelect.value;
    if (!value) return undefined;
    const [providerID, id, variant] = value.split("::");
    if (!providerID || !id) return undefined;
    return { providerID, id, ...(variant ? { variant } : {}) };
  };

  K.loadSessions = async () => {
    const payload = await K.api.sessions.list({ order: "desc", limit: 50 });
    K.state.sessions = Array.isArray(payload?.data) ? payload.data : [];
    K.renderSessions();
    return K.state.sessions;
  };

  K.loadActiveSessions = async () => {
    try { K.state.activeSessions = (await K.api.sessions.active())?.data || {}; }
    catch { K.state.activeSessions = {}; }
  };

  K.renderSessions = () => {
    const list = K.els.sessions;
    list.textContent = "";
    if (!K.state.sessions.length) {
      const empty = document.createElement("div");
      empty.className = "sidebar-empty";
      empty.textContent = "No sessions in this project yet.";
      list.appendChild(empty);
      return;
    }
    for (const session of K.state.sessions) {
      const button = document.createElement("button");
      button.className = `session-item${K.state.session?.id === session.id ? " active" : ""}`;
      const title = document.createElement("strong");
      const meta = document.createElement("span");
      title.textContent = session.title || "Untitled session";
      meta.textContent = `${session.agent || "default"} · ${K.relativeTime(session.time?.updated || session.time?.created)}`;
      button.append(title, meta);
      button.addEventListener("click", () => K.selectSession(session));
      list.appendChild(button);
    }
  };

  K.renderSessionHeader = () => {
    const s = K.state.session;
    if (!s) {
      K.els.sessionTitle.textContent = "New session";
      K.els.sessionMeta.textContent = "Local agent workspace";
      return;
    }
    K.els.sessionTitle.textContent = s.title || "Untitled session";
    const model = s.model ? `${s.model.providerID}/${s.model.id}` : "default model";
    K.els.sessionMeta.textContent = `${s.agent || "default agent"} · ${model}`;
  };

  K.syncSelectors = () => {
    const s = K.state.session;
    if (!s) return;
    K.els.agentSelect.value = s.agent && [...K.els.agentSelect.options].some((o) => o.value === s.agent) ? s.agent : "";
    if (s.model) {
      const value = K.modelValue(s.model);
      K.els.modelSelect.value = [...K.els.modelSelect.querySelectorAll("option")].some((o) => o.value === value) ? value : "";
    }
  };
})();
