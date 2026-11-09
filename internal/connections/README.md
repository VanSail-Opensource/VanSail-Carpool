# Administrator connection diagnostics and retirement cleanup

`service.go` exposes only `RequestAdmin`/`PollAdmin` for owner-scoped native
administrator diagnostics. `worker.go` executes through the shared outbox and
native per-node command queue. Scan IPs and signed handles are transient.
`handles.go` and `block_crypto.go` retain upgrade encryption/signature contexts.

`block_service.go` lists existing encrypted targets and queues administrator
unblocks. `block_contracts.go` defines the retained interfaces.
`unblock_worker.go` and `block_expiry.go` reconcile existing restrictions. Offline
cleanup returns deferred capacity status and preserves encrypted targets until
node confirmation, including after repeated failures. No member route admits
scans, drops, blocks or unblocks; no block/drop executor is registered.

Migration 36 records retirement, fails obsolete operations without changing
terminal receipts, advances firewall/config revisions, and makes all existing
manual blocks due for queued cleanup. Payment, audit and ledger evidence remain.
The X-Node module owns member observations and free volatile single-IP grants.

`types.go` retains administrator contracts. `scoped_control.go` scopes administrator control. `admin_target_scan_test.go`, `block_worker_test.go`, `handles_test.go`, `native_scan_test.go`, `unblock_expiry_test.go` and `worker_test.go` cover authorization, native targets, receipts and retained offline cleanup.

`block_crypto.go` uses the shared read-only historical-owner resolver for retained
IP targets. `block_legacy_scope_test.go` and `block_legacy_expiry_test.go` verify
that recovered targets still use the current owner's scoped native control and
preserve ciphertext on refusal.
Alias evidence is limited to opening the exact node/digest; native authorization
and offline cleanup dependencies remain unchanged.
