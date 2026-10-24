# PostgreSQL persistence

This package owns pgx stdlib connections, bounded pools/deadlines and embedded
forward-only Goose migrations. `Open` selects PostgreSQL explicitly; it never
falls back to SQLite. `Migrate` uses a dedicated advisory session lock and
`VerifyVersion` rejects incomplete/future histories. Run migrations with the
migration role, then use the restricted application role with startup migration
disabled.

`00001` contains bounded compatibility functions for existing JSON/string/time
queries. Financial and signed JSON remains TEXT. Timestamps also remain TEXT,
per the owner's explicit decision to preserve nanosecond evidence. `00002` is
the final post-startup SQLite baseline: all tables, constraints, partial indexes,
bootstrap rows and trigger predicates are retained. No historical SQLite script
is replayed against PostgreSQL. Downgrades deliberately refuse execution.

Regenerate the reviewed baseline with `go run ./cmd/db-inventory`, then
`python scripts/postgresql_baseline.py`. The generator consumes only schema and
synthetic bootstrap rows; source inventory never exports business values.
New schema changes belong in new numbered migrations. Update `SchemaVersion` and
hosted schema-parity checks in the same change.

Application activation remains disabled until domain contracts, locking,
migration verification and recovery integration have been delivered.

`open.go` owns connections and schema policy. `schema.go` checks the generated
`contract.json` catalog manifest. `schema_test.go` exercises fresh/reapplied
migrations, immutable histories, exact timestamps and future-version refusal in
hosted PostgreSQL. `tokens.go`, `bind.go` and `connector.go` provide the SQL-aware
legacy binding boundary while retaining `database/sql` repository contracts.

`bind_test.go` verifies SQL-aware quoting, comments, dollar strings, native
parameters, aggregate/scalar distinctions and null equality in hosted CI.
