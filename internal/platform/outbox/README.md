# Durable outbox worker

- `worker.go` registers typed handlers and drains claimed durable jobs with bounded retries and persisted outcomes.
- `worker_test.go` verifies dispatch, unknown-kind handling, failure persistence, and registration validation.

- `deferred_test.go` verifies that long node commands retain their lease while purchase dispatch continues.

`priority.go` reserves admission for dynamic firewall jobs through the same
repository, handlers, leases and retry path. The application polls this claim
path and ordinary admission once per second independently of scheduled
maintenance work. Ordinary claims prefer other work before dynamic bans. PostgreSQL
transactions prevent ordinary and reserved dispatch from claiming a job twice;
native executor limits still bound actual execution. It is not a separate queue.

`executor.go` enables four normal attempts and one urgent slot after application Start. Attempts have five-minute contexts, 20-second fenced lease heartbeats, panic isolation and joined shutdown. Embedded callers retain synchronous Drain. Deferred node commands retain the native executor lease.

`executor_test.go` checks independent completion and cancellation joins without relying on timing sleeps.

Admission reserves a lane slot and joins the lifecycle before the database
claim, then releases the lifecycle mutex while PostgreSQL responds. A slow
ordinary claim cannot stall urgent/compute claims or shutdown cancellation.
Shutdown also joins claims still waiting on the database; a late claimed job
is completed for recovery without invoking its handler. These boundaries are
covered by `claim_isolation_test.go`.

`claim.go` requires durable generation-aware completion and renewal. PostgreSQL claims serialize only resource
admission, lock candidates with SKIP LOCKED, then release locks before providers.

- `compute.go`: Independent compute execution lane using the existing durable job infrastructure.

`wait.go` defines dependency waits with explicit retry delays, terminal failures and operator-review failures. Dependency waits preserve attempt counts; provider delays are bounded to 24 hours. Permanent failures retain their failed row. These classifications do not authorize account deletion or replay uncertain financial/native commands.
