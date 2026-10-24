# PostgreSQL cutover and rehearsal

Repository code and synthetic CI evidence are not production acceptance.
Record the source image/schema, final snapshot hashes, protected master key,
callback ingress policy, operator, target, measured downtime and recovery results
in the deployment change record. Do not log live database URLs or raw reports.

The image contains `db-schema` and `db-migrate`. Both use database-only
configuration and do not start HTTP, providers, schedulers or the X-Node master.
Use a protected `DATABASE_URL_FILE` for the selected role. Migration commands use
the schema owner; the application uses the restricted DML role with
`DB_MIGRATE_ON_START=false`. No command automatically selects a backup database.

## Rehearsal

1. Select an isolated staging stack with outbound upstream effects disabled by
   environment/network policy. Complete source-side identity migrations with
   the compatible SQLite release before freezing an access-controlled copy.
2. Run `db-migrate inspect --source ... --output new-report.json`, then `export
   --source ... --snapshot new-snapshot.db --output new-export.json`. Output
   files must not exist; the source remains read-only. Secure the snapshot and
   CONFIG_MASTER_KEY separately. Compare source sizes, row counts, backlog,
   transaction latency and lock contention with the baseline report.
3. Start PostgreSQL 18 using `compose.yml` plus `compose.recovery.yml`. Initialize
   pgBackRest. Run `db-schema migrate` and `db-schema verify` with migration-role
   credentials. The target must contain only fresh bootstrap rows.
4. Run `db-migrate import --source new-snapshot.db --output new-import.json`,
   `verify --source new-snapshot.db --output new-verification.json` with the
   original master key, then `cutover-check --output new-readiness.json`.
   Import is one transaction, never overwrites an active/populated target, and
   validates deferred FKs before commit. Hashes cover all original columns,
   ordering keys and deleted identity high water marks. Verification checks all
   exact records, immutable histories, financial totals and encrypted contexts.
5. Demonstrate logical/full/PITR restoration using [recovery.md](recovery.md).
   Select the off-site repository and verify that provider's actual restore.
6. Set `VANSAIL_IMAGE` to the reviewed image digest. Create protected
   `secrets/application.env` containing the existing bootstrap variables and
   `secrets/application_url` containing the application-role URL using hostname
   `postgres`. Merge `compose.application.yml` and start one `application` with
   that profile. Its `/data` uses the existing `tx-carpool-data` volume, while
   PostgreSQL has a separate version-aware database volume. Migrate assets and
   preserve source snapshots when staging on a separate host.
7. Verify admin/member access, balances, payments, purchases, resets,
   subscriptions, native config/routing, journal replay and interrupted provider
   work using isolated/synthetic upstreams. Check mobile/desktop flows and
   compare latency/throughput. Resolve discrepancies before production.

Startup refuses imported targets that have not passed verification, changed
verified records, mismatched keys and processing provider/outbox work. Reconcile
uncertain work with the old application and supported provider APIs before taking
a new final snapshot. Do not force status values with SQL. The first accepted
startup marks the target active before bootstrap mutations. A dedicated session
lock rejects another application; loss of that session stops the runtime. Keep
one deployment replica: this is not an HA fencing or failover system.

## Production window

Block new authoritative mutations, stop the old application/workers/master and
verify they are stopped. Keep agent journals intact and stop acknowledging new
reports. Protect callbacks using a provider-specific verified retry or durable
ingress mechanism: a reverse-proxy 503 by itself does not prove callback safety.
If a provider cannot replay and durable ingress is unavailable, do not cut over
while it can deliver callbacks. Capture the final source snapshot only after
those controls are established, then repeat import/verify/readiness against a
fresh target. Retain the untouched source and independently protected snapshots.

Start one candidate, reconnect agents, replay unacknowledged reports and reconcile
provider effects. Check zero duplicate settlement, zero unexplained ledger
difference, contiguous durable journal cursors, paid successors, subscriptions,
leases and outbox exclusivity. Reopen traffic only after those checks and the
selected recovery path pass. Monitor queues, accounting, locks and backups.
PostgreSQL automatic retention preserves legacy SQLite backup files/records;
remove retained source evidence only through a deliberate reviewed operation.

Before candidate activation/writes, rollback can return to the untouched source
after verifying no new authoritative or provider effects occurred. After
PostgreSQL accepts writes or effects, do not revert to an old SQLite file. Repair
or recover PostgreSQL and reconcile later financial/provider/journal changes.
A reverse migration requires its own separately verified plan.
