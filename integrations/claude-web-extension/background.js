"use strict";

const CLAUDE_API = "https://claude.ai/api";
const DEFAULT_MODEL = "claude-sonnet-5-5";
const BRIDGE_VERSION = "0.6.2-background-fetch";
const MAX_PROMPT_BYTES = 8 * 1024 * 1024;
const MODEL_CATALOG = [
  "claude-fable-5-1",
  "claude-opus-5-5",
  "claude-sonnet-5-5",
  "claude-haiku-4-5"
];
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
  const model = String(command?.model || DEFAULT_MODEL).trim() || DEFAULT_MODEL;
  if (kind === "complete" && !MODEL_CATALOG.includes(model)) {
    throw new Error("Unsupported Claude Web model: " + model);
  }
  if (new TextEncoder().encode(prompt).byteLength > MAX_PROMPT_BYTES) {
    throw new Error("Claude Web transport prompt is too large.");
  }
  return { id, kind, prompt, model };
};

const claudeFetch = async (path, options = {}) => {
  return await fetch(CLAUDE_API + path, {
    cache: "no-store",
    credentials: "include",
    ...options,
    headers: {
      "Accept": options.accept || "application/json",
      "Content-Type": "application/json",
      "Cache-Control": "no-cache",
      "Pragma": "no-cache",
      "anthropic-client-platform": "web_claude_ai",
      ...(options.headers || {})
    }
  });
};

const probeSession = async () => {
  try {
    const response = await claudeFetch("/organizations", { method: "GET" });
    const text = await response.text();
    if (!response.ok) {
      return {
        ok: false,
        connected: false,
        status: response.status,
        models: MODEL_CATALOG,
        error: text.slice(0, 1000)
      };
    }
    let organizations;
    try {
      organizations = JSON.parse(text);
    } catch {
      return {
        ok: false,
        connected: false,
        status: response.status,
        models: MODEL_CATALOG,
        error: "Claude Web returned invalid organization data."
      };
    }
    const org = Array.isArray(organizations) && organizations.length ? organizations[0] : null;
    const organizationId = String(org?.uuid || org?.id || "").trim();
    if (!organizationId) {
      return {
        ok: false,
        connected: false,
        status: response.status,
        models: MODEL_CATALOG,
        error: "Claude organization was not found."
      };
    }
    return {
      ok: true,
      connected: true,
      status: response.status,
      organizationId,
      organizationName: org?.name ? String(org.name) : "",
      models: MODEL_CATALOG
    };
  } catch (error) {
    return {
      ok: false,
      connected: false,
      models: MODEL_CATALOG,
      error: String(error?.message || error)
    };
  }
};

const readSSEText = async (response) => {
  if (!response.body) throw new Error("Claude Web returned no response body.");
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  let dataLines = [];
  let output = "";
  let routedModel = "";

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

    const eventModel = String(
      event.model ||
      event.message?.model ||
      event.delta?.model ||
      ""
    ).trim();
    if (eventModel) routedModel = eventModel;

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
    return { text: output.trim(), model: routedModel };
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
  return { text: output.trim(), model: routedModel };
};

const complete = async (prompt, requestedModel) => {
  const org = await probeSession();
  if (!org.ok || !org.connected || !org.organizationId) return org;

  const organizationId = org.organizationId;
  const conversationId = crypto.randomUUID();
  const humanMessageId = crypto.randomUUID();
  const assistantMessageId = crypto.randomUUID();
  const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
  const locale = Intl.DateTimeFormat().resolvedOptions().locale || "en-US";
  let routedModel = "";

  try {
    const body = {
      prompt: String(prompt || ""),
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

    const response = await claudeFetch(
      "/organizations/" + encodeURIComponent(organizationId) +
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
      return {
        ok: false,
        status: response.status,
        error: detail.slice(0, 1500),
        models: MODEL_CATALOG
      };
    }

    const stream = await readSSEText(response);
    routedModel = String(stream.model || "").trim();
    if (!stream.text) {
      return { ok: false, error: "Claude Web returned an empty response.", models: MODEL_CATALOG };
    }

    try {
      const detailsResponse = await claudeFetch(
        "/organizations/" + encodeURIComponent(organizationId) +
        "/chat_conversations/" + encodeURIComponent(conversationId) +
        "?tree=true&rendering_mode=messages&render_all_tools=true",
        { method: "GET" }
      );
      if (detailsResponse.ok) {
        const details = await detailsResponse.json();
        const recordedModel = String(details?.model || "").trim();
        if (recordedModel) routedModel = recordedModel;
      }
    } catch {}

    return {
      ok: true,
      status: response.status,
      text: stream.text,
      model: routedModel || requestedModel,
      organizationId,
      organizationName: org.organizationName || "",
      models: MODEL_CATALOG
    };
  } catch (error) {
    return {
      ok: false,
      error: String(error?.message || error),
      models: MODEL_CATALOG
    };
  } finally {
    try {
      await claudeFetch(
        "/organizations/" + encodeURIComponent(organizationId) +
        "/chat_conversations/" + encodeURIComponent(conversationId),
        { method: "DELETE" }
      );
    } catch {}
  }
};

const execute = async (rawCommand) => {
  const command = validateCommand(rawCommand);
  if (command.kind === "probe") {
    return { id: command.id, ...(await probeSession()) };
  }
  return {
    id: command.id,
    ...(await complete(command.prompt, command.model))
  };
};

chrome.runtime.onMessageExternal.addListener((message, sender, sendResponse) => {
  const senderOrigin = localSenderOrigin(sender);
  if (!senderOrigin) {
    sendResponse({
      ok: false,
      connected: false,
      stage: "extension-rejected-sender",
      error: "TL Studio pairing request did not come from localhost."
    });
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
      sendResponse({
        ok: false,
        connected: false,
        stage: "extension-rejected-pairing",
        error: "Claude Web bridge pairing data is invalid."
      });
      return;
    }

    void probeSession().then((probe) => {
      if (probe?.ok && probe?.connected) {
        pairedToken = token;
        pairedOrigin = senderOrigin;
      }
      sendResponse({
        ...probe,
        stage: probe?.connected ? "claude-session-ready" : "claude-session-probe-failed",
        extensionVersion: chrome.runtime.getManifest().version,
        bridgeVersion: BRIDGE_VERSION
      });
    }, (error) => {
      sendResponse({
        ok: false,
        connected: false,
        stage: "claude-session-probe-failed",
        error: String(error?.message || error),
        models: MODEL_CATALOG,
        extensionVersion: chrome.runtime.getManifest().version,
        bridgeVersion: BRIDGE_VERSION
      });
    });
    return true;
  }

  if (message?.type === "tlstudio-execute-direct") {
    const token = String(message?.token || "").trim();
    if (!pairedToken || token !== pairedToken || senderOrigin !== pairedOrigin) {
      sendResponse({
        id: String(message?.command?.id || ""),
        ok: false,
        error: "Claude Web bridge is not paired with this TL Studio origin."
      });
      return;
    }

    void execute(message.command || {}).then(sendResponse, (error) => {
      sendResponse({
        id: String(message?.command?.id || ""),
        ok: false,
        error: String(error?.message || error)
      });
    });
    return true;
  }

  sendResponse({ ok: false, error: "Unsupported TL Studio Claude Web bridge message." });
});
