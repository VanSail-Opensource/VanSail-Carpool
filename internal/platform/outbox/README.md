# Durable outbox worker

- `worker.go` registers typed handlers and drains claimed durable jobs with bounded retries and persisted outcomes.
- `worker_test.go` verifies dispatch, unknown-kind handling, failure persistence, and registration validation.

- `deferred_test.go` verifies that long node commands retain their lease while purchase dispatch continues.

`priority.go` reserves admission for dynamic firewall jobs through the same
repository, handlers, leases and retry path. The application polls this claim
path once per second independently of long provider/maintenance work. SQLite
transactions prevent ordinary and reserved dispatch from claiming a job twice;
native executor limits still bound actual execution. It is not a separate queue.
