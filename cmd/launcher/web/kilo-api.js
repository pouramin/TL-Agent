(() => {
  "use strict";

  const K = window.KLU;

  const enc = encodeURIComponent;
  const json = (value) => JSON.stringify(value);
  const body = (value) => ({ body: json(value) });
  const unwrapData = (payload) => payload && typeof payload === "object" && "data" in payload ? payload.data : payload;

  const request = (path, options) => K.request(`/kilo${path}`, options);

  const parseSSE = (handler) => (event) => {
    if (!event?.data) return;
    try { handler(JSON.parse(event.data)); }
    catch (error) { console.warn("[Kilo Local UI] Ignoring invalid SSE payload", error); }
  };

  const openEventSource = (path, { onEvent, onOpen, onError } = {}) => {
    const source = new EventSource(`/kilo${path}`);
    if (onOpen) source.addEventListener("open", onOpen);
    if (onError) source.addEventListener("error", onError);
    if (onEvent) source.addEventListener("message", parseSSE(onEvent));
    return source;
  };

  const pageQuery = ({ order, limit, cursor }) => {
    const query = new URLSearchParams();
    if (cursor) query.set("cursor", cursor);
    else if (order) query.set("order", order);
    if (limit !== undefined) query.set("limit", String(limit));
    return query;
  };

  K.api = Object.freeze({
    version: "kilo-v7.6.2-protocol-v2",

    health: () => request("/api/health"),
    location: () => request("/api/location"),

    agents: async () => {
      const payload = await request("/api/agent");
      return Array.isArray(payload?.data) ? payload.data : [];
    },

    models: async () => {
      const payload = await request("/api/model");
      return Array.isArray(payload?.data) ? payload.data : [];
    },

    providers: async () => {
      const payload = await request("/api/provider");
      return Array.isArray(payload?.data) ? payload.data : [];
    },

    // This route is part of Kilo's official provider HttpApi used by its own clients.
    // It is intentionally isolated here because connection/default state is not exposed
    // by the public Protocol v2 Provider.Info schema.
    providerRuntimeState: async () => {
      const payload = unwrapData(await request("/provider"));
      return {
        connected: new Set(Array.isArray(payload?.connected) ? payload.connected : []),
        defaults: payload?.default && typeof payload.default === "object" ? payload.default : {},
      };
    },

    sessions: {
      list: ({ order = "desc", limit = 50, cursor } = {}) => {
        const query = pageQuery({ order, limit, cursor });
        return request(`/api/session?${query}`);
      },
      active: () => request("/api/session/active"),
      get: (sessionID) => request(`/api/session/${enc(sessionID)}`),
      create: (input = {}) => request("/api/session", { method: "POST", ...body(input) }),
      switchAgent: (sessionID, agent) => request(`/api/session/${enc(sessionID)}/agent`, {
        method: "POST", ...body({ agent }),
      }),
      switchModel: (sessionID, model) => request(`/api/session/${enc(sessionID)}/model`, {
        method: "POST", ...body({ model }),
      }),
      prompt: (sessionID, prompt, options = {}) => request(`/api/session/${enc(sessionID)}/prompt`, {
        method: "POST",
        ...body({ prompt, ...(options.delivery ? { delivery: options.delivery } : {}), ...(options.resume === undefined ? {} : { resume: options.resume }) }),
      }),
      interrupt: (sessionID) => request(`/api/session/${enc(sessionID)}/interrupt`, { method: "POST" }),
      messages: (sessionID, { order = "asc", limit = 200, cursor } = {}) => {
        const query = pageQuery({ order, limit, cursor });
        return request(`/api/session/${enc(sessionID)}/message?${query}`);
      },
      message: (sessionID, messageID) => request(`/api/session/${enc(sessionID)}/message/${enc(messageID)}`),
      context: (sessionID) => request(`/api/session/${enc(sessionID)}/context`),
      history: (sessionID, { after, limit } = {}) => {
        const query = new URLSearchParams();
        if (after !== undefined) query.set("after", String(after));
        if (limit !== undefined) query.set("limit", String(limit));
        const suffix = query.size ? `?${query}` : "";
        return request(`/api/session/${enc(sessionID)}/history${suffix}`);
      },
      subscribeDurable: (sessionID, options = {}) => {
        const query = options.after === undefined ? "" : `?after=${enc(String(options.after))}`;
        return openEventSource(`/api/session/${enc(sessionID)}/event${query}`, options);
      },
    },

    permissions: {
      list: async (sessionID) => {
        const payload = await request(`/api/session/${enc(sessionID)}/permission`);
        return Array.isArray(payload?.data) ? payload.data : [];
      },
      get: (sessionID, requestID) => request(`/api/session/${enc(sessionID)}/permission/${enc(requestID)}`),
      reply: (sessionID, requestID, reply, message) => request(`/api/session/${enc(sessionID)}/permission/${enc(requestID)}/reply`, {
        method: "POST", ...body({ reply, ...(message ? { message } : {}) }),
      }),
    },

    questions: {
      list: async (sessionID) => {
        const payload = await request(`/api/session/${enc(sessionID)}/question`);
        return Array.isArray(payload?.data) ? payload.data : [];
      },
      reply: (sessionID, requestID, answers) => request(`/api/session/${enc(sessionID)}/question/${enc(requestID)}/reply`, {
        method: "POST", ...body({ answers }),
      }),
      reject: (sessionID, requestID) => request(`/api/session/${enc(sessionID)}/question/${enc(requestID)}/reject`, { method: "POST" }),
    },

    oauth: {
      authorizeKilo: async () => unwrapData(await request("/provider/kilo/oauth/authorize", { method: "POST", ...body({ method: 0 }) })),
      callbackKilo: async (signal) => unwrapData(await request("/provider/kilo/oauth/callback", { method: "POST", ...body({ method: 0 }), signal })),
    },

    events: {
      subscribe: (options = {}) => openEventSource("/api/event", options),
    },
  });
})();
