# TL Studio documentation site

This directory contains the public documentation and product site for **TL Studio**. It is isolated from the Go application and from internal engineering/design notes under `../docs/`.

## Source of truth

The stable `main` branch is the sole source of truth for this public documentation site. Reconcile the site with `main`, `VERSION`, the root READMEs, `docs/ARCHITECTURE.md`, and relevant stable implementation files.

Current documented state:

```text
Stable: v0.5.0
main:   v0.5.0
```

Document only behavior present in stable `main`; do not publish unreleased branch state.

## v0.5 public coverage

The public site covers:

- one-process fully native TL Studio architecture;
- Editor-centered workbench and locally bundled Monaco;
- Project files, Search, Terminal, Changes, Command Palette, and Preview;
- native Sessions, Questions, Permissions, Events, and Agent execution;
- model connection registry, discovery, discovery cache, and credential vault;
- Native Tool Registry/Executor;
- Plugins/MCP with current stdio transport and Project/Global scope;
- generic account-connection foundation without claiming unavailable integrations;
- local security/trust boundaries;
- stable npm launcher and portable release model.

The public site should describe the current product and implementation. Historical architecture belongs in release history, not current user-facing documentation.

## Languages

- English: `/` and `/docs/...`
- فارسی: `/fa/` and `/fa/docs/...` with RTL layout

The language switcher is in the top navigation beside social/appearance controls. Persian uses Vazirmatn and established technical terms where natural.

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
