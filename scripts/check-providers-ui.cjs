"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");

const repoRoot = path.resolve(__dirname, "..");
const source = fs.readFileSync(path.join(repoRoot, "cmd", "launcher", "web", "providers-ui.js"), "utf8");

const K = { __providersUiInstalled: false };
const context = vm.createContext({
  window: { KLU: K },
  document: { getElementById: () => null },
  console,
  URL,
  Object,
  Number,
  String,
  Set,
});
vm.runInContext(source, context, { filename: "providers-ui.js" });

const hooks = K.__providersUi;
assert.ok(hooks, "provider UI hooks should be installed even when settings DOM is unavailable");
assert.equal(hooks.PACKAGES["openai-compatible"], "@ai-sdk/openai-compatible");
assert.equal(hooks.AGENTROUTER_PRESET.providerID, "agentrouter");
assert.equal(hooks.AGENTROUTER_PRESET.baseURL, "https://co.agentrouter.org/v1");
assert.equal(hooks.AGENTROUTER_PRESET.modelID, "deepseek-v4-flash");
assert.equal(hooks.validateDraft(hooks.AGENTROUTER_PRESET), "");

const config = hooks.buildProviderConfig({
  ...hooks.AGENTROUTER_PRESET,
  contextLimit: "128000",
  outputLimit: "16384",
  apiKey: "super-secret-should-never-enter-config",
});
assert.equal(config.name, "AgentRouter");
assert.equal(config.npm, "@ai-sdk/openai-compatible");
assert.equal(config.options.baseURL, "https://co.agentrouter.org/v1");
assert.equal(config.models["deepseek-v4-flash"].name, "DeepSeek V4 Flash");
assert.equal(config.models["deepseek-v4-flash"].tool_call, true);
assert.equal(config.models["deepseek-v4-flash"].reasoning, true);
assert.equal(config.models["deepseek-v4-flash"].limit.context, 128000);
assert.equal(config.models["deepseek-v4-flash"].limit.output, 16384);
assert.equal(JSON.stringify(config).includes("super-secret"), false, "API keys must not be written into provider config");
assert.equal("apiKey" in config.options, false, "provider options must not contain the API key");

const existing = {
  name: "Existing",
  npm: "@ai-sdk/openai-compatible",
  options: { baseURL: "https://old.example/v1", headers: { "X-Test": "yes" } },
  models: {
    "other-model": { name: "Other", tool_call: true },
  },
};
const merged = hooks.buildProviderConfig({
  providerID: "agentrouter",
  name: "AgentRouter",
  protocol: "openai-compatible",
  baseURL: "https://co.agentrouter.org/v1/",
  modelID: "deepseek-v4-flash",
  modelName: "DeepSeek V4 Flash",
  toolCall: true,
  reasoning: false,
  contextLimit: "",
  outputLimit: "",
}, existing);
assert.equal(merged.options.baseURL, "https://co.agentrouter.org/v1");
assert.equal(merged.options.headers["X-Test"], "yes", "editing must preserve unrelated provider options");
assert.ok(merged.models["other-model"], "editing one model must preserve other configured models");
assert.ok(merged.models["deepseek-v4-flash"]);

const entries = hooks.customProviderEntries({
  effective: {
    provider: {
      agentrouter: config,
      anthropic: { name: "Anthropic" },
      customResponses: { name: "Responses", npm: "@ai-sdk/openai", models: {} },
      unsupported: { name: "Unsupported", npm: "some-other-package", models: {} },
    },
  },
});
assert.deepEqual(Array.from(entries, (entry) => entry.id).sort(), ["agentrouter", "customResponses"]);

assert.match(hooks.validateDraft({ ...hooks.AGENTROUTER_PRESET, providerID: "Bad ID" }), /Provider ID/);
assert.match(hooks.validateDraft({ ...hooks.AGENTROUTER_PRESET, baseURL: "not-a-url" }), /Base URL/);
assert.match(hooks.validateDraft({ ...hooks.AGENTROUTER_PRESET, modelID: "" }), /Model ID/);
assert.match(hooks.validateDraft({ ...hooks.AGENTROUTER_PRESET, contextLimit: "12.5" }), /Context limit/);

console.log("custom provider UI regressions: ok");
