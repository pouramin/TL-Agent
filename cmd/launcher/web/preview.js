(() => {
  "use strict";

  const K = window.KLU;
  if (!K || K.__previewInstalled) return;
  K.__previewInstalled = true;

  const ensureStyles = () => {
    if (document.querySelector('link[href="/preview.css"]')) return;
    const link = document.createElement("link");
    link.rel = "stylesheet";
    link.href = "/preview.css";
    document.head.appendChild(link);
  };

  const request = async (path, options = {}) => {
    const response = await fetch(path, {
      cache: "no-store",
      ...options,
      headers: { ...(options.body ? { "Content-Type": "application/json" } : {}), ...(options.headers || {}) },
    });
    const type = response.headers.get("content-type") || "";
    const payload = response.status === 204
      ? null
      : type.includes("application/json") ? await response.json().catch(() => null) : await response.text().catch(() => "");
    if (!response.ok) {
      const detail = payload && typeof payload === "object"
        ? payload.error || payload.message || JSON.stringify(payload)
        : String(payload || `${response.status} ${response.statusText}`);
      const error = new Error(detail);
      error.status = response.status;
      throw error;
    }
    return payload;
  };

  ensureStyles();

  const workspace = document.querySelector(".files-workspace");
  const headActions = document.querySelector(".files-head-actions");
  if (!workspace || !headActions) return;

  const button = document.createElement("button");
  button.id = "livePreviewButton";
  button.type = "button";
  button.className = "ghost small";
  button.textContent = "Preview";
  button.title = "Open live web preview";
  headActions.insertBefore(button, document.getElementById("refreshFiles") || headActions.firstChild);

  const pane = document.createElement("section");
  pane.id = "livePreviewPane";
  pane.className = "live-preview-pane hidden";
  pane.setAttribute("aria-label", "Live web preview");
  pane.innerHTML = `
    <div class="live-preview-head">
      <div class="live-preview-title">
        <div><strong>Live Preview</strong><span id="livePreviewKind">Web output</span></div>
        <span id="livePreviewURL" class="live-preview-url">Not running</span>
      </div>
      <div class="live-preview-actions">
        <button id="livePreviewStart" class="primary small" type="button">Start</button>
        <button id="livePreviewStop" class="ghost small" type="button" disabled>Stop</button>
        <button id="livePreviewReload" class="ghost small" type="button" disabled>Reload</button>
        <button id="livePreviewOpen" class="ghost small" type="button" disabled>Open</button>
        <button id="livePreviewClose" class="icon-button" type="button" aria-label="Close preview">×</button>
      </div>
    </div>
    <div class="live-preview-stage">
      <div id="livePreviewEmpty" class="live-preview-empty">
        <strong>Preview this project</strong>
        <span>Static HTML reloads automatically. Supported dev servers keep their normal HMR behavior.</span>
      </div>
      <iframe id="livePreviewFrame" class="live-preview-frame hidden" title="Live web preview"></iframe>
    </div>
    <div class="live-preview-footer">
      <span id="livePreviewStatus">Ready</span>
      <button id="livePreviewLogsToggle" class="preview-link-button hidden" type="button">Logs</button>
    </div>
    <pre id="livePreviewLogs" class="live-preview-logs hidden"></pre>`;
  workspace.appendChild(pane);

  const ui = {
    button,
    pane,
    kind: document.getElementById("livePreviewKind"),
    url: document.getElementById("livePreviewURL"),
    start: document.getElementById("livePreviewStart"),
    stop: document.getElementById("livePreviewStop"),
    reload: document.getElementById("livePreviewReload"),
    open: document.getElementById("livePreviewOpen"),
    close: document.getElementById("livePreviewClose"),
    empty: document.getElementById("livePreviewEmpty"),
    frame: document.getElementById("livePreviewFrame"),
    status: document.getElementById("livePreviewStatus"),
    logsToggle: document.getElementById("livePreviewLogsToggle"),
    logs: document.getElementById("livePreviewLogs"),
  };

  K.state.preview = {
    info: null,
    process: null,
    processPoll: null,
    versionPoll: null,
    targetURL: "",
    bridgeURL: "",
    output: "",
    version: "",
    open: false,
  };

  const state = () => K.state.preview;
  const stripANSI = (value) => String(value || "").replace(/\x1b\[[0-?]*[ -/]*[@-~]/g, "");
  const lastLines = (value, count = 8) => stripANSI(value).trim().split(/\r?\n/).filter(Boolean).slice(-count).join("\n");

  const setStatus = (text, tone = "") => {
    ui.status.textContent = text;
    ui.status.dataset.tone = tone;
  };

  const setLogs = (output) => {
    const text = stripANSI(output || "");
    state().output = text;
    ui.logs.textContent = text;
    ui.logsToggle.classList.toggle("hidden", !text.trim());
  };

  const setPaneOpen = (open) => {
    state().open = !!open;
    workspace.classList.toggle("preview-open", !!open);
    ui.pane.classList.toggle("hidden", !open);
    ui.button.classList.toggle("preview-toggle-active", !!open);
  };

  const bridgeURL = (target, nonce = "") => {
    const query = new URLSearchParams({ url: target });
    if (nonce) query.set("reload", nonce);
    return `/local/preview/bridge?${query}`;
  };

  const showTarget = (target, { reload = false } = {}) => {
    if (!target) return;
    state().targetURL = target;
    state().bridgeURL = bridgeURL(target, reload ? String(Date.now()) : "");
    ui.frame.src = state().bridgeURL;
    ui.frame.classList.remove("hidden");
    ui.empty.classList.add("hidden");
    ui.url.textContent = target;
    ui.url.title = target;
    ui.reload.disabled = false;
    ui.open.disabled = false;
  };

  const clearTarget = () => {
    state().targetURL = "";
    state().bridgeURL = "";
    ui.frame.removeAttribute("src");
    ui.frame.classList.add("hidden");
    ui.empty.classList.remove("hidden");
    ui.url.textContent = "Not running";
    ui.url.title = "";
    ui.reload.disabled = true;
    ui.open.disabled = true;
  };

  const stopProcessPolling = () => {
    if (state().processPoll) window.clearInterval(state().processPoll);
    state().processPoll = null;
  };

  const stopVersionPolling = () => {
    if (state().versionPoll) window.clearInterval(state().versionPoll);
    state().versionPoll = null;
  };

  const normalizeLoopbackURL = (candidate) => {
    try {
      const parsed = new URL(String(candidate).replace(/[),.;]+$/, ""));
      const host = parsed.hostname.toLowerCase();
      if (host === "0.0.0.0") parsed.hostname = "127.0.0.1";
      else if (host === "[::]" || host === "::") parsed.hostname = "[::1]";
      const normalized = parsed.hostname.replace(/^\[|\]$/g, "").toLowerCase();
      if (normalized === "localhost" || normalized === "::1" || normalized.startsWith("127.")) return parsed.toString();
    } catch {}
    return "";
  };

  const URL_PATTERN = /https?:\/\/(?:localhost|127(?:\.\d{1,3}){3}|0\.0\.0\.0|\[::1\]|\[::\])(?::\d+)?(?:\/[^\s]*)?/ig;
  const discoverServerURL = (output) => {
    const matches = stripANSI(output).match(URL_PATTERN) || [];
    for (let index = matches.length - 1; index >= 0; index--) {
      const value = normalizeLoopbackURL(matches[index]);
      if (value) return value;
    }
    return "";
  };

  const detect = async () => {
    const info = await request("/local/preview/info");
    state().info = info;
    const label = [info?.framework, info?.mode === "dev-server" ? info?.packageManager : ""].filter(Boolean).join(" · ");
    ui.kind.textContent = label || "Web output";
    return info;
  };

  const pollStaticVersion = async () => {
    if (state().info?.mode !== "static" || !state().targetURL) return;
    try {
      const payload = await request("/local/preview/version");
      const next = String(payload?.version || "");
      if (!next) return;
      if (!state().version) {
        state().version = next;
        return;
      }
      if (next !== state().version) {
        state().version = next;
        showTarget(state().targetURL, { reload: true });
        setStatus("Updated from project files", "ok");
      }
    } catch {}
  };

  const startStaticVersionPolling = async () => {
    stopVersionPolling();
    state().version = "";
    await pollStaticVersion();
    state().versionPoll = window.setInterval(pollStaticVersion, 900);
  };

  const renderProcessSnapshot = (snapshot) => {
    state().process = snapshot;
    setLogs(snapshot?.output || "");
    const discovered = discoverServerURL(snapshot?.output || "");
    if (discovered && !state().targetURL) {
      showTarget(discovered);
      setStatus("Preview server running", "ok");
    } else if (!state().targetURL && snapshot?.running) {
      const tail = lastLines(snapshot.output, 1);
      setStatus(tail || "Waiting for the preview server…", "working");
    }
    ui.stop.disabled = !snapshot?.running;
    if (!snapshot?.running) {
      stopProcessPolling();
      ui.start.disabled = false;
      ui.stop.disabled = true;
      if (!state().targetURL) {
        const code = snapshot?.exitCode;
        setStatus(`Preview command exited${code == null ? "" : ` with code ${code}`}`, "error");
      } else {
        setStatus("Preview server stopped", "warning");
      }
    }
  };

  const pollProcess = async () => {
    const id = state().process?.id;
    if (!id) return stopProcessPolling();
    try {
      renderProcessSnapshot(await request(`/local/process/${encodeURIComponent(id)}`));
    } catch (error) {
      stopProcessPolling();
      ui.start.disabled = false;
      ui.stop.disabled = true;
      setStatus(`Preview process error: ${error.message || error}`, "error");
    }
  };

  const stopPreview = async ({ clear = true } = {}) => {
    stopVersionPolling();
    stopProcessPolling();
    const id = state().process?.id;
    if (id && state().process?.running) {
      try { await request(`/local/process/${encodeURIComponent(id)}`, { method: "DELETE" }); }
      catch (error) { console.warn("[TL Agent] Could not stop preview process", error); }
    }
    state().process = null;
    state().version = "";
    ui.start.disabled = false;
    ui.stop.disabled = true;
    if (clear) clearTarget();
    setStatus("Stopped");
  };

  const startPreview = async () => {
    if (state().process?.running) return;
    setPaneOpen(true);
    ui.start.disabled = true;
    setStatus("Detecting project…", "working");
    K.showError("");
    try {
      const info = await detect();
      if (!info?.supported) {
        ui.start.disabled = false;
        clearTarget();
        setStatus(info?.reason || "This project does not have a supported preview target.", "warning");
        return;
      }
      if (info.mode === "static") {
        showTarget(info.url);
        ui.start.disabled = false;
        ui.stop.disabled = true;
        setStatus("Static preview · auto reload on file changes", "ok");
        await startStaticVersionPolling();
        return;
      }
      if (info.mode !== "dev-server" || !info.command) {
        ui.start.disabled = false;
        setStatus("Unsupported preview configuration", "warning");
        return;
      }

      const approved = window.confirm(`Live Preview will run this project command:\n\n${info.command}\n\nProject scripts can execute code on this computer. Run it?`);
      if (!approved) {
        ui.start.disabled = false;
        setStatus("Preview command was not started");
        return;
      }

      clearTarget();
      setLogs("");
      setStatus(`Starting ${info.command}…`, "working");
      const snapshot = await request("/local/process", {
        method: "POST",
        body: JSON.stringify({ command: info.command }),
      });
      state().process = { ...snapshot, output: snapshot?.output || "" };
      ui.stop.disabled = false;
      stopProcessPolling();
      state().processPoll = window.setInterval(pollProcess, 350);
      await pollProcess();
    } catch (error) {
      ui.start.disabled = false;
      ui.stop.disabled = true;
      setStatus(error.message || String(error), "error");
    }
  };

  const reloadPreview = () => {
    if (!state().targetURL) return;
    showTarget(state().targetURL, { reload: true });
    setStatus("Reloaded", "ok");
  };

  button.addEventListener("click", async () => {
    const opening = ui.pane.classList.contains("hidden");
    setPaneOpen(opening);
    if (opening && !state().targetURL && !state().process?.running) await startPreview();
  });
  ui.close.addEventListener("click", () => setPaneOpen(false));
  ui.start.addEventListener("click", startPreview);
  ui.stop.addEventListener("click", () => stopPreview());
  ui.reload.addEventListener("click", reloadPreview);
  ui.open.addEventListener("click", () => {
    if (state().targetURL) window.open(state().targetURL, "_blank", "noopener,noreferrer");
  });
  ui.logsToggle.addEventListener("click", () => ui.logs.classList.toggle("hidden"));

  const baseAfterProjectChange = K.afterProjectChange;
  if (typeof baseAfterProjectChange === "function") {
    K.afterProjectChange = async (...args) => {
      await stopPreview();
      state().info = null;
      ui.kind.textContent = "Web output";
      const result = await baseAfterProjectChange(...args);
      if (state().open) setStatus("Project changed · start preview when ready");
      return result;
    };
  }

  window.addEventListener("beforeunload", () => {
    const id = state().process?.id;
    if (!id || !state().process?.running) return;
    try { fetch(`/local/process/${encodeURIComponent(id)}`, { method: "DELETE", keepalive: true }); }
    catch {}
  });

  K.preview = Object.freeze({
    open: async () => { setPaneOpen(true); if (!state().targetURL && !state().process?.running) await startPreview(); },
    close: () => setPaneOpen(false),
    start: startPreview,
    stop: stopPreview,
    reload: reloadPreview,
  });
})();
