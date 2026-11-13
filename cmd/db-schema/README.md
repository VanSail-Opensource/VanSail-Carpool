# PostgreSQL schema command

`main.go` exposes `migrate`, `verify` and `fingerprint` using database-only validated environment
configuration. `migrate` uses migration-role credentials and forward-only Goose;
`verify` reads version/catalog contracts. `fingerprint` emits a JSON recovery
report for retained rows and sequences and checks encrypted settings and financial
invariants. Supply the original `CONFIG_MASTER_KEY` and quiesce source writers
when comparing recovery evidence. No application, provider, scheduler or
X-Node master is started. No automatic driver fallback is allowed.

Schema 46 requires the old application runtime to be stopped before migration.
The migration tries its existing owner advisory lock, drains outstanding traffic
transactions and atomically establishes raw 1x acceptance boundaries. A live
owner causes a refusal without partial accounting changes; restart with the new
binary after migration succeeds. See [the accounting cutover](../../internal/platform/database/multiplier_retirement.md).
