# Node compensation

This package owns outage policy validation, complete-snapshot observation,
inbound-to-squad mapping, strict-threshold calculation, and reviewed event
commands. SQLite freezes squad identities and recipient user IDs; public
projections expose counts only. X-Node reads remain behind the application
adapter's provider queue, while approved exact-state writes use the shared
provider-operation worker.

- `types.go` defines configuration, observation, event, page, and review contracts.
- `service.go` validates revisioned configuration and idempotent review commands.
- `observer.go` accepts only complete node/squad samples and maps documented inbound UUIDs.
- `calculation.go` performs overflow-safe floor rounding and the extension cap.
- `calculation_test.go` covers strict equality, floor rounding, no-recipient precedence, and caps.
- `observer_test.go` covers incomplete provider snapshots, inbound intersection, and squad deduplication.

Scheduled maintenance removes event details seven days after creation, including open/reviewed events. The next complete observation starts a new window for an ongoing outage. Approval applies extensions locally and stores exact-state user targets in durable operations; those targets and approved purchase dates remain valid after event detail retention.
