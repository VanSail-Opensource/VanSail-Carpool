# Application composition

`xnode_catalog_topology.go` adapts the native revision-keyed topology snapshot to
catalog squad/node metadata in one pass. It deduplicates published inbound tags
and group node IDs without caching commerce, permissions or live node health.

`pm_profile_payg.go` reads existing 72-hour PAYG minute summaries for PM cards
before the optional Combo projection. It performs no provisioning or upstream
requests and creates no duplicate accounting storage.

Startup retires dormant `xnode_prepare_continuity` jobs before admission starts;
native purchase compilation and ordinary activation preserve paid successors.

`persistence.go` selects the configured backend and applies SQLite file recovery
or PostgreSQL import/ownership gates before bootstrap mutations. PostgreSQL never
falls back to SQLite and never prunes retained source migration snapshots.
`database_runtime.go` joins ownership monitoring on shutdown;
`database_observability.go` logs bounded pool/transaction/journal aggregates every
30 seconds without SQL, payload or user labels. `postgres_bootstrap_test.go`
rehearses verified activation, health and refusal of a second hosted application.

`scheduler_priority.go` runs `runOutboxAdmission` once per second for both dynamic
firewall and ordinary jobs while maintenance or entitlement scans wait. Reserved
and ordinary claims share the existing outbox, executor, handlers and fenced
leases. The pump is cancelled/joined with the scheduler before storage closes.
`scheduler_admission_test.go` verifies ordinary member work starts without a
scheduled drain, with cancellation and executor shutdown joined in hosted CI.

`xnode_rollover.go` resolves native rollover through panel ownership and explicit purchase references, retaining frozen imported usage in purchase-owned settlement.

`xnode_scan_test.go` exercises real native scan nonce allocation with a panel identity and no legacy account. Both adapter scan entry points allocate the minimum accepted 128-bit nonce.

`pm_workflows.go` injects `PUBLIC_BASE_URL` into the existing queued Telegram PM
worker. New profile cards link to `/admin/users/{id}` without bearer credentials;
the independent browser access-link/session flow authorizes the administrator.

`xnode_connectivity.go` derives monitoring targets from native identities and published endpoints, keeping credentials server-side. The application owns the dedicated connectivity lane and joins its checker before closing SQLite.

The entitlement adapter intersects applied squads with account preferences at
execution; member receipts reuse the entitlement executor and invalidate usage.
- `adapters_part2.go` continues the focused implementation from its original package module.

The `app` package is the process composition root. It opens SQLite, constructs
domain services and provider adapters, registers durable outbox handlers, and
owns HTTP, scheduler, backup, and provider-queue lifecycles.

X-Node and Emby clients are intentionally created only inside their adapter
queue callbacks. Reads and writes therefore share bounded per-provider pacing;
durable outbox handlers keep their existing retry semantics and use the same
adapters for the final network attempt.

`Application.Run` starts provider workers before accepting HTTP traffic and
waits for the scheduler and provider workers to finish before returning.
`Application.Close` checkpoints and closes SQLite only after that runtime has
stopped.

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
- `mutation_operation_composition.go` registers subscription, Emby, questionnaire, retry, and refund command handlers.
- `maintenance_operation.go` registers the durable admin maintenance command with bounded execution, receipt completion, retries, lease-aware recovery, and structured failure diagnostics.
- `maintenance_runner.go` shares the full backup-gated database and migration-snapshot cleanup flow between the scheduler and manual command worker.
- `payment_refund_adapter.go` reconciles ambiguous Telegram Stars refunds from authoritative transaction history.
- `application_lifecycle.go` starts and stops provider queues, the scheduler, HTTP serving, and database resources in dependency order.
- Startup upload reconciliation receives an independent ten-minute context so a large verified candidate is not truncated by the ordinary bootstrap deadline.
- `bootstrap_settings.go` persists startup defaults, including the automatic-renewal rollover-balance policy, and creates the encrypted provider settings required at first startup.
- `provider_queues.go` configures, starts, and shuts down the independent X-Node and Emby admission queues.
- `adapters.go` implements Telegram identity/membership and payment-provider bridges used by domain services.
- `node_multiplier_cache.go` owns the copied five-minute node-multiplier cache shared by queued X-Node projections and rollover usage mapping.
- `node_multiplier_cache_test.go` covers cache expiry at the five-minute boundary.
  the narrow traffic-threshold projection.
  live counts and byte rates are rounded into the integer public contract.
- `maintenance_operation_test.go` covers successful, failed, and active-lease maintenance receipts.
- `emby_adapter.go` implements queued Emby client creation, account operations, policy updates, and metadata lookups.
- `scheduler.go` runs prompt temporary-ban restoration, automatic due-renewal revalidation before entitlement transitions, plus recurring outbox, rollover, backup, and maintenance work until application cancellation; verified-backup retention warnings are logged separately from completed cleanup.
- `notification_scheduler.go` runs the bounded startup and five-minute reminder
  and traffic scan independently from the outbox drain.
- `statistics_scheduler.go` schedules the 30-minute statistics refresh, startup
- The scheduler delegates the daily backup-gated cleanup order to
  `internal/maintenance`; the durable local-date lease prevents overlapping
  runs across process instances.
- `telegram_scheduler.go` configures Telegram delivery and reconciles Telegram Stars transactions.
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

`pm_forum_validation.go` performs bounded, queued getChat/getMe/getChatMember preflight. `telegram_queue_pm.go` exposes factory callbacks so PM rechecks eligibility and flags after queue waits. `pm_profile.go` builds ephemeral card facts from admin profiles and queued refund/rollover projections. `pm_workflows.go` registers relay/profile/repair provider kinds and localized refusal notices through the existing outbox.

