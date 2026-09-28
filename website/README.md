# TL Studio documentation site

This directory contains the public documentation and product site for **TL Studio**. It is isolated from the Go application and from internal engineering/design notes under `../docs/`.

## Sources of truth

Released behavior comes from stable `main`.

Development-preview documentation may also track `dev`, but it must be labeled clearly and must never be presented as part of the stable release before promotion.

Current documented state:

```text
Stable:      v0.5.0
Development: v0.6.0-alpha.1
```

Reconcile public content with the root READMEs, `VERSION`, `docs/ARCHITECTURE.md`, and the relevant implementation branch.

## Public coverage

The site covers:

- TL Studio's local-first standalone product architecture;
- Editor-centered workspace and locally bundled Monaco;
- Project files, Search, Terminal, Changes, Command Palette, and Preview;
- native Sessions, Questions, Permissions, Events, and Agent execution;
- model connection registry, discovery, catalog cache, and credential vault;
- Native Tool Registry/Executor;
- Plugins/MCP with Project/Global scope;
- stable v0.5 model connections;
- the explicitly labeled v0.6 Provider Account preview for OpenRouter, Hugging Face, and Google/Gemini;
- deferred account boundaries for ChatGPT/Codex, Claude, and GitHub Copilot;
- local security/trust boundaries;
- stable npm launcher, portable releases, and development Preview Builds.

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
