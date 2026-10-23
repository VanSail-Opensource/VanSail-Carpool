# Application composition

`scheduler_priority.go` keeps dynamic firewall admission active once per second
while ordinary outbox or maintenance work waits. It shares the existing outbox
and handlers and is cancelled/joined with the scheduler before storage closes.

`xnode_rollover.go` resolves native rollover through panel ownership and explicit purchase references. It retains the migration final-accounting fence and queued legacy fallback only while the source bridge is active.

`xnode_scan_test.go` exercises real native scan nonce allocation with a panel identity and no legacy account. Both adapter scan entry points allocate the minimum accepted 128-bit nonce.

`pm_workflows.go` injects `PUBLIC_BASE_URL` into the existing queued Telegram PM
worker. New profile cards link to `/admin/users/{id}` without bearer credentials;
the independent browser access-link/session flow authorizes the administrator.

`connectivity_source.go` reads the monitoring account and protected raw subscription inside the X-Node queue, preserving exact host UUIDs and keeping credentials server-side. `connectivity_source_test.go` covers accessible hidden hosts, disabled filtering, credential exclusion, and ambiguous identities. The application owns the dedicated connectivity lane and joins its checker before closing SQLite.

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

`member_subscription.go` admits member inventory reads through the existing X-Node queue and verifies user/credential consistency. `member_subscription_inventory.go` intersects owned enabled and upstream active squads, shared inbounds and excluded hosts, and derives node countries. `member_subscription_links.go` matches native enabled/hidden import keys by protocol, endpoint and remark; ambiguity fails closed. Raw configuration remains transient and only the owner's link reaches the member API.
`member_subscription_test.go` covers shared/hidden/excluded host joins, unique native key matching and credential-rotation rejection in hosted CI.
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
  transport liveness remains independent of migration ownership in Master.Metrics.
  Commerce and compensation retain their queued migration bridge provider.


- `xnode_usage.go`: Entry points: `NodeUsage`.

- `xnode_compensation.go` merges provider ownership and qualifies inbound identities for node compensation.
- `xnode_missing_source_test.go` covers confirmed missing registrations, purchase/native-accounting protection, database failures, source outages and cancellation. A source 404 can be omitted only when the local account has neither purchases nor an imported native identity; links and historical financial facts are never rewritten.


Bootstrap applies container-aware memory budgeting and writable SQLite temporary
storage before application construction. Application wiring opens the disposable
proxy cache after fallible service construction; cache failure degrades inventory
coverage independently of USDT. Shutdown joins workers/readers before closing the
cache and authoritative database.

`retired_detector.go` registers one bounded, durable `proxy_restriction_release` compatibility job per acknowledged historical detector ban. It preserves manual restrictions and paid entitlement terms; all legacy mutations use the existing provider queue. `retired_detector_test.go` covers manual precedence, acknowledgement checks, replay and retry failures. `payment_profile_manager_test.go` retains localized payment-profile alert coverage.




`xnode_refund_state.go` routes purchase-scoped refund usage through the migration settlement fence and native accounting.

`xnode_commerce_test.go` verifies that mixed-provider reset prechecks reconcile the combined counter during migration while refunds retain the final-accounting fence. Compensation uses fresh ready native reports and preserves disabled source-node state; empty native fleets do not call retired upstream statistics.

Native compensation readiness matches live node health, including report freshness. During bridging the source administrative disabled flag wins. Extensions update expiry on the existing period and retain combined quota usage.

source terms, aggregate traffic and complete per-node history inside the existing
with the remaining combined allowance and current legacy group bindings; its
policy-shaped purchase value is never persisted as a purchase. The temporary
subscription bridge supports these same explicit standalone groups.
`xnode_service_connectivity.go` reuses that mixed-provider inventory for imported
monitoring accounts, retaining legacy credentials only on source-owned hosts.
Provider name/ID resolution accepts explicit service identities without creating
a Telegram association; internal bounded probes retain their separate path.

`xnode_admin_scan.go` starts native-only scans in the existing durable scan workflow and reads fresh eligible nodes through the per-node command executor. Ownership/freshness is checked again before projecting observations; raw source IPs stay memory-only.

`native_statistics_test.go` guards statistics against any source-provider fallback, including an empty native fleet and legacy-only accounting references.

`xnode_dashboard.go` resolves native dashboard statistics from panel ownership without requiring the legacy link field. Only an active migration bridge can fall back to a queued source dashboard.

`xnode_connectivity_test.go` rejects disabled native monitoring identities before host resolution. Native source loading uses published enabled endpoints, standalone service names and ephemeral native outbounds; it preserves the existing queued migration fallback. `xnode_member_subscription.go` resolves panel ownership before native subscription reads without requiring a legacy link.

`xnode_squad_catalog_test.go` verifies disabled native groups are omitted from new purchases while names remain available for owned groups. Native bridge policy reads member preferences by panel ownership, and subscription GETs derive enabled groups without changing preferences.

`xnode_purchase_remote.go` and `xnode_purchase_reset.go` bind native reset/refund reads, mutations and recovery to a panel owner and purchase. Queued legacy behavior remains available only through the migration bridge; partial bridge resets remain uncertain. `xnode_purchase_test_helpers_test.go` builds native-only paid fixtures. `xnode_purchase_operations_test.go` covers reset charging, purchase-wide refund evidence, zero-use refunds and retry exhaustion; `xnode_reset_reconciliation_test.go` distinguishes operation proof from natural cadence and interrupted attempts. These regressions run only in hosted CI.

`connectivity_errors.go` maps native probe failures into stable client codes. `metric_values.go` bounds rounded telemetry projections. Commerce adapters call the authoritative native service; source fallback and migration workers are removed.
