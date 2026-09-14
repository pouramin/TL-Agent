# Security

Kilo Local UI is intentionally local-first. The browser UI and the Kilo backend are bound to loopback by default and the launcher refuses a non-loopback UI address.

## Threat model

The Kilo runtime can execute commands and modify files when the active agent and permission policy allow it. Treat prompts, connected providers, MCP servers, and repositories as potentially security-sensitive inputs.

The launcher protects its local browser bridge by:

- using a random per-run password for `kilo serve`,
- keeping Kilo Basic Auth credentials out of browser-side code,
- checking loopback Host values,
- rejecting cross-origin requests,
- adding a restrictive Content Security Policy, and
- not exposing a project-owned remote control plane.

## Reporting

Please open a private GitHub security advisory if the repository has that feature enabled. Do not publish working exploits for unresolved vulnerabilities in a public issue.
