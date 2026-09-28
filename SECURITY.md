# Security

TL Studio is intentionally local-first. Its Browser control UI binds to loopback and the launcher refuses a non-loopback UI address.

The Native Agent can read or modify project files and execute commands only through TL Studio tools and permission policy. Treat prompts, connected providers, MCP servers, repositories, tool output, and plugin processes as potentially security-sensitive inputs.

The launcher protects its local control surface by:

- binding the control server to loopback;
- rejecting non-loopback Host values;
- requiring Browser Origin to match the local control origin;
- keeping provider credentials out of Browser code and provider registry files;
- enforcing project boundaries on filesystem operations;
- rejecting traversal and symlink escapes;
- isolating Preview content from the TL Studio control origin;
- permission-gating sensitive tool actions.

TL Studio does not start or authenticate a second local coding-runtime server. External model providers and MCP servers are ordinary external trust boundaries configured by the user.

Do not expose the local control port through a public proxy. Review project code, provider endpoints, plugin commands, and requested permissions before approving sensitive actions.

Please report security issues privately to the repository owner rather than opening a public exploit report.
