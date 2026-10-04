# TL Studio documentation site

This directory contains the public documentation and product site for **TL Studio**. It is isolated from the Go application and from internal engineering/design notes under `../docs/`.

## Sources of truth

Released behavior comes from stable `main`.

Development-preview documentation may also track `dev`, but it must be labeled clearly and must never be presented as stable behavior before promotion.

Current documented state:

```text
Stable:      v0.6.0
Development: v0.7.0-alpha.1
```

Reconcile public content with the root READMEs, `VERSION`, `docs/ARCHITECTURE.md`, release notes, and the relevant implementation branch.

## Public coverage

The site covers:

- TL Studio's local-first standalone product architecture and Native Agent ownership;
- Editor-centered workspace, Monaco, Search, Terminal, Preview, Sessions, Questions, Permissions, and local persistence;
- direct model protocols and discovery-first Provider configuration;
- ChatGPT/Codex account access, Claude/Anthropic API and Claude account access, plus Claude Web Bridge transport;
- Laya Router and JEV Direct model-routing profiles while TL Studio retains Agent/tool ownership;
- Graphify as a global integration with project-local `graphify-out/` data and explicit graph builds;
- Plugins/MCP, including global/project scope and responsive plugin cards;
- stable npm launcher, portable releases, SHA-256 checksums, and the Windows x64 in-app updater;
- the stable `v0.6.0` release and clearly labeled `v0.7.0-alpha.1` development line.

Historical implementation details do not belong in current product-facing documentation.

## Languages

- English: `/` and `/docs/...`
- فارسی: `/fa/` and `/fa/docs/...` with RTL layout

The language switcher is in the top navigation beside social and sponsor controls. Persian uses Vazirmatn and keeps commands, paths, and technical tokens left-to-right where needed.

## Stack

- Next.js 16
- Fumadocs
- Tailwind CSS 4
- static search
- GitHub Pages deployment
- per-page Markdown export plus `llms.txt` / `llms-full.txt`

## Local development

```bash
cd website
npm install
npm run dev
```

## GitHub Pages build

```bash
DEPLOY_TARGET=static NEXT_PUBLIC_BASE_PATH=/TL-Studio npm run build
```

Output is written to `website/out/`.

Nothing under `website/` is imported by the Go launcher or included in TL Studio release archives.
