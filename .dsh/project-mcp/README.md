# VanSail project MCP bundle

Configuration-only DSH bundle that connects the project's documentation MCP
servers:

| Server name | Endpoint | Tools |
| --- | --- | --- |
| `nuxt_ui` | `https://ui.nuxt.com/mcp` | `mcp__nuxt_ui__*` |
| `vue-docs` | `https://mcp.vue-mcp.org/mcp` | `mcp__vue-docs__*` |

## Scope

DSH profiles own plugins, not workspaces. Installing this bundle adds the MCP
connections to the active **DSH profile**, so every workspace in that profile
can use them. The bundle is committed here so the connection set is
reviewable and reproducible; installation itself is a one-time local action.

## Install

From a DSH session:

```text
plugin_manager {
  action: "install_bundle",
  target: "D:\\Remna-User-Panel\\.dsh\\project-mcp"
}
```

The result must report `application: "applied"`. The MCP tools appear as
`mcp__nuxt_ui__*` and `mcp__vue-docs__*` in the next model request.

## Design choices

- **Fail-open.** `failOnStartupError: false` keeps the harness usable when a
  documentation site is unreachable; the affected tools are simply absent
  and the failure is logged. A hard failure would make an offline checkout
  unusable for unrelated work.
- **No local process.** Both servers use Streamable HTTP, so there is no
  `npx` process to install, warm up, or clean up.
- **Read-only documentation.** These servers expose docs search and API
  lookup only. They are not a place for credentials or project data.
- **No secrets.** No headers, tokens, or environment variables are required.

## Add another server

Add a row to `cordis.patch.yml`:

```yaml
- id: mcp-example
  name: '@deepseek-ai/dsh-mcp-client'
  config:
    serverName: example
    transport: streamable-http
    url: https://example.com/mcp
    failOnStartupError: false
```

Reinstall the bundle after changing it. For stdio servers, use
`transport: stdio`, `command`, and optional `args`, `env`, and `cwd`;
credentials are scrubbed from the ambient environment, so pass only the
variables the server needs.

## Files

| File | Purpose |
| --- | --- |
| `package.json` | Bundle manifest and `dsh.bundle.patch` |
| `cordis.patch.yml` | MCP client rows |
| `locale/en.json`, `locale/zh.json` | Plugin manager display metadata |
| `icon.svg` | Plugin manager icon |

See [docs/agent-tooling.md](../../docs/agent-tooling.md) for the Codex
counterpart and the validation matrix.
