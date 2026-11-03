# Platform and runtime module

The platform owns configuration, logging, encryption, PostgreSQL lifecycle,
embedded Vue assets, durable outbox execution and backups. `cmd/server` starts
HTTP and workers; `cmd/db-schema` performs worker-free migration, verification and
recovery fingerprinting. Bootstrap validates required environment values without
printing secrets. Business credentials belong in encrypted application settings.

## Persistence and startup

`DB_DRIVER` defaults to `postgres` and accepts only PostgreSQL. A configured URL
or URL file is required. Missing/unavailable PostgreSQL fails startup clearly.
The [embedded deployment](../../deploy/embedded/README.md) starts PostgreSQL before
schema work and the application, using preserved data and recovery volumes,
restricted roles, a read-only container root and a 1 GiB memory limit.

The default pool is eight open/four idle connections. Statement/lock deadlines,
serializable write transactions and bounded whole-operation retries limit
contention. Native SQL and pgx row types have no translation layer. Numbered
Goose migrations remain forward-only; applied PostgreSQL history is immutable.
The migration role owns schema changes, while the application role verifies the
schema and performs DML. A dedicated ownership session fences another master;
ownership loss stops the runtime.

Business changes and required outbox jobs commit together. Claims use SKIP
LOCKED, resource exclusion and durable generations. LISTEN notifications only
wake polling. X-Node, Emby and other provider calls enter the existing bounded
execution queues after commit. Worker cancellation and joined shutdown precede
database closure. Credentials, committed money, accounting and desired state
remain durable; live observations do not become database mirrors.

## HTTP and scheduling

Hashed assets have immutable cache headers; the embedded Vite output is loaded
once into bounded memory. Non-API routes fall back to the SPA. API and health
misses retain HTTP error semantics. Signed browser requests bind exact bodies,
paths, timestamps and nonces to an independent browser session. Protocol-specific
authentication/callback routes retain explicit signing exceptions.

One application scheduler handles entitlement transitions, expired access,
retryable provider work, Stars reconciliation and daily exports. The durable
outbox preserves work across crashes. `/healthz` checks the application and
PostgreSQL; `/readyz` also requires necessary dashboard setup.

## Backup and recovery

Administrative exports use pg_dump custom archives and pg_restore structural
inspection, with authenticated download, guarded deletion and audit history.
Encrypted physical backups and continuous WAL archiving use pgBackRest. Archive
creation does not establish recovery success. Hosted full/PITR drills compare
exact native table and sequence fingerprints, ledger invariants and encrypted
contexts using the independently secured original master key.

Recovery runs with HTTP, workers and the master stopped, into a separate volume.
Verify the restored database and reconcile upstream effects before activating
one application. The live API has no upload or file-swap restore. Historical
audit/financial records are retained. See [PostgreSQL recovery](../../deploy/postgres/recovery.md).

Hosted validation covers fresh/populated upgrades, restart persistence,
restricted roles, ownership, commerce concurrency, logical/physical recovery,
supervision, resource limits and HTTP contracts. Local automated suites remain
prohibited.
