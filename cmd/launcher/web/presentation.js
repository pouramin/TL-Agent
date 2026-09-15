(() => {
  "use strict";
  const K = window.KLU;

  const safeJSON = (value) => {
    try { return JSON.stringify(value, null, 2); }
    catch { return String(value); }
  };

  const errorText = (error) => {
    if (!error) return "";
    if (typeof error === "string") return error;
    return String(error.message || error.data?.message || error.error?.message || safeJSON(error));
  };

  const partsOf = (message) => Array.isArray(message?.parts)
    ? message.parts
    : Array.isArray(message?.content) ? message.content : [];

  const textOf = (message) => {
    if (typeof message?.text === "string" && message.text) return message.text;
    return partsOf(message)
      .filter((part) => part?.type === "text" && !part.ignored)
      .map((part) => part.text || "")
      .filter(Boolean)
      .join("\n")
      .trim();
  };

  const normalizeStatus = (input) => {
    const value = String(input || "pending").toLowerCase();
    if (["completed", "success", "done"].includes(value)) return "completed";
    if (["error", "failed", "failure"].includes(value)) return "failed";
    if (["running", "in_progress", "active"].includes(value)) return "running";
    return value;
  };

  const fileHint = (item) => {
    const state = item?.state || {};
    const metadata = state.metadata || {};
    const input = state.input || {};
    const diff = metadata.filediff || metadata.fileDiff || {};
    const path = input.filePath || input.path || input.file || metadata.filepath || metadata.path || diff.file;
    if (!path) return "";
    const bits = String(path).split(/[\\/]/);
    return bits[bits.length - 1] || String(path);
  };

  const diffHint = (item) => {
    const state = item?.state || {};
    const metadata = state.metadata || {};
    const diff = metadata.filediff || metadata.fileDiff || state.output?.filediff || {};
    const additions = Number(diff.additions);
    const deletions = Number(diff.deletions);
    if (!Number.isFinite(additions) && !Number.isFinite(deletions)) return "";
    return `+${Number.isFinite(additions) ? additions : 0} −${Number.isFinite(deletions) ? deletions : 0}`;
  };

  const toolDetails = (item) => {
    const state = item?.state || {};
    const blocks = [];
    if (state.input && Object.keys(state.input).length) blocks.push(["Input", safeJSON(state.input)]);
    const output = state.output ?? state.result ?? item.output ?? item.result;
    if (output !== undefined && output !== "") blocks.push(["Output", typeof output === "string" ? output : safeJSON(output)]);
    if (state.error) blocks.push(["Error", errorText(state.error)]);
    if (state.metadata && Object.keys(state.metadata).length) blocks.push(["Metadata", safeJSON(state.metadata)]);
    return blocks;
  };

  const messageNode = (kind, author, text, time, error) => {
    const row = document.createElement("article");
    row.className = `message ${kind}${error ? " error" : ""}`;

    const avatar = document.createElement("div");
    avatar.className = "avatar";
    avatar.textContent = kind === "user" ? "YOU" : kind === "system" ? "SYS" : "AI";

    const content = document.createElement("div");
    content.className = "message-content";

    const head = document.createElement("div");
    head.className = "message-head";
    const strong = document.createElement("strong");
    strong.textContent = author;
    const stamp = document.createElement("span");
    stamp.textContent = K.formatTime(time);
    head.append(strong, stamp);

    const body = document.createElement("div");
    body.className = "message-text";
    body.textContent = error ? `${text}${text ? "\n\n" : ""}${error}` : text;
    content.append(head, body);
    row.append(avatar, content);
    return row;
  };

  const activityCard = ({ title, status, meta = "", blocks = [], reasoning = false }) => {
    const details = document.createElement("details");
    const normalized = normalizeStatus(status);
    details.className = `activity-card${reasoning ? " reasoning-card" : ""}`;
    details.dataset.status = normalized;
    details.open = normalized === "running" || normalized === "failed";

    const summary = document.createElement("summary");
    const icon = document.createElement("span");
    icon.className = "activity-icon";
    icon.textContent = reasoning ? "◌" : normalized === "completed" ? "✓" : normalized === "failed" ? "!" : "•";

    const name = document.createElement("strong");
    name.textContent = title;
    const descriptor = document.createElement("span");
    descriptor.className = "activity-meta";
    descriptor.textContent = meta;
    const state = document.createElement("span");
    state.className = "activity-status";
    state.textContent = normalized;

    summary.append(icon, name, descriptor, state);
    details.appendChild(summary);

    if (blocks.length) {
      const body = document.createElement("div");
      body.className = "activity-body";
      for (const [label, value] of blocks) {
        if (!value) continue;
        const section = document.createElement("section");
        const heading = document.createElement("div");
        const pre = document.createElement("pre");
        heading.className = "activity-label";
        heading.textContent = label;
        pre.textContent = value;
        section.append(heading, pre);
        body.appendChild(section);
      }
      details.appendChild(body);
    }
    return details;
  };

  const appendParts = (node, message) => {
    const content = node.querySelector(".message-content");
    for (const item of partsOf(message)) {
      if (item?.type === "reasoning" && item.text) {
        content.appendChild(activityCard({
          title: "Reasoning",
          status: item.time?.end || item.time?.completed ? "completed" : item.status || "completed",
          blocks: [["Thought process", item.text]],
          reasoning: true,
        }));
        continue;
      }

      if (item?.type === "tool") {
        const state = item.state || {};
        const meta = [fileHint(item), diffHint(item)].filter(Boolean).join(" · ");
        content.appendChild(activityCard({
          title: item.tool || item.name || "Tool",
          status: state.status || item.status || "pending",
          meta,
          blocks: toolDetails(item),
        }));
        continue;
      }

      if (item?.type === "subtask") {
        content.appendChild(activityCard({
          title: "Subtask",
          status: item.status || "created",
          meta: item.agent || "",
          blocks: [["Task", item.description || item.prompt || ""]],
        }));
      }
    }
  };

  const renderEnvelope = (view, message) => {
    if (!message?.info || !Array.isArray(message.parts)) return false;
    const info = message.info;
    const time = info.time?.created ?? info.time?.completed;
    if (info.role === "user") {
      view.appendChild(messageNode("user", "You", textOf(message), time));
      return true;
    }
    if (info.role === "assistant") {
      const node = messageNode("assistant", info.agent || "Agent", textOf(message), time, errorText(info.error));
      appendParts(node, message);
      view.appendChild(node);
      return true;
    }
    return false;
  };

  K.renderMessages = () => {
    const view = K.els.conversation;
    view.textContent = "";

    for (const message of K.state.messages) {
      if (renderEnvelope(view, message)) continue;

      if (message?.type === "user") {
        view.appendChild(messageNode("user", "You", message.text || textOf(message), message.time?.created));
      } else if (message?.type === "assistant") {
        const node = messageNode("assistant", message.agent || "Agent", textOf(message), message.time?.created, errorText(message.error));
        appendParts(node, message);
        view.appendChild(node);
      } else if (message?.type === "shell") {
        const node = messageNode("assistant", "Shell", "", message.time?.created);
        node.querySelector(".message-content").appendChild(activityCard({
          title: message.command || "Command",
          status: message.time?.completed ? "completed" : "running",
          blocks: [["Output", message.output || ""]],
        }));
        view.appendChild(node);
      } else if (message?.type === "system" || message?.type === "synthetic") {
        view.appendChild(messageNode("system", "System", message.text || "", message.time?.created));
      }
    }

    if (K.state.session && (K.isSessionRunning(K.state.session.id) || K.state.sending)) {
      const row = messageNode("assistant", K.state.session.agent || "Agent", "", Date.now());
      row.classList.add("working-message");
      row.querySelector(".message-text").innerHTML = 'Working <span class="typing"><i></i><i></i><i></i></span>';
      view.appendChild(row);
    }

    K.refreshWorkspaceControls?.();
    requestAnimationFrame(() => { view.scrollTop = view.scrollHeight; });
  };

  K.presentation = Object.freeze({ partsOf, textOf });
})();
