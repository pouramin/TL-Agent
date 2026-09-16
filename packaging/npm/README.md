# TL Agent

This npm package is a lightweight launcher for **TL Agent**, a local, standalone coding-agent workspace.

It does not bundle or replace the TL Agent application. The launcher detects the current OS/architecture, downloads the matching official GitHub Release, verifies its SHA-256 checksum, caches it locally, and runs it in the current project directory.

## Quick start

```bash
npx --yes tl-agent
```

Run without automatically opening the browser:

```bash
npx --yes tl-agent -- --no-browser
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
