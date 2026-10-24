# Platform and runtime module

## Ownership

The platform layer owns bootstrap configuration, structured logging, IDs, AES-256-GCM setting encryption, SQLite lifecycle and migrations, HTTP lifecycle, embedded Vue assets, the durable outbox scheduler, and online backups. `cmd/server` only parses `serve` or `healthcheck`, loads configuration, builds the application, and manages process cancellation.

Bootstrap environment is intentionally narrow: comma-separated `ADMIN_TELEGRAM_ID`, `TELEGRAM_BOT_TOKEN`, `PUBLIC_BASE_URL`, and `CONFIG_MASTER_KEY` are required; `PORT`, `DATA_DIR`, `TZ`, and `LOG_LEVEL` have documented defaults. Provider credentials and business configuration belong in the encrypted settings registry rather than process environment.

## Persistence contract

The embedded Vite output is copied into a read-only in-memory filesystem before HTTP starts. The asset cache is fully populated during bootstrap and cannot grow during request handling; hashed assets retain immutable cache headers.

- The authoritative database lives on disk at `${DATA_DIR}/tx-carpool.db`; backups live below `${DATA_DIR}/backups`. It is never mirrored wholesale into memory.
- Every connection enables foreign keys, WAL, a busy timeout, a 16 MiB page cache, a 128 MiB mmap ceiling, memory-backed temporary storage, and `synchronous=FULL`. The pool is fixed at four connections; passive checkpoints run normally and a final checkpoint runs before backup and shutdown.
- Embedded migrations run in order before HTTP readiness. A migration failure aborts startup without partially advertising readiness.
- Business transactions write durable state and their required outbox job together. The dispatcher routes each claimed job to the handler registered for its exact kind; X-Node, rollover, Emby, and questionnaire workers can never claim one another's work. Workers use bounded leases/transactions, increment attempts, and preserve the last operator-safe error.
- Typed outbox payloads are the only target-ID source. A partial unique index on canonical `(kind,payload)` deduplicates pending/processing work, and processing jobs cannot be deleted. Audit insertion transactionally retains the newest 200 audit events. The platform never offers raw SQL over HTTP.
- X-Node and Emby synchronous adapters run through separate bounded FIFO queues with provider-specific pacing. Queue workers share the application context, contain panics, honor caller cancellation/backpressure, and stop before SQLite closes.
- The embedded filesystem contains the Vite production output. Unknown non-API paths fall back to the SPA entry point; API and operational misses return normal HTTP errors and never the SPA.

Authenticated browser requests carry a timestamp, nonce, and HMAC-SHA256 signature derived from a companion key issued beside the HttpOnly session. Verification hashes the exact body, preserves escaped paths/raw queries, rejects stale timestamps and nonce replay, and restores the body for handlers. Telegram authentication/webhooks, provider callbacks, payment returns, probes, and static delivery are explicit protocol exceptions.

## Scheduler and backup behavior

One application-owned scheduler starts after migrations and stops when the root context is cancelled. It handles queued entitlement transitions, expired access, retryable X-Node work, Stars reconciliation, and the daily backup trigger. Work is idempotent so a crash between provider success and job acknowledgement can be replayed safely.

The backup task first checkpoints WAL, uses SQLite's online-safe mechanism, writes a temporary file, verifies that file by opening/checking it, then atomically renames it. The destructive schema migration similarly creates and integrity-checks a pre-migration snapshot before rebuilding tables. Backup deletion is denied while restore is staging/restarting, resolves the stored path beneath the configured backup directory, removes file and metadata consistently, and records an audit event.

A restore accepts only a verified stored snapshot. It creates a rescue backup, checks SQLite integrity and migration compatibility, stages the candidate beside the live database, writes a durable restore marker, returns `202`, and triggers graceful shutdown. On the next startup, the marker is processed before opening the application database: the files are swapped atomically, the replacement is opened and verified, and failure restores the rescue copy. Restore status is recorded for the reconnecting administrator, who must reauthenticate after restart.

## Failure behavior

- `/healthz` returns success when the process can serve HTTP and SQLite responds. It does not depend on external providers.
- `/readyz` returns 503 for database failure or incomplete required dashboard setup and includes non-secret checks.
- Graceful shutdown stops admission, cancels workers, waits within the configured timeout, closes SQLite, and returns wrapped errors.
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
