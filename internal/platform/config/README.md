# Runtime configuration

- `config.go` loads environment-backed server, database, Telegram, provider, and security configuration with validated defaults.
- `ADMIN_TELEGRAM_ID` accepts one or more comma-separated positive Telegram user IDs; duplicates are ignored.
- `config_test.go` verifies canonical public-origin validation and complete valid environment loading.
- `memory.go` configures writable SQLite temporary storage and derives a Go soft
  memory limit from cgroup v1/v2 limits (including ancestors) unless GOMEMLIMIT
  is explicit. A 256 MiB container gets a 160 MiB runtime budget. `memory_test.go`
  covers finite, inherited, missing and unlimited container limits in hosted CI.
- PROXY_CACHE_MAX_BYTES bounds the rebuildable proxy cache, including staging
  and sidecars; the default is 1073741824 (1 GiB), with a 16 MiB minimum.

- `memory_observe.go` emits bounded runtime/metrics summaries without publishing
  heap profiles, secrets or upstream responses.

Leave GOMEMLIMIT unset for container-derived budgeting (160 MiB for a 256 MiB
container), or explicitly override it. This is a soft runtime limit, not an RSS
guarantee. SQLite temporary files default to DATA_DIR/tmp; SQLITE_TMPDIR remains
an explicit override. Cache exhaustion degrades inventory coverage independently
of USDT collection.
