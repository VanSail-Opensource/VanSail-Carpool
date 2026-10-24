# Durable outbox worker

- `worker.go` registers typed handlers and drains claimed durable jobs with bounded retries and persisted outcomes.
- `worker_test.go` verifies dispatch, unknown-kind handling, failure persistence, and registration validation.

- `deferred_test.go` verifies that long node commands retain their lease while purchase dispatch continues.

`priority.go` reserves admission for dynamic firewall jobs through the same
repository, handlers, leases and retry path. The application polls this claim
path and ordinary admission once per second independently of scheduled
maintenance work. Ordinary claims prefer other work before dynamic bans. SQLite
transactions prevent ordinary and reserved dispatch from claiming a job twice;
native executor limits still bound actual execution. It is not a separate queue.

`executor.go` enables four normal attempts and one urgent slot after application Start. Attempts have five-minute contexts, 20-second fenced lease heartbeats, panic isolation and joined shutdown. Embedded callers retain synchronous Drain. Deferred node commands retain the native executor lease.

`executor_test.go` checks independent completion and cancellation joins without relying on timing sleeps.

`claim.go` chooses durable generation-aware completion/renewal when supported,
while retaining legacy test adapters. PostgreSQL claims serialize only resource
admission, lock candidates with SKIP LOCKED, then release locks before providers.
