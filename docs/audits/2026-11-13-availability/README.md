# Availability, efficiency and interface audit

## Scope and evidence

This follow-up audits all thirteen requested areas against Carpool baseline
`985958f9bea54bd6145f27dd0f5b09ab0841609e` and X-Node baseline
`41f04f945454282e3bfd6dde7e638bd29cfb9b0f`. The merged full-file inventory in
`../2026-11-13/` supplies the prior repository-wide review; this report records
the additional area-focused investigation, fixes and independent cross-review.
It does not claim every unchanged file was reread. Final commit identifiers,
hosted CI runs, merge results and signed release provenance belong to the
delivery pull requests and release.

Detailed findings are in [backend.md](backend.md), [frontend.md](frontend.md)
and the matching X-Node `docs/audits/2026-11-13-availability.md`.

## Required-area coverage

| Requested area | Audited boundary and outcome |
|---|---|
| Queue failures and high pressure | Outbox claim calls no longer hold the lifecycle mutex across database waits. Lane reservations join shutdown before the lock is released, and cancellation fences late claims. X-Node repeatable jobs bypass unrelated generic-history recovery/capacity, with independent bounded accounting and inspection workers. Overflow retains explicit recovery instead of silently accepting lost durable work. |
| Feature availability | Independent X-Node workspace tabs load only their dependencies; optional metrics failures do not block configuration/recovery. Server enrollment I/O no longer holds the fleet mutex. Native runtime, state actor, traffic acknowledgments and critical command results retain separate progress paths. |
| X-Node failed-task isolation | Repeatable desired-state work uses its native replay authority instead of permanently occupying generic uncertain-history slots. Unknown utilities reject before admission. Cancelled lock waiters leave promptly; scheduled geodata network transfers occur in private staging outside activation locking. Non-repeatable uncertain side effects still require exact reconciliation. |
| Database write/read pressure | Removed the redundant process-wide write mutex; retain PostgreSQL row/transaction/CAS constraints. Group writes and exact retention deletes batch 512 records; purchase squad hydration batches 256 purchases. UPDATE RETURNING avoids a second query and preserves the winning statement's result. |
| Unnecessary database storage | No new tables, columns, persistent telemetry cache or mirrored provider catalog. Metrics/source-IP observations stay volatile. Existing bounded retention uses batch deletes, and X-Node removes redundant generic desired-state receipts while retaining native financial/recovery evidence. |
| Master down/restart | Ordinary accepted Xray state remains serving. The control transport's hourly negotiation is distinct from existing local authority. Unreachable masters do not prematurely revoke reserved bytes or unexpired original rate grants; session/day/paid/health boundaries still apply. Successful reconnection fences and reconciles old authority. |
| Protocol efficiency | Preserved exact integer encoding, negotiated bounded compact/compressed frames and weighted lanes. Identical queued immutable traffic replay is coalesced. Command receipts no longer wait for state database writes; generic state errors retain wire order. Strict current DNS/settings schemas remove inert fields. |
| Input validation and domain coverage | Token validation rejects ambiguous duplicate JSON without erasing case-sensitive map keys. Node writes accept six editable fields. Current DNS/DDNS inputs reject retired or unknown fields. Native DDNS validates IDNA/apex/wildcards, label-boundary ownership, address family/source, HTTPS lookup and TTL before provider I/O. Routing grammar was compared to the pinned Xray parser; a reused protocol matcher removes repeated compilation without changing semantics. |
| Frontend layout | Long node names wrap, status/actions remain separate, and mobile rows stack without hiding recovery actions. Existing console/member roots remain separate. Desktop fixture inspection and hosted responsive screenshots cover the changed layouts. |
| Counter-intuitive UI/UX | Missing/stale observations no longer falsely imply offline status. Deep links retain all supported tabs. DDNS allows explicit zones when discovery is unavailable and reports field errors before save. Failed saves retain drafts; successful close no longer flashes a false load error. Settings writes require a successful current read. |
| Performance and efficiency | Eliminated shared SQL waiting, N+1 ownership hydration, repeated retention round trips, repeated metrics scans and repeated protocol regex compilation. Polling backs off, pauses when hidden and runs only for consuming tabs. No production latency or throughput claim is inferred from static changes. |
| Business loopholes | Reviewed purchase/coupon/quote/replay/refund authority and immutable financial history. Corrected duplicate-object validation, exact command replay binding and returned-record races. Offline hourly use spends only already reserved credit, without extending payment or borrowing expired shared rate. Failed required reads cannot overwrite settings with uninitialized defaults. Stable editor creation IDs prevent duplicate node/group/endpoint rows after a lost response; configuration/snippet recovery accepts only matching same-ID content without bypassing revision checks. |
| Retired code/backward compatibility | Removed public node multiplier and server-owned write fields, live DNS mode/upstreams/legacy marker, singular DDNS bindings, obsolete database PUT alias, unused repository adapters and fixture-only entitlement UI/client behavior. Narrow stored-state normalization remains necessary to preserve accepted configurations and financial recovery. |

## Concurrency changes

Outbox admission now reserves both the lane slot and the active-operation count
under the lifecycle mutex, then releases that mutex before the database claim.
Shutdown can cancel a blocked claim and join it. A late database result cannot
start a provider after shutdown, and its completion still records recovery.
`internal/platform/outbox/claim_isolation_test.go` covers both paths.

Enrollment gates include waiters in their reference count and remove the entry
after the last holder/waiter leaves. Rotation, deletion and handshake of one node
serialize without blocking another node's status or transport. Successful
registration reacquires the fleet lock only to publish the authenticated peer,
checking service lifetime before adding its goroutines.

The independent receipt lane preserves accepted/result/progress order for each
command. Cross-review caught and fixed a state-error ordering issue: an error
without a pending command ID remains behind earlier applied-state frames. A
blocked older database ACK therefore cannot erase the later rejection.

## Verification and operating boundaries

Carpool's repository policy forbids local automated suites. Local verification
uses diff/control-flow review, gofmt, Go build/vet, frontend typecheck/lint,
contract generation and production build. Authored regression suites run through
hosted Go race/PostgreSQL/frontend/browser/memory gates before merge.

Manual desktop browser inspection used the actual development fixture and
checked populated/loading/empty/unavailable states, feature isolation, deep-link
reload, long-name layout, DDNS bounds, failed-read write protection and retained
failed-save input. The environment blocks browser DevTools device emulation;
mobile evidence comes from the hosted real-browser screenshot gate. Extension
metadata warnings were distinguished from application errors.

Database outages still require authoritative storage to recover; removing a
shared mutex does not make writes independent of PostgreSQL. Queue/lane bounds
remain explicit. Unknown non-repeatable side effects are never reported as
successful or refunded without evidence. Ordinary unacknowledged native traffic
retains the documented bounded volatile-stream loss boundary.

Hourly reduced-rate leases remain limited to their original ten-second demand
deadline. Existing reserved full-speed bytes can continue offline only until
exhaustion, paid expiry, UTC midnight, health failure or authority change. There
is no unlimited offline quota and no reuse of unsealed credit after process
restart. These limits avoid selling the same shared capacity twice.

Use the coordinated current master before deploying the new X-Node release:
new live DNS input no longer accepts retired fields. Existing accepted native
snapshots and encrypted panel settings normalize on read without requiring an
extra startup write; historical accounting remains immutable.
