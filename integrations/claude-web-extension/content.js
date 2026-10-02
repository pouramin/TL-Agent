"use strict";

const INPUT_SELECTORS = [
  'div[contenteditable="true"].ProseMirror',
  'fieldset div[contenteditable="true"]',
  '[contenteditable="true"][aria-label*="prompt" i]',
  'div[contenteditable="true"]'
];

const SEND_SELECTORS = [
  'button[data-testid="send-button"]',
  'button[aria-label="Send message"]',
  'button[aria-label="Send Message"]',
  'button[aria-label*="Send" i]',
  'fieldset button[type="submit"]'
];

const RESPONSE_SELECTORS = [
  'div.font-claude-message',
  '[data-testid="chat-message-content"]',
  'div.font-claude-response'
];

const STREAMING_SELECTORS = [
  '[data-is-streaming="true"]',
  'button[data-testid="stop-button"]',
  'button[aria-label*="Stop" i]'
];

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

const firstMatch = (selectors) => {
  for (const selector of selectors) {
    const element = document.querySelector(selector);
    if (element) return element;
  }
  return null;
};

const allResponses = () => {
  const seen = new Set();
  const result = [];
  for (const selector of RESPONSE_SELECTORS) {
    for (const element of document.querySelectorAll(selector)) {
      if (seen.has(element)) continue;
      seen.add(element);
      result.push(element);
    }
  }
  return result;
};

const waitFor = async (fn, timeoutMs, interval = 100) => {
  const end = Date.now() + timeoutMs;
  while (Date.now() < end) {
    const value = fn();
    if (value) return value;
    await sleep(interval);
  }
  return null;
};

const insertPrompt = (editor, text) => {
  editor.focus();
  if (editor instanceof HTMLTextAreaElement || editor instanceof HTMLInputElement) {
    const setter = Object.getOwnPropertyDescriptor(Object.getPrototypeOf(editor), "value")?.set;
    if (setter) setter.call(editor, text);
    else editor.value = text;
    editor.dispatchEvent(new Event("input", { bubbles: true }));
    return;
  }

  const selection = window.getSelection();
  const range = document.createRange();
  range.selectNodeContents(editor);
  selection.removeAllRanges();
  selection.addRange(range);
  document.execCommand("insertText", false, text);
  editor.dispatchEvent(new InputEvent("input", {
    bubbles: true,
    inputType: "insertText",
    data: text
  }));
};

const sendPrompt = async (prompt) => {
  const editor = await waitFor(() => firstMatch(INPUT_SELECTORS), 15000);
  if (!editor) return { ok: false, error: "Claude message box was not found." };

  const before = allResponses().length;
  insertPrompt(editor, prompt);
  await sleep(500);

  const send = firstMatch(SEND_SELECTORS);
  if (send && !send.disabled) {
    send.click();
  } else {
    editor.dispatchEvent(new KeyboardEvent("keydown", {
      key: "Enter",
      code: "Enter",
      keyCode: 13,
      which: 13,
      bubbles: true,
      cancelable: true
    }));
  }

  let lastText = "";
  let stableSince = 0;
  const deadline = Date.now() + 180000;

  while (Date.now() < deadline) {
    const responses = allResponses();
    const candidate = responses.length > before ? responses[responses.length - 1] : null;
    const text = String(candidate?.innerText || candidate?.textContent || "").trim();
    const streaming = !!firstMatch(STREAMING_SELECTORS);

    if (text) {
      if (text !== lastText) {
        lastText = text;
        stableSince = Date.now();
      } else if (!streaming && Date.now() - stableSince >= 1200) {
        return { ok: true, text: lastText };
      }
    }
    await sleep(250);
  }

  return lastText
    ? { ok: true, text: lastText }
    : { ok: false, error: "Timed out waiting for Claude response." };
};

const pairFromFragment = async () => {
  const raw = location.hash.startsWith("#") ? location.hash.slice(1) : "";
  if (!raw) return;
  const params = new URLSearchParams(raw);
  const token = params.get("tlstudio_pair");
  const origin = params.get("tlstudio_origin");
  if (!token || !origin) return;

  try { await chrome.runtime.sendMessage({ type: "tlstudio-pair", token, origin }); } catch {}
  params.delete("tlstudio_pair");
  params.delete("tlstudio_origin");
  const nextHash = params.toString();
  history.replaceState(history.state, "", location.pathname + location.search + (nextHash ? "#" + nextHash : ""));
};

chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
  if (message?.type === "tlstudio-probe") {
    const editor = firstMatch(INPUT_SELECTORS);
    sendResponse({ ok: true, connected: !!editor });
    return;
  }
  if (message?.type === "tlstudio-complete") {
    void sendPrompt(String(message.prompt || "")).then(sendResponse, (error) => {
      sendResponse({ ok: false, error: String(error?.message || error) });
    });
    return true;
  }
});

void pairFromFragment();
setInterval(() => {
  try { chrome.runtime.sendMessage({ type: "tlstudio-ensure-polling" }); } catch {}
}, 1500);
