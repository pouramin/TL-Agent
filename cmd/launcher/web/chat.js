(() => {
  "use strict";
  const K = window.KLU;

  const safeJSON = (value) => {
    try { return JSON.stringify(value, null, 2); }
    catch { return String(value); }
  };

  K.showConversation = () => {
    K.els.emptyState.classList.add("hidden");
    K.els.conversation.classList.remove("hidden");
  };

  K.stopEvents = () => {
    if (K.state.eventSource) K.state.eventSource.close();
    K.state.eventSource = null;
    if (K.state.fallbackPolling) window.clearInterval(K.state.fallbackPolling);
    K.state.fallbackPolling = null;
  };

  K.newSession = () => {
    Object.assign(K.state, {
      session: null,
      messages: [],
      sending: false,
      attentionKey: "",
      live: { text: "", reasoning: "", assistantMessageID: "" },
    });
    K.showError("");
    K.renderSessions();
    K.renderSessionHeader();
    K.els.conversation.textContent = "";
    K.els.conversation.classList.add("hidden");
    K.els.emptyState.classList.remove("hidden");
    K.els.prompt.focus();
  };

  K.selectSession = async (session) => {
    K.state.session = session;
    K.state.messages = [];
    K.state.live = { text: "", reasoning: "", assistantMessageID: "" };
    K.renderSessions();
    K.renderSessionHeader();
    K.showConversation();
    K.showError("");
    K.syncSelectors();
    await Promise.all([K.loadMessages(), K.loadActiveSessions(), K.loadAttention?.()]);
    K.renderMessages();
  };

  K.createSession = async () => {
    const input = {};
    if (K.els.agentSelect.value) input.agent = K.els.agentSelect.value;
    const model = K.selectedModel();
    if (model) input.model = model;
    const session = (await K.api.sessions.create(input))?.data;
    if (!session?.id) throw new Error("Kilo did not return a session ID");
    K.state.session = session;
    K.showConversation();
    K.renderSessionHeader();
    await K.loadSessions().catch(() => {});
    return session;
  };

  K.ensureSessionSelection = async () => {
    const session = K.state.session;
    if (!session) return;

    const agent = K.els.agentSelect.value || undefined;
    if (agent && session.agent !== agent) {
      await K.api.sessions.switchAgent(session.id, agent);
      session.agent = agent;
    }

    const model = K.selectedModel();
    if (model && (session.model?.providerID !== model.providerID || session.model?.id !== model.id || session.model?.variant !== model.variant)) {
      await K.api.sessions.switchModel(session.id, model);
      session.model = model;
    }
    K.renderSessionHeader();
  };

  K.loadMessages = async () => {
    if (!K.state.session) return [];
    const revision = ++K.state.revision;
    const payload = await K.api.sessions.messages(K.state.session.id, { order: "asc", limit: 200 });
    if (revision === K.state.revision) K.state.messages = Array.isArray(payload?.data) ? payload.data : [];
    return K.state.messages;
  };

  const errorText = (error) => {
    if (!error) return "";
    if (typeof error === "string") return error;
    if (typeof error.message === "string") return error.message;
    return safeJSON(error);
  };

  const toolContentText = (content) => {
    if (!Array.isArray(content)) return "";
    return content.map((item) => {
      if (!item) return "";
      if (item.type === "text") return item.text || "";
      if (item.type === "file") return item.name || item.filename || item.url || item.uri || safeJSON(item);
      return safeJSON(item);
    }).filter(Boolean).join("\n");
  };

  const toolSummary = (item) => {
    const state = item?.state || {};
    const status = state.status || "pending";
    if (status === "pending") return { status, detail: typeof state.input === "string" ? state.input : "" };
    if (status === "error") return { status, detail: errorText(state.error) || toolContentText(state.content) };

    const details = [];
    if (state.input && Object.keys(state.input).length) details.push(safeJSON(state.input));
    const content = toolContentText(state.content);
    if (content) details.push(content);
    if (state.outputPaths?.length) details.push(`Output: ${state.outputPaths.join(", ")}`);
    if (state.result !== undefined) details.push(typeof state.result === "string" ? state.result : safeJSON(state.result));
    return { status, detail: details.join("\n\n") };
  };

  const messageNode = (kind, author, text, time, error) => {
    const row = document.createElement("article");
    const avatar = document.createElement("div");
    const content = document.createElement("div");
    row.className = `message ${kind}${error ? " error" : ""}`;
    avatar.className = "avatar";
    avatar.textContent = kind === "user" ? "YOU" : kind === "system" ? "SYS" : "AI";
    content.className = "message-content";

    const head = document.createElement("div");
    const strong = document.createElement("strong");
    const stamp = document.createElement("span");
    const body = document.createElement("div");
    head.className = "message-head";
    strong.textContent = author;
    stamp.textContent = K.formatTime(time);
    head.append(strong, stamp);
    body.className = "message-text";
    body.textContent = error ? `${text}${text ? "\n\n" : ""}${error}` : text;
    content.append(head, body);
    row.append(avatar, content);
    return row;
  };

  const toolNode = (name, status, detail) => {
    const card = document.createElement("div");
    const head = document.createElement("div");
    const title = document.createElement("strong");
    const label = document.createElement("span");
    card.className = "tool-card";
    head.className = "tool-head";
    title.textContent = name;
    label.textContent = status;
    head.append(title, label);
    card.appendChild(head);
    if (detail) {
      const body = document.createElement("div");
      body.className = "tool-body";
      body.textContent = detail;
      card.appendChild(body);
    }
    return card;
  };

  const assistantText = (message) => (message.content || [])
    .filter((item) => item?.type === "text")
    .map((item) => item.text || "")
    .filter(Boolean)
    .join("\n")
    .trim();

  const appendAssistantContent = (node, message) => {
    const content = node.querySelector(".message-content");
    for (const item of message.content || []) {
      if (item?.type === "reasoning" && item.text) content.appendChild(toolNode("Reasoning", "completed", item.text));
      if (item?.type === "tool") {
        const summary = toolSummary(item);
        content.appendChild(toolNode(item.name || "tool", summary.status, summary.detail));
      }
    }
  };

  K.renderMessages = () => {
    const view = K.els.conversation;
    view.textContent = "";
    for (const message of K.state.messages) {
      switch (message?.type) {
        case "user":
          view.appendChild(messageNode("user", "You", message.text || "", message.time?.created));
          break;
        case "assistant": {
          const node = messageNode("assistant", message.agent || "Kilo", assistantText(message), message.time?.created, errorText(message.error));
          appendAssistantContent(node, message);
          view.appendChild(node);
          break;
        }
        case "shell": {
          const node = messageNode("assistant", "Shell", "", message.time?.created);
          node.querySelector(".message-content").appendChild(toolNode(message.command || "command", message.time?.completed ? "completed" : "running", message.output || ""));
          view.appendChild(node);
          break;
        }
        case "system":
        case "synthetic":
          view.appendChild(messageNode("system", "System", message.text || "", message.time?.created));
          break;
        case "compaction":
          view.appendChild(messageNode("system", "Context", message.summary || "Conversation context was compacted.", message.time?.created));
          break;
        case "agent-switched":
        case "model-switched":
          break;
      }
    }

    if (K.state.live.text || K.state.live.reasoning) {
      const row = messageNode("assistant", K.state.session?.agent || "Kilo", K.state.live.text, Date.now());
      if (K.state.live.reasoning) row.querySelector(".message-content").appendChild(toolNode("Reasoning", "streaming", K.state.live.reasoning));
      view.appendChild(row);
    } else if (K.state.session && (K.state.activeSessions[K.state.session.id] || K.state.sending)) {
      const row = messageNode("assistant", "Kilo", "", Date.now());
      row.querySelector(".message-text").innerHTML = 'Working <span class="typing"><i></i><i></i><i></i></span>';
      view.appendChild(row);
    }
    requestAnimationFrame(() => { view.scrollTop = view.scrollHeight; });
  };

  let refreshTimer = null;
  const scheduleSelectedRefresh = (delay = 80) => {
    if (refreshTimer) window.clearTimeout(refreshTimer);
    refreshTimer = window.setTimeout(async () => {
      refreshTimer = null;
      if (!K.state.session) return;
      try {
        await Promise.all([K.loadMessages(), K.loadActiveSessions(), K.loadAttention?.()]);
        const fresh = (await K.api.sessions.get(K.state.session.id).catch(() => null))?.data;
        if (fresh) K.state.session = fresh;
        K.renderMessages();
        K.renderSessionHeader();
        K.renderSessions();
      } catch (err) { K.showError(err.message || String(err)); }
    }, delay);
  };

  K.handleKiloEvent = (event) => {
    const type = event?.type || "";
    const data = event?.data || {};
    const selectedID = K.state.session?.id;
    const sessionID = data.sessionID;

    if (type === "server.connected") return;
    if (sessionID && sessionID !== selectedID) {
      if (type.startsWith("session.next.")) window.setTimeout(() => K.loadSessions().catch(() => {}), 100);
      return;
    }

    if (type === "session.next.text.started") {
      K.state.live.assistantMessageID = data.assistantMessageID || "";
      K.state.live.text = "";
      K.renderMessages();
      return;
    }
    if (type === "session.next.text.delta") {
      if (!K.state.live.assistantMessageID || K.state.live.assistantMessageID === data.assistantMessageID) {
        K.state.live.assistantMessageID = data.assistantMessageID || K.state.live.assistantMessageID;
        K.state.live.text += data.delta || "";
        K.renderMessages();
      }
      return;
    }
    if (type === "session.next.reasoning.started") {
      K.state.live.assistantMessageID = data.assistantMessageID || K.state.live.assistantMessageID;
      K.state.live.reasoning = "";
      return;
    }
    if (type === "session.next.reasoning.delta") {
      K.state.live.reasoning += data.delta || "";
      K.renderMessages();
      return;
    }

    if (type === "session.next.step.started") {
      K.state.sending = true;
      if (selectedID) K.state.activeSessions[selectedID] = { type: "running" };
      K.renderMessages();
      return;
    }

    if (type === "session.next.step.ended" || type === "session.next.step.failed") {
      K.state.sending = false;
      K.state.live = { text: "", reasoning: "", assistantMessageID: "" };
      scheduleSelectedRefresh(0);
      return;
    }

    if (type === "permission.v2.asked" || type === "permission.v2.replied" || type === "question.v2.asked" || type === "question.v2.replied" || type === "question.v2.rejected") {
      K.loadAttention?.().catch(() => {});
    }

    if (type.startsWith("session.next.")) {
      if (type === "session.next.text.ended" || type === "session.next.reasoning.ended") {
        K.state.live = { text: "", reasoning: "", assistantMessageID: "" };
      }
      scheduleSelectedRefresh(type.includes(".delta") ? 250 : 40);
    }
  };

  K.startEvents = () => {
    K.stopEvents();
    if (!("EventSource" in window)) {
      K.state.fallbackPolling = window.setInterval(async () => {
        if (!K.state.session) return;
        await Promise.all([K.loadMessages(), K.loadActiveSessions(), K.loadAttention?.()]).catch(() => {});
        K.renderMessages();
      }, 1200);
      return;
    }
    K.state.eventSource = K.api.events.subscribe({
      onEvent: K.handleKiloEvent,
      onError: () => {
        // EventSource reconnects automatically. Projected messages remain the source of truth.
      },
    });
  };

  K.sendPrompt = async () => {
    const text = K.els.prompt.value.trim();
    if (!text || K.state.sending) return;
    K.showError("");
    K.state.sending = true;
    K.els.sendButton.disabled = true;
    try {
      if (!K.state.session) await K.createSession();
      await K.ensureSessionSelection();
      K.els.prompt.value = "";
      K.resizePrompt();
      K.state.messages.push({ type: "user", text, time: { created: Date.now() } });
      K.renderMessages();
      await K.api.sessions.prompt(K.state.session.id, { text }, { delivery: "queue" });
      await Promise.all([K.loadMessages(), K.loadActiveSessions(), K.loadAttention?.()]);
      K.renderMessages();
      window.setTimeout(() => K.loadSessions().then(() => {
        const fresh = K.state.sessions.find((item) => item.id === K.state.session?.id);
        if (fresh) {
          K.state.session = fresh;
          K.renderSessionHeader();
          K.renderSessions();
        }
      }).catch(() => {}), 500);
    } catch (err) {
      K.state.sending = false;
      K.showError(err.message || String(err));
      K.renderMessages();
    } finally {
      K.els.sendButton.disabled = false;
      K.els.prompt.focus();
    }
  };

  K.pickProject = async () => {
    K.showError("");
    try {
      K.state.local = await K.request("/local/pick-directory", { method: "POST" });
      await K.afterProjectChange();
    } catch (err) {
      if (/no supported folder picker/i.test(err.message || "")) return K.openManualProject();
      K.showError(err.message || String(err));
    }
  };

  K.openManualProject = () => {
    K.els.pathInput.value = K.state.local?.project || "";
    K.els.pathDialog.showModal();
    window.setTimeout(() => { K.els.pathInput.focus(); K.els.pathInput.select(); }, 0);
  };

  K.setManualProject = async (event) => {
    event.preventDefault();
    if (event.submitter?.value === "cancel") return K.els.pathDialog.close();
    const path = K.els.pathInput.value.trim();
    if (!path) return;
    try {
      K.state.local = await K.request("/local/project", { method: "POST", body: JSON.stringify({ path }) });
      K.els.pathDialog.close();
      await K.afterProjectChange();
    } catch (err) { K.showError(err.message || String(err)); }
  };

  K.afterProjectChange = async () => {
    Object.assign(K.state, { session: null, sessions: [], messages: [], location: null });
    K.els.projectName.textContent = K.basename(K.state.local.project);
    K.els.projectPath.textContent = K.state.local.project;
    K.els.projectPath.title = K.state.local.project;
    K.newSession();
    await K.checkBackend();
    await Promise.all([K.loadCatalog(), K.loadSessions()]);
    K.startEvents();
  };

  K.switchAgent = async () => {
    if (!K.state.session || !K.els.agentSelect.value) return;
    try {
      await K.api.sessions.switchAgent(K.state.session.id, K.els.agentSelect.value);
      K.state.session.agent = K.els.agentSelect.value;
      K.renderSessionHeader();
    } catch (err) { K.showError(err.message || String(err)); }
  };

  K.switchModel = async () => {
    const model = K.selectedModel();
    if (!K.state.session || !model) return;
    try {
      await K.api.sessions.switchModel(K.state.session.id, model);
      K.state.session.model = model;
      K.renderSessionHeader();
    } catch (err) { K.showError(err.message || String(err)); }
  };

  K.resizePrompt = () => {
    K.els.prompt.style.height = "auto";
    K.els.prompt.style.height = `${Math.min(190, Math.max(56, K.els.prompt.scrollHeight))}px`;
  };
})();
