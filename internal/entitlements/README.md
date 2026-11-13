# Entitlements package

The durable entitlement worker translates outbox jobs and desired purchase state into idempotent X-Node mutations.

## Files

- `worker.go` defines dependencies and implements shared-outbox dispatch, retry,
  expiration, entitlement synchronization, and post-success notification release.
- `worker_sync.go` reconciles user desired state. Native adapters resolve the
  stable panel owner before provisioning or closure, including absent or stale
  historical proxy links; generic compatibility fixtures keep their existing
  identity-verification path. Notifications are released only after successful
  synchronization.
- `identity.go` reconciles the reserved username through the X-Node queue when
  paid access exists, creates queued users disabled, and routes confirmed name
  conflicts to the transactional refund and username restart.
- `identity_test.go` covers first purchase, missing linked users, name conflicts,
  and retryable provider outages.
- `continuity.go` only acknowledges retired preparation jobs without provider calls. Native compilation retains paid-successor access.
- `continuity_test.go` verifies legacy jobs cannot change provider authorization or accounting.
- `worker_test.go` covers production HandleOutbox results, terminal failures and durable traffic-reset resumption.
- `worker_process_test.go` covers individual processing branches, expiration, desired-state synchronization, and unknown jobs.
- `worker_test_helpers_test.go` contains shared repository and X-Node test doubles.
- `README.md` documents the package layout.

The unused standalone `Worker.Drain` and its generic claim/completion dependency
were removed. Domain workers run exclusively through the shared outbox executor,
which owns claiming, fencing, retry scheduling and unrelated job kinds. Retired
continuity jobs remain bounded upgrade acknowledgements without provider effects.

Native purchase activation enters `worker_native_activation.go`. Each delivery
refreshes its owned purchase, uses `ApplyMemberPurchase` without historical
account-wide quiesce/reset phases, and completes through the storage revision
compare. Local preflight or completion-storage failures do not become provider
failure receipts. Attempted failures can mark only the exact current term;
completed activation evidence remains unchanged on replay.
`worker_native_activation_test.go` covers refreshed targets, storage recovery,
conditional failure completion, and refusal of unsafe fallback adapters.

Native activation retains the original owner generation across provider work and both success/failure completion; a revived account cannot inherit an old job outcome.
