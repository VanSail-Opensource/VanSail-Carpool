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

Application activation requires a new empty installation or an unchanged verified
import. Production cutover additionally requires the operator acceptance gates.

`open.go` owns connections and schema policy. `schema.go` checks the generated
`contract.json` catalog manifest. `schema_test.go` exercises fresh/reapplied
migrations, immutable histories, exact timestamps and future-version refusal in
hosted PostgreSQL. `tokens.go`, `bind.go` and `connector.go` provide the SQL-aware
legacy binding boundary while retaining `database/sql` repository contracts.

`bind_test.go` verifies SQL-aware quoting, comments, dollar strings, native
parameters, aggregate/scalar distinctions and null equality in hosted CI.

`contract.go` exposes the generated source contract and `source-schema.sha256`
fingerprint to offline conversion. Migration `00005_cutover_state.sql` records
inactive import/verification gates and makes source FKs transaction-deferrable.

Original TEXT columns explicitly use PostgreSQL `C` collation. This preserves
SQLite binary range/pagination behavior, including compact IPv4 subnet reuse,
independently of the operator's cluster locale. The catalog verifier checks it.
JSON merge-patch completion documents follow RFC 7396; byte-sensitive stored
payloads are copied unchanged and are never implicitly converted to JSONB.

`runtime.go` owns the dedicated single-master advisory session and reconnecting
LISTEN session; `runtime_test.go` checks exclusion, wake-up and ownership loss.
Notifications carry hints only and never replace durable polling. `metrics.go`
keeps bounded aggregate transaction/retry counters in memory. Ownership loss
stops the application; this initial design requires one application process and
does not enable HA or automatic failover.
