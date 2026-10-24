# Persistence inventory

`go run ./cmd/db-inventory` creates an isolated SQLite fixture using the real
embedded migrations and startup identity rekey, then writes its schema and a
deterministic metadata inventory. It never opens the application's configured
database. This is an operator utility, not a local test runner.

`--source /path/to/snapshot.db --output /protected/directory` inventories an
existing installation in a read-only consistent transaction. Output contains
schema, migration versions and row counts, never business row values or secrets.
The source must already have completed application startup migrations.

Fixture mode additionally writes `fixture-seeds.sql` from only synthetic
bootstrap tables. Production/source mode never exports row values. Fixture seed
timestamps are normalized to a fixed epoch for reproducible baseline generation.

The database package owns migration semantics; this module only inspects them.
Add new schema metadata to `Inventory` when baseline or reconciliation tooling
needs it. Keep production inventories access-controlled and out of Git.

`cmd/db-inventory/main_test.go` checks deterministic independent fixtures and
post-startup username removal in hosted CI. No local automated suite is needed
to run the inventory utility itself.
