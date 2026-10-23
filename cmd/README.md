# Commands

Executable entry points live in child directories so command wiring remains separate from reusable application code.

- `ARCHITECTURE.md` documents the command-layer boundary and startup responsibilities.

## Persistence inventory

`db-inventory` produces the post-startup SQLite schema from an isolated fixture,
or inventories an explicitly selected source in a read-only transaction. It
does not start HTTP, schedulers or provider executors. See
[`dbinventory`](../internal/platform/dbinventory/README.md).
