# Provider operations

This package defines provider-neutral command, item, replay, and connection-scan
metadata. PostgreSQL persists request fingerprints and sanitized lifecycle facts;
the outbox performs external calls. Raw connection IPs and signed connection
handles remain short-lived service data and are never stored in these records.

- `types.go` defines operation states, commands, receipts, items, and results.
- `connections.go` defines metadata-only connection scan inputs and progress.
- `validation.go` canonicalizes commands and bounded JSON result objects.
- `dispatcher.go` routes the one shared outbox lane to kind-specific handlers.
- `dispatcher_test.go` covers kind routing without provider calls.
- `dispatcher_fence_test.go` verifies that lifecycle-denied unfinished receipts
  retain their outbox continuation as a dependency wait. The dispatcher does not
  issue new mutations while fenced or infer failure from missing authorization;
  succeeded, failed and compensated receipts remain safe no-ops.
- `dispatcher_completion_test.go` covers durable receipt reloading after handler
  completion. Review/partial outcomes retain encrypted targets in failed jobs
  for operator reconciliation; unfinished receipts remain dependency waits.

`telegram_pm.go` reserves relay/profile/repair kinds in the existing durable lane. Targets store conversation and Telegram message references; no message body is part of an operation.

`admin_kinds.go` defines supported administrator command kinds. Retired mutation kinds are no longer dispatched.

`command_kinds.go` defines subscription, Emby, retry, and refund command identifiers.

`validation_test.go` verifies exact nested numeric result preservation and the
single bounded JSON-object contract. Canonical durable results never round
integer counters or decimal evidence through floating-point conversion.
