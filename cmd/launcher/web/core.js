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
      local: null, sessions: [], session: null, messages: [], activeSessions: {}, agents: [], models: [], providerDefaults: {},
      connectedProviders: new Set(), polling: null, sending: false, revision: 0, authController: null, authURL: "", attentionKey: "",
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
      cache: "no-store", ...options,
      headers: { ...(options.body ? { "Content-Type": "application/json" } : {}), ...(options.headers || {}) },
    });
    const type = response.headers.get("content-type") || "";
    const payload = type.includes("application/json") ? await response.json().catch(() => null) : await response.text().catch(() => "");
    if (!response.ok) {
      const message = payload && typeof payload === "object"
        ? payload.error || payload.message || payload.data?.message || JSON.stringify(payload)
        : String(payload || `${response.status} ${response.statusText}`);
      throw new Error(message);
    }
    return payload;
  };

  K.kilo = (path, options) => K.request(`/kilo${path}`, options);
  K.unwrap = (input) => input && typeof input === "object" && "data" in input ? input.data : input;

  K.normalizeAgents = (input) => {
    if (!Array.isArray(input)) return [];
    return input.flatMap((agent) => {
      if (!agent || agent.hidden || agent.mode === "subagent") return [];
      const id = agent.id || agent.name;
      if (!id) return [];
      const label = agent.displayName || agent.name || agent.id || id;
      return [{ ...agent, id: String(id), label: String(label) }];
    });
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
      await K.kilo("/global/health");
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

  K.extractModels = (input) => {
    const providers = Array.isArray(input) ? input : Array.isArray(input?.all) ? input.all : [];
    const result = [];
    for (const provider of providers) {
      if (!provider?.id) continue;
      const models = provider.models || {};
      for (const [id, model] of Array.isArray(models) ? models.map((m) => [m?.id || m?.modelID || m?.name, m]) : Object.entries(models)) {
        if (!id || model?.enabled === false) continue;
        result.push({ providerID: provider.id, id, name: model?.name || id, providerName: provider.name || provider.id });
      }
    }
    return result.sort((a, b) => `${a.providerName} ${a.name}`.localeCompare(`${b.providerName} ${b.name}`));
  };

  K.modelValue = (model) => model ? `${model.providerID}::${model.id}` : "";

  K.preferredKiloModel = () => {
    if (!K.state.connectedProviders.has("kilo")) return undefined;
    const candidates = ["kilo-auto/free", K.state.providerDefaults?.kilo].filter(Boolean);
    for (const id of candidates) {
      const match = K.state.models.find((model) => model.providerID === "kilo" && model.id === id);
      if (match) return { providerID: match.providerID, id: match.id };
    }
    const first = K.state.models.find((model) => model.providerID === "kilo");
    return first ? { providerID: first.providerID, id: first.id } : undefined;
  };

  K.renderAccount = () => {
    const connected = K.state.connectedProviders.has("kilo");
    K.els.accountButton.textContent = connected ? "Kilo signed in" : "Sign in to Kilo";
    K.els.accountButton.classList.toggle("signed-in", connected);
    K.els.accountButton.title = connected ? "Kilo account is connected" : "Connect a Kilo account for Kilo-hosted models";
  };

  K.renderAgents = () => {
    const select = K.els.agentSelect, current = select.value;
    select.innerHTML = '<option value="">Default</option>';
    for (const agent of K.state.agents) {
      const id = String(agent?.id || "");
      if (!id) continue;
      const option = document.createElement("option");
      option.value = id;
      option.textContent = agent.label || id;
      option.title = agent.description || agent.label || id;
      select.appendChild(option);
    }
    if ([...select.options].some((o) => o.value === current)) select.value = current;
  };

  K.renderModels = () => {
    const select = K.els.modelSelect, current = select.value;
    select.innerHTML = '<option value="">Backend default</option>';
    let group, last = "";
    for (const model of K.state.models) {
      if (model.providerID !== last) {
        group = document.createElement("optgroup");
        group.label = model.providerName;
        select.appendChild(group);
        last = model.providerID;
      }
      const option = document.createElement("option");
      option.value = `${model.providerID}::${model.id}`;
      option.textContent = model.name;
      group.appendChild(option);
    }
    const values = [...select.querySelectorAll("option")].map((o) => o.value);
    if (current && values.includes(current)) {
      select.value = current;
      return;
    }
    const preferred = K.modelValue(K.preferredKiloModel());
    if (preferred && values.includes(preferred)) select.value = preferred;
  };

  K.loadAgentsAndModels = async () => {
    const [agents, providers] = await Promise.allSettled([K.kilo("/agent"), K.kilo("/provider")]);
    if (agents.status === "fulfilled") {
      const data = K.unwrap(agents.value);
      K.state.agents = K.normalizeAgents(data);
      K.renderAgents();
    }
    if (providers.status === "fulfilled") {
      const data = K.unwrap(providers.value);
      K.state.models = K.extractModels(data);
      K.state.connectedProviders = new Set(Array.isArray(data?.connected) ? data.connected : []);
      K.state.providerDefaults = data?.default && typeof data.default === "object" ? data.default : {};
      K.renderModels();
    } else {
      K.state.connectedProviders = new Set();
      K.state.providerDefaults = {};
    }
    K.renderAccount();
  };

  K.selectedModel = () => {
    const value = K.els.modelSelect.value, split = value.indexOf("::");
    return split > 0 ? { providerID: value.slice(0, split), id: value.slice(split + 2) } : undefined;
  };

  K.loadSessions = async () => {
    const payload = await K.kilo("/api/session?order=desc&limit=50");
    K.state.sessions = Array.isArray(payload?.data) ? payload.data : [];
    K.renderSessions();
    return K.state.sessions;
  };

  K.loadActiveSessions = async () => {
    try { K.state.activeSessions = (await K.kilo("/api/session/active"))?.data || {}; }
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
      const title = document.createElement("strong"), meta = document.createElement("span");
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
    if (s.agent && [...K.els.agentSelect.options].some((o) => o.value === s.agent)) K.els.agentSelect.value = s.agent;
    if (s.model) {
      const value = `${s.model.providerID}::${s.model.id}`;
      if ([...K.els.modelSelect.querySelectorAll("option")].some((o) => o.value === value)) K.els.modelSelect.value = value;
    }
  };
})();
