# PostgreSQL persistence

`community_retirement_upgrade_test.go` seeds old questionnaire rewards, imports,
and pending work in the isolated forward-upgrade rehearsal. It verifies archived
evidence, unchanged financial records, terminal receipts, and removed tables/jobs.

`bind_cache.go` shares a bounded FIFO of translated SQL grammar across one pool
(256 statements, 2 MiB of SQL). Keys include argument cast kinds; boolean
normalization preserves authored PostgreSQL parameters. It never caches argument
values or results. Oversized statements use the uncached, SQL-aware binder.
`bind_cache_test.go` checks protected grammar, argument-type changes, bounded
retention and concurrent reuse under hosted race checks.

Schema version 21 adds `pricing_active_commands` to match the existing SQLite
schema contract. It supports active pricing-command lookups and repairs fresh
and upgraded PostgreSQL startup without changing historical migrations or data.

Schema version 26 permits zero finite capacity when provider capacity is marked
unlimited, matching the SQLite pricing contract.

Schema version 16 permits multiple endpoint references per DDNS family while
retaining global host uniqueness. Schema version 15 updates community-preference
transition functions and backfills Show Around TX for existing combo/PAYG
spenders. Community access is derived from current combo validity or existing
lifetime PAYG purchased bytes. No new data columns or indexes are required.

Schema version 8 adds prepaid PAYG remainder/charge timestamps and rolling raw
report windows through migration 00008. The contract and source inventory are
regenerated from an isolated SQLite fixture; the original baseline is unchanged.

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
`contract.json` catalog manifest, including SQLite `REAL` and explicit `DOUBLE PRECISION` forecast columns mapped to PostgreSQL double precision. `schema_test.go` exercises fresh/reapplied
migrations, immutable histories, exact timestamps and future-version refusal in
hosted PostgreSQL. `tokens.go`, `bind.go` and `connector.go` provide the SQL-aware
legacy binding boundary while retaining `database/sql` repository contracts.

`bind_test.go` verifies SQL-aware quoting, comments, dollar strings, native
parameters, aggregate/scalar distinctions and null equality in hosted CI.

`contract.go` exposes the generated source contract and `source-schema.sha256`
fingerprint to offline conversion. Migration `00005_cutover_state.sql` records
inactive import/verification gates and makes source FKs transaction-deferrable.
After every forward schema migration, regenerate the isolated inventory with
`go run ./cmd/db-inventory`, then run
`python scripts/postgresql_baseline.py --contract-only`. This updates the import
fingerprint and verification manifest while preserving the immutable baseline.

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

`upgrade_test.go` applies a supported earlier Goose prefix in a separate database,
refuses it without migration, then upgrades through the application connector and
verifies the final catalog and safe reapplication.

PAYG upgrade 00007 adds only required durable financial attribution and bounded presentation aggregates. Existing historical migrations and the baseline remain immutable; the inventoried schema and conversion contract include the new reference columns and tables.

After a forward migration, refresh the isolated SQLite inventory and run `python scripts/postgresql_baseline.py --contract-only` to update import/schema verification metadata while preserving the immutable baseline and prior Goose migrations.

`retirement_upgrade_test.go` seeds unsettled and settled historical credits at Goose 17 inside the existing isolated hosted upgrade rehearsal. It verifies unchanged wallet, ledger and purchase facts, archived settled/daily evidence and cancelled calculation/notice jobs after migration through the application connector.

Schema version 25 adds provider rate and unlimited-capacity metadata without changing historical records; the source contract and fingerprint reflect the matching SQLite migration 111.

Schema version 29 records account lifecycle and authenticated Telegram block ordering. Minimal non-login account subjects retain immutable FK-backed accounting after purge; operational profile/credential data is removed.
