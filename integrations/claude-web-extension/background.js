"use strict";

const CLAUDE_API = "https://claude.ai/api";
const SESSION_RULE_ID = 42001;
const DEFAULT_MODEL = "claude-sonnet-5-5";

const validLocalOrigin = (value) => {
  try {
    const url = new URL(String(value || ""));
    return url.protocol === "http:" &&
      (url.hostname === "127.0.0.1" || url.hostname === "localhost");
  } catch {
    return false;
  }
};

const validExternalSender = (sender) => {
  try {
    const raw = String(sender?.url || sender?.origin || "");
    const url = new URL(raw);
    return url.protocol === "http:" &&
      (url.hostname === "127.0.0.1" || url.hostname === "localhost");
  } catch {
    return false;
  }
};

const cookieHeader = async () => {
  const cookies = await chrome.cookies.getAll({ url: "https://claude.ai/api/" });
  const applicable = cookies
    .filter((cookie) => cookie.name && typeof cookie.value === "string")
    .sort((a, b) => String(b.path || "").length - String(a.path || "").length);

  if (!applicable.some((cookie) => cookie.name === "sessionKey" && cookie.value)) {
    throw new Error("Claude is not signed in in this Chrome profile.");
  }
  return applicable.map((cookie) => cookie.name + "=" + cookie.value).join("; ");
};

const installRequestRule = async () => {
  const cookie = await cookieHeader();
  await chrome.declarativeNetRequest.updateSessionRules({
    removeRuleIds: [SESSION_RULE_ID],
    addRules: [{
      id: SESSION_RULE_ID,
      priority: 1000,
      action: {
        type: "modifyHeaders",
        requestHeaders: [
          { header: "cookie", operation: "set", value: cookie },
          { header: "origin", operation: "set", value: "https://claude.ai" },
          { header: "referer", operation: "set", value: "https://claude.ai/new" },
          { header: "anthropic-client-platform", operation: "set", value: "web_claude_ai" }
        ]
      },
      condition: {
        regexFilter: "^https://claude\\.ai/api/",
        resourceTypes: ["xmlhttprequest"]
      }
    }]
  });
};

const removeRequestRule = async () => {
  try {
    await chrome.declarativeNetRequest.updateSessionRules({
      removeRuleIds: [SESSION_RULE_ID]
    });
  } catch {}
};

const claudeFetch = async (path, options = {}) => {
  await installRequestRule();
  try {
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
  } finally {
    await removeRequestRule();
  }
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
        error: "Claude Web returned invalid organization data."
      };
    }

    const org = Array.isArray(organizations) && organizations.length
      ? organizations[0]
      : null;
    const organizationId = String(org?.uuid || org?.id || "").trim();
    if (!organizationId) {
      return { ok: false, connected: false, error: "Claude organization was not found." };
    }

    const result = {
      ok: true,
      connected: true,
      status: response.status,
      organizationId,
      organizationName: org?.name ? String(org.name) : ""
    };
    await chrome.storage.local.set({
      claudeOrganizationId: result.organizationId,
      claudeOrganizationName: result.organizationName
    });
    return result;
  } catch (error) {
    return {
      ok: false,
      connected: false,
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

  const consumeEvent = () => {
    if (!dataLines.length) return false;
    const data = dataLines.join("\n");
    dataLines = [];
    if (!data || data === "[DONE]") return data === "[DONE]";

    let event;
    try {
      event = JSON.parse(data);
    } catch {
      return false;
    }

    if (event.type === "error") {
      throw new Error(
        String(event.message || event.error?.message || "Claude Web stream error")
      );
    }

    // Claude Web has used both legacy completion events and the newer
    // content-block stream shape. The legacy event does not always carry
    // type="completion", so key off the payload itself.
    if (typeof event.completion === "string") {
      output += event.completion;
    }

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

const resolveOrganization = async () => {
  const stored = await chrome.storage.local.get([
    "claudeOrganizationId",
    "claudeOrganizationName"
  ]);

  if (stored.claudeOrganizationId) {
    return {
      ok: true,
      connected: true,
      organizationId: String(stored.claudeOrganizationId),
      organizationName: String(stored.claudeOrganizationName || "")
    };
  }
  return probeSession();
};

const complete = async (prompt) => {
  try {
    let org = await resolveOrganization();
    if (!org.ok || !org.connected || !org.organizationId) {
      org = await probeSession();
    }
    if (!org.ok || !org.connected || !org.organizationId) return org;

    const organizationId = org.organizationId;
    const conversationId = crypto.randomUUID();
    const humanMessageId = crypto.randomUUID();
    const assistantMessageId = crypto.randomUUID();
    const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
    const locale = Intl.DateTimeFormat().resolvedOptions().locale || "en-US";

    const body = {
      prompt: String(prompt || ""),
      model: DEFAULT_MODEL,
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
        model: DEFAULT_MODEL,
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
      if ([401, 403, 404].includes(response.status)) {
        await chrome.storage.local.remove([
          "claudeOrganizationId",
          "claudeOrganizationName"
        ]);
      }
      return {
        ok: false,
        status: response.status,
        error: detail.slice(0, 1500)
      };
    }

    const text = await readSSEText(response);
    if (!text) {
      return { ok: false, error: "Claude Web returned an empty response." };
    }

    return {
      ok: true,
      status: response.status,
      text,
      organizationId,
      organizationName: org.organizationName || ""
    };
  } catch (error) {
    return { ok: false, error: String(error?.message || error) };
  }
};

const execute = async (command) => {
  if (command.kind === "probe") {
    return { id: command.id, ...(await probeSession()) };
  }
  if (command.kind === "complete") {
    return { id: command.id, ...(await complete(command.prompt)) };
  }
  return {
    id: command.id,
    ok: false,
    error: "Unsupported Claude Web bridge command."
  };
};

chrome.runtime.onMessageExternal.addListener((message, sender, sendResponse) => {
  if (!validExternalSender(sender)) {
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
      extensionVersion: chrome.runtime.getManifest().version
    });
    return;
  }

  if (message?.type === "tlstudio-pair-direct") {
    void probeSession().then((probe) => {
      sendResponse({
        ...probe,
        stage: probe?.connected ? "claude-session-ready" : "claude-session-probe-failed",
        extensionVersion: chrome.runtime.getManifest().version
      });
    }, (error) => {
      sendResponse({
        ok: false,
        connected: false,
        stage: "claude-session-probe-failed",
        error: String(error?.message || error),
        extensionVersion: chrome.runtime.getManifest().version
      });
    });
    return true;
  }

  if (message?.type === "tlstudio-execute-direct") {
    void execute(message.command || {}).then(sendResponse, (error) => {
      sendResponse({
        id: String(message?.command?.id || ""),
        ok: false,
        error: String(error?.message || error)
      });
    });
    return true;
  }
});
