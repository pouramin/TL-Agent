(() => {
  "use strict";
  const K = window.KLU;
  const $ = (id) => document.getElementById(id);

  const ui = {
    button: $("filesButton"),
    panel: $("filesPanel"),
    close: $("closeFiles"),
    refresh: $("refreshFiles"),
    up: $("filesUp"),
    breadcrumb: $("filesBreadcrumb"),
    list: $("filesList"),
    previewTitle: $("filePreviewTitle"),
    previewPath: $("filePreviewPath"),
    previewMeta: $("filePreviewMeta"),
    previewBody: $("filePreviewBody"),
    changesPanel: $("changesPanel"),
    changesButton: $("changesButton"),
  };

  K.state.filesPath = "";
  K.state.filesEntries = [];
  K.state.filePreview = null;
  K.state.filesProject = "";
  K.state.filesLoading = false;

  const sizeText = (bytes) => {
    const value = Number(bytes) || 0;
    if (value < 1024) return `${value} B`;
    if (value < 1024 * 1024) return `${(value / 1024).toFixed(value < 10 * 1024 ? 1 : 0)} KB`;
    return `${(value / (1024 * 1024)).toFixed(1)} MB`;
  };

  const parentPath = (value) => {
    const bits = String(value || "").split("/").filter(Boolean);
    bits.pop();
    return bits.join("/");
  };

  const extensionLabel = (path) => {
    const name = String(path || "").split("/").pop() || "";
    const index = name.lastIndexOf(".");
    return index > 0 ? name.slice(index + 1).toUpperCase() : "TEXT";
  };

  const resetFiles = () => {
    K.state.filesPath = "";
    K.state.filesEntries = [];
    K.state.filePreview = null;
    K.state.filesProject = K.state.local?.project || "";
    K.state.filesLoading = false;
    renderFiles();
    renderPreview();
  };

  const renderFiles = () => {
    if (!ui.list || !ui.breadcrumb || !ui.up) return;
    ui.breadcrumb.textContent = K.state.filesPath || "Project root";
    ui.breadcrumb.title = K.state.filesPath || K.state.local?.project || "Project root";
    ui.up.disabled = !K.state.filesPath;
    ui.list.textContent = "";

    if (K.state.filesLoading) {
      const loading = document.createElement("div");
      loading.className = "files-empty";
      loading.textContent = "Reading folder…";
      ui.list.appendChild(loading);
      return;
    }

    if (!K.state.filesEntries.length) {
      const empty = document.createElement("div");
      empty.className = "files-empty";
      empty.textContent = "This folder is empty.";
      ui.list.appendChild(empty);
      return;
    }

    for (const entry of K.state.filesEntries) {
      const row = document.createElement("button");
      row.type = "button";
      row.className = `file-row file-${entry.type || "file"}`;
      row.title = entry.path || entry.name;

      const icon = document.createElement("span");
      icon.className = "file-icon";
      icon.textContent = entry.type === "directory" ? "▸" : entry.type === "symlink" ? "↗" : "·";

      const copy = document.createElement("span");
      copy.className = "file-copy";
      const name = document.createElement("strong");
      name.textContent = entry.name || entry.path || "Untitled";
      const meta = document.createElement("small");
      meta.textContent = entry.type === "directory" ? "Folder" : entry.type === "symlink" ? "Symlink" : sizeText(entry.size);
      copy.append(name, meta);
      row.append(icon, copy);

      row.addEventListener("click", () => {
        if (entry.type === "directory") loadDirectory(entry.path);
        else loadPreview(entry.path);
      });
      ui.list.appendChild(row);
    }
  };

  const renderPreview = () => {
    if (!ui.previewTitle || !ui.previewPath || !ui.previewMeta || !ui.previewBody) return;
    const file = K.state.filePreview;
    ui.previewBody.textContent = "";

    if (!file) {
      ui.previewTitle.textContent = "Select a file";
      ui.previewPath.textContent = "Read-only preview";
      ui.previewMeta.textContent = "";
      const empty = document.createElement("div");
      empty.className = "file-preview-empty";
      empty.textContent = "Choose a text file from the project explorer to preview it here.";
      ui.previewBody.appendChild(empty);
      return;
    }

    ui.previewTitle.textContent = file.name || file.path || "File";
    ui.previewPath.textContent = file.path || "";
    ui.previewMeta.textContent = `${extensionLabel(file.path)} · ${sizeText(file.size)} · read-only`;

    if (file.binary) {
      const empty = document.createElement("div");
      empty.className = "file-preview-empty";
      empty.textContent = `Binary preview is unavailable (${file.mime || "unknown type"}).`;
      ui.previewBody.appendChild(empty);
      return;
    }

    const pre = document.createElement("pre");
    pre.className = "file-code";
    const code = document.createElement("code");
    code.textContent = file.content || "";
    pre.appendChild(code);
    ui.previewBody.appendChild(pre);
  };

  const loadDirectory = async (path = "") => {
    K.state.filesLoading = true;
    K.state.filesPath = path || "";
    renderFiles();
    try {
      const params = new URLSearchParams();
      if (path) params.set("path", path);
      const payload = await K.request(`/local/files${params.size ? `?${params}` : ""}`);
      K.state.filesPath = payload?.path || "";
      K.state.filesEntries = Array.isArray(payload?.entries) ? payload.entries : [];
    } catch (error) {
      K.state.filesEntries = [];
      K.showError(error.message || String(error));
    } finally {
      K.state.filesLoading = false;
      renderFiles();
    }
  };

  const loadPreview = async (path) => {
    if (!path) return;
    ui.previewTitle.textContent = "Loading…";
    ui.previewPath.textContent = path;
    ui.previewMeta.textContent = "";
    ui.previewBody.textContent = "";
    try {
      K.state.filePreview = await K.request(`/local/file?${new URLSearchParams({ path })}`);
      renderPreview();
    } catch (error) {
      K.state.filePreview = null;
      renderPreview();
      K.showError(error.message || String(error));
    }
  };

  const openFiles = async () => {
    if (!ui.panel) return;
    ui.changesPanel?.classList.add("hidden");
    ui.panel.classList.remove("hidden");
    const project = K.state.local?.project || "";
    if (K.state.filesProject !== project) resetFiles();
    await loadDirectory(K.state.filesPath || "");
    if (K.state.filePreview?.path) await loadPreview(K.state.filePreview.path).catch(() => {});
  };

  ui.button?.addEventListener("click", openFiles);
  ui.close?.addEventListener("click", () => ui.panel.classList.add("hidden"));
  ui.refresh?.addEventListener("click", async () => {
    await loadDirectory(K.state.filesPath || "");
    if (K.state.filePreview?.path) await loadPreview(K.state.filePreview.path);
  });
  ui.up?.addEventListener("click", () => loadDirectory(parentPath(K.state.filesPath)));
  ui.changesButton?.addEventListener("click", () => ui.panel?.classList.add("hidden"));

  const originalAfterProjectChange = K.afterProjectChange;
  K.afterProjectChange = async (...args) => {
    resetFiles();
    ui.panel?.classList.add("hidden");
    return originalAfterProjectChange(...args);
  };

  const originalHandleKiloEvent = K.handleKiloEvent;
  K.handleKiloEvent = (event) => {
    originalHandleKiloEvent(event);
    const type = event?.type || "";
    if (!ui.panel?.classList.contains("hidden") && type.startsWith("file.")) {
      window.setTimeout(async () => {
        await loadDirectory(K.state.filesPath || "");
        if (K.state.filePreview?.path) await loadPreview(K.state.filePreview.path).catch(() => {});
      }, 100);
    }
  };

  resetFiles();
})();
