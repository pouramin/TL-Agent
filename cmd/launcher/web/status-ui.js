(() => {
  "use strict";
  const K = window.KLU;
  const baseRenderMessages = K.renderMessages;
  const RESUME_PROMPT = "Continue the current task from the existing workspace state. Inspect what is already complete, do not repeat finished work, and finish the user's latest request.";

  const partsOf = (message) => Array.isArray(message?.parts)
    ? message.parts
    : Array.isArray(message?.content) ? message.content : [];

  const messageRole = (message) => message?.info?.role || message?.type || "";
  const messageTime = (message) => message?.info?.time || message?.time || {};

  const exactUserText = (message) => {
    if (typeof message?.text === "string") return message.text;
    return partsOf(message)
      .filter((part) => part?.type === "text" && !part.ignored)
      .map((part) => typeof part.text === "string" ? part.text : "")
      .filter((text) => text.length > 0)
      .join("\n");
  };

  const normalizeCancellation = () => {
    const rows = K.els.conversation?.querySelectorAll(".message.error") || [];
    for (const row of rows) {
      const error = row.querySelector(".message-error-text");
      const text = String(error?.textContent || "").trim();
      if (!/\b(aborted|cancelled|canceled|interrupted)\b/i.test(text)) continue;
      row.classList.remove("error");
      row.classList.add("cancelled");
      if (error) {
        error.classList.remove("message-error-text");
        error.classList.add("message-cancelled-text");
        error.textContent = "Cancelled by user";
      }
    }
  };

  const normalizeRetryableToolErrors = () => {
    const cards = K.els.conversation?.querySelectorAll('.activity-card[data-status="failed"]') || [];
    const running = !!K.state.session && (K.state.sending || K.isSessionRunning(K.state.session.id));
    for (const card of cards) {
      const text = String(card.textContent || "");
      const invalidArguments = /invalid arguments/i.test(text)
        && /is missing and is required/i.test(text);
      if (!invalidArguments) continue;

      card.dataset.status = "retrying";
      if (running) card.open = false;

      const status = card.querySelector(".activity-status");
      if (status) status.textContent = running ? "retrying" : "invalid args";

      const meta = card.querySelector(".activity-meta");
      if (meta && !meta.textContent.trim()) {
        meta.textContent = running ? "agent is correcting the tool call" : "tool call was rejected before execution";
      }
    }
  };

  const fallbackCopy = (text) => {
    const area = document.createElement("textarea");
    area.value = text;
    area.setAttribute("readonly", "");
    area.style.position = "fixed";
    area.style.opacity = "0";
    area.style.pointerEvents = "none";
    document.body.appendChild(area);
    area.select();
    const ok = document.execCommand("copy");
    area.remove();
    if (!ok) throw new Error("Copy command failed");
  };

  const copyText = async (text, button) => {
    try {
      if (navigator.clipboard?.writeText) await navigator.clipboard.writeText(text);
      else fallbackCopy(text);
      const previous = button.getAttribute("aria-label") || "Copy prompt";
      button.classList.add("copied");
      button.setAttribute("aria-label", "Copied");
      button.title = "Copied";
      window.setTimeout(() => {
        button.classList.remove("copied");
        button.setAttribute("aria-label", previous);
        button.title = previous;
      }, 1300);
    } catch (error) {
      K.showError?.(`Could not copy prompt: ${error.message || String(error)}`);
    }
  };

  const copyIcon = () => {
    const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
    svg.setAttribute("viewBox", "0 0 24 24");
    svg.setAttribute("aria-hidden", "true");
    svg.innerHTML = '<rect x="8" y="8" width="11" height="11" rx="2" fill="none" stroke="currentColor" stroke-width="1.8"/><path d="M16 8V6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h2" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/>';
    return svg;
  };

  const addPromptCopyButtons = () => {
    const prompts = K.state.messages
      .filter((message) => messageRole(message) === "user")
      .map(exactUserText);
    const rows = K.els.conversation?.querySelectorAll(".message.user") || [];
    rows.forEach((row, index) => {
      const content = row.querySelector(".message-content");
      if (!content || content.querySelector(".prompt-copy-button")) return;
      const text = prompts[index] ?? String(row.querySelector(".message-text")?.textContent || "");
      const actions = document.createElement("div");
      actions.className = "user-message-actions";
      const button = document.createElement("button");
      button.type = "button";
      button.className = "prompt-copy-button";
      button.setAttribute("aria-label", "Copy prompt");
      button.title = "Copy prompt";
      button.appendChild(copyIcon());
      button.addEventListener("click", () => copyText(text, button));
      actions.appendChild(button);
      content.appendChild(actions);
    });
  };

  const tokenShape = (value) => {
    const tokens = value && typeof value === "object" ? value : {};
    const cache = tokens.cache && typeof tokens.cache === "object" ? tokens.cache : {};
    return {
      input: Number(tokens.input || 0),
      output: Number(tokens.output || 0),
      reasoning: Number(tokens.reasoning || 0),
      cacheRead: Number(cache.read || 0),
      cacheWrite: Number(cache.write || 0),
    };
  };

  const tokenTotal = (tokens) => tokens.input + tokens.output + tokens.reasoning + tokens.cacheRead + tokens.cacheWrite;

  const assistantTokens = (message) => {
    const direct = tokenShape(message?.info?.tokens || message?.tokens);
    if (tokenTotal(direct) > 0) return direct;

    const total = { input: 0, output: 0, reasoning: 0, cacheRead: 0, cacheWrite: 0 };
    for (const part of partsOf(message)) {
      if (part?.type !== "step-finish") continue;
      const next = tokenShape(part.tokens);
      total.input += next.input;
      total.output += next.output;
      total.reasoning += next.reasoning;
      total.cacheRead += next.cacheRead;
      total.cacheWrite += next.cacheWrite;
    }
    return total;
  };

  const partTimestamp = (part) => {
    const time = part?.time || {};
    return Number(time.end || time.completed || time.start || 0);
  };

  const endTimestamp = (message) => {
    const time = messageTime(message);
    let end = Number(time.completed || time.updated || time.created || 0);
    for (const part of partsOf(message)) end = Math.max(end, partTimestamp(part));
    return end;
  };

  const activeWorkMs = (messages) => {
    let total = 0;
    for (let index = 0; index < messages.length; index++) {
      if (messageRole(messages[index]) !== "user") continue;
      const start = Number(messageTime(messages[index]).created || 0);
      if (!start) continue;

      let end = start;
      let cursor = index + 1;
      for (; cursor < messages.length; cursor++) {
        if (messageRole(messages[cursor]) === "user") break;
        end = Math.max(end, endTimestamp(messages[cursor]));
      }

      const isLastTurn = cursor >= messages.length;
      if (isLastTurn && K.state.session && (K.state.sending || K.isSessionRunning(K.state.session.id))) {
        end = Math.max(end, Date.now());
      }
      if (end > start) total += end - start;
    }
    return total;
  };

  const compactNumber = (value) => {
    const number = Number(value || 0);
    if (number < 1000) return String(Math.round(number));
    if (number < 1_000_000) return `${(number / 1000).toFixed(number < 10_000 ? 1 : 0)}K`;
    return `${(number / 1_000_000).toFixed(number < 10_000_000 ? 1 : 0)}M`;
  };

  const formatDuration = (milliseconds) => {
    const seconds = Math.max(0, Math.round(Number(milliseconds || 0) / 1000));
    if (seconds < 60) return `${seconds}s`;
    const minutes = Math.floor(seconds / 60);
    const remaining = seconds % 60;
    if (minutes < 60) return `${minutes}m ${remaining}s`;
    const hours = Math.floor(minutes / 60);
    return `${hours}h ${minutes % 60}m`;
  };

  const sessionStats = () => {
    const totals = { input: 0, output: 0, reasoning: 0, cacheRead: 0, cacheWrite: 0 };
    let requests = 0;
    for (const message of K.state.messages) {
      if (messageRole(message) !== "assistant") continue;
      requests++;
      const next = assistantTokens(message);
      totals.input += next.input;
      totals.output += next.output;
      totals.reasoning += next.reasoning;
      totals.cacheRead += next.cacheRead;
      totals.cacheWrite += next.cacheWrite;
    }
    return {
      tokens: tokenTotal(totals),
      requests,
      duration: activeWorkMs(K.state.messages),
      breakdown: totals,
    };
  };

  const statItem = (label, value, title = "") => {
    const item = document.createElement("div");
    const strong = document.createElement("strong");
    const span = document.createElement("span");
    item.className = "session-stat-item";
    strong.textContent = value;
    span.textContent = label;
    if (title) item.title = title;
    item.append(strong, span);
    return item;
  };

  const renderSessionStats = () => {
    if (!K.state.session || !K.state.messages.length) return;
    const view = K.els.conversation;
    if (!view || view.querySelector(".session-stats")) return;
    const stats = sessionStats();
    if (!stats.requests && !stats.tokens) return;

    const card = document.createElement("section");
    card.className = "session-stats";
    card.setAttribute("aria-label", "Session statistics");

    const heading = document.createElement("div");
    heading.className = "session-stats-heading";
    heading.textContent = "Session stats";

    const values = document.createElement("div");
    values.className = "session-stats-values";
    const breakdown = stats.breakdown;
    values.append(
      statItem(
        "Tokens",
        compactNumber(stats.tokens),
        `Input ${compactNumber(breakdown.input)} · Output ${compactNumber(breakdown.output)} · Reasoning ${compactNumber(breakdown.reasoning)} · Cache read ${compactNumber(breakdown.cacheRead)} · Cache write ${compactNumber(breakdown.cacheWrite)}`,
      ),
      statItem("Requests", String(stats.requests), "Model requests recorded in this session"),
      statItem("Work time", formatDuration(stats.duration), "Time spent processing user turns; idle time between prompts is excluded"),
    );
    card.append(heading, values);
    view.appendChild(card);
  };

  const addTimeoutRecovery = () => {
    const rows = K.els.conversation?.querySelectorAll(".message.error") || [];
    for (const row of rows) {
      const error = row.querySelector(".message-error-text");
      if (!error || !/upstream idle timeout exceeded/i.test(error.textContent || "")) continue;
      const content = row.querySelector(".message-content");
      if (!content || content.querySelector(".timeout-recovery")) continue;

      const recovery = document.createElement("div");
      recovery.className = "timeout-recovery";
      const copy = document.createElement("span");
      copy.textContent = "The upstream model stopped responding. Existing file changes are preserved.";
      const button = document.createElement("button");
      button.type = "button";
      button.className = "timeout-resume-button";
      button.textContent = "Resume";
      button.title = "Continue the task from the current workspace state";
      button.addEventListener("click", async () => {
        if (K.state.sending || (K.state.session && K.isSessionRunning(K.state.session.id))) return;
        K.els.prompt.value = RESUME_PROMPT;
        K.resizePrompt?.();
        await K.sendPrompt?.();
      });
      recovery.append(copy, button);
      content.appendChild(recovery);
    }
  };

  K.renderMessages = (...args) => {
    const result = baseRenderMessages(...args);
    normalizeCancellation();
    normalizeRetryableToolErrors();
    addPromptCopyButtons();
    addTimeoutRecovery();
    renderSessionStats();
    return result;
  };
})();
