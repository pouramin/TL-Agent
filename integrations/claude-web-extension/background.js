"use strict";

let polling = false;
let pair = null;

const validOrigin = (value) => {
  try {
    const u = new URL(String(value || ""));
    return u.protocol === "http:" && (u.hostname === "127.0.0.1" || u.hostname === "localhost");
  } catch { return false; }
};

const savePair = async (origin, token) => {
  pair = { origin: origin.replace(/\/$/, ""), token };
  await chrome.storage.local.set({ tlStudioOrigin: pair.origin, tlStudioPairToken: pair.token });
};

const loadPair = async () => {
  const stored = await chrome.storage.local.get(["tlStudioOrigin","tlStudioPairToken"]);
  if (validOrigin(stored.tlStudioOrigin) && stored.tlStudioPairToken) {
    pair = { origin: String(stored.tlStudioOrigin).replace(/\/$/, ""), token: String(stored.tlStudioPairToken) };
  }
};

const waitForTabReady = (tabId, timeoutMs = 20000) => new Promise((resolve, reject) => {
  const timer = setTimeout(() => {
    chrome.tabs.onUpdated.removeListener(onUpdated);
    reject(new Error("Claude tab did not finish loading."));
  }, timeoutMs);
  const onUpdated = (id, info) => {
    if (id !== tabId || info.status !== "complete") return;
    clearTimeout(timer);
    chrome.tabs.onUpdated.removeListener(onUpdated);
    resolve();
  };
  chrome.tabs.onUpdated.addListener(onUpdated);
});

const executeProbe = async (command) => {
  const tabs = await chrome.tabs.query({ url: "https://claude.ai/*" });
  const tab = tabs.find((item) => Number.isInteger(item.id));
  if (!tab) return { id: command.id, ok: true, connected: false };
  try {
    const result = await chrome.tabs.sendMessage(tab.id, { type: "tlstudio-probe" });
    return { id: command.id, ...(result || { ok: true, connected: false }) };
  } catch {
    return { id: command.id, ok: true, connected: false };
  }
};

const executeCompletion = async (command) => {
  const tab = await chrome.tabs.create({ url: "https://claude.ai/new", active: false });
  try {
    if (!Number.isInteger(tab.id)) throw new Error("Claude tab could not be created.");
    if (tab.status !== "complete") await waitForTabReady(tab.id);
    await new Promise((resolve) => setTimeout(resolve, 800));
    const result = await chrome.tabs.sendMessage(tab.id, {
      type: "tlstudio-complete",
      prompt: String(command.prompt || "")
    });
    return { id: command.id, ...(result || { ok: false, error: "Claude Web returned no result." }) };
  } catch (error) {
    return { id: command.id, ok: false, error: String(error?.message || error) };
  } finally {
    if (Number.isInteger(tab.id)) {
      try { await chrome.tabs.remove(tab.id); } catch {}
    }
  }
};

const execute = async (command) => {
  if (command.kind === "probe") return executeProbe(command);
  if (command.kind === "complete") return executeCompletion(command);
  return { id: command.id, ok: false, error: "Unsupported Claude Web bridge command." };
};

const postResult = async (result) => {
  if (!pair) return;
  await fetch(pair.origin + "/local/claude-web-extension/result?token=" + encodeURIComponent(pair.token), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(result),
    cache: "no-store",
    credentials: "omit"
  });
};

const poll = async () => {
  if (!pair) return;
  const response = await fetch(pair.origin + "/local/claude-web-extension/poll?token=" + encodeURIComponent(pair.token), {
    cache: "no-store",
    credentials: "omit"
  });
  if (response.status === 401 || response.status === 403) {
    pair = null;
    return;
  }
  if (!response.ok) throw new Error("TL Studio bridge returned HTTP " + response.status);
  const payload = await response.json();
  if (payload?.command) await postResult(await execute(payload.command));
};

const startPolling = async () => {
  if (polling) return;
  if (!pair) await loadPair();
  if (!pair) return;
  polling = true;
  try {
    while (pair) {
      try { await poll(); } catch {}
      await new Promise((resolve) => setTimeout(resolve, 500));
    }
  } finally {
    polling = false;
  }
};

chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
  if (message?.type === "tlstudio-pair") {
    const origin = String(message.origin || "");
    const token = String(message.token || "");
    if (!validOrigin(origin) || token.length < 32) {
      sendResponse({ ok: false });
      return;
    }
    void savePair(origin, token).then(() => {
      void startPolling();
      sendResponse({ ok: true });
    });
    return true;
  }
  if (message?.type === "tlstudio-ensure-polling") {
    void startPolling();
    sendResponse({ ok: true });
  }
});

chrome.runtime.onStartup.addListener(() => { void startPolling(); });
chrome.runtime.onInstalled.addListener(() => { void startPolling(); });
void startPolling();
