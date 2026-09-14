(() => {
  "use strict";
  const K = window.KLU;

  K.showConversation = () => {
    K.els.emptyState.classList.add("hidden");
    K.els.conversation.classList.remove("hidden");
  };

  K.stopPolling = () => {
    if (K.state.polling) window.clearInterval(K.state.polling);
    K.state.polling = null;
  };

  K.newSession = () => {
    K.stopPolling();
    Object.assign(K.state, { session: null, messages: [], sending: false, attentionKey: "" });
    K.showError("");
    K.renderSessions();
    K.renderSessionHeader();
    K.els.conversation.textContent = "";
    K.els.conversation.classList.add("hidden");
    K.els.emptyState.classList.remove("hidden");
    K.els.prompt.focus();
  };

  K.selectSession = async (session) => {
    K.stopPolling();
    K.state.session = session;
    K.state.messages = [];
    K.renderSessions();
    K.renderSessionHeader();
    K.showConversation();
    K.showError("");
    K.syncSelectors();
    await Promise.all([K.loadMessages(), K.loadActiveSessions(), K.loadAttention?.()]);
    K.renderMessages();
    if (K.state.activeSessions[session.id]) K.startPolling();
  };

  K.createSession = async () => {
    const body = {};
    if (K.els.agentSelect.value) body.agent = K.els.agentSelect.value;
    const model = K.selectedModel();
    if (model) body.model = model;
    const session = (await K.kilo("/api/session", { method: "POST", body: JSON.stringify(body) }))?.data;
    if (!session?.id) throw new Error("Kilo did not return a session ID");
    K.state.session = session;
    K.showConversation();
    K.renderSessionHeader();
    await K.loadSessions().catch(() => {});
    return session;
  };

  K.ensureSessionModel = async () => {
    const model = K.selectedModel();
    if (!K.state.session || !model) return;
    const current = K.state.session.model;
    if (current?.providerID === model.providerID && current?.id === model.id) return;
    await K.kilo(`/api/session/${encodeURIComponent(K.state.session.id)}/model`, {
      method: "POST",
      body: JSON.stringify({ model }),
    });
    K.state.session.model = model;
    K.renderSessionHeader();
  };

  K.loadMessages = async () => {
    if (!K.state.session) return [];
    const revision = ++K.state.revision;
    const payload = await K.kilo(`/api/session/${encodeURIComponent(K.state.session.id)}/message?order=asc&limit=200`);
    if (revision === K.state.revision) K.state.messages = Array.isArray(payload?.data) ? payload.data : [];
    return K.state.messages;
  };

  const partsOf = (message) => Array.isArray(message?.parts)
    ? message.parts
    : Array.isArray(message?.content) ? message.content : [];

  const textOf = (message) => {
    if (typeof message?.text === "string" && message.text) return message.text;
    return partsOf(message)
      .filter((item) => item?.type === "text" && !item.ignored)
      .map((item) => item.text || "")
      .filter(Boolean)
      .join("\n")
      .trim();
  };

  const errorText = (error) => {
    if (!error) return "";
    if (typeof error === "string") return error;
    if (typeof error.message === "string") return error.message;
    if (typeof error.data?.message === "string") return error.data.message;
    if (typeof error.error?.message === "string") return error.error.message;
    try { return JSON.stringify(error); } catch { return String(error); }
  };

  const toolSummary = (item) => {
    const status = item.state?.status || item.status || "pending";
    let detail = "";
    try {
      const input = item.state?.input ?? item.input;
      if (typeof input === "string") detail = input;
      else if (input) detail = JSON.stringify(input, null, 2);
      if (!detail && item.state?.structured) detail = JSON.stringify(item.state.structured, null, 2);
      const failure = item.state?.error ?? item.error;
      if (status === "error" && failure) detail = errorText(failure);
      else if (!detail) {
        const output = item.state?.output ?? item.state?.result ?? item.output ?? item.result;
        if (output !== undefined) detail = typeof output === "string" ? output : JSON.stringify(output, null, 2);
      }
    } catch { detail = ""; }
    return { status, detail };
  };

  const messageNode = (kind, author, text, time, error) => {
    const row = document.createElement("article"), avatar = document.createElement("div"), content = document.createElement("div");
    row.className = `message ${kind}${error ? " error" : ""}`;
    avatar.className = "avatar"; avatar.textContent = kind === "user" ? "YOU" : "AI";
    content.className = "message-content";
    const head = document.createElement("div"), strong = document.createElement("strong"), stamp = document.createElement("span"), body = document.createElement("div");
    head.className = "message-head"; strong.textContent = author; stamp.textContent = K.formatTime(time); head.append(strong, stamp);
    body.className = "message-text"; body.textContent = error ? `${text}${text ? "\n\n" : ""}${error}` : text;
    content.append(head, body); row.append(avatar, content); return row;
  };

  const toolNode = (name, status, detail) => {
    const card = document.createElement("div"), head = document.createElement("div"), title = document.createElement("strong"), label = document.createElement("span");
    card.className = "tool-card"; head.className = "tool-head"; title.textContent = name; label.textContent = status; head.append(title, label); card.appendChild(head);
    if (detail) { const body = document.createElement("div"); body.className = "tool-body"; body.textContent = detail; card.appendChild(body); }
    return card;
  };

  const appendTools = (node, message) => {
    for (const item of partsOf(message)) {
      if (item?.type !== "tool") continue;
      const tool = toolSummary(item);
      node.querySelector(".message-content").appendChild(toolNode(item.tool || item.name || "tool", tool.status, tool.detail));
    }
  };

  const renderCurrentEnvelope = (view, message) => {
    if (!message?.info || !Array.isArray(message.parts)) return false;
    const info = message.info;
    const role = info.role;
    const time = info.time?.created ?? info.time?.completed ?? info.createdAt;
    if (role === "user") {
      view.appendChild(messageNode("user", "You", textOf(message), time));
      return true;
    }
    if (role === "assistant") {
      const err = errorText(info.error);
      const node = messageNode("assistant", info.agent || "Kilo", textOf(message), time, err);
      appendTools(node, message);
      view.appendChild(node);
      return true;
    }
    return false;
  };

  K.renderMessages = () => {
    const view = K.els.conversation;
    view.textContent = "";
    for (const message of K.state.messages) {
      if (renderCurrentEnvelope(view, message)) continue;
      if (message.type === "user") view.appendChild(messageNode("user", "You", message.text || "", message.time?.created));
      else if (message.type === "assistant") {
        const node = messageNode("assistant", message.agent || "Kilo", textOf(message), message.time?.created, errorText(message.error));
        appendTools(node, message);
        view.appendChild(node);
      } else if (message.type === "shell") {
        const node = messageNode("assistant", "Shell", "", message.time?.created);
        node.querySelector(".message-content").appendChild(toolNode(message.command || "command", "completed", message.output || ""));
        view.appendChild(node);
      } else if (message.type === "system" || message.type === "synthetic") view.appendChild(messageNode("assistant", "System", message.text || "", message.time?.created));
      else if (message.type === "compaction") view.appendChild(messageNode("assistant", "Context", "Conversation context was compacted.", message.time?.created));
    }
    if (K.state.session && (K.state.activeSessions[K.state.session.id] || K.state.sending)) {
      const row = messageNode("assistant", "Kilo", "", Date.now());
      row.querySelector(".message-text").innerHTML = 'Working <span class="typing"><i></i><i></i><i></i></span>';
      view.appendChild(row);
    }
    requestAnimationFrame(() => { view.scrollTop = view.scrollHeight; });
  };

  K.sendPrompt = async () => {
    const text = K.els.prompt.value.trim();
    if (!text || K.state.sending) return;
    K.showError(""); K.state.sending = true; K.els.sendButton.disabled = true;
    try {
      if (!K.state.session) await K.createSession();
      await K.ensureSessionModel();
      K.els.prompt.value = ""; K.resizePrompt();
      K.state.messages.push({ type: "user", text, time: { created: Date.now() } }); K.renderMessages();
      await K.kilo(`/api/session/${encodeURIComponent(K.state.session.id)}/prompt`, { method: "POST", body: JSON.stringify({ prompt: { text }, delivery: "queue" }) });
      await Promise.all([K.loadMessages(), K.loadActiveSessions(), K.loadAttention?.()]);
      K.state.sending = false; K.renderMessages(); K.startPolling();
      window.setTimeout(() => K.loadSessions().then(() => {
        const fresh = K.state.sessions.find((item) => item.id === K.state.session?.id);
        if (fresh) { K.state.session = fresh; K.renderSessionHeader(); K.renderSessions(); }
      }).catch(() => {}), 800);
    } catch (err) {
      K.state.sending = false; K.showError(err.message || String(err)); K.renderMessages();
    } finally { K.els.sendButton.disabled = false; K.els.prompt.focus(); }
  };

  K.startPolling = () => {
    K.stopPolling();
    K.state.polling = window.setInterval(async () => {
      if (!K.state.session) return K.stopPolling();
      try {
        await Promise.all([K.loadMessages(), K.loadActiveSessions(), K.loadAttention?.()]); K.renderMessages();
        if (!K.state.activeSessions[K.state.session.id]) {
          K.stopPolling(); await K.loadSessions().catch(() => {});
          const fresh = K.state.sessions.find((item) => item.id === K.state.session?.id);
          if (fresh) K.state.session = fresh;
          K.renderSessionHeader(); K.renderSessions();
        }
      } catch (err) { K.stopPolling(); K.showError(err.message || String(err)); }
    }, 900);
  };

  K.pickProject = async () => {
    K.showError("");
    try { K.state.local = await K.request("/local/pick-directory", { method: "POST" }); await K.afterProjectChange(); }
    catch (err) { if (/no supported folder picker/i.test(err.message || "")) return K.openManualProject(); K.showError(err.message || String(err)); }
  };

  K.openManualProject = () => {
    K.els.pathInput.value = K.state.local?.project || ""; K.els.pathDialog.showModal();
    window.setTimeout(() => { K.els.pathInput.focus(); K.els.pathInput.select(); }, 0);
  };

  K.setManualProject = async (event) => {
    event.preventDefault();
    if (event.submitter?.value === "cancel") return K.els.pathDialog.close();
    const path = K.els.pathInput.value.trim(); if (!path) return;
    try {
      K.state.local = await K.request("/local/project", { method: "POST", body: JSON.stringify({ path }) });
      K.els.pathDialog.close(); await K.afterProjectChange();
    } catch (err) { K.showError(err.message || String(err)); }
  };

  K.afterProjectChange = async () => {
    K.stopPolling(); Object.assign(K.state, { session: null, sessions: [], messages: [] });
    K.els.projectName.textContent = K.basename(K.state.local.project); K.els.projectPath.textContent = K.state.local.project; K.els.projectPath.title = K.state.local.project;
    K.newSession(); await Promise.all([K.loadAgentsAndModels(), K.loadSessions()]);
  };

  K.switchAgent = async () => {
    if (!K.state.session || !K.els.agentSelect.value) return;
    try {
      await K.kilo(`/api/session/${encodeURIComponent(K.state.session.id)}/agent`, { method: "POST", body: JSON.stringify({ agent: K.els.agentSelect.value }) });
      K.state.session.agent = K.els.agentSelect.value; K.renderSessionHeader();
    } catch (err) { K.showError(err.message || String(err)); }
  };

  K.switchModel = async () => {
    const model = K.selectedModel(); if (!K.state.session || !model) return;
    try {
      await K.kilo(`/api/session/${encodeURIComponent(K.state.session.id)}/model`, { method: "POST", body: JSON.stringify({ model }) });
      K.state.session.model = model; K.renderSessionHeader();
    } catch (err) { K.showError(err.message || String(err)); }
  };

  K.resizePrompt = () => {
    K.els.prompt.style.height = "auto";
    K.els.prompt.style.height = `${Math.min(190, Math.max(56, K.els.prompt.scrollHeight))}px`;
  };
})();
