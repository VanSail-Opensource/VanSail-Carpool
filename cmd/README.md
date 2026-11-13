# Commands

Executable entry points live in child directories so command wiring remains separate from reusable application code.

- `ARCHITECTURE.md` documents the command-layer boundary and startup responsibilities.

## Executable inventory

- `server/main.go` starts the application after validated configuration and database initialization; its healthcheck subcommand only checks the selected HTTP health endpoint.
- `db-schema/main.go` owns native PostgreSQL `migrate`, `verify` and recovery `fingerprint` commands using database-only configuration. Use migration-role credentials for `migrate`; `verify` checks version history and the complete catalog contract without schema mutation. The fingerprint command produces retained recovery evidence without starting the application, schedulers or providers.

See [the schema command](db-schema/README.md) and
[PostgreSQL](../internal/platform/database/postgres/README.md).
