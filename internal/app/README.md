# Application composition

`pm_workflows.go` injects `PUBLIC_BASE_URL` into the existing queued Telegram PM
worker. New profile cards link to `/admin/users/{id}` without bearer credentials;
the independent browser access-link/session flow authorizes the administrator.

`connectivity_source.go` reads the monitoring account and protected raw subscription inside the Remnawave queue, preserving exact host UUIDs and keeping credentials server-side. `connectivity_source_test.go` covers accessible hidden hosts, disabled filtering, credential exclusion, and ambiguous identities. The application owns the dedicated connectivity lane and joins its checker before closing SQLite.

`remna_member_squads.go` reads live names through the existing request queue.
The entitlement adapter intersects applied squads with account preferences at
execution; member receipts reuse the entitlement executor and invalidate usage.
- `adapters_part2.go` continues the focused implementation from its original package module.

The `app` package is the process composition root. It opens SQLite, constructs
domain services and provider adapters, registers durable outbox handlers, and
owns HTTP, scheduler, backup, and provider-queue lifecycles.

Remnawave and Emby clients are intentionally created only inside their adapter
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
- `provider_queues.go` configures, starts, and shuts down the independent Remnawave and Emby admission queues.
- `adapters.go` implements Telegram identity/membership and payment-provider bridges used by domain services.
- `remna_adapter.go` owns queued Remnawave client creation, shared call helpers, user-ID validation, and domain mapping.
- `remna_admin_user_adapter.go` verifies replacement identities and queues documented account disable and enable actions.
- `remna_accounts_adapter.go` implements queued Remnawave account lookup and creation operations.
- `remna_catalog_adapter.go` implements queued dashboard reads and subscription revocation for catalog workflows.
- `remna_usage_adapter.go` implements queued date-bounded per-node usage reads for the member dashboard.
- `remna_admin_adapter.go` implements queued live squad and node projections for catalog composition; it exposes no node-assignment mutation.
- `node_multiplier_cache.go` owns the copied five-minute node-multiplier cache shared by queued Remnawave projections and rollover usage mapping.
- `node_multiplier_cache_test.go` covers cache expiry at the five-minute boundary.
- `remna_entitlements_adapter.go` implements queued entitlement, traffic reset, removal, and rollover operations.
- `remna_entitlements_rollover_statistics.go` validates complete per-node rollover snapshots against the provider aggregate and rejects malformed, truncated, or unweighted data as retryable.
- `remna_entitlements_rollover_validation_test.go` covers complete multi-node rollover data and retryable malformed-statistics failures for hosted CI.
- `remna_notifications_adapter.go` maps documented queued user-stream pages to
  the narrow traffic-threshold projection.
- `remna_member_operations_adapter.go` implements queued connection scans, plugin block/unblock execution, disconnect reconciliation, usage, quiesce, and restore calls.
- `remna_statistics_adapter.go` implements queue-backed Remnawave digest, node,
  traffic, Geocheck, and host operations for product statistics; documented fractional
  live counts and byte rates are rounded into the integer public contract.
- `remna_compensation_adapter.go` maps complete queue-backed node and squad snapshots without persisting authoritative assignments.
- `remna_abuse_adapter.go` supplies queue-backed node identity to detector key provisioning.
- `remna_abuse_actions.go` keeps every detector-triggered Remnawave call in the provider queue.
- `abuse_ip_ban.go` persists and resumes asynchronous Remnawave connection scans before applying an IP ban.
- `abuse_outbox.go` consumes durable detector punishment, restoration, and Telegram delivery jobs. Incident messages retain the approved Chinese field layout and have a matching English version; both use the linked username, actual detection time in UTC, reason and QPS, selected action, optional expiry, and escaped dynamic values.
- `abuse_scheduler.go` performs startup catch-up and UTC-aligned `:00`/`:30` durable abuse processing.
- `abuse_outbox_test.go` and `payment_profile_manager_test.go` cover localized private alerts and MarkdownV2 escaping.
- `maintenance_operation_test.go` covers successful, failed, and active-lease maintenance receipts.
- `remna_statistics_adapter_test.go` covers bounded rounding of Remnawave live numeric fields.
- `emby_adapter.go` implements queued Emby client creation, account operations, policy updates, and metadata lookups.
- `scheduler.go` runs prompt temporary-ban restoration, automatic due-renewal revalidation before entitlement transitions, plus recurring outbox, rollover, backup, and maintenance work until application cancellation; verified-backup retention warnings are logged separately from completed cleanup.
- `notification_scheduler.go` runs the bounded startup and five-minute reminder
  and traffic scan independently from the outbox drain.
- `statistics_scheduler.go` schedules the 30-minute statistics refresh, startup
  and six-hour node Geocheck cache work, node-compensation observations, and queued host-multiplier reconciliation.
- `statistics_setup.go` composes the statistics service and its provider adapter.
- The scheduler delegates the daily backup-gated cleanup order to
  `internal/maintenance`; the durable local-date lease prevents overlapping
  runs across process instances.
