# Native period closure

`internal/xnode/period_closure.go` separates admission fencing from settlement.
The database fence runs before releasing `Service.mu`; synchronization, probe
flushes, and committed traffic barriers then run without that global lock.
Only recorded participants can settle. Missing agents and uncertain commands
retain closing periods and their original accounting evidence.

Refund quiescence and reconciliation use `period_strict_closure.go`. Their
captured target includes the native identity, purchase, period, and durable
closure generation. Success requires the same owner and current period after
settlement. A newly paid successor remains an independent period and is never
selected by its predecessor's flush. Compatibility term settlement captures
only the selected purchase's periods. Already-closed snapshots receive the same
final target check. Refund aggregation retains that captured generation, checks
it again after history reads, and requires every same-purchase period closed
before a quiesced aggregate can qualify for settlement.

`BeginXNodePeriodClosure` atomically changes open to closing and returns the
incremented generation. `SealXNodeClosingPeriodNode` locks that exact closing
generation before publishing participant evidence; `FinalizeXNodePeriod` checks
the same generation and all participant barriers. Reopening clears participant
markers for nodes that may admit traffic again. Previously sealed evidence for
a disabled node with a durable removal or retirement fence remains unchanged;
unconfirmed evidence never becomes sealed. Even if two closures share a
timestamp, an old barrier cannot seal or
finalize the newer generation. Migration 41 adds only this control metadata;
existing closing generation zero and all purchase/financial facts remain valid.

Node removal uses the separate typed `SealXNodeRemovalPeriodNode` contract in
`xnode_period_seal.go`. It locks the exact period owner/purchase and permanently
disabled node fence, then validates native agent, node-bound legacy cursor and
boot-stream evidence before preserving a participant marker. It can drain an
open member period without closing that member's access elsewhere. Both sealer
contracts preserve closed-stream surviving sequence semantics and reject future
or foreign-node evidence. The old unqualified sealer APIs are removed.

The signed agent's `Registry.QuiescePeriod` rejects a flush when the same period
already admits traffic. If a flush began first, its temporary period fence waits
only for captured old handlers and releases after capture or cancellation. A
different accounting period remains admitted. The panel also rechecks the
closure before dispatch and rejects late evidence after reopen; it never treats
a timeout as proof that a native command did not execute.

Successor and scheduled cadence paths retain their existing nonblocking close:
they leave unflushed participants to the bounded, rotating recovery worker.
Purge closes each selected owner's periods without the shared mutation lock.

Hosted regressions in `period_strict_closure_test.go` hold an agent flush while
an unrelated account mutation completes, cancel unfinished settlement, and
reopen during a late barrier. `xnode_period_closure_test.go` covers durable
generation recovery and both legacy/stream barriers at identical timestamps.
`member_refund_snapshot_test.go` reopens real stored periods between closed
capture and return or history aggregation. `xnode_period_retirement_test.go`
checks removing/retired proof retention and genuine offline dependencies;
`xnode_period_seal_test.go` checks cursor ownership and ordinary disabled nodes.
The native guard also has signed-agent regression coverage in
`internal/sessions/period_barrier_test.go`. Local automated suites are prohibited.
