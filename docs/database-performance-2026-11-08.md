# Database pressure, bounded firewall sweeps and PAYG funding reads

## Scope and baseline

Read-only production inspection on 2026-11-08 used the running panel image
`f642e76f509b80757d5799166a3a006ce794f868`, embedded PostgreSQL 18.6, an eight-connection
application pool, 19 configured nodes, 14,628 PAYG accounts and about 32,000
firewall generations. No production rows, settings, jobs or processes changed.
PostgreSQL statistics were not reset. No local automated suites were executed.

Runtime log deltas from 03:23:10 through 03:52:40 UTC (11:23:10 to 11:52:40 Asia/Shanghai):

| Counter | Interval result |
| --- | --- |
| Completed application transactions | 153,257; about 86.6/s |
| Pool waits | 66,574; 47.662 s total waiting |
| Serialization retries | 86 |
| Journal commits | 3,787; 472.171 s total, 124.7 ms mean |
| Commit-to-ACK | 1.683 s total, 0.444 ms mean |
| New ACK failures | 0 |

These are cumulative-counter deltas, not latency percentiles. Six activity
samples found no blocked sessions; that does not exclude short-lived contention.
Historical failed jobs remain diagnostic evidence and were not retried or cleared.

A fresh 12.1-second `pg_stat_statements` interval, restricted to the application
role, current database and top-level statements, identified:

| Read | Calls | Execution time | Mean |
| --- | --- | --- | --- |
| Funded PAYG accounts | 213 | 2,291.18 ms | 10.757 ms |
| Retired firewall page | 14 | 378.42 ms | 27.030 ms |

Match statement statistics by database, role, query ID and top-level flag.
Different roles can share a query ID; matching the ID alone creates false deltas.

## Implemented bounded read

`Store.SweepFirewall` retains its existing serializable transaction, shared active
audience, bitmap expiry semantics, 256-record cursor and whole-operation retries.
Only the retired-page SELECT changes. Each bank seeks its existing primary key,
filters released generations and the strict original composite cursor, and emits
at most 256 candidates. The outer sort handles at most 512 candidates and returns
the same first 256 records. Existing tombstones may still require extra index
visits; there is no new persisted index or duplicated state.

The bank prefixes and C collation preserve global order. Inclusive index bounds
plus the original strict key predicate preserve empty identifiers and avoid
revisiting a cursor record. Completed cycles still wrap and examine the entire
inventory. Startup, retirement changes, drift and recovery retain full coverage.
Transaction failure or uncertain commit restores the cursor; replay uses the
existing idempotent expiry logic. Expiry, exemption pruning, atomic writes,
durable delivery receipts and urgent firewall worker capacity are unchanged.

