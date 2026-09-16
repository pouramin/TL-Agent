# TL Agent

This npm package is a lightweight launcher for **TL Agent**, a local, standalone coding-agent workspace.

It does not bundle or replace the TL Agent application. The launcher detects the current OS/architecture, downloads the matching official GitHub Release, verifies its SHA-256 checksum, caches it locally, and runs it in the current project directory.

Each npm package version is pinned to the matching TL Agent GitHub Release. For example, `tl-agent@0.1.0-alpha.28` launches `v0.1.0-alpha.28`, keeping runs reproducible.

## Quick start

While TL Agent is in alpha, use the `alpha` dist-tag:

```bash
npx --yes tl-agent@alpha
```

Run without automatically opening the browser:

```bash
npx --yes tl-agent@alpha --no-browser
```

When a stable release is published under the `latest` dist-tag, the shorter command will become:

```bash
npx --yes tl-agent
```

## What gets stored locally?

The npm package itself uses the normal npm/npx cache. TL Agent release files are cached separately:

- Windows: `%LOCALAPPDATA%\\TL-Agent\\cache`
- Linux/macOS: `${XDG_CACHE_HOME:-~/.cache}/tl-agent`

Nothing is installed as a Windows service, system package, or global CLI unless you explicitly choose to install the npm package globally.

## Source and releases

- Source: https://github.com/pouramin/TL-Agent
- Releases: https://github.com/pouramin/TL-Agent/releases
- Issues: https://github.com/pouramin/TL-Agent/issues

TL Agent is MIT licensed. See the repository for third-party notices and runtime attribution.
