# Application composition

Native node catalog and statistics adapters expose the deprecated traffic multiplier as constant `1`. The unused multiplier cache is removed. Billing uses the database acceptance cutover and retained recorded accounting, independently of display metadata.

`account_cleanup.go` registers the evidence probe with the existing outbox and
provides scheduler/bot-verification hooks. `app.go` configures protected admin IDs
before workers and constructs the shared cleanup service; `application.go`
retains it. `scheduler.go` admits bounded staggered work at startup and every
30 seconds; `telegram_scheduler.go` refreshes verified bot identity every ten
minutes. `telegram_queue_reachability.go` adapts the numeric typing action through
the existing paced Telegram executor. `telegram_queue_reachability_test.go` checks
queue-only execution and independent progress beside a blocked probe, while
`account_cleanup_test.go` checks malformed bot identifiers never expose tokens.

`hourly.go` attaches the hourly commerce module to the existing provider-operation
dispatcher and outbox, and installs the master-only extra buyer before startup.
`app.go` exposes that same service to the HTTP dependency graph; `application.go`
retains it for `scheduler.go`'s bounded one-second/startup admission hints. No
second executor, native queue or financial clock is introduced.

`xnode_catalog_topology.go` adapts the native revision-keyed topology snapshot to
catalog squad/node metadata in one pass. It deduplicates published inbound tags
and group node IDs without caching commerce, permissions or live node health.

`pm_profile_payg.go` reads existing 72-hour PAYG minute summaries for PM cards
before the optional Combo projection. It performs no provisioning or upstream
requests and creates no duplicate accounting storage.

Startup retires dormant `xnode_prepare_continuity` jobs before admission starts;
native purchase compilation and ordinary activation preserve paid successors.

`persistence.go` opens the configured PostgreSQL connection and verifies native
schema/ownership fencing before bootstrap mutations. Embedded deployments supply
their PostgreSQL connection through the same path. Missing or unavailable
PostgreSQL fails startup; no file import, conversion or swap runs at bootstrap.
`database_runtime.go` joins ownership monitoring on shutdown;
`database_observability.go` logs bounded pool/transaction aggregates every
30 seconds without SQL, payload or user labels. `postgres_bootstrap_test.go`
rehearses verified activation, health and refusal of a second hosted application.

`scheduler_priority.go` runs `runOutboxAdmission` once per second for both dynamic
firewall and ordinary jobs while maintenance or entitlement scans wait. Reserved
and ordinary claims share the existing outbox, executor, handlers and fenced
leases. The pump is cancelled/joined with the scheduler before storage closes.
`scheduler_admission_test.go` verifies ordinary member work starts without a
scheduled drain, with cancellation and executor shutdown joined in hosted CI.


`xnode_scan_test.go` exercises real native scan nonce allocation with a panel identity and no legacy account. Both adapter scan entry points allocate the minimum accepted 128-bit nonce.

`pm_workflows.go` injects `PUBLIC_BASE_URL` into the existing queued Telegram PM
worker. New profile cards link to `/admin/users/{id}` without bearer credentials;
the independent browser access-link/session flow authorizes the administrator.


The entitlement adapter intersects applied squads with account preferences at
execution; member receipts reuse the entitlement executor and invalidate usage.
- `adapters_part2.go` continues the focused implementation from its original package module.

The `app` package is the process composition root. It opens PostgreSQL, constructs
domain services and provider adapters, registers durable outbox handlers, and
owns HTTP, scheduler, backup, and provider-queue lifecycles.

X-Node and Emby clients are intentionally created only inside their adapter
queue callbacks. Reads and writes therefore share bounded per-provider pacing;
durable outbox handlers keep their existing retry semantics and use the same
adapters for the final network attempt.

`Application.Run` starts provider workers before accepting HTTP traffic and
waits for the scheduler and provider workers to finish before returning.
`Application.Close` closes PostgreSQL only after that runtime has stopped.