The SQL uses PostgreSQL's documented [parenthesized UNION branch limits](https://www.postgresql.org/docs/18/queries-union.html)
and [index ordering](https://www.postgresql.org/docs/18/indexes-ordering.html).
The local X-Node protocol and queued upstream interfaces do not change.

## Production query comparison

Read-only `EXPLAIN (ANALYZE, BUFFERS, TIMING OFF, FORMAT JSON)` used the exact old
and proposed queries against the same live inventory. Five executions per query
produced these medians; timings include normal production variation:

| Page | Original | Bounded | Shared buffer hits, original / bounded |
| --- | --- | --- | --- |
| First | 19.441 ms | 3.682 ms | 4,048 / 551 |
| Middle (after 7,000 active bans) | 60.144 ms | 2.701 ms | 4,048 / 560 |
| After the last ban | 13.052 ms | 0.120 ms | 4,048 / 6 |

Six representative cursors returned zero symmetric differences across all six
projected fields in a single statement snapshot. A forced generic prepared plan
also used `xnode_firewall_bans_pkey` with `generation >= $3`, returning 256 rows in
1.582 ms. Production sources were empty; hosted synthetic fixtures cover source
pages, mixed-bank boundaries, released rows and empty identifiers.

These measurements establish a query improvement, not a deployed reduction in
CPU, pool waits or end-to-end report latency. Deployment and a comparable runtime
counter interval are required before making those claims.

## Validation and follow-up

Hosted PostgreSQL regressions exercise original/proposed page equivalence,
multi-page all-node expiry, cursor wrap, partial-page rollback and recovery.
Existing firewall regressions retain shared-audience and history-pruning checks.
Static review includes formatting, `go vet` and the complete Git diff.

## PAYG funding projection

Further read-only analysis found that 62 of 81 active-backend observations
were running the funded-PAYG query, versus eight running firewall sweeps.
The 100 independent activity samples used a separate read-only transaction each;
one long transaction would cache the activity snapshot and give false evidence.
No blockers appeared. The longest sampled transaction was 124 ms. Pending outbox
jobs were scheduled for future retries; none were due at that sample. Cgroup
counters showed no CPU quota, throttling or recorded OOM events.

The journal latency counter starts before `persistTraffic`, including its existing
100 ms batch collection window. Its 124.7 ms mean is not database execution time
alone. The batch delay, commit-before-ACK guarantee and atomic report ordering
remain unchanged; reducing this delay would trade batching for more commits.

For PAYG, forcing later per-row period lookups was rejected: fleet summed medians
fell from 200.404 to 136.460 ms, but four of 13 nodes became slower and buffer
visits increased. A narrower funding projection instead removes historical
`purchased_bytes`/`used_bytes`, which admission does not consume, and the period
join. The live schema has a validated account-to-period foreign key.

Four alternating executions per query across all 13 PAYG nodes measured:

| Funding read | Sum of node medians | Sum of shared buffer hits |
| --- | --- | --- |
| Full account projection | 226.856 ms | 8,469 |
| Funding fields only | 93.409 ms | 4,036 |

Each node had zero symmetric differences in the funding fields. These are sums
of per-node medians, not a measured reconciliation wall time. One node's sampled
time increased under variable production load, while all nodes used fewer buffers.
Follow-up checks of the exact final SQL found zero full-account differences and
zero funding-field differences across all 13 nodes. Two representative final
queries measured 26.179 to 11.754 ms and 9.430 to 3.400 ms in four-execution medians.

`Store.XNodePAYGFunding` implements a dedicated `xnode.PAYGFunding` projection.
Shared SQL fields, predicates, scanning and prospective personal pricing preserve
current balances, prices, disablement and exact prepaid timestamps. Private zero
counter slots reuse the existing scanner; those slots never leave as accounting
data. Full account reads still join periods and return exact historical counters.
The master prefers the optional funding interface with legacy repository fallback.
It rereads every renewal and delegates funding/expiry predicates to `PAYGAccount`.
No admission decision is cached; startup, drift, recovery, clock caps and finance
transactions retain their existing full reconciliation and isolation behavior.

Hosted regressions cover wallet/credit changes, exact nanosecond expiry, current
price and disablement, node isolation, unchanged accounting, fresh projection
selection and error propagation without falling back to stale inputs.

## CI dependency repair

The first hosted run passed all eight race shards and the CPU profile gate,
but `govulncheck` reported existing `GO-2026-6888` and `GO-2026-6865` findings.
The CI repair updates only the affected patch releases:
[Brotli 1.2.7](https://github.com/andybalholm/brotli/releases/tag/v1.2.7) and
[klauspost/compress 1.20.2](https://github.com/klauspost/compress/releases/tag/v1.20.2).
The pinned X-Node protocol, compression negotiation and wire format stay unchanged.

The funding commit passed all eight race shards, but the standalone routing
deployment gate exposed a reconciliation race: after a full access prune commits
an empty disabled plan, the next pass could queue the empty graph again and
overwrite its revocation reason. The check could miss this short-lived state.
The correction retains only a committed, disabled `routing_access_removed` plan
with no choices. It avoids redundant graph/operation work and preserves durable
revocation evidence. Partial prunes, other disabled reasons, startup inventory,
uncertain commits and in-flight recovery keep their existing reconciliation.
A deterministic regression repeats authoritative revision changes and checks
that full prunes remain terminal while partial/dependency recovery still runs.
