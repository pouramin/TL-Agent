"use strict";

const assert = require("node:assert/strict");
const vm = require("node:vm");
const { loadBrowserModule } = require("./browser-source-harness.cjs");

const source = loadBrowserModule("runtime-api.ts");
const calls = [];
const K = {
  state: { local: { project: "C:\\Projects\\demo" } },
  request: async (path, options = {}) => {
    calls.push({ path, options });
    if (path.startsWith("/local/providers/catalog")) {
      return {
        all: [{ id: "agentrouter", name: "AgentRouter", models: { "deepseek-v4-flash": { name: "DeepSeek V4 Flash" } } }],
        connected: ["agentrouter"],
        default: { agentrouter: "deepseek-v4-flash" },
        failed: [],
      };
    }
    if (path === "/local/providers/config" && (!options.method || options.method === "GET")) {
      return { providers: [] };
    }
    if (path.startsWith("/local/provider-accounts")) return [];
    return { ok: true };
  },
};

const context = vm.createContext({
  K,
  window: {},
  console,
  URLSearchParams,
  JSON,
  Object,
  encodeURIComponent,
  EventSource: class {},
});
vm.runInContext(source, context, { filename: "runtime-api.ts" });

async function main() {
  assert.ok(K.api?.providerAccounts?.list);
  assert.ok(K.api?.providerAccounts?.status);
  assert.ok(K.api?.providerAccounts?.authorize);
  assert.ok(K.api?.providerAccounts?.complete);
  assert.ok(K.api?.providerAccounts?.callback);
  assert.ok(K.api?.providerAccounts?.cancel);
  assert.ok(K.api?.providerAccounts?.refresh);
  assert.ok(K.api?.providerAccounts?.models);
  assert.ok(K.api?.providerAccounts?.disconnect);
  assert.ok(K.api?.providers?.config);
  assert.ok(K.api?.providers?.upsert);
  assert.ok(K.api?.providers?.remove);
  assert.ok(K.api?.providers?.discover);

  const state = await K.api.providerState();
  assert.equal(state.all[0].id, "agentrouter");
  assert.equal(state.connected.has("agentrouter"), true);
  assert.equal(state.defaults.agentrouter, "deepseek-v4-flash");
  assert.match(calls.at(-1).path, /^\/local\/providers\/catalog\?/);
  assert.match(calls.at(-1).path, /directory=C%3A%5CProjects%5Cdemo/);

  const accounts = await K.api.providerAccounts.list();
  assert.deepEqual(Array.from(accounts), []);
  assert.match(calls.at(-1).path, /^\/local\/provider-accounts\?/);

  await K.api.providers.config();
  assert.equal(calls.at(-1).path, "/local/providers/config");

  const provider = {
    id: "agentrouter",
    name: "AgentRouter",
    protocol: "openai-compatible",
    baseURL: "https://co.agentrouter.org/v1",
    models: [{ id: "deepseek-v4-flash", name: "DeepSeek V4 Flash", toolCall: true, reasoning: false }],
  };
  await K.api.providers.upsert("agentrouter", { provider, apiKey: "secret-key" });
  const put = calls.at(-1);
  assert.equal(put.path, "/local/providers/config/agentrouter");
  assert.equal(put.options.method, "PUT");
  const putBody = JSON.parse(put.options.body);
  assert.deepEqual(putBody.provider, provider);
  assert.equal(putBody.apiKey, "secret-key");
  assert.equal(JSON.stringify(putBody).includes("@ai-sdk"), false);

  await K.api.providers.remove("agentrouter");
  const remove = calls.at(-1);
  assert.equal(remove.path, "/local/providers/config/agentrouter");
  assert.equal(remove.options.method, "DELETE");

  await K.api.providers.discover({
    providerID: "agentrouter",
    protocol: "openai-compatible",
    baseURL: provider.baseURL,
    apiKey: "secret-key",
  });
  const discover = calls.at(-1);
  assert.equal(discover.path, "/local/providers/discover");
  assert.equal(discover.options.method, "POST");

  for (const forbidden of [
    "/runtime/",
    "/config/overlay",
    "/provider/kilo/",
    "/kilo/auth-status",
    "kilo-auto/free",
    "providerID: \"kilo\"",
    "@ai-sdk/",
  ]) {
    assert.equal(source.includes(forbidden), false, `runtime-api.ts leaked removed implementation detail: ${forbidden}`);
  }

  console.log("provider API adapter regressions: ok");
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