- `app.go` composes configuration, persistence, domain services, adapters, workers, and HTTP delivery.
- `lucky_draw_outbox.go` registers durable publish, edit, delete, entry reply, and settlement jobs; only the queued Telegram adapter calls the Bot API.
- `lucky_draw_outbox_results.go` sends private and group outcomes and records delivery IDs.
- `lucky_draw_format.go` keeps raffle entry and progress details ahead of a bounded prize list; `lucky_draw_result_format.go` builds escaped mentions and paginated outcomes.
- `lucky_draw_format_test.go` covers long announcements retaining the seat count and join command within Telegram's limit.
- `application.go` defines the process-owned application resource container.
- `member_workflows.go` composes member connection/reset/refund services and registers legacy drop plus block/unblock operation workers on one dispatcher.
- `outbox_composition.go` registers core jobs, user/payment/affiliate Telegram notifications, the scheduled IP-unblock backstop, and the shared provider-operation dispatcher.
- `admin_user_outbox.go` schedules due manual temporary-ban restoration through the shared provider-operation lane.
- `payment_operations.go` registers durable payment create and cancellation handlers on that dispatcher.
- `payment_profile_manager.go` probes BEPUSDT profiles at startup and after saves, publishes process-local discovered rails, and reports disabled profiles in each Telegram administrator's locale when available.
- `maintenance_operation.go` registers the durable admin maintenance command with bounded execution, receipt completion, retries, lease-aware recovery, and structured failure diagnostics.
- `maintenance_runner.go` shares the full backup-gated database and migration-snapshot cleanup flow between the scheduler and manual command worker.
- `payment_refund_adapter.go` reconciles ambiguous Telegram Stars refunds from authoritative transaction history.
- `payment_refund_adapter_test.go` checks exact charge, amount, owner, direction and later-page proof before a local Stars refund is accepted.
- `stars_test_helpers_test.go` supplies a queued Telegram history endpoint for refund and reconciliation regressions.
- `application_lifecycle.go` starts and stops provider queues, the scheduler, HTTP serving, and database resources in dependency order. It attaches the IP lookup lease runtime before outbox dispatch and joins that runtime on startup failure.
- Startup upload reconciliation receives an independent ten-minute context so a large verified candidate is not truncated by the ordinary bootstrap deadline.
- `provider_queues.go` configures, starts, and shuts down the independent X-Node and Emby admission queues.
- `adapters.go` implements Telegram identity/membership and payment-provider bridges used by domain services.
  the narrow traffic-threshold projection.
  live counts and byte rates are rounded into the integer public contract.
- `maintenance_operation_test.go` covers successful, failed, and active-lease maintenance receipts.
- `emby_adapter.go` implements queued Emby client creation, account operations, policy updates, and metadata lookups.
- `notification_scheduler.go` runs the bounded startup and five-minute reminder
  and traffic scan independently from the outbox drain.
- `statistics_scheduler.go` schedules the 30-minute statistics refresh, startup
- The scheduler delegates the daily backup-gated cleanup order to
  `internal/maintenance`; the durable local-date lease prevents overlapping
  runs across process instances.
- `telegram_scheduler.go` configures Telegram delivery; Stars history advances through durable page jobs.
- `telegram_queue_adapter.go` queues bot identity, membership, invite approval, decline, revocation, setup, and message operations.
- `telegram_queue_payments.go` queues Telegram Stars invoice, query, history, and refund operations.
- `telegram_command_registration.go` installs localized private-chat, configured-group, and PM-topic admin command menus.
- `app_test.go` verifies normalized Telegram Stars transaction directions.
- `adapter_queue_test.go` verifies X-Node and Emby adapter calls enter their queue before client construction.
- `provider_queues_test.go` verifies provider queue configuration and independent worker operation.
- `adapters_part2.go` contains the remaining payment-provider verification and callback mapping helpers.

- `activity_boost_adapter.go` resolves the configured supergroup and member, obtains boosts through `queuedTelegram`, filters expired and duplicate identities, and derives the Telegram boost link without local persistence.
- `activity_boost_adapter_test.go` covers personal identity and configured-group targeting, active boost filtering, missing configuration and provider failures.

`telegram_member_tag_adapter.go` checks live member role and bot tag permissions inside the existing upstream queue before a tag change.
`telegram_member_tag_adapter_test.go` verifies queue entry, tag clearing and execution-time membership checks.

`telegram_boost_appreciation.go` registers the durable appreciation handler. It rechecks the configured group and sends localized, escaped identities through queuedTelegram; `telegram_boost_appreciation_test.go` covers username, name and anonymous sources.

Turnstile uses an independent bounded provider queue in `provider_queues.go`, with the same lifecycle and executor as other providers. Composition injects the panel public hostname into accounts.FirstEntry and the verifier.


`xnode_member_subscription.go` derives owner-scoped member hosts from native access and current group preferences. Configuration and links remain transient, and DIY tuning attaches only to owner exports.
- `ip_lookup.go` composes bounded provider queues and registers the shared IP Lookup operation worker.
- `ip_lookup_outbox.go` registers the outbox handlers that retry failed stages of partial reputation reports and partial live snapshots in the background.





- `xnode_catalog.go`: Entry points: `Dashboard`, `RevokeSubscription`, `ListInternalSquads`, `ListCatalogSquads`.

