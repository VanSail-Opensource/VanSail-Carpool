# Host connectivity

This module measures authenticated proxy connectivity for the enabled hosts
accessible to an existing Remnawave test account, including hidden hosts. It
does not modify upstream accounts or hosts or publish notifications.
Member uptime is derived from retained observations, without new persistence.
The administrator interface uses the service's safe snapshot and the
repository's rolling 24-hour attempt history.

## Entry points and dependencies

- `NewService` receives the existing settings facade, queued Remnawave source,
  probe engine, SQLite repository, and a dedicated `upstreamqueue` lane.
- `Run` recovers interrupted attempts, prunes expired diagnostics, starts one
  batch worker, and joins it before returning. The application owns starting and
  stopping the queue; it must stop the service before closing persistence.
- `Start` returns immediately with a process-owned run and deduplicates repeated
  manual requests. `Snapshot` combines fresh or last successful safe metadata
  with latest results derived from retained history. `Resolve` returns only the
  existing account's numeric ID, username, and availability.
- `ValidateConfig` validates settings and the upstream account. `Invalidate`
  immediately cancels obsolete work after a settings save. Root composition
  decides which settings edits need upstream validation so disabling monitoring
  remains possible during an upstream outage.

## Configuration and data flow

`connectivity.config` is one atomic JSON setting. Its defaults are scheduling
disabled, no account selected, a 300-second interval, a 15-second timeout, and
`https://cp.cloudflare.com/generate_204`. Intervals must be 60–86400 seconds;
timeouts must be 1–60 seconds per attempt. Failed checks allow 0–10 additional
retries (default 10), delayed 1–60 seconds (default 1) after each failure. Zero
retries preserves single-attempt behavior. Existing saved JSON settings that
omit retry fields inherit these defaults; explicit zero remains zero. Both
fields participate in the configuration hash, so a policy change cancels obsolete
work and separates current results from retained earlier history.
Targets must be absolute HTTPS URLs without URL
credentials. Account ID zero is permitted only while scheduling is disabled.

Each batch freshly resolves the selected account and protected raw subscription
through the application's Remnawave adapter. `Source.Load` supplies only that
account's enabled accessible hosts and ephemeral resolved client configurations.
The source contract excludes disabled and unrelated hosts. Raw configurations
contain bearer secrets and may only enter the probe engine; cached inventory
contains identity/display metadata with `Resolved` cleared. Metadata fetches and
failures share a 30-second cache to bound administrator polling load.

Before each queued network probe, the repository inserts a running attempt. The
service completes the attempt with a separately bounded context, even when its
batch is cancelled. It does not probe if admission cannot be persisted, and it
stops the batch after a persistence failure. Configured setup failures receive a
retained attempt without a host UUID; unconfigured manual requests are rejected.
Only stable `CONNECTIVITY_` codes leave this module.

One persisted running attempt represents the entire host check. `retry.go`
dispatches every individual attempt through the same queue with a fresh timeout,
then waits outside the queue using a cancellable timer. Only `failed` outcomes
retry; success, unsupported configurations and checker errors stop immediately.
Settings changes or shutdown interrupt probing and waiting. Only the final
sanitized result is persisted, with the final probe's TTFB/HTTP diagnostics;
intermediate failures do not publish outages or increment completed-host counts.
The probe interface remains a single-attempt contract. The per-host worst-case
budget is `(1 + maxRetries) * timeoutSeconds + maxRetries * retryIntervalSeconds`.

## Scheduling, recovery, and retention

The next scheduled batch begins after the preceding manual or scheduled batch
finishes plus the configured interval. A batch executes hosts serially through
the dedicated admission lane, preserving the shared provider-operation outbox
for paid workflows. Settings reload every second; saves can additionally call
`Invalidate`. Cancelled generations cannot publish a current successful result.

