# Product statistics

Node snapshots retain the deprecated multiplier property as constant `1`; historical traffic totals continue to use recorded accounting.

Node user counts are distinct members with open connections, including idle
flows. Paid and PAYG principals are deduplicated by owner. The separate
usersOnlineAvailable flag preserves unknown counts for legacy or stale agents.

- `provider.go` defines the narrow queue-backed X-Node metrics and host contracts.
- `service.go` refreshes remote/database partitions independently and returns contract-safe array snapshots; `partitions.go` persists and restores last-good values.
- `snapshot_test.go` verifies public snapshots serialize every required array as an array instead of `null`.
- `squad_names.go` resolves live queued X-Node squad names independently of aggregate refreshes and retains the last resolved labels through the local distribution snapshot; `squad_names_test.go` covers mapping and cached-label isolation.
- `node_cache.go` implements the shared on-demand ten-second node cache from the documented live node collection.

The aggregate service keeps remote and database partitions independently timestamped so a partial refresh serves the last good partition as stale.

Statistics now performs read-only digest, traffic and node refreshes. Legacy node-image collection and scheduled host mutation workers are retired.

Exact native usage statistics and its hosted regression: `service_test.go`, `usage_projection.go`.

Persisted partitions hydrate independently: a corrupt or unreadable partition does
not discard another valid last-good partition. Cache failures do not block the next
authoritative refresh. Delayed initial cache values or errors cannot replace state
already published by another refresh. `partition_recovery_test.go` covers corrupt
cache repair, preserved database history through a failed recompute, and delayed
initial reads racing a successful refresh.

`usage_projection_bounds_test.go` verifies that non-finite or unrepresentable
derived percentages preserve the last-good remote partition. Representable
overage remains visible; there is no new 100-percent policy cap.
