# PostgreSQL persistence

This package owns pgx connections, serializable write transactions, forward-only
Goose migrations, schema verification, single-master ownership and recovery
fingerprints. SQL callers author native PostgreSQL parameters and expressions;
the connector passes values and rows directly to pgx without dialect translation.

- `open.go` configures UTC, pool bounds and statement/lock/connect deadlines.
  `Migrate` takes the existing advisory session lock; `VerifyVersion` refuses
  incomplete or future histories. Migrate as the schema owner, then start the
  restricted application role with startup migration disabled.
- `connector.go` defaults write transactions to serializable and delegates native
  preparation/querying/session reset. `metrics.go` records bounded transaction
  duration, failure and retry counters, without SQL or argument labels.
- `runtime.go` owns the independent scheduler lock and reconnecting LISTEN hints.
  Losing ownership stops the runtime. Notifications never replace durable polling.
- `schema.go`, `contract.go` and `contract.json` verify native column types,
  primary keys, C text collation, indexes and triggers through PostgreSQL catalogs.
  Canonical JSON, encrypted credentials and timestamps remain unchanged TEXT.
- `recovery.go` takes a repeatable-read snapshot and checks schema/encryption.
  `recovery_digest.go` streams exact values into deterministic table/sequence
  digests; `recovery_invariants.go` verifies ledger balances and associated-data
  encryption contexts. These functions never export plaintext records or mutate
  accounting. `db-schema fingerprint` exposes the bounded evidence document.

Schema 32 replaces the live JSON checks, time/string expression indexes and paid
access trigger functions, then drops obsolete compatibility functions without
CASCADE. Applied migrations 1 through 31 are immutable upgrade history. No data
import, identity relinking, source fingerprint activation or file replacement is
part of startup. Historical migration-state and audit rows remain preserved.
Native timestamp indexes have microsecond precision; cache selection compares
original timestamp text in Go to retain nanosecond ordering within equal buckets.

Hosted checks: `schema_test.go` covers fresh/repeated migration, JSON contracts,
native values, immutable financial constraints and future-version refusal.
`upgrade_test.go` replays populated upgrades through version 31, fingerprints all
authoritative records before/after version 32, checks removal of live compatibility
objects and verifies idempotence. `retirement_upgrade_test.go` and
`community_retirement_upgrade_test.go` seed and verify retained historical
accounting/reward evidence. `runtime_test.go` checks ownership and notifications.
No local automated suites are permitted by repository policy.

Native scalar flag projections use PostgreSQL boolean-to-integer casts explicitly; Go integer consumers do not depend on connector row coercion. Fixture users use existing Telegram identity columns and never recreate retired username fields.
