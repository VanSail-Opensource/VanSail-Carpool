# Shared hourly ingress contract

This optional contract adds account-wide hourly ingress enforcement without
changing legacy native accounting, purchases or ordinary admission. Masters must
keep hourly sales unavailable until every participating agent supports it.

## Negotiation and authorization

The operational agent advertises `hourly_account_limits_v1` in `agent.hello` and
offers `X-XNode-Hourly: v1` with a fresh 32-character lowercase hexadecimal
`X-XNode-Hourly-Session`. A supporting master echoes both values exactly. Missing,
partial or conflicting echoes grant no hourly authority. Disconnect or disabling
the capability clears authority synchronously; reconnection uses a fresh session.

An hourly native user supplies `hourly.owner.{id,generation}`,
`hourly.enrollment_id`, `accounting_period` and paid `valid_until`. Owner,
enrollment, native principal and period form one immutable live-flow binding.
The ordinary `User.id` encoding remains unchanged. The new hourly sample's
`user_id`, owner generation, sequences and byte counters are decimal strings;
masters must preserve their exact integers. Units are payload bytes and
bytes/second. UTC days use `YYYY-MM-DD`.

Authorization publication and native interrupt callbacks run outside the session
admission lock. New hourly flows require acknowledgment of the latest committed
authorization publication. A failed controller publication closes existing hourly
flows; missing/unhealthy enforcement never selects ordinary-only admission.
Authenticated transit skips this additional gate and retains ordinary accounting.

## Live messages

| Direction | Type | Meaning |
| --- | --- | --- |
| Agent to master | `hourly.demand` | Current session, bounded owner demand and cumulative installed-credit progress |
| Master to agent | `hourly.grant` | Matching owner/session/demand nonce, monotonic sequence, UTC day, lifetime, combined rate and burst |
| Agent to master | `hourly.grant.ack` | Applied grant sequence after required native draining, or a rejected grant reason |
| Agent to master | `hourly.grant.acks` | Bounded batch of drained acknowledgments for multiple owners in one current session |
| Agent to master | `hourly.credit.proofs` | Replayable immutable final partitions of durable byte-credit obligations |

The [hourly schemas](schemas/hourly.schema.json) describe these payloads and
credit arguments; the master envelope also includes `hourly.grant`. Hourly control
arrays contain at most 128 items; frames contain at most 64 KiB of serialized
payload. Lifetime is at
most 10,000 milliseconds, measured from the matching demand's original local
deadline basis. Retransmission cannot extend it. Grants also require the current
agent-derived UTC day. A new session cannot restore a previous grant or burst.

The master allocates one account's reduced rate across demanding nodes every two
seconds. Shrink existing allocations and wait for their acknowledgment before
increasing others. Disconnected allocations remain reserved until their original
deadline or a confirmed revocation. Aggregate burst allowance is at most 16 KiB;
a single node may receive the whole allocation. Rate allocations remain in memory.
No valid grant means hourly payload pauses. Grant expiry, session change and
shrink cancel unstarted tickets and drain started handoffs. Authorized idle or
waiting connections survive and need fresh authority before another admission.
Paid expiry and authorization revocation interrupt their native transports,
including idle connections. New sessions start with empty rate buckets and
cannot reuse old-session full-speed credit.

Stale queued demand and grant acknowledgments cannot cross session boundaries.
Final credit proofs deliberately retain their original installation session and
can replay on a newer capable connection. An immutable traffic batch containing
hourly samples is suppressed as a whole on an incompatible connection; stripping
the samples and acknowledging the remainder would lose accounting evidence.

## Durable byte credit commands

All three commands use the existing utility executor and durable command receipts,
keeping disk writes and draining off the reserved traffic/control executor.

| Command | Arguments and result | Execution bound |
| --- | --- | --- |
| `hourly.credit.install` | Reservation ID, globally monotonic issue sequence, owner generation, UTC day, agent/boot/session identities, byte ceiling and `free`/`extra` kind; installs the exact obligation durably before use | 30 seconds |
| `hourly.credit.seal` | Reservation ID and optional exact expected installation; freezes admission and returns its financial partition, or proves a never-issued obligation absent using retained sequence evidence | 2 minutes |
| `hourly.credit.ack` | Reservation ID, issue sequence and exact proof digest; retires the matching accepted proof | 30 seconds |

Masters reserve full-speed credits centrally before installation. Replay cannot
create another obligation or make old-process credit spendable. Unacknowledged
proofs persist independently of live grants. A final proof partitions the original
ceiling exactly into admitted upload, admitted download, unused and unknown bytes.
Normal proofs have no unknown residual. Crash recovery retains unknown residual
and reports no newly inferred unused bytes. A disconnect alone never proves unused
credit; reclaim only authenticated barrier/proof evidence. Acknowledge a proof
only after durable, idempotent financial settlement.

Financial sealing cancels matching unstarted tickets under the admission lock.
Started handoffs already consumed their bytes irrevocably at `Begin`, so their
later completion or write failure cannot change the proof. Sealing does not
interrupt an active transfer or wait for its delivery. This admission barrier
is separate from the native draining required for rate-allocation retirement.

The private credit journal retains a compaction floor for acknowledged issue
sequences. An expected obligation missing above that floor is provably unissued
only when no retained record or tombstone has its identity or sequence. The
agent persists a wholly unused proof before returning it, which prevents a late
install. Missing evidence at or below the floor stays uncertain. A disconnect
or a missing live controller entry alone never authorizes returned credit.

## Payload and history

The native adapter reserves and begins each actual handoff before delivery.
Successful `Begin` consumes exactly the sliced payload bytes and records raw
directional counters once. Completion drains resources; write failure never
refunds an already-admitted handoff. UDP packets retain their boundaries and
metadata, including empty datagrams. Packets larger than the 16 KiB indivisible
bound fail before admission. Chained outbounds reuse one logical flow.

Optional `traffic.report.hourly_samples` carry accepted raw admission deltas with
their captured owner, enrollment, principal, period, UTC day and coverage times.
They exclude transit duplication and traffic multipliers. Native legacy samples
retain their existing meaning. `dropped_hourly_samples` is a decimal count of
omitted history samples: display incomplete coverage, never invent their bytes.
Financial credit proofs remain independent of presentation history retention.

Historical report replay uses the independent master response header
`X-XNode-Hourly-Accounting: v1` (or a current live-v1 negotiation). It remains
usable when credit-store failure disables hourly admission. Compatibility is
connection-local and never authorizes grants, hourly state or credit commands.
Unknown masters receive no rewritten partial journal: the entire mixed report
remains pending until a compatible connection can acknowledge its exact receipt.

Native admission and interruption cannot withdraw bytes already accepted by the
operating system or network. Expiry guarantees apply at admission and draining
boundaries, rather than promising an instantaneous removal of bytes from the wire.
