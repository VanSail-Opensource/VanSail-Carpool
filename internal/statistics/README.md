# Product statistics

- `provider.go` defines the narrow queue-backed X-Node metrics and host contracts.
- `service.go` refreshes remote/database partitions independently and returns contract-safe array snapshots; `partitions.go` persists and restores last-good values.
- `snapshot_test.go` verifies public snapshots serialize every required array as an array instead of `null`.
- `squad_names.go` resolves live queued X-Node squad names independently of aggregate refreshes and retains the last resolved labels through the local distribution snapshot; `squad_names_test.go` covers mapping and cached-label isolation.
- `node_cache.go` implements the shared on-demand ten-second node cache from the documented live node collection.

The aggregate service keeps remote and database partitions independently timestamped so a partial refresh serves the last good partition as stale.

Statistics now performs read-only digest, traffic and node refreshes. Legacy node-image collection and scheduled host mutation workers are retired.

Exact native usage statistics and its hosted regression: `service_test.go`, `usage_projection.go`.
