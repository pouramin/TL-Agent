# TL Agent documentation site

This directory contains the public documentation and product site for **TL Agent**. It is intentionally isolated from the Go application and from the internal engineering notes under `../docs/`.

The public site treats TL Agent as the product identity. Runtime-specific implementation details are kept in a dedicated maintainer reference instead of being repeated across user-facing pages.

## Languages

- English: `/` and `/docs/...`
- فارسی: `/fa/` and `/fa/docs/...` with RTL layout

The language switcher is part of the top navigation beside the appearance controls.

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
DEPLOY_TARGET=static NEXT_PUBLIC_BASE_PATH=/TL-Agent npm run build
```

Output is written to `website/out/`.

Nothing under `website/` is imported by the Go launcher or included in TL Agent release archives.
