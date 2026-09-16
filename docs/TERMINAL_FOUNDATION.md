# Terminal foundation

The integrated Terminal is a local project command runner owned by TL Agent.

## Current scope

- Commands run only with the current project as their working directory.
- The browser talks only to TL Agent's loopback-only `/local/process` API.
- Output is retained in a bounded in-memory buffer and streamed to the UI by local polling.
- The active process can be stopped from the Terminal panel.
- No CDN, remote shell service, or cloud control plane is involved.

## Why this comes before Live Preview

Live Preview for Vite, React, Next.js, Vue, Astro, and similar projects needs a reliable way to start and stop long-lived project processes, collect their logs, and discover when they are ready. The Terminal/process foundation supplies that lifecycle without hard-coding a particular frontend framework.

## Deliberate limitation

This milestone is a command runner, not a full PTY implementation. Interactive TUI programs that require terminal emulation are out of scope for the foundation. A future PTY layer can replace the transport without changing the higher-level process ownership model.
