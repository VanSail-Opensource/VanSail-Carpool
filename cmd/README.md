# Commands

Executable entry points live in child directories so command wiring remains separate from reusable application code.

- `ARCHITECTURE.md` documents the command-layer boundary and startup responsibilities.

## Persistence inventory

or inventories an explicitly selected source in a read-only transaction. It
does not start HTTP, schedulers or provider executors. See
[`dbinventory`](../internal/platform/dbinventory/README.md).

`db-schema migrate|verify` manages the embedded forward-only PostgreSQL schema
using database-only configuration. Use migration-role credentials for `migrate`;
`verify` checks version history and the complete catalog contract without schema
mutation. See [PostgreSQL](../internal/platform/database/postgres/README.md).

snapshot into an inactive, empty PostgreSQL target and records exact verification
evidence. See [conversion](../internal/platform/datamigrate/README.md).

`db-schema/main.go` owns native PostgreSQL migrate, verify and recovery fingerprint commands.