`xnode_member_subscription.go` derives owner-scoped member hosts from native access and current group preferences. Configuration and links remain transient, and DIY tuning attaches only to owner exports.
- `ip_lookup.go` composes bounded provider queues and registers the shared IP Lookup operation worker.
- `ip_lookup_outbox.go` registers the outbox handlers that retry failed stages of partial reputation reports and partial live snapshots in the background.

`ip_lookup_enhanced.go` composes shared proxy inventories and queued probes. Application startup attaches IP work to the runtime; shutdown joins it before SQLite closes. Checks and probes have separate bounded upstream workers so long observations do not hold the serial outbox dispatcher.




- `xnode_catalog.go`: Entry points: `Dashboard`, `RevokeSubscription`, `ListInternalSquads`, `ListCatalogSquads`.

- `xnode_connections.go`: Entry points: `RequestConnectionScan`, `PollConnectionScan`, `ControlMemberIP`, `DropConnection`.

- `xnode_connectivity.go`: Entry points: `Resolve`, `Validate`, `Load`.

- `xnode_entitlements.go`: Entry points: `ApplyEntitlement`, `RemoveEntitlement`, `ResetTraffic`, `QuiesceForRollover`.

- `xnode_member_subscription.go`: Entry points: `LoadMemberSubscription`.



- `xnode_notifications.go`: Entry points: `ListNotificationUsers`.

- `xnode_provider.go`: Entry points: `FindUserByUsername`, `FindUserByTelegramID`, `FindUserByID`, `CreateUser`.

- `xnode_statistics.go`: Entry points: `Digest`, `Traffic`, `Nodes`.

- `native_statistics.go`: Entry points: `nativeStatisticsProvider.Digest`,
  `Traffic`, `Nodes`, `SquadNames`, `UsageSnapshotForRollover`. Native accounting
  drives aggregate partitions. Statistics read only X-Node activity, and nodes
  without native ownership or fresh connected telemetry show unavailable. Agent
  transport liveness remains independent of accounting settlement in Master.Metrics.


- `xnode_usage.go`: Entry points: `NodeUsage`.

- `xnode_compensation.go` qualifies native inbound identities for node compensation.


Bootstrap applies container-aware memory budgeting and writable SQLite temporary
storage before application construction. Application wiring opens the disposable
proxy cache after fallible service construction; cache failure degrades inventory
coverage independently of USDT. Shutdown joins workers/readers before closing the
cache and authoritative database.

`retired_detector.go` registers one bounded, durable `proxy_restriction_release` compatibility job per acknowledged historical detector ban. It preserves manual restrictions and paid entitlement terms; all legacy mutations use the existing provider queue. `retired_detector_test.go` covers manual precedence, acknowledgement checks, replay and retry failures. `payment_profile_manager_test.go` retains localized payment-profile alert coverage.




`xnode_refund_state.go` routes purchase-scoped refund usage through native accounting, including frozen imported evidence.

Native compensation readiness requires fresh ready reports. Extensions update
expiry on the existing period and retain combined quota usage.
`xnode_service_connectivity.go` derives monitoring targets from the native
standalone inventory and current host access.
Provider name/ID resolution accepts explicit service identities without creating
a Telegram association; internal bounded probes retain their separate path.

`xnode_admin_scan.go` starts native-only scans in the existing durable scan workflow and reads fresh eligible nodes through the per-node command executor. Ownership/freshness is checked again before projecting observations; raw source IPs stay memory-only.

`native_statistics_test.go` guards statistics against any source-provider fallback, including an empty native fleet and legacy-only accounting references.

`xnode_dashboard.go` resolves native dashboard statistics from panel ownership.

`xnode_connectivity_test.go` rejects disabled native monitoring identities before host resolution. Target loading uses published enabled endpoints, standalone service names and ephemeral native outbounds. Member subscription reads resolve panel ownership first.

`xnode_subscription_projection.go` keeps owner-scoped split XHTTP import links
while restricting Squad status membership to the original published inventory.
`xnode_subscription_projection_test.go` verifies link retention, enabled-group
filtering and identical monitoring membership with and without export profiles.
The unified member subscription adapter returns empty hosts for members without
a native identity, allowing the entrance to remain available before provisioning.
PAYG-only identities use the same owner-scoped export path without a Combo gate.

`xnode_squad_catalog_test.go` verifies disabled native groups are omitted from new purchases while names remain available for owned groups. Subscription GETs derive enabled groups without changing preferences.

`xnode_purchase_remote.go` and `xnode_purchase_reset.go` bind native reset/refund reads, mutations and recovery to a panel owner and purchase. `xnode_purchase_test_helpers_test.go` builds native-only paid fixtures. `xnode_purchase_operations_test.go` covers reset charging, purchase-wide refund evidence, zero-use refunds and retry exhaustion; `xnode_reset_reconciliation_test.go` distinguishes operation proof from natural cadence and interrupted attempts. These regressions run only in hosted CI.

`connectivity_errors.go` maps native probe failures into stable client codes. `metric_values.go` bounds rounded telemetry projections. Commerce adapters call the authoritative native service; source fallback and migration workers are removed.

PAYG jobs use the existing durable outbox and master executor. Startup and 30-second scans provision missing member/squad/node references in bounded batches. Connection scans and IP controls resolve every internal PAYG principal back to the authenticated member; source IP observations stay memory-only.

`retention.go` runs bounded PAYG expiry independently at startup and each transition tick, reporting counts and failures without coupling deletion to account repairs.
