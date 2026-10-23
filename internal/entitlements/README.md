# Entitlements package

The durable entitlement worker translates outbox jobs and desired purchase state into idempotent X-Node mutations.

## Files

- `worker.go` defines dependencies and implements job draining, dispatch, retry,
  expiration, entitlement synchronization, and post-success notification release.
- `identity.go` reconciles the reserved username through the X-Node queue when
  paid access exists, creates queued users disabled, and routes confirmed name
  conflicts to the transactional refund and username restart.
- `identity_test.go` covers first purchase, missing linked users, name conflicts,
  and retryable provider outages.
- `continuity.go` reserves and applies a queued or renewed successor before upstream expiry without exposing an expired gap.
- `continuity_test.go` covers the three-minute continuity window and boundary settlement behavior.
- `worker_test.go` covers draining, retry behavior, traffic-reset resumption, and infrastructure errors.
- `worker_process_test.go` covers individual processing branches, expiration, desired-state synchronization, and unknown jobs.
- `worker_test_helpers_test.go` contains shared repository and X-Node test doubles.
- `README.md` documents the package layout.
