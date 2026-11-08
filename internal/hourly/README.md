# Central hourly rate allocator

This package allocates one account's reduced payload rate and burst across
authenticated node processes. It contains no database, wallet, pricing, native
control, timers or network calls. `Tick` returns immutable grant actions; the
existing per-node request queue sends them after the allocator lock is released.
Financial byte credits and hourly eligibility remain separate authoritative inputs.

`types.go` defines opaque account generations, complete node/process/session
identities, original-receipt demand, policy, grant, acknowledgment and reservation
types. `New(now, Limits)` creates a ten-second cold-start quiet period so all
untracked pre-restart grants expire before new allocations. Production callers
pass their monotonic `Clock.Now` result; clock rollback returns no fresh actions.

`Connect` installs only authenticated negotiated peers. `Disconnect` stops new
demand eligibility without releasing held allocation. A new session invalidates
old input but retains old reservations until conservative expiry. `Observe` uses
the ordered receiver's original `ReceivedAt`, never a sender-supplied timestamp.
Repeated current nonces retain that original basis, and bounded recent nonce
history rejects superseded hints. Delayed ordered input cannot become fresh after
its original lifetime. The agent independently validates its original nonce/start.

`Tick(now, policies)` receives the complete current eligible policy view. All
generations with the same opaque account ID share the held budget. Missing or
invalid policy never grants fresh positive capacity. Decisions occur at most once
every two seconds, including after an acknowledgment; immediate hints cannot
increase allocations. `planning.go` uses equal rate shares and exact integer
remainders. It allocates UDP minima before sharing remaining burst, rotates peers
that cannot simultaneously fit, and holds targets through drain/increase
transitions to avoid starving two alternating datagram peers. Zero-rate/zero-burst
grants can authorize separately reserved full-speed credits without minting burst.
Fresh recent-activity demands remain active even with zero pending bytes; a
minimum one-byte capacity preserves stable grants for already-flowing credit
traffic. When no new recent demand arrives, the original deadline releases it.

`grants.go` retains at most eight possibly-live offers per peer/account generation.
Each reservation is the maximum potentially live rate and burst, rather than the
last requested values. `Acknowledge` requires the exact current peer, owner,
nonce and monotonic issued sequence. A successful drain ACK retires only matching
or older offers; newer uncertain offers remain reserved. Negative responses never
release capacity: an earlier successful application may have lost its ACK, and a
newer shrink may still be draining. They only suppress retries of that offer.
Without successful acknowledgment, the original first receipt plus
granted lifetime remains the conservative upper bound, even after disconnect.

Grant lifetimes are measured from the agent's original demand start. Master
lifetimes are bounded by the original receipt plus requested lifetime, paid
boundary and UTC midnight; integer milliseconds round downward. Duplicate hints,
retries and refreshes never move that nonce's basis. Native authorization remains
responsible for paid-through expiry. A policy reduction can temporarily leave old
reserved capacity above the new policy while it drains; no other allocation
increases until that excess is acknowledged or conservatively expires. The
account's global 16 KiB burst limit remains conserved throughout.

State is bounded by `Limits`: defaults are 8192 accounts, 4096 connected peers,
128 latest per-account node/generation demands and 512 live nonce tombstones.
Capacity rejection never evicts live reservations or replay evidence. Superseded
generation/disconnected demand expires after ten seconds; current-peer latest
nonce evidence remains while the account is eligible. Inactive accounts retire
after their policy grace, live reservations and nonce evidence expire.

`allocator.go` owns locking and bounded cleanup; `demand.go` validates and orders
input; `tick.go` stages shrinking before increases. `Snapshot` returns copied
conservative reservations for inspection, never measured traffic or billable use.
No grant refresh creates token capacity: the agent preserves its existing bucket
balance, and centrally reserved full-speed credits remain independently bounded.

Hosted regressions use `fixtures_test.go`, `allocator_test.go`, `replay_test.go`,
`retention_test.go`, `fairness_test.go` and `capacity_test.go`. They cover restart
quiet, two-second admission, exact ACK ordering, reconnect/generation retention,
paid/midnight boundaries, nonce replay, capacity and atomic datagram fairness.
Local automated suites are prohibited; static vet and formatting remain allowed.
