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
  const route = (path, params = {}) => {
    const query = new URLSearchParams();
    const directory = projectDirectory();
    if (directory) query.set("directory", directory);
    for (const [key, value] of Object.entries(params)) {
      if (value !== undefined && value !== null && value !== "") query.set(key, String(value));
    }
    return query.size ? `${path}?${query}` : path;
  };

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
      console.warn("[Kilo Local UI] Ignoring invalid SSE payload", error);
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
    // Kilo's generated @kilocode/sdk/v2/client exposes both the production
    // HttpApi and the experimental /api Protocol v2 surface. The official
    // VS Code client in v7.6.2 uses the production routes below for coding.
    version: "kilo-v7.6.2-production-httpapi",

    health: () => request("/global/health"),
    path: () => request(route("/path")),

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

    sessions: {
      list: async ({ limit = 50 } = {}) => {
        const payload = unwrapData(await request(route("/session", { limit })));
        return wrapData(Array.isArray(payload) ? payload : []);
      },
      status: async () => {
        const payload = unwrapData(await request(route("/session/status")));
        return wrapData(payload && typeof payload === "object" ? payload : {});
      },
      get: async (sessionID) => wrapData(unwrapData(await request(route(`/session/${enc(sessionID)}`)))),
      create: async (input = {}) => {
        // Product Session creation and prompt selection are separate concerns.
        // Official clients send the effective model again on promptAsync; doing
        // so also avoids validating a custom/test model before the first prompt.
        const payload = {};
        if (input.parentID) payload.parentID = input.parentID;
        if (input.title) payload.title = input.title;
        if (input.agent) payload.agent = input.agent;
        return wrapData(unwrapData(await request(route("/session"), { method: "POST", ...body(payload) })));
      },
      messages: async (sessionID, { limit = 200 } = {}) => {
        const payload = unwrapData(await request(route(`/session/${enc(sessionID)}/message`, { limit })));
        return wrapData(Array.isArray(payload) ? payload : []);
      },
      promptAsync: (sessionID, { text, agent, model, variant, messageID } = {}) => {
        const payload = {
          parts: [{ type: "text", text: text || "" }],
          ...(messageID ? { messageID } : {}),
          ...(agent ? { agent } : {}),
          ...(model ? { model: wireModel(model) } : {}),
          ...(variant ? { variant } : {}),
        };
        return request(route(`/session/${enc(sessionID)}/prompt_async`), { method: "POST", ...body(payload) });
      },
      abort: (sessionID) => request(route(`/session/${enc(sessionID)}/abort`), { method: "POST" }),
    },

    permissions: {
      list: async (sessionID) => {
        const payload = unwrapData(await request(route("/permission")));
        return (Array.isArray(payload) ? payload : []).filter((item) => !sessionID || item?.sessionID === sessionID);
      },
      reply: (sessionID, requestID, reply, message) => request(route(`/permission/${enc(requestID)}/reply`), {
        method: "POST",
        ...body({ reply, ...(message ? { message } : {}) }),
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
      authorizeKilo: async () => unwrapData(await request(route("/provider/kilo/oauth/authorize"), { method: "POST", ...body({ method: 0 }) })),
      callbackKilo: async (signal) => unwrapData(await request(route("/provider/kilo/oauth/callback"), { method: "POST", ...body({ method: 0 }), signal })),
    },

    events: {
      subscribe: (options = {}) => openEventSource("/global/event", options),
    },
  });
})();
