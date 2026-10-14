# Remnawave integration
- `subscriptions_raw.go` implements protected raw subscription retrieval with documented enabled/hidden-host behavior; `subscriptions_raw_test.go` verifies bearer authentication, identity, empty collections, and HWID restriction decoding. Resolved client configurations contain credentials and must not be logged, persisted, or exposed through HTTP responses.
- `types_part2.go` continues the focused implementation from its original package module.
- `users_part2.go` continues the focused implementation from its original package module.

This package implements the Remnawave v3.3.0 operations used by VanSail Carpool. Endpoint and response shapes follow `reference/Upstream/Remnawave/api.json`; the shared transport applies bearer authentication and sanitizes provider errors.

- `client.go` owns client construction, authenticated HTTP transport, bounded response decoding, and sanitized API errors; `response_limits.go` keeps the larger Geocheck SVG allowance isolated from the default response cap.
- `users.go` implements user creation and identity lookup; `users_part2.go` implements documented stream pagination, update, credential revocation, and traffic reset operations.
- `user_actions.go` implements documented disable and enable account actions used only by durable administrator restrictions.
- `subscriptions.go` implements protected subscription retrieval and the documented `GET /api/bandwidth-stats/users/{userId}` date-bounded usage statistics.
- `connections.go` implements queued user scans, canonicalizes IPv4/IPv6 observations (including bracketed socket forms), selected-IP drops, and exact `blockIps`/`unblockIps` node-plugin executor payloads; `connection_types.go` keeps transient provider contracts focused.
- `geocheck.go` starts documented node Geocheck jobs and maps completed SVG results without retaining the upstream raw report.
- `statistics.go` implements digest and raw node-usage reads; the documented node collection supplies the live node-card metrics in `squads.go`, including fractional number fields that are projected at the application boundary, and `hosts.go` owns the minimal host remark patch contract.
- `squads.go` implements internal-squad, node, accessible-node, and inbound assignment operations; squad and node inbound UUIDs also support transient compensation mapping.
- `types.go` defines the provider request, response, status, traffic, subscription, node, and squad contracts.
- `user_identity.go` binds user responses to the exact requested identifier before callers can use bearer data or issue mutations.
- `user_validation.go` strictly validates every reference-required user and nested traffic field during JSON decoding.
- `doc.go` supplies the package documentation and supported upstream version.
- `client_test.go` verifies endpoint, method, payload, bearer, error-code, and lookup contracts.
- `connections_test.go` verifies a completed scan with an unsuccessful upstream result is surfaced as failed.
- `connection_ip_executor_test.go` verifies accepted IPv4/IPv6 block and unblock executor payloads plus strict target validation.
- `geocheck_test.go` verifies the node Geocheck request, polling, empty-object payload, large SVG response, and completed failure contract.
- `redirect_test.go` verifies provider redirects cannot receive the bearer credential.
- `squad_identity_test.go` rejects a squad update response whose UUID differs from the requested squad.
- `user_validation_test.go` verifies strict required-field response validation and provides canonical user fixtures.
- `user_identity_test.go` rejects mismatched identities from every user-returning operation.
- `README.md` documents the package layout and upstream contract boundary.
- `users_part2.go` contains paged user lookup, mutation payload, and action helpers.

`subscription_keys.go` implements the documented protected connection-keys endpoint with typed enabled/hidden/disabled collections. Native links are bearer secrets: caller authorization, queued execution and no logging/persistence are required. Only enabled and hidden keys are considered by the member adapter.
