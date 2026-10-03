"use strict";

const DEFAULT_MODEL = "claude-sonnet-5-5";
const BRIDGE_VERSION = "0.6.1-page-context";
const MAX_PROMPT_BYTES = 8 * 1024 * 1024;
let pairedToken = "";
let pairedOrigin = "";

const localSenderOrigin = (sender) => {
  try {
    const raw = String(sender?.url || sender?.origin || "");
    const url = new URL(raw);
    if (url.protocol !== "http:") return "";
    if (url.hostname !== "127.0.0.1" && url.hostname !== "localhost") return "";
    return url.origin;
  } catch {
    return "";
  }
};

const validateCommand = (command) => {
  const id = String(command?.id || "").trim();
  const kind = String(command?.kind || "").trim();
  if (!id) throw new Error("Claude Web command id is missing.");
  if (kind !== "probe" && kind !== "complete") {
    throw new Error("Unsupported Claude Web transport command.");
  }
  const prompt = kind === "complete" ? String(command?.prompt || "") : "";
  if (new TextEncoder().encode(prompt).byteLength > MAX_PROMPT_BYTES) {
    throw new Error("Claude Web transport prompt is too large.");
  }
  return { id, kind, prompt };
};

const waitForTabReady = async (tabId) => {
  try {
    const tab = await chrome.tabs.get(tabId);
    if (tab?.status === "complete") return;
  } catch {}
  await new Promise((resolve, reject) => {
    let settled = false;
    const finish = (error) => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      chrome.tabs.onUpdated.removeListener(onUpdated);
      chrome.tabs.onRemoved.removeListener(onRemoved);
      if (error) reject(error);
      else resolve();
    };
    const onUpdated = (updatedId, changeInfo) => {
      if (updatedId === tabId && changeInfo.status === "complete") finish();
    };
    const onRemoved = (removedId) => {
      if (removedId === tabId) finish(new Error("Claude Web transport tab closed before it loaded."));
    };
    const timer = setTimeout(() => finish(new Error("Claude Web transport tab did not load in time.")), 20000);
    chrome.tabs.onUpdated.addListener(onUpdated);
    chrome.tabs.onRemoved.addListener(onRemoved);
  });
};

