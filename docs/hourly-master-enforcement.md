# Master hourly enforcement

Task 5 adds enforcement capability while hourly commerce is unavailable. No
existing purchase, native identity or legacy billing mode enters hourly mode.
`HourlyPolicyRepository.HourlyEnforcementPolicies` is an optional authoritative
commerce interface. Until implemented, its missing/empty policy view creates no
rate or funded credit eligibility, even when transport capability is available.

`HourlyTrafficHistoryRepository.HourlyTrafficHistoryReady` is a separate,
memory-only admission gate. It may be implemented only when accepted hourly
samples and dropped-history coverage persist atomically with native traffic
receipts and replay deduplication. Production `Store` does not implement it yet;
Task 6 must add that writer before enabling the gate. Credit-settlement
construction remains available, but neither live hourly negotiation nor the
historical receipt header is advertised while this contract is unavailable.
Mixed reports are rejected before persistence/ACK, preserving their whole journal
instead of silently acknowledging hourly evidence that has no durable writer.

`HourlyAccountPolicy` supplies the existing opaque user ID, account generation,
paid-through boundary, raw reduced bytes/second, daily allowance and per-node
paid-through boundaries. Cached snapshots clone the node map, expire after four
seconds and clear after query failure. Generation, owner and participating node
must match before a demand enters the allocator. Credit progress and final proofs
remain admissible after enrollment or account closure and do not use that gate.

The handshake echoes a valid `X-XNode-Hourly: v1` and exact 32-hex session only
when credit settlement and durable history readiness are both supported. The
independent historical receipt header additionally requires the boot-stream
repository and exact stream-format offer. HTTP upgrade headers necessarily
precede `agent.hello`, so this echo is provisional. A peer gains hourly authority
only after hello advertises `hourly_account_limits_v1` and `traffic_stream_v1`,
and supplies valid agent and boot identities. The live offer itself also requires
boot-stream repository support and its exact HTTP offer. `Master.HourlyCapable(nodeID)` checks that current connection
for source or transit participants; version strings never imply support.

The independent `X-XNode-Hourly-Accounting: v1` response advertises complete
historical journal receipt support even when the agent offers no live admission
capability. It requires repository boot-stream support and the exact traffic
stream offer. A failed hourly store therefore pauses hourly access while complete
mixed reports can still drain. Acceptance checks the authenticated current peer,
boot-scoped traffic identity and every hourly sample; it does not require a live
hourly session. No receipt header grants commands, credit or rate authority.

The coordinator starts with a fresh ten-second allocator quiet period. Every two
seconds it reads policies, makes bounded memory-only allocation decisions, and
returns per-peer grant intents. No master, policy, allocator or sender mutex is
held while waiting on the existing per-node outbound queue. One sender per peer
coalesces at most 8192 owner grants and enters each frame separately, preserving
the existing command/ACK writer. Expired, old-sequence and replaced-session
intents cannot gain authority. Dropped or unsent offers remain reserved by the
allocator until a matching successful native drain ACK or conservative expiry.

Native demand lifetime retains the socket receiver's original receipt time,
including while queued. Its cap includes both account and node paid boundaries.
Grant terms remain immutable across retry. If current policy shortens below an
already-issued boundary, the sender drops that intent without freeing capacity;
fresh authorization/demand must establish the shorter term. The allocator itself
also caps the original receipt's UTC midnight and rounds milliseconds downward.

Hourly input is validated before acceptance: at most 64 KiB per payload and 128
items per typed array, with exact current peer/session scope. Live demand and
native-drain ACKs use a separate 8192-owner/8 MiB bounded coalescer. A fixed owner
ring drains fair batches of at most 128. Duplicate demand nonces retain their first
socket receipt; newer activity does not extend a lease. The highest successful
native-drain ACK survives alongside a higher negative ACK; only exact successful
proof can release earlier allocations. Allocator batch APIs prune once per batch.

Financial progress and final proofs share a separate bounded map: 1024 distinct
reservation IDs and 2 MiB of accounted payload per peer. Both directional maxima
and an immutable recovery proof can coexist for one ID. Same-ID issue/scope/digest
conflicts are rejected. A single SQL worker has a four-second whole-turn deadline,
including a three-second progress child deadline. Failed entries retry fairly
after one second. Unattempted proofs move ahead of a slow attempted proof; failed
progress aggregates split so one unresolved ID cannot poison healthy neighbors.
No SQL or native hook runs while either inbound-map mutex is held.

Overload does not close the entire agent connection. New live hints can be
coalesced/dropped conservatively; excess financial input is dropped without ACK
and counted in bounded memory-only evidence. This relies on the signed agent's
protocol: cumulative progress is resent after its two-second cache reset and
final proofs remain in its durable journal until an exact digest ACK. Incoming
progress/proof copies are never removed merely because SQL timed out; updates
arriving during a transaction survive unless that exact copy was accepted.
An 8192-owner capture can therefore coexist with blocked financial SQL while rate
controls, command results and ordinary traffic acknowledgments continue.
Credit obligations and immutable journals remain durable; live grants, demand,
speed allocations and coalesced transport intents do not.

Hosted TLS regressions cover provisional headers versus hello capability,
repository support, replacement sessions, policy absence, exact grant delivery
and independent command/traffic progress during a blocked credit transaction.
Pure regressions cover cache cloning/freshness, node paid boundaries, whole-batch
ACK validation, sender sequencing and bounded lanes. Static validation uses the
exact verified signed-release pin. Automated suites and benchmarks run only in
hosted CI.
