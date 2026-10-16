# Product statistics

- `provider.go` defines the narrow queue-backed Remnawave metrics and host contracts.
- `service_test.go` pins seven-day created-user semantics plus shared-snapshot usage and predicted-rollover averaging, including valid zero projections.
- `service.go` refreshes remote/database partitions independently and returns contract-safe array snapshots; `partitions.go` persists and restores last-good values.
- `snapshot_test.go` verifies public snapshots serialize every required array as an array instead of `null`.
- `squad_names.go` resolves live queued Remnawave squad names independently of aggregate refreshes and retains the last resolved labels through the local distribution snapshot; `squad_names_test.go` covers mapping and cached-label isolation.
- `usage_projection.go` derives both live usage and the nearest-minor predicted rollover average from one queued snapshot pass. Rollover includes valid zero forecasts only for active/activating purchases with automatic renewal enabled and skips unusable snapshots.
- `node_cache.go` implements the shared on-demand ten-second node cache from the documented live node collection.

The aggregate service keeps remote and database partitions independently timestamped so a partial refresh serves the last good partition as stale.

Statistics now performs read-only digest, traffic and node refreshes. Legacy node-image collection and scheduled host mutation workers are retired.
