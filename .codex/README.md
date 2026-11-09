# Codex project configuration

`config.toml` is project-scoped Codex configuration. Codex loads it for a
trusted project and merges it over the user-level `~/.codex/config.toml`.

## What is configured

| Server | Transport | Purpose |
| --- | --- | --- |
| `nuxt_ui` | Streamable HTTP | Nuxt UI v4 component, composable, icon, and page documentation |
| `vue-docs` | Streamable HTTP | Vue, Vue Router, Pinia, Vite, and Nuxt documentation |

The project does not pin a model, reasoning effort, approval policy, or
sandbox mode here. Those remain user choices.

Chrome DevTools MCP is a machine-level dependency used for frontend
validation. It stays in the user config because it needs a local Chrome
installation and, on Windows, a `cmd /c npx` launcher.

## Verify

```powershell
codex mcp list
```

The output should include `nuxt_ui` and `vue-docs` from this file, alongside
any user-level servers.

## Add a server

```toml
[mcp_servers.example]
url = "https://example.com/mcp"
enabled = true
```

Do not put API keys in this file. Use environment variables or Codex's
credential storage.

See [docs/agent-tooling.md](../docs/agent-tooling.md) for the DSH
counterpart and the full runtime matrix.
