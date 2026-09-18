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
assert.equal(hooks.PROTOCOLS.has("openai-compatible"), true);
assert.equal(hooks.PROTOCOLS.has("openai-responses"), true);
assert.equal(hooks.PROTOCOLS.has("anthropic-messages"), true);

for (const forbidden of ["@ai-sdk/", "K.api.config", "K.api.auth", "config/overlay"]) {
  assert.equal(source.includes(forbidden), false, `provider UI leaked runtime config detail: ${forbidden}`);
}
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

const definition = hooks.buildProviderDefinition(draft);
assert.equal(definition.id, "example-provider");
assert.equal(definition.name, "Example Provider");
assert.equal(definition.protocol, "openai-compatible");
assert.equal(definition.baseURL, "https://api.example.com/v1");
assert.equal(definition.models[0].id, "example-model");
assert.equal(definition.models[0].name, "Example Model");
assert.equal(definition.models[0].toolCall, true);
assert.equal(definition.models[0].reasoning, true);
assert.equal(definition.models[0].contextLimit, 128000);
assert.equal(definition.models[0].outputLimit, 16384);
assert.equal(JSON.stringify(definition).includes("super-secret"), false, "API keys must not enter TL Agent provider config");

const existing = {
  id: "example-provider",
  name: "Existing",
  protocol: "openai-compatible",
  baseURL: "https://old.example/v1",
  models: [{ id: "other-model", name: "Other", toolCall: true, reasoning: false }],
};
const merged = hooks.buildProviderDefinition({
  ...draft,
  baseURL: "https://api.example.com/v1/",
  reasoning: false,
  contextLimit: "",
  outputLimit: "",
}, existing);
assert.equal(merged.baseURL, "https://api.example.com/v1");
assert.ok(merged.models.some((model) => model.id === "other-model"), "editing one model must preserve other TL Agent model definitions");
assert.ok(merged.models.some((model) => model.id === "example-model"));

const entries = hooks.customProviderEntries({
  providers: [
    definition,
    { id: "z-provider", name: "Zed", protocol: "openai-compatible", baseURL: "https://z.example/v1", models: [] },
  ],
});
assert.deepEqual(Array.from(entries, (entry) => entry.id), ["example-provider", "z-provider"]);

assert.match(hooks.validateDraft({ ...draft, providerID: "Bad ID" }), /Provider ID/);
assert.match(hooks.validateDraft({ ...draft, baseURL: "not-a-url" }), /Base URL/);
assert.match(hooks.validateDraft({ ...draft, modelID: "" }), /Model ID/);
assert.match(hooks.validateDraft({ ...draft, contextLimit: "12.5" }), /Context limit/);

console.log("custom provider UI regressions: ok");
