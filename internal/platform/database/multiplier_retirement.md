# Traffic multiplier retirement

Native traffic accepted after PostgreSQL migration 46 adds raw upload plus
download bytes at 1x. Report timestamps do not select historical rates: offline
backlog is included. Already committed receipts remain deduplicated. Historical
weighted totals, fractional remainders, imported usage, purchases, settlements,
charges, refunds and ledger evidence retain their exact recorded values.

## Entry points and data flow

`RecordXNodeTraffic` and `RecordXNodeTrafficBatch` still use the existing
serializable transaction, node/cursor/wallet/period lock ordering, report
savepoint, durable cursor and whole-operation retries. `xnode_traffic_apply.go`
passes the constant `xnode.MultiplierScale` to the arithmetic helper. Batch
preparation no longer reads historical policy or stores per-sequence prices.
PAYG and hourly charging retain their independent recorded-finance paths.

`nextNativeUsage` retains exact fixed-point arithmetic for historical evidence.
At 1x it adds whole raw bytes and carries the previous remainder unchanged;
retirement never rounds or retrospectively recomputes committed usage.

Node reads/writes and new stream/binding defaults expose/store 1x. Deprecated
`SetXNodeRate` / `SetXNodeStreamRate` retain input, identity and cursor fencing
for older callers, but only write a future 1x boundary. They cannot change
already committed history or restore zero/weighted traffic.

## Atomic upgrade boundary

`postgres/migrations/00046_retire_traffic_multiplier.sql` runs transactionally:

1. Select READ COMMITTED before any query. Goose executes migration statements
   before its version insert. A fresh statement snapshot after draining writers
   must include their final committed cursor, even though the advisory-lock
   check ran before the table-lock wait. Ordinary application writes remain
   serializable; only this migration opts into READ COMMITTED.
2. Try transaction advisory lock 72173680, conflicting with the runtime's
   session owner lock. An active old runtime causes an explicit refusal: stop
   it before running the migration with the schema-owner role.
3. Hold ACCESS EXCLUSIVE locks on nodes, legacy/stream cursors and rate banks
   through commit. In-flight ingest/rate SQL transactions complete first.
4. Update every node row to 1x, including rows already at 1x, fencing old
   binding/rate/stream-registration FOR UPDATE reads. Traffic's node KEY SHARE
   may follow a non-key update chain, so also version every existing legacy and
   stream cursor without changing its values. Its mandatory cursor FOR UPDATE
   rejects an old repeatable-read/serializable snapshot. Initialize missing
   bound legacy cursors at the actual sequence 0; stale inserts then encounter
   an invisible unique conflict and abort too.
5. Normalize only future policy and install 1x at each final cursor+1. Bound
   agents without a legacy cursor start at sequence 1. MaxInt64 cursors have no
   representable next report and receive no overflowing boundary. Unbound orphan
   rate history remains untouched. Existing cursor sequence/digest/time values
   never change; no traffic receipt is fabricated or cursor advanced.
6. Install narrow policy-write triggers before releasing locks. A resumed old
   writer cannot restore non-1x node/future-rate policy. Existing committed rate
   rows reject semantic/key changes; same-value updates preserve old values.
   Repeated inserts into past sequence numbers do not replace history. Existing
   bounded rate-retention deletes remain available.

PostgreSQL documents the conflicting session/transaction advisory lock and
ACCESS EXCLUSIVE behavior in [explicit locking](https://www.postgresql.org/docs/18/explicit-locking.html),
and stale locking reads in [transaction isolation](https://www.postgresql.org/docs/18/transaction-iso.html).
The KEY SHARE update-chain exception is explicit in PostgreSQL's
[heap tuple locking implementation](https://github.com/postgres/postgres/blob/REL_18_STABLE/src/backend/access/heap/heapam.c).
Migration is forward-only. Rollback to an old binary cannot restore multipliers.

## Verification and extension points

Hosted `postgres/multiplier_*_test.go` covers owner refusal, writer drain, the
post-wait cursor snapshot, stale snapshots, historical 1.25/zero rates, existing
future boundaries, orphan history, MaxInt64, policy guards and idempotence.
Upgrade fingerprints retain every original financial/accounting column and
node identity/credential/revision/timestamp; only operational node multiplier,
future policy, missing zero-cursor initialization and derived compile invalidation
may change. Original cursor and committed rate rows are compared separately by
complete keyed JSON values; a dedicated assertion bounds new cursors to the
missing bound-agent zero state.

`xnode_traffic_retirement_test.go` covers old-timestamp backlog through individual
and batch legacy/new-stream commits, replays, retained carry and unchanged money.
`xnode_traffic_arithmetic_test.go` keeps historical arithmetic coverage separate
from current acceptance policy. No external API or new queue is introduced.
