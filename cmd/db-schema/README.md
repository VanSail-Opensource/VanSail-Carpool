# PostgreSQL schema command

`main.go` exposes `migrate` and `verify` using database-only validated environment
configuration. `migrate` uses migration-role credentials and forward-only Goose;
`verify` reads version/catalog contracts. No application, provider, scheduler or
X-Node master is started. No automatic driver fallback is allowed.
