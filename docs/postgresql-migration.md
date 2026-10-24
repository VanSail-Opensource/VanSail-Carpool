# VanSail-Carpool PostgreSQL Migration — Development Plan

**Document version:** 1.1
**Status:** Implementation in progress; acceptance evidence in [progress](plans/postgresql-progress.md)  
**Reference commit:** `808a59e`  
**Target:** PostgreSQL 18 + pgx + Goose + pgBackRest + pg_stat_statements + postgres_exporter  
**Optional:** Redis/Valkey, sqlc, PgBouncer

Checked items below mean implemented and audited repository work, with hosted
PostgreSQL evidence in [run 40065516216](https://github.com/VanSail-Opensource/VanSail-Carpool/actions/runs/40065516216).
Its PostgreSQL job passed schema/upgrade, business/concurrency, conversion,
logical restore, encrypted full/PITR, restricted application startup and exporter
scrape gates. The complete release and new embedded-image checks remain pending.
Staging, production measurements/cutover, optional Redis and unselected R2 remain
unticked. Timestamp TEXT preserves the owner-selected nanosecond policy.

## Architecture decisions

Before implementation, adopt the following decisions:

1. PostgreSQL is the single authoritative database for business records, financial transactions, X-Node configuration and accounting, and durable background jobs.
2. Retain SQLite support temporarily to support existing installations and migration rehearsals.
3. Use `pgx/v5/stdlib` to preserve the existing `database/sql` contracts during migration.
4. Use Goose for all new PostgreSQL schema migrations.
5. Build a PostgreSQL baseline from the current final SQLite schema, not by blindly replaying historical SQLite migrations.
6. Retain the existing transactional outbox and external-provider request queues.
7. Preserve the existing X-Node control protocol and journal semantics.
8. Introduce Redis only after PostgreSQL correctness and cutover have been demonstrated.
9. Keep one active Go application/X-Node master during the initial production migration.
10. Never switch a production database automatically based on its availability.
11. Per owner decision on 2026-10-24, embed PostgreSQL 18 server in the application image, with a 1 GiB container memory limit and a persistent database volume. Keep the separate-service profile for CI and external/recovery deployments.

---

## Step 01 — Repository audit and migration inventory

**Goal:** Produce an exhaustive map of SQLite dependencies and database invariants.

**Files to inspect:**

- `internal/platform/database/`
- `internal/platform/database/migrations/`
- `internal/platform/databaseadmin/`
- `internal/platform/backup/`
- `internal/platform/outbox/`
- `internal/app/`
- `internal/requestauth/`
- `internal/xnode/`
- `internal/platform/config/`

### Tasks

- [x] Inventory all current SQLite tables, indexes, constraints, triggers, and migration scripts.
- [x] Generate the actual deployed SQLite schema using an isolated, up-to-date database fixture.
- [x] Record every direct `*sql.DB` and `*sql.Tx` dependency.
- [x] Inventory every database write protected by `Store.writeMu`.
- [x] Identify uses of SQLite-specific syntax: `PRAGMA`, `sqlite_schema`, `sqlite_master`, `rowid`, `GLOB`, `json_extract`, `json_valid`, scalar `MAX()`, `INSERT OR ...`, `IS ?`, and SQLite-specific error messages.
- [x] Identify timestamp, boolean, BLOB, JSON, and auto-increment compatibility assumptions.
- [x] Enumerate all database triggers implementing immutability, accounting revisions, and audit protections.
- [x] Identify all operations spanning business records and outbox jobs.
- [ ] Record production database size, key table row counts, journal backlog, transaction latency, and lock contention.
- [x] Identify SQLite file-backup, checkpoint, schema-editor, and startup-rekey dependencies.

### Deliverables

- `docs/plans/postgresql-compatibility.md`
- `docs/plans/postgresql-schema-mapping.md`
- `docs/plans/postgresql-invariants.md`
- Baseline performance and migration-risk report

**Acceptance gate:** Every authoritative table, write workflow, SQL incompatibility, and SQLite-specific maintenance operation has an identified replacement strategy.

---

## Step 02 — PostgreSQL infrastructure and configuration

**Goal:** Provide an explicit embedded PostgreSQL deployment and an isolated infrastructure profile without automatically changing existing installations.

### Tasks

- [x] Add PostgreSQL 18 to a new development Docker Compose profile.
- [x] Link `docker-compose.example.yml` to the explicit embedded PostgreSQL Compose deployment and retain the separate-service profile.
- [x] Mount PostgreSQL 18 persistent storage at `/var/lib/postgresql`, respecting the official image's version-specific PGDATA layout.
- [x] Add database health checks and startup dependencies.
- [x] Configure PostgreSQL authentication using SCRAM-SHA-256.
- [x] Use Docker secrets or protected environment files for credentials.
- [x] Do not publish PostgreSQL port 5432 externally.
- [x] Separate application DML and migration DDL roles; run physical backups as the database OS owner (shared OS user in embedded mode).
- [x] Preserve `/data` for application files, assets, temporary data, and any remaining local artifacts.
- [x] Keep the application container's read-only filesystem and existing security restrictions.
- [x] Embed PostgreSQL server with persistent storage, startup/migration ordering and coordinated shutdown.
- [x] Set a 1 GiB container ceiling and reserve memory separately for Go and PostgreSQL.
- [ ] Pass the hosted embedded-image startup, persistence, encrypted recovery and failure-supervision gate.

### Configuration contract

Extend `internal/platform/config/config.go` with:

| Variable | Purpose |
|---|---|
| `DB_DRIVER` | `sqlite` or `postgres` |
| `DATABASE_URL` | PostgreSQL connection string |
| `DB_MAX_OPEN_CONNS` | Maximum application connections |
| `DB_MAX_IDLE_CONNS` | Idle connection limit |
| `DB_CONN_MAX_LIFETIME` | Connection recycling |
| `DB_CONNECT_TIMEOUT` | Initial connection deadline |
| `DB_STATEMENT_TIMEOUT` | Statement timeout |
| `DB_LOCK_TIMEOUT` | Lock acquisition timeout |
| `DB_MIGRATE_ON_START` | Explicit migration policy |

Keep SQLite as the default for legacy installations until an explicit migration has been performed. New PostgreSQL deployments must opt in explicitly.

### Acceptance gate

Both database configurations load correctly, secrets are not logged, and PostgreSQL can start, become healthy, and accept an authenticated application connection.

---

## Step 03 — PostgreSQL schema and Goose migrations

**Goal:** Create a faithful PostgreSQL representation of the current application schema.

**Proposed location:** `internal/platform/database/postgres/migrations/`

### Tasks

- [x] Add `github.com/pressly/goose/v3`.
- [x] Embed PostgreSQL migrations in the application or a dedicated migration command.
- [x] Create one initial PostgreSQL baseline matching the final deployed SQLite schema.
- [x] Use separate, numbered PostgreSQL migrations for future changes.
- [x] Keep the old SQLite migration files immutable.
- [x] Preserve all foreign keys, uniqueness rules, partial indexes, and check constraints.
- [x] Reimplement immutable financial/audit triggers in PostgreSQL.
- [x] Reimplement X-Node compile-revision invalidation triggers.
- [x] Preserve the special identity-rekey exceptions for immutable references.
- [x] Recreate partial unique indexes that deduplicate active outbox work.
- [x] Explicitly define sequence ownership and starting values.
- [x] Implement migration-version compatibility and refusal of unsupported downgrades.
- [x] Add schema-diff checks against the inventoried SQLite schema.

### Data type strategy

- Financial values: `BIGINT`
- Stable IDs and externally referenced identifiers: preserve existing exact representations
- Binary tokens and hashes: `BYTEA`
- Timestamps: preserve exact existing `TEXT` initially, per owner decision on 2026-10-24; defer `TIMESTAMPTZ` conversion and its precision reconciliation
- Existing 0/1 flags: preserve constrained integer compatibility initially; convert deliberately later
- Canonical, byte-sensitive JSON payloads: preserve as `TEXT`
- New structured/queryable JSON: use `JSONB` where appropriate
- Generated numeric identities: PostgreSQL identity columns

Do not convert existing outbox payloads or payment fingerprints to a different canonical representation without compatibility tests.

### Acceptance gate

An empty PostgreSQL database initializes deterministically, all expected constraints and triggers exist, and migrations can be reapplied safely without schema drift.

---

## Step 04 — Database driver and SQL compatibility layer

**Goal:** Support both database backends without rewriting domain services.

### Tasks

- [x] Add `github.com/jackc/pgx/v5/stdlib`.
- [x] Introduce a backend-aware database-opening function.
- [x] Preserve existing domain repository interfaces.
- [x] Add database dialect helpers for explicitly supported SQL differences.
- [x] Move SQLite-only setup, checkpoint, and migration routines behind the SQLite backend.
- [x] Normalize PostgreSQL SQLSTATE errors into existing domain errors.
- [x] Support context cancellation, bounded transactions, and statement deadlines.
- [x] Introduce explicit transaction retries for deadlock/serialization failures where the entire operation can safely be retried.
- [x] Avoid retries of individual statements inside failed transactions.
- [x] Establish a deterministic transaction lock-ordering convention.
- [x] Ensure database transactions do not retain locks while performing slow upstream API calls.
- [x] Set conservative PostgreSQL pool limits initially and expose metrics for later tuning from measured demand.

### SQL conversion rules

- Convert `?` placeholders to PostgreSQL positional parameters using explicitly authored SQL or a verified SQL-aware mechanism.
- Do not use naive string replacement that could change operators, literals, or comments.
- Replace SQLite-specific JSON and string operations with tested PostgreSQL equivalents.
- Replace SQLite schema introspection with PostgreSQL catalog queries.
- Replace SQLite-specific uniqueness/error-string detection with driver error-code handling.
- Reconcile timestamp scanning and comparison behavior.
- Preserve existing integer overflow checks and fixed-point arithmetic.

### Acceptance gate

The same repository contracts work under both backends, with equivalent results for reads, writes, conflicts, nulls, and transaction rollback.

---

## Step 05 — Domain repository migration

**Goal:** Make every business feature PostgreSQL-compatible.

Migrate by functional area, preserving the service layer's public interfaces.

### Recommended order

1. **Identity and settings:** users, sessions, encrypted settings, onboarding, Telegram identity aliases.
2. **Financial:** balances, ledger entries, payment orders, provider callbacks, refunds, and settlement records.
3. **Commerce:** products, combos, purchases, entitlements, renewals, coupons, and rollover.
4. **X-Node:** nodes, identities, periods, traffic usage, report cursors, configuration, routing, and firewall state.
5. **Durable work:** outbox, provider operations, compensation, notifications, and recovery workflows.
6. **Supporting features:** activity, Emby, questionnaires, IP lookup, statistics, and maintenance.
7. **Administration:** table metadata, cursor pagination, mutation reviews, audit, and emergency operations.

### Special treatment: database administration

`internal/platform/databaseadmin/` currently relies on SQLite metadata and `rowid` behavior.

Implement a separate PostgreSQL schema-introspection backend using PostgreSQL system catalogs.

Require declared primary keys or another explicitly safe row identity. Do not emulate SQLite's implicit `rowid`.

Preserve sensitive-column masking, approved mutations, optimistic checks, typed values, confirmations, and audit trails.

Until parity is verified, disable PostgreSQL generic table mutations rather than exposing a weaker administrative editor.

### Acceptance gate

All domain workflows operate with PostgreSQL, and the existing HTTP API and external integration contracts remain unchanged.

---

## Step 06 — Financial transaction concurrency hardening

**Goal:** Preserve financial correctness while allowing independent users to execute transactions concurrently.

**Critical file:** `internal/platform/database/balance_transactions.go`

### Tasks

- [x] Replace global SQLite write serialization with PostgreSQL row-level concurrency control.
- [x] Lock balance rows or use checked atomic updates for debit/credit operations.
- [x] Preserve 64-bit overflow checks.
- [x] Preserve insufficient-balance protection.
- [x] Preserve administrator adjustments and legitimate reversal/debt semantics.
- [x] Ensure balances and ledger records commit atomically.
- [x] Ensure purchases, reservations, entitlements, and outbox jobs are consistent within transactions.
- [x] Preserve payment provider deduplication and unique transaction references.
- [x] Use a consistent lock order for accounts, orders, reservations, and entitlements.
- [x] Audit all check-then-insert and check-then-update logic for races introduced by removing `writeMu`.

### Required hosted CI scenarios

- Two concurrent debits against one account.
- A debit racing against a refund or payment credit.
- Repeated identical payment callbacks.
- Concurrent purchase attempts against limited stock.
- Simultaneous automatic renewal and user cancellation.
- Deadlock and retry handling.
- Integer overflow and negative-balance boundaries.

**Acceptance gate:** No double credit, double debit, negative balance where disallowed, missing ledger entry, or unaccounted reserved stock.

---

## Step 07 — X-Node accounting migration

**Goal:** Preserve every acknowledged traffic report and billing invariant.

**Critical files:**

- `internal/platform/database/xnode_traffic.go`
- `internal/platform/database/xnode_periods.go`
- `internal/platform/database/xnode_barriers.go`
- `internal/xnode/master_receive.go`
- `internal/xnode/periods.go`

### Tasks

- [x] Introduce transaction-level serialization of reports by agent.
- [x] Lock the agent's durable report cursor before evaluating a new sequence.
- [x] Handle missing initial cursors safely under concurrent first reports.
- [x] Require contiguous journal sequences.
- [x] Preserve the digest check for replay of an existing sequence.
- [x] Lock all affected accounting periods in deterministic order.
- [x] Preserve the exact fixed-point multiplier and remainder calculations.
- [x] Preserve separate upload, download, raw, and weighted usage values.
- [x] Advance usage totals and report cursors atomically.
- [x] Send ACK only after successful PostgreSQL commit.
- [x] Preserve late reports, migrated identities, service identities, and metered probes.
- [x] Preserve period closing, successor publication, reset barriers, and incomplete accounting states.
- [x] Ensure unrelated agents can commit independently without violating shared-period locking.

### Required failure tests

- Crash before database commit.
- Crash after database commit but before ACK.
- Duplicate report after reconnect.
- Out-of-order sequence arrival.
- Same sequence with a different digest.
- Concurrent reports affecting one period.
- Identity rekey during report replay.
- Period reset during unfinished traffic accounting.
- Agent reconnection after an interrupted cutover.

**Acceptance gate:** No accepted journal sequence is lost, no report is charged twice, and no successor period is incorrectly blocked or settled.

---

## Step 08 — Transactional outbox and PostgreSQL notifications

**Goal:** Implement safe concurrent job claiming without changing the queue's durability model.

**Critical files:**

- `internal/platform/database/outbox_jobs.go`
- `internal/platform/database/outbox_jobs_part2.go`
- `internal/platform/outbox/worker.go`
- `internal/app/scheduler.go`

### Tasks

- [x] Rewrite outbox claims using atomic PostgreSQL updates and `FOR UPDATE SKIP LOCKED`.
- [x] Preserve job priority, retry policy, max attempts, and error reporting.
- [x] Introduce persistent claim generation/fencing.
- [x] Ensure a recovered job cannot be completed by an older worker attempt.
- [x] Preserve exclusive processing requirements for sensitive job categories.
- [x] Keep priority firewall work separated from ordinary backlog processing.
- [x] Preserve provider-specific concurrency and pacing queues.
- [x] Add PostgreSQL `LISTEN/NOTIFY` to wake the dispatcher after new work commits.
- [x] Use a dedicated PostgreSQL session for `LISTEN`.
- [x] Preserve scheduled polling and recovery when notifications are missed.
- [x] Avoid holding database locks during upstream network requests.
- [x] Keep the scheduler single-owner during the initial deployment.

### Correctness requirement

`NOTIFY` is only a wake-up hint. The durable outbox row is the source of truth.

Do not use Redis Pub/Sub or an independently committed Redis queue for financial or entitlement jobs.

**Acceptance gate:** Concurrent workers cannot claim the same active job, stale acknowledgements cannot overwrite newer attempts, and interrupted work is recovered without duplicate non-idempotent effects.

---

## Step 09 — Backup, restore, and disaster recovery

**Goal:** Replace SQLite file-based backup/restore with PostgreSQL-native recovery.

**Critical files:**

- `internal/platform/backup/`
- `internal/platform/database/database.go`
- `internal/app/application_lifecycle.go`
- `docs/modules/platform-runtime.md`

### Tasks

- [x] Add pgBackRest as an operational integration.
- [x] Configure physical base backups and continuous WAL archiving.
- [x] Configure encrypted backup repositories.
- [x] Add an off-site repository option using S3-compatible storage.
- [ ] Validate Cloudflare R2 compatibility if selected.
- [x] Implement PostgreSQL logical exports for administrative use.
- [x] Define backup retention, recoverability, and storage-capacity policies.
- [x] Expose verified backup metadata through supported administrative APIs.
- [x] Replace SQLite-specific restore semantics for PostgreSQL installations.
- [x] Prevent direct production restore while applications or workers remain active.
- [x] Implement explicit maintenance, restore, and validation steps.
- [x] Retain the old SQLite restore workflow only for SQLite installations.
- [x] Conduct a full backup restoration drill and a point-in-time recovery drill.

### Operational safeguards

Backups must include compatible application configuration and preserve the `CONFIG_MASTER_KEY` through an independently secured recovery process.

A successful backup command is not proof of recoverability; restoration must be verified.

**Acceptance gate:** A separate PostgreSQL deployment can be restored to a verified state, including financial, session, encrypted configuration, and X-Node accounting data.

---

## Step 10 — PostgreSQL observability and optional Redis

**Goal:** Add monitoring and targeted performance integrations.

### PostgreSQL observability

- [x] Enable `pg_stat_statements`.
- [x] Add postgres_exporter.
- [ ] Integrate with Prometheus and Grafana where available.
- [x] Measure connection-pool saturation.
- [x] Measure transaction latency, locks, deadlocks, and long-running transactions.
- [x] Record outbox queue age and claim/retry counts.
- [x] Record known X-Node accounting-barrier backlog and commit-to-ACK latency; full agent journal depth requires agent-side metrics.
- [x] Monitor backup age and WAL archive failures.
- [x] Add alerts for financial reconciliation anomalies, accounting backlogs, deadlocks, and backup failures.

### Redis/Valkey integration (optional, separately gated)

- [ ] Add `REDIS_URL` and a bounded connection pool.
- [ ] Make Redis completely optional in single-instance mode.
- [ ] Implement atomic cross-replica HMAC nonce reservation.
- [ ] Preserve the existing per-session nonce-capacity semantics.
- [ ] Reject signed requests when the shared replay backend is unavailable in distributed mode.
- [ ] Optionally introduce distributed rate limits for expensive endpoints.
- [ ] Add short-lived caches only for rebuildable, non-authoritative data.
- [ ] Implement clear namespace, expiration, invalidation, and eviction policies.

Redis must never store authoritative TXB balances, accounting cursors, purchases, or durable outbox state.

### Deferred integrations

Introduce these only after measured need:

- `sqlc` for selected complex or frequently modified SQL modules.
- PgBouncer for high connection counts.
- `pg_trgm` for measured partial-text search bottlenecks.
- `pg_partman` for sufficiently large time-series/history tables.
- pgAudit for independent DBA-level audit requirements.

**Acceptance gate:** Monitoring distinguishes SQL, lock, external-provider, and queue latency. Redis failure does not corrupt or prevent recovery of durable application state.

---

## Step 11 — SQLite-to-PostgreSQL migration tool

**Goal:** Provide a deterministic, reproducible migration of existing installations.

**Proposed location:** `cmd/db-migrate/` and `internal/platform/datamigrate/`

### CLI requirements

Create a dedicated migration command with the following conceptual operations:

- `inspect`: validate source database and migration compatibility.
- `export`: create a verified consistent SQLite snapshot/export.
- `import`: populate a fresh PostgreSQL target.
- `verify`: compare authoritative data and derived invariants.
- `cutover-check`: validate readiness for production activation.

Commands must be non-destructive by default and must never silently overwrite a populated production database.

### Data migration

- [x] Freeze and verify the exact source schema version.
- [x] Create a verified SQLite snapshot.
- [ ] Perform required source-side identity migrations on the selected source using the compatible old application before export (the CLI currently verifies/refuses noncanonical sources).
- [x] Transfer records in foreign-key dependency order.
- [x] Preserve exact identifiers and encrypted ciphertext.
- [x] Preserve payment idempotency keys and provider event references.
- [x] Preserve immutable ledger, audit, and refund histories.
- [x] Preserve X-Node proxy IDs, accounting periods, journal cursors, multiplier remainders, and usage history.
- [x] Preserve unfinished recovery, rollover, and migration metadata.
- [x] Preserve outbox payloads and retry attempts.
- [x] Classify in-flight operations before resumption; do not blindly replay uncertain provider-side effects.
- [x] Reset PostgreSQL identity sequences appropriately.
- [x] Validate all constraints and relationships after import.
- [x] Compare row counts, identities, hashes, and financial/accounting invariants.
- [x] Produce a signed or hashed verification report.

### Reconciliation checks

At minimum, verify:

- Total and per-user balance consistency with the ledger.
- Payment credit references and idempotency constraints.
- Refund, purchase, and entitlement ownership.
- X-Node cursor and daily usage parity.
- Period status and reset-barrier continuity.
- Outbox state and uniqueness.
- Encryption/decryption compatibility.
- Session and Telegram identity references.

### Acceptance gate

Migration can be repeated from the same snapshot into an empty PostgreSQL database and produces an equivalent verified result without modifying the source.

---

## Step 12 — Hosted CI, staging rollout, production cutover

**Goal:** Release PostgreSQL safely without introducing unrelated feature regressions.

### CI

Extend `.github/workflows/ci.yml`.

- [x] Add a PostgreSQL 18 service for backend integration tests.
- [x] Add fresh-schema and upgrade-migration verification.
- [x] Add repository contract tests for both SQLite and PostgreSQL.
- [x] Add financial transaction concurrency tests.
- [x] Add X-Node accounting replay/crash tests.
- [x] Add PostgreSQL outbox locking and lease tests.
- [x] Add migration export/import and integrity tests.
- [x] Add backup and restoration integration checks.
- [ ] Add optional Redis integration tests.
- [x] Preserve existing race detection, vulnerability scans, and build checks.

Follow the repository's `AGENTS.md` restriction against running local automated test suites. Execute the full automated suites through the permitted hosted CI pipeline.

### Staging rehearsal

- [ ] Deploy a separate PostgreSQL-backed VanSail-Carpool environment.
- [ ] Use a sanitized or access-controlled copy of production data.
- [ ] Verify the administration console and Mini App.
- [ ] Verify balances, payments, purchases, resets, subscription generation, and X-Node control.
- [ ] Validate routing and configuration deployment semantics.
- [ ] Validate interrupted traffic reports and provider operations.
- [ ] Verify actual backup restoration.
- [ ] Measure latency and throughput against the SQLite baseline.
- [ ] Document all discrepancies and resolve critical issues.

### Production cutover

1. Produce a final verified SQLite backup.
2. Enter application maintenance mode and prevent new authoritative mutations.
3. Stop the current scheduler and outbox executors.
4. Quiesce X-Node control and pause new journal acknowledgements, preserving agent-side durable reports.
5. Handle incoming payment/provider callbacks through a verified retry or durable ingress mechanism; do not assume all providers retry.
6. Capture the final SQLite-consistent snapshot.
7. Import into PostgreSQL.
8. Run mandatory verification.
9. Start one PostgreSQL-backed application instance.
10. Reconcile interrupted provider operations before automatic retry.
11. Reconnect X-Node agents and replay unacknowledged journals.
12. Verify financial, subscription, identity, and accounting state.
13. Reopen normal traffic.
14. Monitor reconciliation, queue age, database locks, and backup health.

### Rollback rules

**Before PostgreSQL accepts production writes:** Stop the candidate instance and return to the untouched SQLite snapshot after verifying that no new authoritative effects were accepted.

**After PostgreSQL accepts production writes:** Never revert to an old SQLite snapshot without reconciling later financial, provider, and traffic changes. Prefer PostgreSQL repair, recovery, or a separately validated reverse migration.

Retain source snapshots until the new recovery path and reconciliation history are verified.

### Production acceptance gate

- [ ] Zero duplicate payment settlement.
- [ ] Zero unexplained financial ledger discrepancy.
- [ ] Zero acknowledged-but-uncommitted X-Node reports.
- [ ] Zero missing accepted journal sequences.
- [ ] All idempotency and exclusive job-processing rules preserved.
- [ ] No loss of encrypted settings or stable identities.
- [ ] All supported application/API workflows operational.
- [ ] PostgreSQL backup and point-in-time recovery verified.
- [ ] Database connection and transaction metrics healthy.
- [ ] No unresolved critical regression compared with SQLite.

---

## Pull request structure

Each development stage should result in a focused pull request, with larger stages split internally by subsystem.

| PR | Subject | Dependency |
|---|---|---|
| 01 | Persistence inventory and invariants | None |
| 02 | PostgreSQL Compose and runtime configuration | 01 |
| 03 | PostgreSQL schema + Goose migration engine | 02 |
| 04 | pgx backend and SQL compatibility | 03 |
| 05 | Domain repository conversion | 04 |
| 06 | Financial concurrency hardening | 05 |
| 07 | X-Node journal/accounting hardening | 05–06 |
| 08 | Outbox locking and notifications | 05–07 |
| 09 | Backup/recovery and admin compatibility | 05–08 |
| 10 | Monitoring and optional Redis | 08–09 |
| 11 | Database conversion and verification CLI | 03–09 |
| 12 | Staging, cutover, release checks | All required preceding PRs |

## Definition of done for each PR

- [x] All modified modules have reviewed SQL and Go interfaces.
- [x] Repository invariants remain documented and enforced.
- [x] SQLite legacy behavior is not accidentally broken.
- [x] PostgreSQL-specific operations have appropriate error handling.
- [x] No new unnecessary copies of upstream X-Node or Remnawave state exist.
- [ ] CI validates affected backend paths.
- [x] Security and data-integrity impacts are reviewed.
- [x] Relevant module documentation and operator instructions are updated.
- [x] Changes are limited to the PR's intended scope.
- [x] The PR does not depend on unimplemented future stages for correctness.

## Final architectural requirements

**PostgreSQL owns durable truth.**  
**The SQL outbox owns guaranteed work.**  
**X-Node journals remain safely replayable.**  
**Redis owns only disposable ephemeral state.**  
**The Go service remains modular and backward compatible during transition.**  
**Operational recovery is demonstrated before production migration is considered complete.**
