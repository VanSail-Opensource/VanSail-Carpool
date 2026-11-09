# PostgreSQL schema command

`main.go` exposes `migrate` and `verify` using database-only validated environment
configuration. `migrate` uses migration-role credentials and forward-only Goose;
`verify` reads version/catalog contracts. No application, provider, scheduler or
X-Node master is started. No automatic driver fallback is allowed.

Schema 46 requires the old application runtime to be stopped before migration.
The migration tries its existing owner advisory lock, drains outstanding traffic
transactions and atomically establishes raw 1x acceptance boundaries. A live
owner causes a refusal without partial accounting changes; restart with the new
binary after migration succeeds. See [the accounting cutover](../../internal/platform/database/multiplier_retirement.md).
