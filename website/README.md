# Public documentation site

This directory is the public documentation/product site for Kilo Local UI. It is intentionally isolated from the Go launcher and from the engineering notes in `../docs/`.

## Stack

- Next.js 16
- Fumadocs
- Tailwind CSS 4
- Static search
- GitHub Pages deployment

## Local development

```bash
cd website
npm install
npm run dev
```

## Production build

```bash
npm run typecheck
npm run build
```

For a GitHub Pages export:

```bash
DEPLOY_TARGET=static NEXT_PUBLIC_BASE_PATH=/TL-Agent npm run build
```

The output is written to `website/out/`.

## Separation from the product

Nothing in this directory is imported by the Go launcher or shipped in release archives. The docs have their own CI and Pages deployment workflows. This keeps documentation changes from affecting the application build or runtime.
