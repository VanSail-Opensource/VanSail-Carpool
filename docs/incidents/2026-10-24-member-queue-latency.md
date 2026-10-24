# Member deployment queue latency

## Production evidence

Read-only administrator API inspection on 2026-10-24 around 22:24 Shanghai time
found four pending provider jobs, two with `context deadline exceeded`. The two
timed-out receipts were `xnode_diy` whitelist changes. Two firewall exemption
receipts were still queued. There were 49 pending firewall jobs in that snapshot.
All 18 nodes had matching desired/applied revisions; subsequent metrics reported
all 18 connected, ready and synchronized. This does not establish that each
member-specific whitelist barrier had completed.

No production settings, jobs, financial records or accounting were modified.
The requested follow-up scope was static analysis without SSH.

## Confirmed implementation bottlenecks

- Both ordinary and reserved claims preferred dynamic firewall jobs, allowing a
  continuously replenished ban backlog to delay unrelated ordinary work.
- Only reserved firewall admission had an independent one-second pump. Ordinary
  admission depended on the main scheduler progressing past scans/maintenance.
- The native executor admitted only one member deployment globally, even though
  transactional outbox resource keys already exclude concurrent same-member work.
- Whitelist publication waited for each node before publishing to the next one,
  consuming the common one-minute deadline and delaying later nodes.

These explain avoidable queue and publication delays. The original generic
timeout did not identify the node or stage, so a separate production runtime
blocker cannot be excluded from these API snapshots alone.

## Repair and interfaces

The existing application admission pump now drains both reserved and ordinary
claims once per second. Ordinary claims prefer other work before dynamic bans;
the reserved firewall path remains available, and idle ordinary capacity can
still deliver dynamic jobs. No second queue or persisted priority state is added.

Native deferred execution admits four independent member operations alongside
the existing ordinary/firewall limits. Durable ownership, resource exclusion,
claim fencing, heartbeat, retries and cancellation remain authoritative.

`syncDeploymentNodes` in `internal/xnode/master_access.go` shares the routing and
whitelist publish-all-then-wait flow. All applied-state barriers remain required.
Errors preserve cancellation and identify sync/apply/capture stages and node IDs;
whitelist confirmation timeouts identify pending nodes without exposing IPs.

No upstream command, signed-agent pin, database schema or frontend contract is
changed. Extension points remain registered outbox handlers and transactional
resource keys; new integration calls must continue through existing executors.

## Validation

Local validation is static only (`gofmt`, `go vet`, diff and control-flow review).
Hosted regressions cover ordinary/reserved admission under repeated busy retries,
ordinary dispatch without scheduled drains, bounded member capacity, and a held
node acknowledgement that must not block publication to later nodes or allow
premature deployment completion. Existing same-resource and routing-phase
regressions continue to apply. Production latency after deployment requires a
fresh member operation and receipt observation; source review cannot prove it.
