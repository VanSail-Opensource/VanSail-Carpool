# Platform and runtime module

## Ownership

The platform layer owns configuration, logging, IDs, AES-256-GCM encryption,
explicit SQLite/PostgreSQL lifecycle, HTTP, embedded Vue assets, the durable
outbox and backups. `cmd/server` parses serve/healthcheck and manages Go process
cancellation; `cmd/db-schema` and `cmd/db-migrate` are worker-free operator tools.
The selected [embedded PostgreSQL container](../../deploy/embedded/README.md)
uses Tini and an unprivileged supervisor, a 1 GiB limit, read-only root, preserved
`/data` and separate persistent PostgreSQL/recovery volumes. PostgreSQL starts
before schema work and the Go process; shutdown reverses that order.

Bootstrap environment is intentionally narrow: comma-separated `ADMIN_TELEGRAM_ID`, `TELEGRAM_BOT_TOKEN`, `PUBLIC_BASE_URL`, and `CONFIG_MASTER_KEY` are required; `PORT`, `DATA_DIR`, `TZ`, and `LOG_LEVEL` have documented defaults. Provider credentials and business configuration belong in the encrypted settings registry rather than process environment.

## Persistence contract

The embedded Vite output is copied into a read-only in-memory filesystem before HTTP starts. The asset cache is fully populated during bootstrap and cannot grow during request handling; hashed assets retain immutable cache headers.

- Legacy SQLite defaults to `${DATA_DIR}/tx-carpool.db`; explicit PostgreSQL never falls back to it. Administrative exports live below `${DATA_DIR}/backups`. Durable records are never mirrored wholesale into memory.
- SQLite keeps foreign keys, WAL, bounded caches, file-backed temporary storage and FULL synchronization. Its default pool is four. PostgreSQL uses an 8/4 pool with statement/lock deadlines, SERIALIZABLE write transactions and safe whole-operation retries; embedded tuning preserves durable commits and reserves a 384 MiB Go heap budget at 1 GiB.
- SQLite migrations run before HTTP. PostgreSQL has independent forward-only Goose migrations owned by the migration role. The application role has DML access and verifies schema version; imported targets require exact-hash/key/readiness verification before activation. A dedicated owner session rejects another master; ownership loss stops the runtime.
- Business transactions write durable state and their required outbox job together. The dispatcher routes each claimed job to the handler registered for its exact kind; X-Node, rollover, Emby, and questionnaire workers can never claim one another's work. Workers use bounded leases/transactions, increment attempts, and preserve the last operator-safe error.
- Typed outbox payloads are the only target-ID source. A partial unique index on canonical `(kind,payload)` deduplicates pending/processing work, and processing jobs cannot be deleted. Audit insertion transactionally retains the newest 200 audit events. The platform never offers raw SQL over HTTP.
- X-Node and Emby adapters use bounded provider queues. Workers honor cancellation/backpressure and stop before the selected database closes. PostgreSQL outbox claims use SKIP LOCKED, resource exclusion and durable generation fencing; dedicated LISTEN wakeups supplement polling. Member routing has one reserved execution slot, five-minute attempts, one-minute phase waits and five-to-thirty-second retry delays. Split XHTTP export links never enter monitored Squad membership.
- The embedded filesystem contains the Vite production output. Unknown non-API paths fall back to the SPA entry point; API and operational misses return normal HTTP errors and never the SPA.

Authenticated browser requests carry a timestamp, nonce, and HMAC-SHA256 signature derived from a companion key issued beside the HttpOnly session. Verification hashes the exact body, preserves escaped paths/raw queries, rejects stale timestamps and nonce replay, and restores the body for handlers. Telegram authentication/webhooks, provider callbacks, payment returns, probes, and static delivery are explicit protocol exceptions.

## Scheduler and backup behavior

One application-owned scheduler starts after migrations and stops when the root context is cancelled. It handles queued entitlement transitions, expired access, retryable X-Node work, Stars reconciliation, and the daily backup trigger. Work is idempotent so a crash between provider success and job acknowledgement can be replayed safely.

SQLite backup checkpoints WAL, uses its online-safe backup mechanism, verifies
the temporary file and atomically renames it. PostgreSQL exports use pg_dump
custom archives and pg_restore structural inspection; encrypted physical/WAL
backup uses pgBackRest. Export creation is not proof of recoverability: hosted
full/PITR drills compare all original table hashes. Retention preserves old SQLite
source evidence after PostgreSQL activation. Deletion retains path/audit guards.

SQLite restore retains verified snapshots, rescue backup, staged markers and
startup rollback. PostgreSQL refuses live upload/file-swap restore. Stop HTTP,
workers and the master; restore into a separate volume, verify offline with the
original master key, reconcile provider/journal effects, then activate one app.
See the [cutover](../../deploy/postgres/cutover.md) and recovery runbooks.

## Failure behavior

- `/healthz` requires HTTP and the selected database, independently of external providers.
- `/readyz` returns 503 for database failure or incomplete required dashboard setup and includes non-secret checks.
- Graceful shutdown stops admission, joins canceled workers, closes the selected database and returns wrapped errors. The embedded supervisor then shuts PostgreSQL down durably; either process failing stops the container.
- A malformed bootstrap value fails fast with its environment variable named, but its secret value omitted.
- Decryption failure marks the setting unusable and setup incomplete; the ciphertext and key are never logged.
- Static asset misses cannot shadow `/api`, `/healthz`, or `/readyz`.

## Verification

Explicit PostgreSQL installations use pg_dump logical exports instead of SQLite
file operations. The live API refuses PostgreSQL upload/file-swap restore.
Encrypted physical backups, continuous WAL archiving and stopped-worker recovery
are documented in [PostgreSQL recovery](../../deploy/postgres/recovery.md).
Archive completion is separate from successful restore/PITR verification.

- Migration tests cover a fresh database and reopening an existing schema with foreign keys/WAL enabled.
- Secret tests cover random nonces, authentication failure, invalid base64 master keys, masking, and no plaintext response.
- Scheduler tests use a controllable clock and cover cancellation, duplicate ticks, crash/retry, and bounded backoff.
- Backup tests cover atomic completion, authenticated download, rescue creation, marker validation, staged restore success/failure rollback, single-flight behavior, and retention boundaries.
- HTTP tests cover SPA fallback, API 404 behavior, health/readiness transitions, request IDs, secure headers, and graceful cancellation.
