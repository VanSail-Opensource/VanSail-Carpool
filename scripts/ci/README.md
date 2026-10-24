# Hosted Go race checks

The quality job also runs the managed two-node deployment barrier regression
early, so a preflight or acknowledgement regression produces focused diagnostics
before the complete race shards finish. The race/coverage inventories still
include that check and remain mandatory.

`race_shards.py` is called only by GitHub Actions. Eight independent runners split
the database and X-Node packages' top-level tests, executable examples and fuzz
seed tests, and partition every other Go package. Splitting X-Node is required
because its large serial test suite otherwise made one shard dominate runtime.
New packages and checks are discovered from Go itself; no handwritten allowlist
can silently omit them.

`race_inventory.py` owns discovery and exact-once partition validation. Each
runner uploads its complete inventory, assigned checks, completion marker and
JSON diagnostics. The existing Go race and coverage gate validates all eight
manifests before running the unchanged domain coverage packages and 70% gate.

Per-runner Go parallelism follows available CPUs. Database tests retain their
existing two-open migration semaphore and isolated temporary databases. Shards
have independent processes and CPU allocations, so migration concurrency does
not multiply on one machine. The X-Node test binary remains serial because its
fixtures bind sockets and publish master state; its test inventory is distributed
across runners instead of weakening those isolation boundaries. Do not invoke
these scripts locally: repository instructions permit automated suites only in
hosted CI.

`change_base.py` selects the latest successful `Container` run on `main` whose
commit is an ancestor of the checked-out tree. Push filters compare against that
published commit, so frontend-only changes skip Go checks after a successful
publication. Backend changes in canceled or failed runs remain in the next diff
and must pass before publication. The read-only GitHub Actions API uses the
workflow token; unavailable history or API errors run all gates. Release tags
also run all gates. Pull requests retain filtering against their base branch.
The workflow files and `scripts/ci` affect both quality gates; extend their path
filters when adding another shared CI input.
The aggregate gate follows the actual shard result, including failures, rather
than reevaluating changed-area output after the matrix has completed.


The ci.yml memory-soak gate compiles a non-race fixture and runs it in a 256 MiB
Docker container with swap disabled and a read-only root for at least three
minutes. Six EVM scanners, failover, WebSocket notifications and verified
2,000-log proof batches overlap inventory refreshes. The first source has
3,000,000 IPv4 and 250,000 IPv6 entries; later cycles check retired readers,
peak RSS below 224 MiB, settled heap growth at most 8 MiB after warm-up, stable
goroutines and the 1 GiB cache allowance. Container state, cycle measurements
and a final heap profile are uploaded as memory-soak artifacts. Failure blocks
image publication. Local automated suites remain prohibited.

`postgresql_contracts.py` selects the same repository business tests for hosted
PostgreSQL while retaining SQLite-only upgrade/metadata fixtures in normal CI;
the selection manifest is saved for review.
