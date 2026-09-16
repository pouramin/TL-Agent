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
assert.equal("AGENTROUTER_PRESET" in hooks, false, "provider settings must remain vendor-agnostic");
assert.equal(source.includes("AgentRouter"), false, "provider UI must not hard-code a provider brand");
assert.equal(source.includes("deepseek-v4-flash"), false, "provider UI must not hard-code a model preset");

const draft = {
  providerID: "example-provider",
  name: "Example Provider",
  protocol: "openai-compatible",
  baseURL: "https://api.example.com/v1",
  modelID: "example-model",
  modelName: "Example Model",
  toolCall: true,
  reasoning: true,
  contextLimit: "128000",
  outputLimit: "16384",
  apiKey: "super-secret-should-never-enter-config",
};
assert.equal(hooks.validateDraft(draft), "");

const config = hooks.buildProviderConfig(draft);
assert.equal(config.name, "Example Provider");
assert.equal(config.npm, "@ai-sdk/openai-compatible");
assert.equal(config.options.baseURL, "https://api.example.com/v1");
assert.equal(config.models["example-model"].name, "Example Model");
assert.equal(config.models["example-model"].tool_call, true);
assert.equal(config.models["example-model"].reasoning, true);
assert.equal(config.models["example-model"].limit.context, 128000);
assert.equal(config.models["example-model"].limit.output, 16384);
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
  providerID: "example-provider",
  name: "Example Provider",
  protocol: "openai-compatible",
  baseURL: "https://api.example.com/v1/",
  modelID: "example-model",
  modelName: "Example Model",
  toolCall: true,
  reasoning: false,
  contextLimit: "",
  outputLimit: "",
}, existing);
assert.equal(merged.options.baseURL, "https://api.example.com/v1");
assert.equal(merged.options.headers["X-Test"], "yes", "editing must preserve unrelated provider options");
assert.ok(merged.models["other-model"], "editing one model must preserve other configured models");
assert.ok(merged.models["example-model"]);

const entries = hooks.customProviderEntries({
  effective: {
    provider: {
      example: config,
      anthropic: { name: "Anthropic" },
      customResponses: { name: "Responses", npm: "@ai-sdk/openai", models: {} },
      unsupported: { name: "Unsupported", npm: "some-other-package", models: {} },
    },
  },
});
assert.deepEqual(Array.from(entries, (entry) => entry.id).sort(), ["customResponses", "example"]);

assert.match(hooks.validateDraft({ ...draft, providerID: "Bad ID" }), /Provider ID/);
assert.match(hooks.validateDraft({ ...draft, baseURL: "not-a-url" }), /Base URL/);
assert.match(hooks.validateDraft({ ...draft, modelID: "" }), /Model ID/);
assert.match(hooks.validateDraft({ ...draft, contextLimit: "12.5" }), /Context limit/);

console.log("custom provider UI regressions: ok");