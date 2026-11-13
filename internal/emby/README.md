# Emby package
- `types_part2.go` continues the focused implementation from its original package module.

Priced Emby setup, durable provisioning, policy hardening, preference updates, and password changes live here.

## Files

- `types.go` defines safe domain types, dependency interfaces, secret adapters, and policy-hardening helpers.
- `service.go` constructs the service and handles account lookup, options, retries, and setup queueing.
- `operations.go` validates and creates durable setup, preference, password, and reviewed-retry commands.
- `operation_targets.go` HMAC-fingerprints desired state and encrypts write-only command payloads.
- `operation_worker.go` owns setup and reviewed-retry reconciliation through provider-operation receipts.
- `operation_updates.go` applies or reconciles linked-account preference and password commands.
- `operation_lifecycle.go` advances the shared operation and item state without blind replay.
- `provisioning.go` dispatches and performs durable provisioning jobs.
- `provisioning_reconcile.go` resolves ambiguous creates, selects collision-safe usernames, validates preferences, and classifies provisioning failures.
- `preferences.go` applies linked-account preferences and password changes.
- `helpers.go` contains username, preference normalization, password-memory, and redacted-error helpers.
- `service_test.go` covers policy hardening, setup, linked updates, and secret adapters.
- `provisioning_test.go` covers provisioning failure, recovery, reconciliation, and validation branches.
- `service_test_helpers_test.go` contains shared repository, remote, price, secret, and cipher doubles.
- `README.md` documents the package layout.
- `types_part2.go` contains policy hardening and policy comparison helpers.

`policy_proof_test.go` verifies that non-negotiable false restrictions require an
explicit JSON boolean; null or missing proof cannot complete provisioning. The
existing pending-review path retains ambiguous setup state for reconciliation.

Terminal operation items are replayed into their parent receipt before secret
decryption or live account/provider reads. Retry preserves the recorded status,
reference, error code and result; a known successful password write cannot be
reclassified as ambiguous after a parent-receipt storage failure.
