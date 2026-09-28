# Security

TL Studio is intentionally local-first. Its Browser control UI binds to loopback and the launcher refuses a non-loopback UI address.

The Native Agent can read or modify project files and execute commands only through TL Studio Tools and permission policy. Treat prompts, connected Providers, MCP servers, repositories, Tool output, and Plugin processes as potentially security-sensitive inputs.

The launcher protects its local control surface by:

- binding the control server to loopback;
- rejecting non-loopback Host values;
- requiring Browser Origin to match the local control origin;
- keeping Provider credentials out of Browser code and Provider Registry files;
- enforcing project boundaries on filesystem operations;
- rejecting traversal and symlink escapes;
- isolating Preview content from the TL Studio control origin;
- permission-gating sensitive Tool actions.

Manual API credentials and account-backed credentials are stored separately in the TL Studio credential vault.

OAuth codes, PKCE verifiers, access tokens, refresh tokens, API keys, cookies, and client secrets remain outside Browser state and normal JSON Provider configuration.

External model Providers and MCP servers are explicit trust boundaries configured by the user.

Do not expose the local control port through a public proxy. Review project code, Provider endpoints, Plugin commands, and requested permissions before approving sensitive actions.

Please report security issues privately to the repository owner rather than opening a public exploit report.
