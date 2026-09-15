"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");

class FakeElement {
  constructor(tag = "div") {
    this.tagName = tag.toUpperCase();
    this.className = "";
    this.textContent = "";
    this.title = "";
    this.type = "";
    this.value = "";
    this.dataset = {};
    this.children = [];
    this.listeners = new Map();
    this.lookup = new Map();
  }

  append(...items) {
    this.children.push(...items);
  }

  appendChild(item) {
    this.children.push(item);
    return item;
  }

  addEventListener(type, handler) {
    this.listeners.set(type, handler);
  }

  querySelector(selector) {
    if (selector === ".timeout-recovery") {
      return this.children.find((item) => item?.className === "timeout-recovery") || null;
    }
    return this.lookup.get(selector) || null;
  }
}

const repoRoot = path.resolve(__dirname, "..");
const statusUIPath = path.join(repoRoot, "cmd", "launcher", "web", "status-ui.js");
const source = fs.readFileSync(statusUIPath, "utf8");
const trailer = "\n})();";
const trailerIndex = source.lastIndexOf(trailer);
assert.notEqual(trailerIndex, -1, "status-ui.js must remain an IIFE so the regression harness can instrument it");

const instrumented = `${source.slice(0, trailerIndex)}\n  K.__statusUiRegression = {\n    normalizePath,\n    samePath,\n    sessionDirectory,\n    activeProjectSessions,\n    projectUsageSnapshot,\n    projectUsageCache,\n    timeoutKind,\n    addTimeoutRecovery,\n  };${source.slice(trailerIndex)}`;

const document = {
  createElement: (tag) => new FakeElement(tag),
  createElementNS: (_ns, tag) => new FakeElement(tag),
  execCommand: () => true,
  body: new FakeElement("body"),
};

const rootProject = "C:\\Users\\Lenovo\\Desktop\\Ktest";
const childProject = "C:\\Users\\Lenovo\\Desktop\\Ktest\\New folder";
const sessions = [
  { id: "root-current", directory: rootProject, time: { updated: 10 } },
  { id: "root-cached", directory: "c:/users/lenovo/desktop/KTEST/", time: { updated: 20 } },
  { id: "child-current", directory: childProject, time: { updated: 30 } },
  { id: "child-cached", directory: "c:/users/lenovo/desktop/ktest/new folder/", time: { updated: 40 } },
];

const rootMessages = [
  { info: { role: "user", time: { created: 1000 } }, parts: [{ type: "text", text: "root" }] },
  {
    info: {
      role: "assistant",
      time: { created: 1100, completed: 2000 },
      tokens: { input: 100, output: 50, reasoning: 10, cache: { read: 20, write: 5 } },
    },
    parts: [{ type: "text", text: "done" }],
  },
];

const childMessages = [
  { info: { role: "user", time: { created: 3000 } }, parts: [{ type: "text", text: "child" }] },
  {
    info: {
      role: "assistant",
      time: { created: 3100, completed: 5000 },
      tokens: { input: 600, output: 200, reasoning: 50, cache: { read: 40, write: 10 } },
    },
    parts: [{ type: "text", text: "done" }],
  },
];

const K = {
  renderMessages: () => {},
  state: {
    local: { platform: "windows", project: rootProject },
    sessions,
    session: sessions[0],
    messages: rootMessages,
    sending: false,
  },
  els: {
    conversation: null,
    prompt: new FakeElement("textarea"),
  },
  isSessionRunning: () => false,
  resizePrompt: () => {},
  sendPrompt: async () => {},
  showError: () => {},
  api: { sessions: { messages: async () => ({ data: [] }) } },
};

const context = vm.createContext({
  window: {
    KLU: K,
    setTimeout: () => 0,
    clearTimeout: () => {},
  },
  document,
  navigator: {},
  console,
});
vm.runInContext(instrumented, context, { filename: statusUIPath });

const hooks = K.__statusUiRegression;
assert.ok(hooks, "status-ui regression hooks were not injected");

// Windows path normalization must be exact, case-insensitive, slash-insensitive,
// and must not treat a nested project as the parent project.
assert.equal(hooks.samePath("C:\\Users\\Lenovo\\Desktop\\Ktest\\", "c:/users/lenovo/desktop/ktest"), true);
assert.equal(hooks.samePath(rootProject, childProject), false);
assert.deepEqual(
  Array.from(hooks.activeProjectSessions(), (session) => session.id),
  ["root-current", "root-cached"],
);

const cachedUsage = (tokens, requests, duration) => ({
  tokens,
  requests,
  duration,
  breakdown: { input: tokens, output: 0, reasoning: 0, cacheRead: 0, cacheWrite: 0 },
});
hooks.projectUsageCache.set("root-cached", { stamp: "20", usage: cachedUsage(315, 2, 1500) });
hooks.projectUsageCache.set("child-cached", { stamp: "40", usage: cachedUsage(100, 1, 500) });

const rootTotal = hooks.projectUsageSnapshot();
assert.equal(rootTotal.tokens, 500, "root project total must not include nested-project usage");
assert.equal(rootTotal.requests, 3);
assert.equal(rootTotal.complete, true);

K.state.local.project = childProject;
K.state.session = sessions[2];
K.state.messages = childMessages;
assert.deepEqual(
  Array.from(hooks.activeProjectSessions(), (session) => session.id),
  ["child-current", "child-cached"],
);
const childTotal = hooks.projectUsageSnapshot();
assert.equal(childTotal.tokens, 1000, "nested project total must not include parent-project usage");
assert.equal(childTotal.requests, 2);
assert.equal(childTotal.complete, true);

const providerTimeout = JSON.stringify({
  code: 503,
  message: "The upstream provider timed out while sending the response. (request id: fra1:test)",
  type: "timeout",
  param: null,
});
assert.equal(hooks.timeoutKind("Upstream idle timeout exceeded"), "idle");
assert.equal(hooks.timeoutKind("The upstream provider timed out while sending the response."), "provider");
assert.equal(hooks.timeoutKind(providerTimeout), "provider");
assert.equal(hooks.timeoutKind('{"code":503,"message":"Service unavailable"}'), "");

// The new provider-timeout shape must get the same safe Resume path as the
// older idle timeout while preserving the raw diagnostic payload.
const error = new FakeElement("div");
error.textContent = providerTimeout;
const content = new FakeElement("div");
const row = new FakeElement("article");
row.lookup.set(".message-error-text", error);
row.lookup.set(".message-content", content);
const conversation = new FakeElement("section");
conversation.querySelectorAll = (selector) => selector === ".message.error" ? [row] : [];
K.els.conversation = conversation;
K.state.sending = false;
K.state.session = sessions[2];
let sent = 0;
K.sendPrompt = async () => { sent += 1; };

hooks.addTimeoutRecovery();
assert.equal(error.textContent, "Upstream provider timeout");
assert.equal(error.dataset.rawError, providerTimeout);
assert.equal(error.title, providerTimeout);
const recovery = content.querySelector(".timeout-recovery");
assert.ok(recovery, "provider timeout should render a recovery control");
assert.equal(recovery.children[1].textContent, "Resume");

const click = recovery.children[1].listeners.get("click");
assert.equal(typeof click, "function");
Promise.resolve(click()).then(() => {
  assert.equal(sent, 1, "Resume should submit exactly one continuation prompt");
  assert.match(K.els.prompt.value, /Continue the current task from the existing workspace state/);
  console.log("status-ui regressions: ok");
}).catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