const withClaudePage = async (kind, prompt) => {
  const existing = await chrome.tabs.query({ url: ["https://claude.ai/*"] });
  let tab = existing.find((item) => Number.isInteger(item?.id));
  let created = false;
  if (!tab?.id) {
    tab = await chrome.tabs.create({ url: "https://claude.ai/new", active: false });
    created = true;
  }
  if (!tab?.id) throw new Error("Chrome did not create a Claude Web transport tab.");

  try {
    await waitForTabReady(tab.id);
    const results = await chrome.scripting.executeScript({
      target: { tabId: tab.id },
      world: "MAIN",
      func: async (requestKind, requestPrompt, requestedModel) => {
        const apiFetch = (path, options = {}) => fetch(path, {
          cache: "no-store",
          credentials: "include",
          ...options,
          headers: {
            "Accept": options.accept || "application/json",
            "Content-Type": "application/json",
            "Cache-Control": "no-cache",
            "Pragma": "no-cache",
            ...(options.headers || {})
          }
        });

        const probe = async () => {
          try {
            const response = await apiFetch("/api/organizations", { method: "GET" });
            const text = await response.text();
            if (!response.ok) {
              return {
                ok: true,
                connected: false,
                status: response.status,
                error: text.slice(0, 1000)
              };
            }
            let organizations;
            try {
              organizations = JSON.parse(text);
            } catch {
              return {
                ok: true,
                connected: false,
                status: response.status,
                error: "Claude Web returned invalid organization data."
              };
            }
            const org = Array.isArray(organizations) && organizations.length ? organizations[0] : null;
            const organizationId = String(org?.uuid || org?.id || "").trim();
            if (!organizationId) {
              return { ok: true, connected: false, status: response.status, error: "Claude organization was not found." };
            }
            return {
              ok: true,
              connected: true,
              status: response.status,
              organizationId,
              organizationName: org?.name ? String(org.name) : ""
            };
          } catch (error) {
            return { ok: false, connected: false, error: String(error?.message || error) };
          }
        };

        const readSSEText = async (response) => {
          if (!response.body) throw new Error("Claude Web returned no response body.");
          const reader = response.body.getReader();
          const decoder = new TextDecoder();
          let buffer = "";
          let dataLines = [];
          let output = "";

          const consumeEvent = () => {
            if (!dataLines.length) return false;
            const data = dataLines.join("\n");
            dataLines = [];
            if (!data || data === "[DONE]") return data === "[DONE]";

            let event;
            try { event = JSON.parse(data); } catch { return false; }
            if (event.type === "error") {
              throw new Error(String(event.message || event.error?.message || "Claude Web stream error"));
            }
            if (typeof event.completion === "string") output += event.completion;
            if (
              event.type === "content_block_delta" &&
              event.delta &&
              event.delta.type === "text_delta" &&
              typeof event.delta.text === "string"
            ) {
              output += event.delta.text;
            }
            const stopReason = String(
              event.stop_reason ||
              event.stopReason ||
              event.delta?.stop_reason ||
              ""
            ).trim();
            return event.type === "message_stop" ||
              event.type === "completion_stop" ||
              stopReason === "stop_sequence" ||
              stopReason === "end_turn" ||
              stopReason === "max_tokens";
          };

          const finish = async () => {
            try { await reader.cancel(); } catch {}
            return output.trim();
          };

          while (true) {
            const { value, done } = await reader.read();
            if (done) break;
            buffer += decoder.decode(value, { stream: true });
            let newline = buffer.indexOf("\n");
            while (newline >= 0) {
              let line = buffer.slice(0, newline);
              buffer = buffer.slice(newline + 1);
              if (line.endsWith("\r")) line = line.slice(0, -1);
              if (line === "") {
                if (consumeEvent()) return await finish();
              } else if (line.startsWith("data:")) {
                dataLines.push(line.slice(5).trimStart());
              }
              newline = buffer.indexOf("\n");
            }
          }

          buffer += decoder.decode();
          if (buffer.trim().startsWith("data:")) {
            dataLines.push(buffer.trim().slice(5).trimStart());
          }
          consumeEvent();
          return output.trim();
        };

        if (requestKind === "probe") return await probe();
        if (requestKind !== "complete") {
          return { ok: false, error: "Unsupported Claude Web transport command." };
        }

        const org = await probe();
        if (!org.ok || !org.connected || !org.organizationId) return org;

        const organizationId = org.organizationId;
        const conversationId = crypto.randomUUID();
        const humanMessageId = crypto.randomUUID();
        const assistantMessageId = crypto.randomUUID();
        const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
        const locale = Intl.DateTimeFormat().resolvedOptions().locale || "en-US";

        try {
          const body = {
            prompt: String(requestPrompt || ""),
            model: requestedModel,
            timezone,
            personalized_styles: [{
              type: "default",
              key: "Default",
              name: "Normal",
              nameKey: "normal_style_name",
              prompt: "Normal\n",
              summary: "Default responses from Claude",
              summaryKey: "normal_style_summary",
              isDefault: true
            }],
            locale,
            tools: [],
            turn_message_uuids: {
              human_message_uuid: humanMessageId,
              assistant_message_uuid: assistantMessageId
            },
            attachments: [],
            effort: "medium",
            files: [],
            sync_sources: [],
            rendering_mode: "messages",
            thinking_mode: "auto",
            create_conversation_params: {
              name: "",
              model: requestedModel,
              include_conversation_preferences: true,
              paprika_mode: null,
              compass_mode: null,
              is_temporary: true,
              enabled_imagine: false,
              tool_search_mode: "auto"
            }
          };

          const response = await apiFetch(
            "/api/organizations/" + encodeURIComponent(organizationId) +
            "/chat_conversations/" + encodeURIComponent(conversationId) +
            "/completion",
            {
              method: "POST",
              accept: "text/event-stream",
              headers: { "Accept": "text/event-stream" },
              body: JSON.stringify(body)
            }
          );
          if (!response.ok) {
            const detail = await response.text();
            return { ok: false, status: response.status, error: detail.slice(0, 1500) };
          }
          const text = await readSSEText(response);
          if (!text) return { ok: false, error: "Claude Web returned an empty response." };
          return {
            ok: true,
            status: response.status,
            text,
            organizationId,
            organizationName: org.organizationName || ""
          };
        } catch (error) {
          return { ok: false, error: String(error?.message || error) };
        } finally {
          try {
            await apiFetch(
              "/api/organizations/" + encodeURIComponent(organizationId) +
              "/chat_conversations/" + encodeURIComponent(conversationId),
              { method: "DELETE" }
            );
          } catch {}
        }
      },
      args: [kind, prompt, DEFAULT_MODEL]
    });
    const result = results?.[0]?.result;
    if (!result || typeof result !== "object") {
      throw new Error("Claude Web page transport returned no result.");
    }
    return result;
  } finally {
    if (created && tab?.id) {
      try { await chrome.tabs.remove(tab.id); } catch {}
    }
  }
};

