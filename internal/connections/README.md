# Member connections

- `types.go` defines transient scans plus active three-day block projections and lifecycle constants.
- `handles.go` signs owner, scan, node, IP, and expiry into a 15-minute HMAC capability accepted by block operations.
- `service.go` starts and polls metadata-only scans, then attaches ephemeral handles.
- `worker.go` starts provider scans and moves interrupted or ambiguous starts to durable `pending_review` without retrying the provider call.
- `block_service.go` verifies signed targets, stores only keyed digests plus encrypted active IPs, lists owner blocks, and queues owner/admin unblocks.
- `block_crypto.go` canonicalizes IPv4/IPv6 targets and derives keyed HMAC digests while retaining the legacy drop encryption context.
- `block_worker.go` accepts the plugin block before any disconnect; `block_disconnect.go` bounds drop reconciliation and retains successful blocks on partial outcomes.
- `unblock_worker.go` removes accepted manual blocks and cancels their scheduled cleanup.
- `block_expiry.go` performs the scheduled unblock backstop and scrubs encrypted rows after bounded retry exhaustion.
- `drop_worker.go` drops once, then reconciles uncertain outcomes through a fresh scan.
- `drop_reconcile.go` bounds fresh-scan polling and checks the selected node/IP.
- `block_service_test.go`, `block_worker_test.go`, and `unblock_expiry_test.go` cover encryption boundaries, ordering, timeout, classification, manual deletion, retry eligibility, and expiry scrubbing.
- `handles_test.go` covers owner binding, tamper rejection, and strict expiry without making provider calls.
- `worker_test.go` covers ambiguous start responses, interrupted starts, terminal polling projection, and replay suppression.
- `admin_target_scan_test.go` verifies profile-target scoping on scan polling.

Provider scans, plugin execution, and drops are invoked through the shared Remnawave queue. Browser payloads never choose a node or IP directly.

- `scoped_control.go` dispatches IP mutations with authenticated user ownership.

Administrator scans use `RequestAdmin`/`PollAdmin` with a native-only durable request fingerprint. The existing outbox worker calls `NativeRemote` with a panel identity and never falls back to legacy scans; old bridge receipts are unavailable in this browser flow. Raw observations remain ephemeral and unavailable nodes carry an explicit flag.

`native_scan_test.go` guards panel-identity native outbox dispatch, retry isolation and refusal of bridge scan polling from browser admin.
