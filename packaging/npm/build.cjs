#!/usr/bin/env node
"use strict";

const fs = require("fs");
const path = require("path");

const here = __dirname;
const root = path.resolve(here, "../..");
const out = path.join(here, "dist");
const versionFile = path.join(root, "VERSION");
const templateFile = path.join(here, "package.template.json");
const launcherFile = path.join(root, "scripts", "npx-launch.cjs");
const readmeFile = path.join(here, "README.md");

const version = fs.readFileSync(versionFile, "utf8").trim();
if (!/^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/.test(version)) {
  throw new Error(`VERSION is not valid npm semver: ${version}`);
}

const manifest = JSON.parse(fs.readFileSync(templateFile, "utf8"));
manifest.version = version;

fs.rmSync(out, { recursive: true, force: true });
fs.mkdirSync(out, { recursive: true });
fs.writeFileSync(path.join(out, "package.json"), `${JSON.stringify(manifest, null, 2)}\n`);
fs.copyFileSync(launcherFile, path.join(out, "tl-agent.cjs"));
fs.copyFileSync(readmeFile, path.join(out, "README.md"));

console.log(`Prepared ${manifest.name}@${manifest.version} in ${out}`);
