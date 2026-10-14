# Provider migration

This module owns resumable Remnawave preview, import, node handover and rollback.
`Source` confines legacy reads and mutations to the existing upstream queue.
`Repository`, `Panel`, `Controller` and `Backups` make ownership changes and
failure paths explicit and independently auditable.

Preview stores a bounded encrypted snapshot and digest. Import requires a verified
panel backup, preserves UUIDs and compatible passwords, imports daily history
separately from weighted quota counters, and publishes each node together with
its legacy ownership checkpoint. Replaying import preserves operator edits and
completed handovers. Monitoring accounts require explicit mappings and receive
only their imported group bindings.

Cutover verifies the running agent against its signed release manifest, validates
complete future server configuration and client endpoints, and obtains a backup
before disabling the old node. Every ambiguous disable or subsequent failure
attempts rollback. Rollback removes native listeners, waits for applied state,
flushes the journal and refreshes combined quota policy before reopening the
source. Failed recovery remains in `review` and may be resumed by an operator.

Commands use provider-operation receipts and bounded controller tasks, retaining
durable leases without occupying purchase dispatch. Receipts contain action and
ownership metadata only. Live connections and telemetry never enter snapshots,
receipts or audits. Production handovers require compatible signed agents. The
bridge remains enabled until verified final source accounting marks migration
complete; initial import does not imply financial settlement.

- `types.go`: Defines source snapshots, mappings and dependency contracts.
- `preview.go`: Captures encrypted previews and reports unsupported data.
- `configuration.go`: Selects active inbounds, strips account lists and maps tags.
- `import.go`: Resumes configuration, group, account and endpoint import after backup.
- `endpoints.go`: Preserves host relationships and client transport overrides.
- `history.go`: Reconciles source node series with daily raw aggregates.
- `monitoring.go`: Applies explicit monitoring account eligibility.
- `cutover.go`: Transfers ownership with recovery on every post-disable failure.
- `rollback.go`: Seals native usage before restoring source listener ownership.
- `operations.go`: Queues and executes migration actions with durable receipts.
- `cutover_test.go`: Covers preflight, interrupted handovers and resumed rollback.

- `history_test.go` covers complete source series reconciliation and fixed-point multiplier conversion.
