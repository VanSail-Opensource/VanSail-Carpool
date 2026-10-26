# Compute pool v1

The agent remains Xray-only. The compute pool is an optional bounded utility lane
on the existing authenticated WebSocket. Both peers negotiate
`X-Xnode-Compute-Pool: v1`; agents also advertise `compute_pool_v1` in their hello.
Queued extension frames are discarded when reconnecting to an older master.

The master sends `compute.announce` with an opaque batch ID, attempt, SHA-256 input
hash, kernel version, item/byte counts and a deadline no more than two minutes away.
An idle, ready agent with fresh CPU/memory observations claims using `compute.claim`.
The master awards the first valid claim with `compute.grant`; other claimants receive
`compute.cancel`. Claim reservations expire after two seconds without a grant.

The dedicated agent worker permits one calculation, including upload and receipt
acknowledgement. It executes only the typed kernels in `pkg/compute`, without shell
commands, database access or financial authority. Batches hold at most 64 tasks and
256 KiB. A five-second `compute.heartbeat` renews the master's attempt lease.

Results use single-threaded zstd and ordered `compute.result` chunks up to 64 KiB,
base64 encoded in JSON, with a compressed-result hash. The maximum compressed size
is 1 MiB and decompressed output is bounded to 4 MiB. Compute reports use their own
bounded low-priority writer lane. Traffic journals, state and commands retain their
own execution/transport lanes. `compute.ack` releases the slot; an unacknowledged
result is retransmitted. `compute.cancel`, disconnect or deadline cancels the worker.

Masters must bind claims/results to the authenticated connection, reject superseded
attempts, validate task sets and numeric results, and commit accepted output
idempotently before acknowledging it. Disconnects reassign unfinished work.