- `telegram_scheduler.go` configures Telegram delivery and reconciles Telegram Stars transactions.
- `telegram_queue_adapter.go` queues bot identity, membership, invite approval, decline, revocation, setup, and message operations.
- `telegram_queue_payments.go` queues Telegram Stars invoice, query, history, and refund operations.
- `telegram_command_registration.go` installs localized private-chat, configured-group, and PM-topic admin command menus.
- `app_test.go` verifies normalized Telegram Stars transaction directions.
- `adapter_queue_test.go` verifies Remnawave and Emby adapter calls enter their queue before client construction.
- `remna_entitlements_adapter_test.go` verifies active expiry propagation and the disabled-user far-future expiry.
- `remna_entitlements_rollover_test.go` verifies that rollover usage keeps Remnawave's inclusive final date.
- `provider_queues_test.go` verifies provider queue configuration and independent worker operation.
- `adapters_part2.go` contains the remaining payment-provider verification and callback mapping helpers.

- `activity_boost_adapter.go` resolves the configured supergroup and member, obtains boosts through `queuedTelegram`, filters expired and duplicate identities, and derives the Telegram boost link without local persistence.
- `activity_boost_adapter_test.go` covers personal identity and configured-group targeting, active boost filtering, missing configuration and provider failures.

`telegram_member_tag_adapter.go` checks live member role and bot tag permissions inside the existing upstream queue before a tag change.
`telegram_member_tag_adapter_test.go` verifies queue entry, tag clearing and execution-time membership checks.
Personal security notices in `abuse_outbox.go` follow account preferences; operator copies retain operational delivery. Skipped personal messages do not acquire a false delivered timestamp.

`telegram_boost_appreciation.go` registers the durable appreciation handler. It rechecks the configured group and sends localized, escaped identities through queuedTelegram; `telegram_boost_appreciation_test.go` covers username, name and anonymous sources.

Turnstile uses an independent bounded provider queue in `provider_queues.go`, with the same lifecycle and executor as other providers. Composition injects the panel public hostname into accounts.FirstEntry and the verifier.

`pm_forum_validation.go` performs bounded, queued getChat/getMe/getChatMember preflight. `telegram_queue_pm.go` exposes factory callbacks so PM rechecks eligibility and flags after queue waits. `pm_profile.go` builds ephemeral card facts from admin profiles and queued refund/rollover projections. `pm_workflows.go` registers relay/profile/repair provider kinds and localized refusal notices through the existing outbox.

`member_subscription.go` admits member inventory reads through the existing Remnawave queue and verifies user/credential consistency. `member_subscription_inventory.go` intersects owned enabled and upstream active squads, shared inbounds and excluded hosts, and derives node countries. `member_subscription_links.go` matches native enabled/hidden import keys by protocol, endpoint and remark; ambiguity fails closed. Raw configuration remains transient and only the owner's link reaches the member API.
`member_subscription_test.go` covers shared/hidden/excluded host joins, unique native key matching and credential-rotation rejection in hosted CI.
- `ip_lookup.go` composes bounded provider queues and registers the shared IP Lookup operation worker.
- `ip_lookup_outbox.go` registers the outbox handlers that retry failed stages of partial reputation reports and partial live snapshots in the background.

`ip_lookup_enhanced.go` composes shared proxy inventories and queued probes. Application startup attaches IP work to the runtime; shutdown joins it before SQLite closes. Checks and probes have separate bounded upstream workers so long observations do not hold the serial outbox dispatcher.

- `xnode_bridge.go`: Entry points: `reconcileBridgeUsage`, `resetBridge`, `requireBridgeSettlement`.

- `xnode_bridge_policy.go`: Entry points: `bridgeEntitlement`, `applyBridgePolicy`, `bridgeGroups`.

- `xnode_bridge_sync.go`: Entry points: `syncBridgeAccounts`.

- `xnode_catalog.go`: Entry points: `Dashboard`, `RevokeSubscription`, `ListInternalSquads`, `ListCatalogSquads`.

- `xnode_connections.go`: Entry points: `RequestConnectionScan`, `PollConnectionScan`, `ControlMemberIP`, `DropConnection`.

- `xnode_connectivity.go`: Entry points: `Resolve`, `Validate`, `Load`.

- `xnode_entitlements.go`: Entry points: `ApplyEntitlement`, `RemoveEntitlement`, `ResetTraffic`, `QuiesceForRollover`.

- `xnode_member_subscription.go`: Entry points: `LoadMemberSubscription`.

- `xnode_migration_setup.go`: Entry points: `migrationCredentials`.

- `xnode_migration_source.go`: Entry points: `Capture`, `SetNodeDisabled`, `SyncUsage`.

- `xnode_notifications.go`: Entry points: `ListNotificationUsers`.

- `xnode_provider.go`: Entry points: `FindUserByUsername`, `FindUserByTelegramID`, `FindUserByID`, `CreateUser`.

- `xnode_statistics.go`: Entry points: `Digest`, `Traffic`, `Nodes`.

- `xnode_subscription_bridge.go`: Entry points: `legacyProxies`, `proxyFromOutbound`, `objectValue`.

- `xnode_usage.go`: Entry points: `NodeUsage`.

- `xnode_compensation.go` merges provider ownership and qualifies inbound identities for node compensation.
- `xnode_final_source.go` provides queued final source inventory and a bounded provisioning fence for bridge completion. The shared fence serializes finalization with entitlement and account writes.
- `xnode_source_accounts.go` streams bounded source pages, reports unmatched active identities without archiving their credentials, and resolves the accounts currently eligible for each source node before handover.
- `xnode_source_accounts_test.go` covers cursor-loop rejection and credential-free unmatched-account previews in hosted CI.