- `xnode_connections.go`: Entry points: `RequestConnectionScan`, `PollConnectionScan`, `ControlMemberIP`, `DropConnection`.



- `xnode_member_subscription.go`: Entry points: `LoadMemberSubscription`.



- `xnode_notifications.go`: Entry points: `ListNotificationUsers`.

- `xnode_provider.go`: Entry points: `FindUserByUsername`, `FindUserByTelegramID`, `FindUserByID`, `CreateUser`.

- `xnode_statistics.go`: Entry points: `Digest`, `Traffic`, `Nodes`.

- `native_statistics.go`: Entry points: `nativeStatisticsProvider.Digest`,
  drives aggregate partitions. Statistics read only X-Node activity, and nodes
  without native ownership or fresh connected telemetry show unavailable. Agent
  transport liveness remains independent of accounting settlement in Master.Metrics.


- `xnode_usage.go`: Entry points: `NodeUsage`.



Bootstrap applies container-aware memory budgeting before application construction.
Application shutdown joins queued IP lookup workers before closing the authoritative database.

`retired_detector.go` registers one bounded, durable `proxy_restriction_release` compatibility job per acknowledged historical detector ban. It preserves manual restrictions and paid entitlement terms; all legacy mutations use the existing provider queue. `retired_detector_test.go` covers manual precedence, acknowledgement checks, replay and retry failures. `payment_profile_manager_test.go` retains localized payment-profile alert coverage.




`xnode_refund_state.go` routes purchase-scoped refund usage through native accounting, including frozen imported evidence.


`xnode_admin_scan.go` starts native-only scans in the existing durable scan workflow and reads fresh eligible nodes through the per-node command executor. Ownership/freshness is checked again before projecting observations; raw source IPs stay memory-only.

`native_statistics_test.go` guards statistics against any source-provider fallback, including an empty native fleet and legacy-only accounting references.

`xnode_dashboard.go` resolves native dashboard statistics from panel ownership.


`xnode_subscription_projection.go` keeps owner-scoped split XHTTP import links
while filtering host references by enabled squads.
`xnode_subscription_projection_test.go` verifies link retention and filtering.
The unified member subscription adapter returns empty hosts for members without
a native identity, allowing the entrance to remain available before provisioning.
PAYG-only identities use the same owner-scoped export path without a Combo gate.

`xnode_squad_catalog_test.go` verifies disabled native groups are omitted from new purchases while names remain available for owned groups. Subscription GETs derive enabled groups without changing preferences.

`xnode_purchase_remote.go` and `xnode_purchase_reset.go` bind native reset/refund reads, mutations and recovery to a panel owner and purchase. `xnode_purchase_test_helpers_test.go` builds native-only paid fixtures. `xnode_purchase_operations_test.go` covers reset charging, purchase-wide refund evidence, zero-use refunds and retry exhaustion; `xnode_reset_reconciliation_test.go` distinguishes operation proof from natural cadence and interrupted attempts. These regressions run only in hosted CI.

`metric_values.go` bounds rounded telemetry projections. Commerce adapters call the authoritative native service; source fallback and migration workers are removed.

PAYG jobs use the existing durable outbox and master executor. Startup and 30-second scans provision missing member/squad/node references in bounded batches. Connection scans and IP controls resolve every internal PAYG principal back to the authenticated member; source IP observations stay memory-only.

`retention.go` runs bounded PAYG expiry independently at startup and each transition tick, reporting counts and failures without coupling deletion to account repairs.

- `pricing_worker.go`: Single joined 20-second worker, three-second shared work budget, rotating bounded batches and pressure/storage gates.
- `pricing_health.go`: Reuse authenticated X-Node CPU/memory and connection health without raw telemetry SQL.
- `pricing_notice.go`: Queued model requests and nonblocking renewal price notifications.
- `pricing_health_notice.go`: Queued administrator incident notifications containing sanitized facts.
- `pricing_change_notice.go`: Notify administrators after direct audited pricing edits and material campaigns; no approval stage.

- `compute_pressure.go`: Transient five-second CPU observer with pressure hysteresis.

- `compute_pressure_test.go`: Hosted pressure-transition regression.

- `pricing_compute.go`: Connects durable numerical jobs to negotiated master/local execution without blocking the observer.

- `pricing_model_actions.go`: Schedules bounded pricing model work.

- `purchase_expiry.go`: Purchase expiry and successor lifecycle orchestration.

- `xnode_accounting.go`: Adapts exact native periods and historical usage into purchase accounting.

