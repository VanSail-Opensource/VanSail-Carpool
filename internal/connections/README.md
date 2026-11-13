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

The retained-block service no longer requires the retired block-admission or scan
repository methods, an unused signer, or the obsolete block-intent fingerprint.
Administrator diagnostics retain their own signer and queue; listing, encrypted
target recovery, explicit unblocking and expiry cleanup remain supported.

Connection polling distinguishes missing/foreign/expired records from transient
storage failures. `ErrScanUnavailable` preserves the cause for HTTP retry mapping.
The worker reads its local owner before recording any external start intent;
`StartConnectionScan` admits only a currently queued row in one transaction.
A competing start returns a conflict before provider work. Already-processing
receipts keep their existing job or ambiguous-start review behavior.
`recovery_test.go` covers transient recovery and unattempted owner reads;
database lookup and concurrent-worker regressions verify the persistence fence.

Admission also rejects expired requests and unavailable owners. The worker uses
a fresh clock after owner reads and honors the returned terminal record before
calling a provider. `admission_test.go` covers both refusals and a concurrent
terminal unblock claim; neither path repeats or overwrites the durable outcome.

`admin_authority.go` binds fresh scan requests and both sides of result projection
to the requesting administrator's original account generation and current active,
unblocked role. Fresh SQL admission checks that actor again while holding account
locks and commits actor audit attribution with the scan/outbox. Already accepted
read-only scan work keeps its original recovery semantics after later demotion;
polling always requires current authority before returning transient IPs.
`admin_authority_test.go` covers revoked admission and demotion during projection.
