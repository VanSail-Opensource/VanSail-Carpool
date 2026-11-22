# Panel load and stale telemetry: 2026-11-22 baseline

Read-only inspection of panel image `772918ed` and its embedded PostgreSQL. No services were restarted, statistics reset, or faults injected. These are pre-change observations, not a benchmark of the new code.

## Workload

At 01:55 UTC the host had one vCPU, a 1-minute load average of 2.75 and a healthy panel container using 840 MiB of its 1 GiB limit. The container's network and block I/O totals span its lifetime: they do not establish current throughput or identify which traffic was avoidable.

A roughly 30-second `pg_stat_statements` delta found 548 funded-PAYG reads (2.62 cumulative execution seconds), 108 rolling-window sums (1.13 s), 75 settled-window prunes (1.01 s), and 9,948 node-by-ID reads (0.22 s). These are database execution times, not CPU times. A bounded funding query plan for the investigated node read 1,408 candidates and returned 414 rows, all data buffers served from shared memory. Query frequency and the data materialized on renewal matter even without physical reads.

Changes in the database adapter keep funding decisions fresh. Renewals scan the seven fields consumed from SQL directly into the authorization projection, without returning the repeated node ID, owner, period, or dummy counters or building an intermediate account slice. Connection observation locks the period before its single full funding read; the owner lock, subsequent fresh read, price resolution and ledger transaction are preserved. Settlement no longer rewrites already settled sample rows, an unchanged offer, or an already-persisted credit remainder. Raw evidence, the 72-hour boundary and purchase rounding do not change.

## What a stale incident establishes

Across retained history for the investigated node (October 27–November 21 UTC), 137 incidents were `TELEMETRY_STALE`, versus five `OFFLINE`. The latest stale interval was November 21 19:55–19:57 UTC, followed by the normal five-minute recovery. Earlier that day there were stale intervals lasting approximately 85, 95 and 120 seconds. This records missing/unusable telemetry at the panel, not a proven loss of existing proxy sessions. No target-specific panel log lines were available in the bounded container-log query for the latest interval. Neither high panel load nor packet loss can be assigned as its historical cause from these records alone.

After deployment, compare **interval deltas** under similar active node/user counts: SQL calls, WAL bytes, pool-wait seconds, journal commit/ACK latency, interface byte rates, and stale/offline transitions. Match SQL statistics by database, role, query ID and top-level flag. Correlate the next incident with both ends of the authenticated control connection and agent report cadence before declaring it fixed. Keep raw telemetry ephemeral.