Attempt timestamps, outcomes, and host/account references survive restarts. The
repository recovers unfinished attempts as interrupted without changing their
original start times. Read queries enforce the rolling 24-hour cutoff; cleanup
runs at startup and every five minutes, including while scheduling is disabled.
No second latest-results table, credential cache, or mirrored host inventory is
needed. If upstream inventory is unavailable, retained results remain visible
with a stale marker and UUID-only identity when no safe metadata cache exists.

## Extension points and validation

`Source`, `Probe`, and `Repository` separate upstream discovery, network execution,
and diagnostic persistence. Additional probes can implement `Probe` without
changing scheduling or storage semantics. Future notifications or dashboards
must consume retained safe outcomes rather than resolved configurations.

Hosted regression tests cover configuration boundaries, credential-safe error
codes and snapshots, admission-before-network, manual deduplication, interruption
on changed settings, completion-based scheduling, startup recovery, disabled
cleanup, and retaining previous results through source outages. Local automated
test suites remain prohibited by repository instructions.

## Embedded Xray probe

`NewXrayProbe` implements `Probe` using the pinned Xray Core release. Each queued
attempt starts one isolated, in-memory engine with a single authenticated
outbound and no listeners or direct fallback. The custom HTTP dialer forces that
outbound; environment proxies, redirects, HTTP connection reuse, access logs and
error logs are disabled. HTTPS certificates use the system trust store. A 2xx
response succeeds, and latency is request-to-first-byte time in milliseconds.
The status check closes the response without downloading its body.

Engine lifecycles share a cancellable process-wide semaphore because Xray owns
global logging hooks and documents a single active server. Waiting counts toward
the 1–60 second timeout. Every constructed engine receives the probe context and
deferred cleanup before startup, so failed or interrupted startup also closes
its resources. Raw engine errors are discarded; retained outcomes contain only
stable codes, optional HTTP status and optional TTFB. The caller still must use
the existing external job queue; the semaphore does not replace queue admission.

## Resolved configuration projection

The projector selects the exact upstream host by `metadata.uuid`, then projects
the already-resolved user credentials and options. Supported upstream protocols
are VLESS, Trojan, Shadowsocks and Hysteria 2; transports are TCP, WebSocket,
HTTPUpgrade, XHTTP, gRPC, KCP and Hysteria. TLS and Reality options, XHTTP extras,
stream `sockopt`/`finalMask`, and Xray mux fields are preserved. No credentials,
share links or engine configurations are written to disk or exposed in snapshots.

The `xrayJson` host mapper applies ordered `set`, `unset` and `copy` operations,
including numeric array paths. Copies follow the upstream raw-inbound and
restricted `$host` rules, and preserve source documents. The final outbound is
revalidated after mapping. Custom templates, dependent proxy chains, local key
or certificate files, unsupported protocols/transports and ambiguous identities
produce explicit configuration outcomes; they are never silently downgraded.
Mapper paths, operation counts, document size and nesting are bounded.

## File responsibilities

