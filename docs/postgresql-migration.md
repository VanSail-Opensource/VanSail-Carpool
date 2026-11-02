# PostgreSQL-only operation

PostgreSQL 18 is the sole authoritative store. Use the
[embedded deployment](../deploy/embedded/README.md) or a configured
[external service](../deploy/postgres/README.md). `DB_DRIVER` defaults to
`postgres`; a missing/unavailable connection fails startup. No alternate engine,
conversion/import command, runtime SQL translator or database file swap exists.

Run `db-schema migrate` with migration credentials before starting the restricted
application role. Startup verifies the complete migration history and native
schema contract, then acquires exclusive scheduler ownership. Forward migration
32 replaces dependent live checks, indexes and trigger functions before dropping
compatibility functions. Previously applied PostgreSQL scripts and historical
audit, accounting, credentials and migration-state records remain unchanged.

Use `db-schema verify` for schema validation and `db-schema fingerprint` with the
unchanged CONFIG_MASTER_KEY for recovery evidence. Follow the
[recovery runbook](../deploy/postgres/recovery.md) for logical archives and
pgBackRest encrypted full/PITR recovery. Restore to an isolated target, compare
authoritative table and identity-sequence digests and verify role boundaries
before allowing a single application/master to resume.

Hosted CI runs fresh startup, populated version-31 upgrades, restart persistence,
concurrent commerce, native SQL contracts, logical and physical recovery. Backend
fixtures clone private migrated PostgreSQL templates; they never emulate another
engine. Automated suites and benchmarks run only in hosted CI.