const execute = async (rawCommand) => {
  const command = validateCommand(rawCommand);
  const result = await withClaudePage(command.kind, command.prompt);
  return { id: command.id, ...result };
};

chrome.runtime.onMessageExternal.addListener((message, sender, sendResponse) => {
  const senderOrigin = localSenderOrigin(sender);
  if (!senderOrigin) {
    sendResponse({ ok: false, connected: false, stage: "extension-rejected-sender", error: "TL Studio pairing request did not come from localhost." });
    return;
  }

  if (message?.type === "tlstudio-ping") {
    sendResponse({
      ok: true,
      connected: false,
      stage: "extension-reached",
      extensionVersion: chrome.runtime.getManifest().version,
      bridgeVersion: BRIDGE_VERSION
    });
    return;
  }

  if (message?.type === "tlstudio-pair-direct") {
    const token = String(message?.token || "").trim();
    const origin = String(message?.origin || "").trim();
    if (token.length < 32 || origin !== senderOrigin) {
      sendResponse({ ok: false, connected: false, stage: "extension-rejected-pairing", error: "Claude Web bridge pairing data is invalid." });
      return;
    }
    void withClaudePage("probe", "").then((probe) => {
      if (probe?.ok && probe?.connected) {
        pairedToken = token;
        pairedOrigin = senderOrigin;
      }
      sendResponse({
        ok: !!probe?.ok,
        connected: !!probe?.connected,
        status: Number(probe?.status || 0),
        stage: probe?.connected ? "claude-session-ready" : "claude-session-probe-failed",
        error: String(probe?.error || ""),
        extensionVersion: chrome.runtime.getManifest().version,
        bridgeVersion: BRIDGE_VERSION
      });
    }, (error) => {
      sendResponse({
        ok: false,
        connected: false,
        stage: "claude-session-probe-failed",
        error: String(error?.message || error),
        extensionVersion: chrome.runtime.getManifest().version,
        bridgeVersion: BRIDGE_VERSION
      });
    });
    return true;
  }

  if (message?.type === "tlstudio-execute-direct") {
    const token = String(message?.token || "").trim();
    if (!pairedToken || token !== pairedToken || senderOrigin !== pairedOrigin) {
      sendResponse({ id: String(message?.command?.id || ""), ok: false, error: "Claude Web bridge is not paired with this TL Studio origin." });
      return;
    }
    void execute(message.command || {}).then(sendResponse, (error) => {
      sendResponse({ id: String(message?.command?.id || ""), ok: false, error: String(error?.message || error) });
    });
    return true;
  }

  sendResponse({ ok: false, error: "Unsupported TL Studio Claude Web bridge message." });
});
