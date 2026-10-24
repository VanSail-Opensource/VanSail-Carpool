# Runtime configuration

`database.go` validates explicit `DB_DRIVER` selection, protected
`DATABASE_URL_FILE` or `DATABASE_URL`, pool bounds, duration deadlines and startup
migration policy. SQLite remains the default. An explicitly configured PostgreSQL
deployment must never silently open SQLite; application activation remains gated
until repository compatibility is implemented. Errors never echo connection URLs
or secret file contents. See [infrastructure](../../../deploy/postgres/README.md).
`database_test.go` covers explicit opt-in, URL redaction, pool/deadline bounds and
migration defaults in hosted CI.

- `config.go` loads environment-backed server, database, Telegram, provider, and security configuration with validated defaults.
- `ADMIN_TELEGRAM_ID` accepts one or more comma-separated positive Telegram user IDs; duplicates are ignored.
- `config_test.go` verifies canonical public-origin validation and complete valid environment loading.
- `memory.go` configures writable process and SQLite temporary storage and derives a Go soft
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

Process temporary files, including GeoIP/GeoSite asset downloads and decoder
spools, live under the resolved DATA_DIR/tmp directory (0700). Bootstrap sets
TMPDIR before app.New; TMP/TEMP and SQLITE_TMPDIR are filled only when empty.
Explicit SQLITE_TMPDIR remains authoritative. The resolved directories are logged
with the memory budget. Read-only-root containers require only writable DATA_DIR.