Existing bootstrap, scheduler, native entitlement and queued Telegram/PM composition: `bootstrap_settings.go`, `pm_forum_validation.go`, `pm_profile.go`, `scheduler.go`, `telegram_queue_pm.go`, `xnode_entitlements.go`.

`mutation_operation_composition.go` registers the shared subscription, Emby, retry, and refund workers.

`account_purge.go` registers the existing outbox purge worker and the queued Emby DELETE adapter. The scheduler resumes incomplete cleanup after restart/retry exhaustion. No upstream call bypasses upstreamqueue.

Node-outage observation and its scheduler have been retired. Existing pre-retirement committed entitlement synchronization uses only the generic admin worker; it cannot create new outage decisions. `proxyInboundID` now lives in `xnode_catalog.go` as shared catalog topology logic.

- `xnode_retired_reset_test.go`: Hosted regression refusing unpaid reset receipts without changing native periods. Historical paid reset fixtures and replay checks remain in the existing purchase tests.

The Emby adapter resolves URL/token from `SettingsService.EmbyCredentials` in one database snapshot inside the existing upstream queue executor. Validation precedes client creation and every outbound request; independent settings reads are forbidden for this coupled credential.

`payment_callback_profiles.go` selects a retained order's exact payment account
before authenticating its complete callback. Legacy two-part orders use the
provider-named migrated account and retain their empty event profile ID; disabled
profiles remain available for historical verification. The all-profile verifier
is used only when the order is missing, after which settlement still requires
matching callback tombstones. `payment_profile_fallback.go` prevents explicit
account, decryption and unavailable-storage failures from selecting global
credentials. `payment_callback_profiles_test.go` covers shared-key account
selection, legacy callbacks, missing-order authentication and failure paths.

`payment_profile_test_helpers_test.go` supplies encrypted multi-account settings and durable-order lookup fixtures for callback account-isolation regressions.

`xnode_admin_entitlements.go` resolves administrator mutations through the panel
owner's native identity. A required entitlement may provision that identity;
an empty stale snapshot rechecks effective paid access before exact accounting
closure. Restriction writes carry the durable ban/unban operation ID and preserve
the existing paid period on release. `xnode_admin_entitlements_test.go` covers
native-only owners, incorrect historical links, edits, refunds and stale empty
snapshots. `xnode_admin_bans_test.go` covers owner isolation, retained usage,
repeated restrictions and refusal of old restoration commands.

`stars_reconciliation.go` registers one durable page job per bot/cursor generation. Startup and five-minute admission resume exhausted work; each job reads at most 100 chronological transactions through the existing Telegram queue. A one-record hash overlap detects shifted offsets and rewinds safely; caught-up polls read only the boundary. `stars_reconciliation_events.go` applies verified credits/refunds and records unmatched outgoing proof in immutable administrator audits before advancing. Refund proof binds charge, invoice, whole Stars amount and owner.

`stars_reconciliation_test.go` covers history beyond 100 records, restart/partial-page recovery, tail-only polling and shifted boundaries. `stars_reconciliation_events_test.go` covers exact-charge refunds, durable unmatched-proof review and retryable storage failures. `stars_reconciliation_test_helpers_test.go` supplies stateful cursor and billing fixtures; PostgreSQL cursor atomicity is tested separately in the database module.

`stars_reconciliation_integration_test.go` runs the real billing validator and PostgreSQL settlement/refund/cursor APIs against queued Telegram history. A matching refund beyond the immediate 1,000-record search resolves a durable pending-review receipt, retains one reversal, and leaves subsequent polling at the history boundary.

The pagination contract follows [Telegram Bot API](https://core.telegram.org/bots/api#getstartransactions) and its [official implementation](https://github.com/tdlib/telegram-bot-api/blob/114fe93d27f8a06554f04b375ad03b90aa98d61b/telegram-bot-api/Client.cpp), whose TDLib [StarManager](https://github.com/tdlib/td/blob/025086e3134b638b44dae8aa0df1609c83c3a309/td/telegram/StarManager.cpp) requests ascending order for bots. Refund IDs match their original charge; outgoing and incoming records therefore remain distinct in cursor hashes.

`maintenance_operation_recovery_test.go` verifies that a completed maintenance item
recovers its exact receipt outcome after an interrupted final commit without
starting another backup or retention pass.

`xnode_member_subscription.go` brackets readiness, inventory, credential export and
subscription URL projection with the native subscription read helper. It retries
the complete read at most three times for first-sync proof gain or benign compile
revision/time contention while binding the original owner generation, restrictions,
native material and token. Authority changes and URL failures discard the entire
response. `xnode_member_subscription_fence_test.go` verifies restriction/token
refusal and first-sync proof recovery with the original requested links.
