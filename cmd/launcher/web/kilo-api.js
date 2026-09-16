(() => {
  "use strict";

  const K = window.KLU;
  const enc = encodeURIComponent;
  const json = (value) => JSON.stringify(value);
  const body = (value) => ({ body: json(value) });
  const unwrapData = (payload) => payload && typeof payload === "object" && "data" in payload ? payload.data : payload;
  const request = (path, options) => K.request(`/kilo${path}`, options);
  const wrapData = (data) => ({ data });

  const projectDirectory = () => K.state?.local?.project || "";
  const withQuery = (path, params = {}) => {
    const query = new URLSearchParams();
    for (const [key, value] of Object.entries(params)) {
      if (value !== undefined && value !== null && value !== "") query.set(key, String(value));
    }
    return query.size ? `${path}?${query}` : path;
  };
  const route = (path, params = {}, directory = projectDirectory()) => withQuery(path, {
    ...(directory ? { directory } : {}),
    ...params,
  });

  const wireModel = (model) => model ? {
    providerID: model.providerID,
    modelID: model.modelID || model.id,
  } : undefined;

  const parseSSE = (handler) => (event) => {
    if (!event?.data) return;
    try {
      const decoded = JSON.parse(event.data);
      handler(decoded?.payload || decoded);
    } catch (error) {
      console.warn("[TL Agent] Ignoring invalid SSE payload", error);
    }
  };

  const openEventSource = (path, { onEvent, onOpen, onError } = {}) => {
    const source = new EventSource(`/kilo${route(path)}`);
    if (onOpen) source.addEventListener("open", onOpen);
    if (onError) source.addEventListener("error", onError);
    if (onEvent) source.addEventListener("message", parseSSE(onEvent));
    return source;
  };

  K.api = Object.freeze({
    version: "kilo-v7.6.2-production-httpapi",

    health: () => request("/global/health"),
    path: () => request(route("/path")),

    runtime: {
      dispose: async () => unwrapData(await request("/global/dispose", { method: "POST" })),
    },

    agents: async () => {
      const payload = unwrapData(await request(route("/agent")));
      return Array.isArray(payload) ? payload : [];
    },

    providerState: async () => {
      const payload = unwrapData(await request(route("/provider"))) || {};
      return {
        all: Array.isArray(payload.all) ? payload.all : [],
        connected: new Set(Array.isArray(payload.connected) ? payload.connected : []),
        defaults: payload.default && typeof payload.default === "object" ? payload.default : {},
        failed: Array.isArray(payload.failed) ? payload.failed : [],
      };
    },

    config: {
      overlay: async ({ scope = "global", directory = projectDirectory() } = {}) => {
        const payload = unwrapData(await request(route("/config/overlay", { scope }, directory)));
        return payload && typeof payload === "object" ? payload : {};
      },
      update: async ({ scope = "global", set, unset, directory = projectDirectory() } = {}) => {
        const payload = {
          scope,
          ...(set && Object.keys(set).length ? { set } : {}),
          ...(Array.isArray(unset) && unset.length ? { unset } : {}),
        };
        return unwrapData(await request(route("/config/overlay", {}, directory), {
          method: "PATCH",
          ...body(payload),
        }));
      },
    },

    auth: {
      setApiKey: async (providerID, key) => unwrapData(await request(`/auth/${enc(providerID)}`, {
        method: "PUT",
        ...body({ type: "api", key }),
      })),
      remove: async (providerID) => unwrapData(await request(`/auth/${enc(providerID)}`, { method: "DELETE" })),
    },

    sessions: {
      list: async ({ limit = 50, directory = projectDirectory() } = {}) => {
        const payload = unwrapData(await request(route("/session", { limit, roots: true }, directory)));
        return wrapData(Array.isArray(payload) ? payload : []);
      },
      status: async () => {
        const payload = unwrapData(await request(route("/session/status")));
        return wrapData(payload && typeof payload === "object" ? payload : {});
      },
      get: async (sessionID, { directory } = {}) => wrapData(unwrapData(await request(route(`/session/${enc(sessionID)}`, {}, directory)))),
      create: async (input = {}) => {
        const payload = {};
        if (input.parentID) payload.parentID = input.parentID;
        if (input.title) payload.title = input.title;
        return wrapData(unwrapData(await request(route("/session"), { method: "POST", ...body(payload) })));
      },
      update: async (sessionID, input = {}, { directory } = {}) => wrapData(unwrapData(await request(route(`/session/${enc(sessionID)}`, {}, directory), {
        method: "PATCH", ...body(input),
      }))),
      remove: async (sessionID, { directory } = {}) => wrapData(unwrapData(await request(route(`/session/${enc(sessionID)}`, {}, directory), { method: "DELETE" }))),
      diff: async (sessionID, { messageID, full, file, directory } = {}) => {
        const payload = unwrapData(await request(route(`/session/${enc(sessionID)}/diff`, { messageID, full, file }, directory)));
        return wrapData(Array.isArray(payload) ? payload : []);
      },
      messages: async (sessionID, { limit = 200, directory } = {}) => {
        const payload = unwrapData(await request(route(`/session/${enc(sessionID)}/message`, { limit }, directory)));
        return wrapData(Array.isArray(payload) ? payload : []);
      },
      promptAsync: (sessionID, { text, parts, agent, model, variant, messageID, directory } = {}) => {
        const payload = {
          parts: Array.isArray(parts) && parts.length ? parts : [{ type: "text", text: text || "" }],
          ...(messageID ? { messageID } : {}),
          ...(agent ? { agent } : {}),
          ...(model ? { model: wireModel(model) } : {}),
          ...(variant ? { variant } : {}),
        };
        return request(route(`/session/${enc(sessionID)}/prompt_async`, {}, directory), { method: "POST", ...body(payload) });
      },
      abort: (sessionID, { scope, directory } = {}) => request(route(`/session/${enc(sessionID)}/abort`, { scope }, directory), { method: "POST" }),
    },

    permissions: {
      list: async (sessionID) => {
        const payload = unwrapData(await request(route("/permission")));
        return (Array.isArray(payload) ? payload : []).filter((item) => !sessionID || item?.sessionID === sessionID);
      },
      // Every reply through this browser adapter is the result of an explicit human click.
      // Kilo 7.6.2 requires `interactive: true` for sensitive permission classes such as
      // skill-shell and sandbox-escalation requests; otherwise an approval is intentionally ignored.
      reply: (sessionID, requestID, reply, message) => request(route(`/permission/${enc(requestID)}/reply`), {
        method: "POST",
        ...body({ reply, interactive: true, ...(message ? { message } : {}) }),
      }),
    },

    questions: {
      list: async (sessionID) => {
        const payload = unwrapData(await request(route("/question")));
        return (Array.isArray(payload) ? payload : []).filter((item) => !sessionID || item?.sessionID === sessionID);
      },
      reply: (sessionID, requestID, answers) => request(route(`/question/${enc(requestID)}/reply`), {
        method: "POST", ...body({ answers }),
      }),
      reject: (sessionID, requestID) => request(route(`/question/${enc(requestID)}/reject`), { method: "POST" }),
    },

    oauth: {
      kiloStatus: async () => {
        const payload = unwrapData(await request(route("/kilo/auth-status"))) || {};
        return {
          authenticated: payload.authenticated === true,
          type: payload.type || "",
          organizationId: payload.organizationId || "",
        };
      },
      authorizeKilo: async () => unwrapData(await request(route("/provider/kilo/oauth/authorize"), { method: "POST", ...body({ method: 0 }) })),
      callbackKilo: async (signal) => unwrapData(await request(route("/provider/kilo/oauth/callback"), { method: "POST", ...body({ method: 0 }), signal })),
      disconnectKilo: async () => unwrapData(await request("/auth/kilo", { method: "DELETE" })),
    },

    events: {
      subscribe: (options = {}) => openEventSource("/global/event", options),
    },
  });
})();