- `types.go` defines configuration, snapshots, retained attempts, and dependency contracts.
- `config.go` validates/defaults the atomic setting and computes its identity hash.
- `errors.go` maps internal failures to credential-safe public codes.
- `service.go` owns construction, manual admission, and configuration invalidation.
- `runtime.go` owns scheduling, lifecycle, cancellation, recovery, and cleanup.
- `source.go` loads bounded upstream inventory and rejects obsolete generations.
- `snapshot.go` combines safe cached metadata with current retained outcomes.
- `worker.go` records attempts, dispatches queued probes, and finalizes runs.
- `retry.go` owns per-attempt queue admission, deadlines and cancellable retry delays.
- `retry_test.go` and `retry_cancellation_test.go` cover final-only results, limits, deadlines, queue failures, cancellation and duration budgets in hosted CI.
- `retry_test_helpers_test.go` composes the existing source/repository fixtures with the production queue for hosted retry coverage.
- `probe.go` executes isolated authenticated HTTPS checks and classifies outcomes.
- `probe_registry.go` registers required Xray handlers/transports and suppresses raw logs.
- `projection.go` validates exact host identity and builds an in-memory engine document.
- `projection_protocol.go` projects protocol credentials and validates endpoint ports.
- `projection_stream.go` projects transport, TLS/Reality, and stream overrides.
- `projection_safety.go` rejects unsafe mapper output, direct fallbacks, and chains.
- `mapper.go` applies ordered upstream operations without mutating source documents.
- `mapper_path.go` bounds and traverses object/array paths with upstream skip semantics.
- `config_test.go` covers settings boundaries, defaults, hashes, and safe errors.
- `test_dependencies_test.go` supplies queued settings/source/probe fixtures.
- `test_repository_test.go` supplies diagnostic repository fixtures.
- `worker_test.go` covers durable admission, deduplication, failure, and completion.
- `runtime_test.go` covers scheduling, configuration changes, recovery, and cleanup.
- `source_test.go` covers obsolete inventory generations and credential clearing.
- `snapshot_test.go` covers safe metadata, freshness, and upstream outages.
- `probe_fixture_test.go` provides a protocol-native VLESS forwarder and trusted TLS target.
- `probe_test.go` covers authentication, TLS, statuses, no redirects/reuse, and no direct fallback.
- `probe_cancellation_test.go` covers deadlines, interruption, lifecycle waits, and options.
- `probe_startup_test.go` covers failed construction/startup and owned-resource cleanup.
- `projection_test.go` covers protocol/transport/security projection without losing options.
- `projection_safety_test.go` covers identity, unsupported configuration, and sanitized errors.
- `mapper_test.go` covers ordered operations, source isolation, skip behavior, and bounds.

Mappings follow the checked-in Remnawave API schema and its
[native Xray generator](https://github.com/remnawave/backend/blob/main/src/modules/subscription-template/generators/xray-json.generator.service.ts)
and [host mapper](https://github.com/remnawave/backend/blob/main/src/modules/subscription-template/host-mapper/apply-host-mapper.util.ts).
The probe uses context-owned construction in the pinned
[core lifecycle](https://github.com/XTLS/Xray-core/blob/64fada32b5b9/core/xray.go)
and its [stable dial API](https://github.com/XTLS/Xray-core/blob/64fada32b5b9/core/functions.go).

The July 10 security revision replaces the initially planned June revision to
fix upstream certificate-pinning advisory GHSA-5wf9-h793-w73c while retaining
Go 1.26 compatibility. CI uses Go 1.26.9 and the container uses Go 1.27.2 to
include the October 8 standard-library security fixes; `golang.org/x/net` and
gRPC are also pinned above the vulnerable versions identified by hosted scans.
Hosted probe tests use a native VLESS forwarding fixture and a trusted HTTPS
target to cover authentication, status failures, redirect refusal, fresh
connections, no direct fallback, certificate verification, timeout and
cancellation. Projection tests cover loss-prone transport/security fields and
mapper isolation. These suites are authored for hosted CI, not run locally.

## Member uptime

`uptime_types.go` defines credential-safe timelines and owner-specific subscription projections. `uptime.go` builds completed-observation intervals and neutral gaps. `uptime_aggregate.go` evaluates simultaneous child states for squads and the home summary. `uptime_projection.go` joins the current configuration's retained history, calculates freshness as twice the interval plus maximum serial batch duration, and rejects configuration changes during reads. `uptime_test.go` covers completion-time boundaries, exact expiry, missing/unsupported observations and nonconcurrent outages in hosted CI. Current active membership is projected over 24 hours; incomplete coverage stays unknown. No aggregate table, host mirror or browser probe is introduced.

`native_source_test.go` covers opaque published endpoint IDs, credential-free metadata, deep-copy isolation and disabled monitoring projections. `native_probe_test.go` exercises an authenticated native outbound with an opaque endpoint reference. `source.go` clears both Resolved and NativeOutbound from metadata; each actual probe reloads its private payload. `uptime_projection.go` exposes monitoringEnabled from scheduled configuration and skips historic timeline reads while disabled, preserving host links.
