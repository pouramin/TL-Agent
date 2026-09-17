"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");

const repoRoot = path.resolve(__dirname, "..");
const source = fs.readFileSync(path.join(repoRoot, "cmd", "launcher", "web", "runtime-api.js"), "utf8");
const calls = [];
const K = {
  state: { local: { project: "C:\\Projects\\demo" } },
  request: async (path, options = {}) => {
    calls.push({ path, options });
    if (path.includes("/config/overlay") && (!options.method || options.method === "GET")) {
      return { effective: { provider: {} } };
    }
    return true;
  },
};

const context = vm.createContext({
  window: { KLU: K },
  console,
  URLSearchParams,
  JSON,
  Object,
  encodeURIComponent,
  EventSource: class {},
});
vm.runInContext(source, context, { filename: "runtime-api.js" });

async function main() {
  assert.ok(K.api?.config?.overlay);
  assert.ok(K.api?.config?.update);
  assert.ok(K.api?.auth?.setApiKey);

  await K.api.config.overlay({ scope: "global" });
  assert.match(calls.at(-1).path, /^\/runtime\/config\/overlay\?/);
  assert.match(calls.at(-1).path, /scope=global/);
  assert.match(calls.at(-1).path, /directory=C%3A%5CProjects%5Cdemo/);

  await K.api.config.update({
    scope: "global",
    set: {
      provider: {
        agentrouter: {
          name: "AgentRouter",
          npm: "@ai-sdk/openai-compatible",
          options: { baseURL: "https://co.agentrouter.org/v1" },
          models: { "deepseek-v4-flash": { name: "DeepSeek V4 Flash" } },
        },
      },
    },
  });
  const patch = calls.at(-1);
  assert.equal(patch.path.startsWith("/runtime/config/overlay?"), true);
  assert.equal(patch.options.method, "PATCH");
  const patchBody = JSON.parse(patch.options.body);
  assert.equal(patchBody.scope, "global");
  assert.equal(patchBody.set.provider.agentrouter.npm, "@ai-sdk/openai-compatible");
  assert.equal("apiKey" in patchBody.set.provider.agentrouter.options, false);

  await K.api.auth.setApiKey("agentrouter", "secret-key");
  const auth = calls.at(-1);
  assert.equal(auth.path, "/runtime/auth/agentrouter");
  assert.equal(auth.options.method, "PUT");
  assert.deepEqual(JSON.parse(auth.options.body), { type: "api", key: "secret-key" });

  await K.api.auth.remove("agentrouter");
  const remove = calls.at(-1);
  assert.equal(remove.path, "/runtime/auth/agentrouter");
  assert.equal(remove.options.method, "DELETE");

  console.log("provider API adapter regressions: ok");
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
