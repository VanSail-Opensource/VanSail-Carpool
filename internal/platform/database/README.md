# Application store

`outbox_retry_policy.go` keeps desired-state routing/DIY retries between five
and thirty seconds while preserving the existing firewall and external-command
policies. `outbox_retry_policy_test.go` verifies a late retry on both backends
through the real claim/completion path. DIY receipts retain `whitelistChanged`
in their existing result JSON so routing-only work avoids unrelated nodes.

`postgres_commerce_concurrency_test.go` races duplicate payment callbacks/debits,
limited stock and renewal cancellation. `postgres_deadlock_test.go` constructs a
real PostgreSQL deadlock and checks whole-operation retry. `transaction_random.go`
retains random choices across rollback; daily check-ins lock the balance under
READ COMMITTED before checking an existing receipt, so concurrent claimants
sample only the winning outcome.
Source calls through the retry tape use the store's short random-source mutex;
the potentially concurrent injected source retains its previous serialization.
Outbox priority/resource/recovery tests use the generation-aware completion
contract on both backends, including interrupted native operation attempts.

`open_configured.go` explicitly selects SQLite or PostgreSQL. `transaction_retry.go`
classifies whole-operation retries; generated adapters retain existing public
repository interfaces. `postgres_concurrency_test.go` exercises debit contention and stale outbox generations.
`postgres_contract_test.go` supplies isolated PostgreSQL
databases to the same business contracts in hosted CI. `outbox_fencing.go` owns
generation-checked completion/heartbeats. `xnode_traffic_sample.go` contains exact
fixed-point sample validation and charging after sorted period locks.

The PostgreSQL transition inventory is maintained in
[the compatibility map](../../../docs/plans/postgresql-compatibility.md),
[schema mapping](../../../docs/plans/postgresql-schema-mapping.md), and
[invariants](../../../docs/plans/postgresql-invariants.md). Regenerate the final
post-startup fixture through `go run ./cmd/db-inventory`; historical migrations
remain immutable. Production snapshots use its explicit read-only source option.

Writable transactions reserve SQLite's writer with the driver's supported
`_txlock=immediate` option before reading mutable state. This prevents WAL
snapshot-upgrade failures from unrelated autocommit writers; the existing busy
timeout applies at acquisition. Explicit read-only transactions keep concurrent
WAL snapshots. `database_lock_test.go` covers both paths with separate connections.

Traffic ingestion returns specific `xnode.ErrTraffic*` conflicts while preserving
`errors.Is(err, xnode.ErrConflict)`. Rejections never advance a cursor or charge
usage. A post-migration sequence gap requires reconciliation with the retained
final SQLite source; skipping the cursor loses already acknowledged accounting.
`xnode_traffic_conflicts_test.go` covers those diagnostic and atomicity contracts.

`xnode_firewall_queue.go` projects lagging dynamic/source banks onto the existing
durable outbox. Periodic reconciliation preserves pending retry deadlines and
processing leases. Journal ingestion queues fleet work only when desired firewall
versions change; below-threshold observations, partial expiry and replay still
commit their event cursors. Reconnects retain full delivery for both banks.
`xnode_firewall_queue_test.go` covers recovery, bank selection and retry pacing.

Rekeyed traffic aggregates accept either side of the recorded owner mapping.
Live cumulative counters can move to the new ID while immutable period samples
retain the old ID, including idle reports with no new samples. Only validated
period samples charge usage. `xnode_traffic_alias_test.go` covers both forms and
replay without duplicating cumulative traffic.

`xnode_traffic_alias_test.go` covers alias-only legacy traffic totals, replay,
owner validation and the node/period evidence required for late billable samples.
Aggregate totals advance the durable journal without being charged a second time;
native accounting continues to charge only period-scoped samples.

Firewall journal cursors also commit for inactive or not-yet-joined nodes,
without admitting new detections. Valid reserved/shared-space IP observations
are skipped under master public-IP policy; malformed addresses remain errors.
`xnode_firewall_journal_test.go` covers replay and mixed public/reserved batches.

`telegram_identity_migration.go` owns the atomic vault-aware startup rekey and
private backup. `telegram_identity_references.go` converts FK and explicit owner
references through disjoint temporary keys. `telegram_identity_cipher.go` reseals
X-Node and pending Emby secrets; `telegram_identity_payloads.go` updates typed
active queue references; `telegram_identity_username.go` removes the assigned
name column. `telegram_identity_migration_test.go` covers rollback, credentials,
financial references, immutable audit owners and colliding late traffic in hosted CI.
`telegram_identity_rekey_test.go` covers every member-bound encrypted context and
sealed firewall IP moved through the identity conversion.
`xnode_access_repair.go` coalesces bounded paid provisioning recovery into the
existing outbox without resetting exhausted or settled purchase periods.
`telegram_identity_commands.go` reseals queued administrator commands using the
new actor ID while retaining their retry keys.

`database.go` escapes literal database paths before adding SQLite URI pragmas.
`database_path_test.go` covers hash/percent/space filenames and isolation between
paths that would otherwise share a truncated URI filename, including subtest paths.

Native traffic aggregation streams indexed node/day groups with checked integer
sums. Backup-gated native retention prunes only expired access and superseded
acknowledged rate boundaries; native purchase/refund evidence and unsettled
rollovers remain protected. See [accounting retention](../../../docs/modules/native-accounting-retention.md).

`xnode_*.go` implements authoritative node/profile/group/endpoint configuration,
encrypted identity references, periods and immutable
report cursors. `xnode_traffic_test.go` covers atomic replay rejection, byte
counter ownership, sequence gaps and fixed-point multiplier carry in hosted CI.
Live connections, rates, IP observations and logs have no storage interface.
`vansail_migration_test.go` covers seeded-brand conversion, preservation of
custom documents/revisions and live browser devices/handoffs, and removal of
retired browser-login credentials in hosted CI.

`browser_sessions.go` persists separate browser-device sessions, daily stable-token
renewal, and single-device revocation. `browser_access_links.go` consumes hashed
five-minute handoff references and creates their session in one transaction.
The browser scope permits multiple devices; legacy `ReplaceSession` and
`UserBySession` remain Mini App-only. Maintenance removes expired browser rows
without applying Mini App replacement rules. `browser_sessions_test.go` and
`browser_access_links_test.go` cover scope/device isolation, renewal, retention,
atomic replay rejection, and expiry in hosted CI.

`member_squad_preferences.go` stores disabled references and validates the last
enabled squad atomically. `member_combo_control_helpers.go` checks ownership,
queued order, pending mutations and bans. `member_early_activation.go` forfeits
and rebases terms without financial credits. `member_squad_controls_test.go`
and `member_early_activation_test.go` cover races, carried preferences, ownership,
custom durations, automatic successors and durable reset-phase resumption in CI.

`member_combo_recovery.go` revalidates live, owned, unsuperseded targets before
reopening a failed control receipt. Administrative job retry and failed-artifact
retention resume the same reset phase without repeating forfeiture or money
movement. `member_combo_recovery_test.go` covers exhausted upstream attempts,
both recovery paths, full term dates and unchanged balances in hosted CI.

The store uses `modernc.org/sqlite` 1.60.1 and its compatible libc dependency.
Go module upgrades preserve the existing Go 1.26 toolchain and database contracts;
full migration/race validation runs in hosted CI rather than local test suites.

Lucky-draw storage keeps configuration, ticket charges, reservations, and immutable outcomes local. Every X-Node effect is enqueued after the result transaction; Telegram delivery uses recorded message IDs.

Private draw coupons (`admin_visible=0`) can only be used through awarded wallet grants. Code lookup excludes them, and the shared grant lookup and wallet listing reject historical `source_type='code'` copies. Checkout and automatic renewal therefore cannot use copied grants; existing financial records remain intact. `activity_draw_coupon_security_test.go` covers these boundaries and normal public-code redemption.

- `activity_draws.go` saves and audits mode-specific configuration; `activity_draws_part2.go` settles instant results atomically, binds replays to the same member and draw, and reserves `raffle:` keys for internal settlements.
- `activity_draw_replay_security_test.go` covers cross-draw conflicts, member isolation, reserved keys, and unchanged balances/randomness on replay.
- `activity_draw_list.go` lists member-safe instant draws and admin definitions; `activity_draw_reservations.go` reconciles active raffle loss holds when reward ranges change.
- `activity_reward_apply.go` applies resolved balance, coupon, purchase, traffic, and squad effects and queues provider synchronization.
- `activity_reward_combo.go` applies core-change prizes while retaining existing custom squads, renewal price, current/renewal traffic and rollover terms, including a zero-price custom renewal. Ordinary core changes adopt the target defaults. Pending rollover metadata is refreshed in the same transaction; provider synchronization stays in the existing outbox path.
- `activity_reward_combo_test.go` covers reward ordering and traffic lifetimes. `activity_reward_combo_migration_test.go` covers restoration, ambiguity exclusions, audit/sync deduplication and unchanged member balances/renewal switches. `CUSTOM_COMBO_RECOVERY.md` documents the repair boundaries and deployment checks.
- `activity_reward_combo_order_test.go` verifies that recovery leaves later custom awards untouched, including same-time settlements, zero-price awards and multiple custom prizes. An older core change cannot justify recovering a newer custom award whose overrides are missing.
- `activity_reward_ordered_recovery_test.go` and `activity_reward_recovery_fixture_test.go` exercise migration 055 with real raffle settlement/ticket ordering and reconstructed legacy states, including multiple custom prizes, temporary/recurring traffic, missing links, pending rollovers and preservation of newer awards.
- `activity_reward_cadence_test.go` and `activity_reward_cadence_migration_test.go` cover cadence snapshots, legacy inheritance, ordinary core defaults, later custom awards and migration 056 recovery across automatic-renewal ancestry. Custom cadence uses the existing purchase override and is synchronized through the normal outbox.
- `activity_reward_traffic.go` tracks current-term and renewal traffic separately so recurring rewards cannot preserve temporary gains or losses. Eligibility and settlement reject deductions that would exhaust either allowance. `activity_reward_traffic_test.go` covers mixed reward ordering and renewal coverage.
- `activity_raffle_entries.go` sells deduplicated seats; `activity_raffle_settle.go` refunds expired eligibility or assigns fixed stock atomically and accepts retries after a refund reopened the raffle.
- `activity_raffle_settle_retry_test.go` covers a recovered settlement job arriving after the raffle has reopened.
- `activity_draw_payload_migration_test.go` covers recovery of numeric-revision decode failures while preserving the unique active-job constraint and unrelated errors.
- `activity_raffle_publish.go` keeps drafts closed until the group announcement message ID is recorded, reserves triggers through settlement, and queues late-message cleanup after cancellation. Publishing terms are frozen until confirmation so the charged price matches the announcement.
- `activity_raffle_lifecycle_test.go` covers publishing edits, late cancellation cleanup, stale settlement jobs, and trigger collisions during settlement.
- `activity_raffle_accounting_test.go` covers concurrent message replay, reserved balances, rollback on randomness failure, exact stock distribution, ledger reconciliation, and settlement replay.
- `activity_raffle_tickets.go` holds ticket scans and exact fee-plus-reservation refunds shared by cancellation and settlement.
- `activity_raffle_eligibility.go` checks cumulative negative-traffic exposure against the smallest entitlement available after configured combo rewards.
- `activity_raffle_queries.go` supplies routing, progress, outcome, and delivery lookups; `activity_forecast.go` prices draws from current catalog and eligible balances.
- `activity_forecast_prices.go` derives reward expense bounds from current combo and squad prices plus eligible member balances.
- `activation_codes.go` validates selected combo/add-on activation-code maps in the purchase transaction while keeping only bcrypt hashes in local overrides.
- `activation_codes_test.go` covers missing, invalid, extra, and bcrypt-validated purchase codes.
- `activity_draws_part2.go` continues the focused implementation from its original package module.
- `activity_extensions_part2.go` continues the focused implementation from its original package module.
- `activity_history_part2.go` continues the focused implementation from its original package module.
- `activity_part2.go` continues the focused implementation from its original package module.
- `billing_payments_part2.go` continues the focused implementation from its original package module.
- `coupon_purchase_part2.go` continues the focused implementation from its original package module.
- `coupons_part2.go` continues the focused implementation from its original package module.
- `coupons_part3.go` continues the focused implementation from its original package module.
- `emby_provisioning_part2.go` continues the focused implementation from its original package module.
- `outbox_jobs_part2.go` continues the focused implementation from its original package module.
- `questionnaire_queries_part2.go` continues the focused implementation from its original package module.
- `questionnaires_part2.go` continues the focused implementation from its original package module.
- `rollover_part2.go` continues the focused implementation from its original package module.
- `statistics_part2.go` continues the focused implementation from its original package module.
- `store_part2.go` continues the focused implementation from its original package module.

This package owns the authoritative SQLite connection, migrations, transactional
repositories, durable outbox records, and restore validation. Domain services
depend on the `Store` methods; provider network calls do not belong here.

The persistence implementation is split by domain operation. The `_part2.go` files contain continuation methods for the corresponding bounded repository module.

- `activity_draws_part2.go`, `activity_extensions_part2.go`, and `activity_part2.go` continue activity persistence.
- `billing_payments_part2.go`, `coupons_part2.go`, and `coupons_part3.go` continue billing and coupon persistence.
- `emby_provisioning_part2.go`, `outbox_jobs_part2.go`, `questionnaires_part2.go`, `statistics_part2.go`, and `store_part2.go` continue their focused repositories.

## Production files

- `billing_purchase_addon_fingerprint.go`, `billing_purchase_addon_quote.go`, and `billing_purchase_addons.go` validate, price, replay-protect, debit, record, and queue active-term squad additions in one transaction.
- `billing_purchase_addons_test.go` covers rounding/caps, replay-safe debits, rollover and renewal totals, queued blocking, and full-squad owner selection.
  repair, and atomic full refunds plus username restart on identity conflict.
  preserved account facts, and the localized conflict receipt.

- `timestamp_cursor.go` signs filter-bound timestamp/ID cursor payloads shared by administrative inventories.
- `admin_users_page.go`, `admin_entitlements_page.go`, `admin_finance_pages.go`, and `admin_jobs_page.go` provide stable filtered pages without fixed-cap truncation.
- `database.go` — opens SQLite, applies embedded migrations, checkpoints WAL,
  and exposes migration versions.
- `store.go` — core store type, user/session persistence, membership, username,
  and X-Node recovery state.
- `store_access.go` exposes the database handle and optional structured logger.
- `community_membership.go` queries only currently active combo windows for entitlement-gated community access.
- `settings.go` — encrypted-or-plain application setting records.
- `onboarding.go` — versioned onboarding content, agreement contracts, and
  onboarding completion.
- `catalog.go` — combo inputs and transactional combo writes.
- `catalog_queries.go` — combo reads, scans, and normalized squad UUID lists.
- `catalog_squads.go` — sparse local merchandising and normalized profile overrides for upstream squads.
- `billing.go` — purchase normalization, quotes, creation, pricing, and debit helpers.
- `billing_bounds.go` persists the singleton inclusive Add TXB range and atomically audits administrator updates.
- `billing_renewal.go` — renewal quotes, contiguous batch creation, and idempotent replay.
- `renewal_addons.go` resolves owned optional-squad identities at current sparse-override prices, including hidden listings and zero-price defaults, and can exclude upstream-unavailable add-ons from either renewal path.
- `renewal_price_changes_test.go` and `renewal_unavailable_addon_test.go` cover optional-squad repricing, hidden/default overrides, unavailable-squad exclusion, commit-time refresh, immutable source charges, and replay-safe debits.
- `renewal_stock_price_test.go` covers current recurring-discount quotes and debits while an existing member retains a seat in a squad closed to new sales.
- `squad_stock_retention_test.go` covers held reservations after stock-limit reductions, new-member rejection, and seat release after expiry or cancellation.
- `automatic_renewal_plan.go`, `automatic_renewal_coupon.go`, `automatic_renewal_state.go`, and `automatic_renewal_commit.go` — automatic-renewal current pricing, attached-coupon policy, owner state/failure records, and atomic one-successor debits.
- Renewal plans project persisted full-squad overrides into `Combo.IncludedSquads` for live validation. Custom reward prices, traffic and rollover settings remain on the purchase and carry into successors; the base combo supplies renewal cadence. Existing awards need no data migration.
- `automatic_renewal_rollover.go` settles a calculated rollover and automatic renewal in one transaction, including a caller-supplied policy for whether rollover counts toward sufficient funds, insufficient-funds expiry, and explicit later re-enable handling.
- `automatic_renewal_rollover_test.go` covers atomic rollover-funded renewal and the policy path that excludes rollover from the required balance.
- `billing_purchase_helpers.go` — distinct-member stock checks that retain existing seats, purchase fingerprints, catalog row loaders, and balance debit helpers.
- `billing_ledger.go` — balances, audited adjustments, deductions, and ledger reads.
- `ledger_page.go` — stable cursor-based ledger pagination.
- `billing_purchases.go` — purchase reads, cancellation, squad hydration, and
  active/queued selection.
- `billing_purchase_summary.go` — bounded active/queued purchase projection used by the dashboard without growing the purchase-read module.
- `billing_payments.go` — payment-order creation, checkout updates, expiry, reads,
  and row scanning.
- `billing_purchase_cancellation.go` — owner-scoped queued cancellation with
  atomic status transition, TXB refund, and immutable ledger entry.
- `billing_payment_settlement.go` — customer cancellation, idempotent provider
  settlement transitions, and atomic channel announcement and private receipt queueing.
- `billing_refunds.go` records provider refund receipts with final TXB balance and
  cancelled combos, gating delivery on upstream sync when entitlements changed.
- `billing_payment_announcement.go` resolves the settlement-time username and
  administrator provider label and encodes the immutable outbox payload inside
  the payment transaction.
- `affiliate_referrals.go` freezes valid private-start inviters before Mini App authentication.
- `affiliate_config.go` reads and atomically versions audited tier configuration.
- `affiliate_queries.go` projects member metrics, progress, and fixed referral pages with Telegram first/last names.
- `affiliate_settlement.go` creates immutable first-payment commission snapshots and jobs.
- `affiliate_rewards.go` applies exact-once tier rewards through shared transaction helpers.
- `affiliate_settlement_test.go` covers first-payment uniqueness, floor rounding, pre-upgrade rates, exact-once TXB awards, and retention-preserved payment evidence.
- `affiliate_referrals_test.go` covers missing, self, frozen, and authentication-locked inviters plus locale normalization.
- `affiliate_config_test.go` covers immutable version increments and stale-write conflicts.
- `notification_events.go` owns semantic event deduplication and atomic outbox
  release, including provider-completion timestamps; `notification_scans.go` owns 48-hour and reset-period eligibility.
- `notification_purchases.go` snapshots expiration, immediate and queued activation, and automatic-renewal rollover outcomes.
- `notification_new_events.go` and `notification_operation_events.go` snapshot scheduled charges, add-on activation, cancellations, renewal failures, and terminal member operation receipts with stable event keys.
- `billing_quote.go` owns read-only purchase quoting; `automatic_renewal_calculation.go` owns pure rollover-credit and renewal-funds calculations.
- `notification_new_events_test.go` and `notification_refund_events_test.go` cover queued charges, terminal-only reset/refund messages, and renewal-failure suppression of generic expiry.
- `store_catalog_renewal_test.go` and `store_outbox_test.go` keep catalog and outbox regression coverage in focused files.
- `notification_receipts_test.go` covers activation and payment receipt snapshots.
- `notification_admin_finance.go`, `notification_admin_cancel.go`, and
  `notification_admin_entitlements.go` snapshot detailed administrator changes.
- `payment_operations.go` atomically stores checkout/cancellation intents with provider-operation receipts.
- `payment_operation_resolution.go` resolves checkout receipts from authoritative paid callbacks without another provider call.
- `billing_callback_tombstones.go` keeps provider callback replays idempotent
  after terminal payment detail has been compacted.
- `billing_courtesy.go` — atomic terminal-payment courtesy credits with linked
  immutable ledger and audit records.
- `billing_refunds.go` — transactional refunds, compensating ledger entries, and
  refund history.
- `balance_transactions.go` — shared checked balance mutation helpers used inside
  larger transactions.
- `coupons.go` — coupon definitions, direct redemption, grants, wallet reads, and
  durable member soft-discard records.
- `coupon_purchase.go` — purchase discount quoting, grant consumption, limits,
  and coupon scans.
- `coupon_records.go` — grant/redemption lookup and scan helpers.
- `activity.go` — game configuration, bets, and daily check-ins.
- `activity_draws.go` — lucky-draw configuration, listing, and atomic play results.
- `activity_history.go` — combined activity history and group-message rewards.
- `activity_group_facts.go` buffers identity-independent configured-group facts and flushes deduplicated batches transactionally.
- `activity_group_facts_test.go` covers buffered batch persistence and in-memory deduplication.
- `activity_queries.go` — game, bet, check-in, draw, and result scanners.
- `activity_extensions.go` — durable subscription-extension credits and activation
  application.
- `questionnaires.go` — questionnaire definitions, active selection, participants,
  and participation history.
- `questionnaire_imports.go` — CSV import creation and analysis.
- `questionnaire_settlement.go` — durable settlement queueing and award application.
- `questionnaire_queries.go` — questionnaire/import state and scan helpers.
- `emby.go` — Emby setup debit/outbox creation and account reads.
- `emby_provisioning.go` — retryable provisioning-saga state transitions, refund,
  preferences, and account touch operations.
- `emby_queries.go` — provisioning scans, preference hydration, folder replacement,
  and provider-error normalization.
- `operations.go` — durable job insertion, entitlement-transition enqueueing, and
  purchase expiry.
- `expansion_backup.go` — expansion-state backup and restore helpers.
- `outbox_jobs.go` — outbox claim, completion, retry, deletion, recovery, listing,
  scanning, and persisted-error sanitization.
- `purchase_sync.go` — entitlement synchronization and traffic-reset phase state.
- `questionnaire_operations.go` atomically confirms an import with its provider-operation receipt.
- `emby_setup.go` owns the shared atomic setup debit, sealed state, and optional legacy outbox transaction.
- `emby_operations.go` atomically binds setup and reviewed retry state to one provider-operation job.
- `outbox_retry_operations.go` atomically reactivates a target job and completes its retry receipt.
- `payment_refund_resolution.go` closes only open Stars refund receipts from authoritative callback evidence.
- `member_operation_begin.go` and `member_operation_validation.go` atomically validate and create member reset/refund commands.
- `member_reset_compensation.go` credits a failed paid-reset debit exactly once with terminal receipt state.
- `traffic_reset_automation.go` reads and immediately saves the account-wide preference.
- `display_currency.go` saves the member-only TXB, CNY, or USD presentation preference without changing authoritative TXB values.
- `automatic_traffic_reset.go` revalidates the preference and active purchase, deduplicates the reset period, then atomically disables-and-notifies or creates the paid provider operation, ledger debit, and gated success event.
- `member_refund_commit.go` atomically credits a first-term refund and advances the independent queued timeline.
- `provider_operations.go`, `provider_operation_lifecycle.go`, `provider_operation_queries.go`, and `provider_operation_items.go` persist provider-neutral receipts, items, attempts, and replay facts.
- `provider_operation_notification_completion.go` releases pending user messages
  only for a real successful provider-worker item.
- `admin_entitlement_edit.go`, `admin_entitlement_refund.go`, and `admin_combo_replacement.go` atomically persist audited administrator mutations with their provider operations.
- `admin_coupon_grants.go` records idempotent, audited purchase-coupon wallet grants and historical discards.
- `admin_bulk_query.go`, `admin_bulk_shift.go`, and `admin_bulk_extension.go` preview inclusive-OR active targets, deduplicate users, shift active and queued terms to the minute, and create one durable bulk job.
- `compensation_config.go`, `compensation_observation.go`, `compensation_events.go`, `compensation_review.go`, `compensation_notification.go`, and `compensation_dismiss.go` persist revisioned outage policy, node observations, frozen snapshots, cursor-safe projections, atomic reviewed extensions, and provider-gated compensation detail cards.
- `automatic_renewal_plan.go` and `automatic_renewal_commit.go` preserve explicit administrator entitlement overrides and edited term lengths when creating a successor.
- `migrations/035_abuse_warning_cooldown.sql` persists the policy-controlled abuse record cooldown under its original column name for compatibility.
- `migrations/038_durable_abuse_processing.sql` adds the bounded streak policy, normalized pending events, 30-minute rollups, emitted-state compatibility, compact incident facts, and punishment completion evidence.
- `migrations/039_abuse_record_retention_and_ip_bans.sql` adds configurable record retention support and resumable IP-ban scan state.
- `migrations/040_admin_user_provider_commands.sql` adds durable manual temporary-ban state.
- `migrations/041_admin_coupon_discard_commands.sql` binds administrative coupon discards to idempotency keys.
- `migrations/042_community_membership.sql` moves legacy membership onboarding users to username or agreement without changing their Telegram facts.
- `community_membership_test.go` covers legacy mapping, membership-fact persistence, and strict active-window boundaries.
- `compensation_observation_test.go` covers persisted/missing nodes, snapshotted policy, disabled precedence, and repeated outages.
- `compensation_review_test.go` covers inactive skips, exact minute shifts, notifications, operation linkage, stale reviews, replay, and dismissal.
- `admin_user_operations.go` and `admin_user_refunds.go` supply the aggregate profile's open-operation and refund projections.
- `admin_operation_resolution.go` atomically resolves review-required operations, stores replay fingerprints, and appends the audit event.
- `admin_workflow_types.go` defines shared aggregate and administrator workflow persistence records.
- `connection_scans.go` and `connection_scan_lifecycle.go` persist metadata-only provider scan progress.
- `connection_ip_blocks.go` atomically creates an encrypted active block, its provider operation, and immediate/scheduled jobs; `connection_ip_blocks_mutations.go` owns owner reads and unblock transitions; `connection_ip_block_completion.go` and `connection_ip_block_expiry.go` atomically close linked receipts with sensitive-row transitions.
- `connection_ip_blocks_test.go` and `connection_ip_block_expiry_test.go` cover replay, target uniqueness, owner isolation, ciphertext-only durability, job atomicity, manual cancellation, and expiry races.
- `maintenance_runs.go` acquires the configured local-day maintenance lease, supports forced same-day history rows, and records backup-gated cleanup completion.
- `administration_records.go` — audit events, administrator user lists, and backup
  run records.
- `rollover.go`, `rollover_calculation.go`, and `automatic_renewal_rollover.go` persist a calculated rollover before atomically crediting, debiting, expiring, and activating an automatic renewal.
- `payment_profiles.go` — provider-account profile masking, protected deletion, and encrypted credential persistence.
- `retention.go` — bounded cleanup of aged operational records.
- `retention_activity_rollups.go`, `retention_payment_rollups.go`, and
  `retention_purchase_rollups.go` preserve compact activity, payment, purchase,
  and per-member rollover facts before pruning.
- `retention_compaction.go` coordinates all backup-gated cleanup writes in one transaction.
- `retention_native.go` prunes aged native accounting maintenance rows without removing purchase-wide financial evidence or rollback dependencies.
- `retention_operations.go` fails stale open provider operations after 24 hours, resolves linked local state, and removes processed receipts plus their queue records.
- `retention_operation_compensation.go` refunds stale traffic-reset and Emby-setup debits before their operations are removed.
- `retention_housekeeping.go` prunes completed notification events, expired or superseded sessions, abandoned pre-username users, and old maintenance runs.
- `continuity.go` — three-minute queued-entitlement preparation jobs and
  provider-expiry continuity projections.
- `statistics.go` — catalog and activity administrator statistics.
- `product_statistics.go` calculates range-free KPIs across live and compacted
  facts, immutable spending flows, and database/WAL size.
- `product_statistics_catalog.go` calculates active combo and squad distributions with user-facing squad names when local merchandising data exists.
- `product_statistics_payments.go` combines live and compacted EZPay, BEPusdt, and Telegram Stars terminal-status facts.
- `product_statistics_usage.go` selects one current non-admin member combo for live weighted usage projection.
- `statistics_snapshots.go` persists independent last-good statistics partitions.
- `destructive.go` — audited feature deletion transactions.
- `restore.go` — restore snapshot preparation and high-level validation.
- `restore_schema.go` — canonical schema-shape types and database introspection.
- `restore_schema_tables.go` — per-table columns, foreign keys, indexes, objects,
  and schema SQL normalization.

## Test files

- `store_test.go` — core users, catalog guards, and store behavior.
- `billing_purchase_test.go` — purchase creation, quoting, and idempotency.
- `billing_bounds_test.go` covers default and inclusive administrator payment bounds.
- `billing_renewal_core_gross_test.go` covers immutable purchase-time core gross pricing for renewal lineages.
- `automatic_renewal_store_test.go`, `automatic_renewal_expired_store_test.go`, and `automatic_renewal_coupon_test.go` – automatic-renewal defaults, toggle/idempotency/failure behavior, eligibility after a settled expired term, and attached recurring-discount policy.
- `billing_balance_test.go` — concurrent and bounded balance mutations.
- `billing_payment_test.go` — payment settlement and deduplication.
- `payment_callback_tombstones_test.go` covers replay protection after terminal
  payment compaction.
- `billing_refund_test.go` — refund, cancellation, and debt behavior.
- `billing_test_helpers_test.go` — shared billing fixtures and payment constructors.
- `activity_bet_test.go` — atomic bet outcomes and replay.
- `activity_daily_test.go` — daily check-in reward boundaries.
- `activity_draw_test.go` — lucky-draw prizes, extensions, and replay.
- `activity_group_reward_test.go` — group-message counting and rewards.
- `activity_test_helpers_test.go` — shared activity random source.
- `coupon_features_test.go` — coupon redemption and purchase discounts.
- `questionnaire_features_test.go` — questionnaire CSV analysis and settlement.
- `emby_test.go` — Emby setup, provisioning, retry, and refund atomicity.
- `rollover_test.go` — rollover transitions and calculations.
- `release1_commerce_migration_test.go` covers rolling-month conversion, immutable pricing backfill, and new release-one persistence tables.
- `retention_test.go` — retained operational-record cleanup behavior.
- `retention_native_test.go` covers bounded native accounting maintenance retention and preservation of settlement evidence.
- `outbox_priority_dispatch_test.go` covers reserved priority dispatch, one-second coalescing and ordinary-job ordering.
- `maintenance_runs_test.go` covers local-day locking, forced same-day history, and backup-gated maintenance state.
- `retention_provider_operations_test.go` covers retaining provider operations referenced by durable administrative and compensation records.
- `continuity_test.go` covers three-minute provider-expiry preparation without an access gap.
- `connection_scans_test.go` covers metadata-only scan lifecycle and expiry.
- `member_operations_test.go` covers paid reset compensation and zero-usage first-term refunds.
- `automatic_traffic_reset_test.go` covers strict transactional debit replay, provider-gated success, insufficient-balance disablement, and failure compensation notice release.
- `display_currency_test.go` covers owner-scoped currency preference persistence and the TXB default for unrelated members.
- `provider_operations_test.go` covers receipt state transitions, replay conflicts, and ambiguous outcomes.
- `notification_events_test.go` covers provider-gate deduplication, reminder
  eligibility, and traffic reset-period rearming.
- `admin_entitlement_workflows_test.go` covers optimistic edits, immutable pricing, exactly-once credits, and zero-TXB replacements.
- `admin_user_commands_test.go` covers purchase-coupon replays, bounded exact refunds, overlapping temporary bans, and restoration scheduling.
- `admin_user_search_test.go` covers AND/OR profile facets, normalized cursor fingerprints, and success-only affiliate history.
- `admin_bulk_workflows_test.go` covers inclusive-OR matching, active-user deduplication, and equal minute-precise queued-term shifts.
- `admin_operation_projection_test.go` covers owned and bulk-target open-operation aggregation.
- `product_statistics_test.go` verifies administrators are excluded from member population, spend, state, and active-catalog metrics.
- `admin_workflow_test_helpers_test.go` contains shared administrator workflow fixtures and purchase builders.
- `restore_test.go` — restore validation and schema compatibility.
- `ledger_page_test.go` — cursor pagination order and validation.
- `squad_profiles_test.go` — typed profile round trips and legacy Markdown preservation.

`payment_profiles_test.go` covers independent provider-account persistence and stable-ID lookup.

The `migrations/` directory contains the ordered embedded schema history. New
schema changes must be additive migrations; deployed migration files are immutable.

Migration `015_payment_provider_profiles.sql` consolidates the temporary
per-rail profile table into one row per provider. Migration
`016_multiple_payment_profiles.sql` removes the provider-wide uniqueness
constraint so additional independent accounts can be stored. Legacy settings
remain available for decryption fallback, while new writes use the stable
profile record and preserve masked credentials.

`billing_courtesy_test.go` covers terminal-payment courtesy-credit atomicity,
idempotent replay, late-provider-callback blocking, and retention-preserved payment evidence.

`renewal_coupon.go` provides read-only recurring coupon reuse for retained legacy
batch pricing without coupon-use writes. `renewal_batch.go` projects retained
renewal batches and their purchase records. Automatic renewal uses its own
attached-coupon policy and a unique source-successor link.

- `activity_boost_test.go` covers sampled-before-scaled check-in settlement, replay and concurrency, zero-boost refusal, overflow rollback, boosted-only progress and immutable paid amounts.
- `activity_boost_floor_test.go` checks the 1x minimum through both settlement paths and their combined ledger balance.
- `activity_boost_migration_test.go` proves unpaid-progress reset preserves paid windows, ledger and notification rows, and event deduplication identities.

`user_preferences.go` merges nine persisted preference flags atomically, projects active-combo eligibility and resolves legacy referral delivery recipients.
`user_preferences_test.go` covers defaults, partial merges, account isolation, eligibility loss and referral attribution.

`telegram_boost_receipts.go` and `telegram_boost_receipts_test.go` atomically deduplicate group/boost IDs with their outbox work, including concurrent and changed updates. No boost count is mirrored.

`panel_entry_verification.go` and `panel_entry_verification_test.go` persist only Telegram ID and a boolean. Panel visits initialize the row, verification or a disabled challenge qualifies it, and onboarding removes it in the same transaction as agreement acceptance.

`telegram_pm_commands.go` deduplicates webhook IDs and atomically queues notices or reference-only relay operations. `telegram_pm_reads.go` infers read timestamps from member replies and stores references against delivery IDs only. `telegram_pm_queries.go` joins routing and normalized user flags; `telegram_pm_pages.go` reuses stable cursors and exposes bounded delivery references with read evidence. `telegram_pm_moderation.go` audits idempotent flags/repair commands and queues profile refreshes; `telegram_pm_topics.go` claims creation and records durable topic/profile certainty. `telegram_pm_recovery.go` resumes only definitely unsent waiters after a proven repair. `telegram_pm_retention.go` bounds receipts at 30 days while preserving routing uncertainty; generic maintenance protects recent PM operations and conversation owners.
`telegram_pm_commands_test.go`, `telegram_pm_permissions_test.go`, `telegram_pm_topics_test.go` and `telegram_pm_retention_test.go` cover concurrent replay, normalized flags across destinations, unauthorized mutation, serialized topic/card phases, safe recovery, retention boundaries and foreign keys in hosted CI.
`telegram_pm_reads_test.go` covers reply-inferred read state. `admin_balance_reference_test.go` covers stable-reference replay without duplicate ledger or audit effects.

`retention_node_events.go` expires every node-down event status seven days after creation, detaches node-state references and cascades frozen details. Already approved purchase extensions and queued `node_compensation` operation targets remain independently executable; a continuing outage starts a fresh window on the next complete provider sample. `retention_sync_jobs.go` removes terminal failed outbound synchronization/delivery jobs after three days from final failure (`updated_at`), while preserving active work and explicit queued retries. It reconciles linked purchase, provisioning, scan and PM certainty state without financial writes. Local rollover, raffle and questionnaire settlement lanes retain their existing recovery paths. `retention_timestamps.go` uses indexed candidates and parsed instants for exact fractional-second boundaries. Counts flow through the existing backup-gated maintenance report. `retention_node_events_test.go` and `retention_sync_jobs_test.go` cover creation/failure boundaries, all event statuses, fresh outages, approved extensions, independent sync, financial ownership, PM uncertainty, explicit retries and foreign keys in hosted CI.

`connectivity_attempts.go`, `connectivity_queries.go`, `connectivity_scan.go` and `connectivity_retention.go` implement the `connectivity.Repository` interface against one indexed diagnostic-attempt table. A probe inserts its identity and running state before execution, then seals a sanitized terminal result with a conditional transition. Host UUID is nullable for setup failures; resolved proxy configurations, host metadata and credentials are never persisted. Latest results are derived by host and configuration hash, while history uses the shared filter-bound timestamp cursor with a default page of 50 and a maximum of 200. Fixed-width UTC nanosecond timestamps provide exact indexed ordering and retention comparisons. Every read excludes attempts at or before `now-24h`, independently of pruning. Checker startup interrupts unfinished attempts without changing their start, and its independent five-minute cleanup calls the pruning method even while scheduling is disabled. Mutations use the existing store write lock and context-aware SQLite calls; the storage module performs no external API calls. The connectivity lifecycle, queries, retention and concurrency tests cover terminal immutability, nullable setup results, sanitized codes, restart recovery, pagination, configuration separation and exact fractional-second expiry in hosted CI.

`connectivity_lifecycle_test.go`, `connectivity_queries_test.go`, `connectivity_retention_test.go` and `connectivity_concurrency_test.go` exercise those repository invariants in hosted CI; `connectivity_test_helpers_test.go` provides credential-free attempt fixtures.

`connectivity_timeline.go` reads all retained terminal host observations for exactly one configuration hash and the rolling 24-hour cutoff. The existing indexed attempt table provides history; setup failures and running checks cannot fabricate downtime, and pagination cannot truncate the derived timeline.
`connectivity_timeline_test.go` covers configuration isolation, exact retention and complete observation retrieval beyond administrative page limits in hosted CI.
- `ip_lookup_settings.go` atomically saves encrypted settings, combo quotas and the admin audit.
- `ip_lookup_queries.go` reads fresh quota/pricing quotes and owner-scoped immutable report receipts.
- `ip_lookup_commands.go` atomically debits quota/TXB and coalesces checks into shared cached or fresh reports.
- `ip_lookup_completion.go` persists minimal attempt checkpoints and settles joined checks atomically. Contacted-provider outages produce charged, cacheable partial reports; only a never-attempted infrastructure failure restores costs exactly once. Freezing a partial report schedules its delayed background repair job in the same transaction.
- `ip_lookup_test.go` covers cache sharing, prices, replays, refunds and purchase quota lifecycle for CI.
- `ip_lookup_migration_test.go` covers the one-check gift for live members, excluding queued and expired purchases.

- `ip_lookup_refunds.go` restores never-attempted costs and closes interrupted shared runs during existing operation maintenance. Attempted calls remain charged and cacheable; IP receipts are excluded from generic compensation after domain settlement.
- `ip_lookup_cache.go` serves free compact quote previews, preferring exact-IP evidence before the newest completed IPv4 /24 report. IPv6 stays exact-only; parsed completion timestamps avoid RFC3339 text ordering errors. Quote previews create no receipt, debit, or upstream request.
- Check metadata retains the requested address and cache match independently of the evaluated report address, including after reload. Signed submission fields exclude the display-only cached report.
- `ip_lookup_compact.go` converts legacy report payloads in bounded migration batches through the core versioned codec. Consolidated facts and minimal pending-stage markers replace per-provider snapshots; historical statuses, receipts, ledger entries, and refunds remain intact.
- `ip_lookup_cache_test.go` covers free previews, exact-IP precedence, IPv4 /24 boundaries, IPv6 isolation, nanosecond completion ordering, cache arrival during submission, and display fields outside quote signatures in hosted CI.
- `ip_lookup_compact_test.go` covers migration of legacy provider snapshots, preservation of pending execution markers, and unchanged historical balances, ledger entries, and refunded receipts in hosted CI.
- `ip_lookup_fixture_test.go` supplies isolated lookup configuration, balances, purchase allowances, signed submissions, and terminal report fixtures for the database regression files.
- `ip_lookup_maintenance_test.go` covers durable receipts and the distinct settlement of never-attempted jobs versus interrupted, chargeable provider calls in hosted CI.
- `ip_lookup_policy_test.go` covers free cache reads with available quota, charged partial/refused/all-error reports, repeat settlement, and fractional completion timestamps in hosted CI.
`telegram_pm_content.go` encrypts operation-bound pending outbound text using the existing application vault, reads live envelopes, and distinguishes content-free expiry/settlement markers from legacy reference-only jobs. `telegram_pm_content_test.go` covers duplicate updates, encryption/AAD binding, outbox privacy, transactional settlement erasure and 24-hour repair expiry. Generic operation completion clears settled ciphertext; PM retention clears expired content while operation pruning cascades the marker.

## Persistent realtime IP evidence

- `ip_lookup_live_cache.go` stores compact shared snapshots and selects exact or IPv4 /24 evidence using the same timestamp/rowid ordering as quality reports. Snapshots do not expire or depend on current credentials. Storing a partial snapshot transactionally schedules its first background section retry through the shared outbox using the current settings policy.
- `ip_lookup_live_stage.go` claims the report's realtime phase, records actual queued attempts and recovers stored snapshot references before any replay. The live checkpoint is independent of quality checkpoint writes.
- `ip_lookup_live_check.go` adds live-phase progress and the referenced final snapshot to owner-scoped receipts without exposing recovery state.
- `ip_lookup_live_cache_test.go` covers persistent matching, restart reads, nanosecond/tie ordering and absence of original paths in hosted CI.
- `ip_lookup_live_refresh_test.go` covers combined refresh pricing, subnet eligibility, replay-safe snapshot recovery and maintenance settlement without refund after live attempts.
- `ip_lookup_retry_jobs.go` schedules delayed first repair jobs for partial reports/snapshots inside the freezing transaction (payload attempt counters keep chains deduplicated) and exposes guarded in-place repair writes: partial-only cache updates keep `completed_at` so newer paid refreshes keep winning, and missing rows map to silent chain termination.
- `ip_lookup_retry_jobs_test.go` covers first-attempt scheduling with the default delay, no scheduling for succeeded freezes, partial-guard conflicts, unchanged completion timestamps and live section retry payloads in hosted CI.

Migration 069 creates snapshots and a private JSON live checkpoint on existing
quality reports. Legacy receipts default to no realtime phase; no backfill calls
run. Joined checks share one refreshed snapshot. Attempted failures settle
partial without changing the quality verdict; pre-attempt failures retain the
existing refund path. Ledger changes and receipt settlement remain atomic.

`ip_lookup_background.go` projects shared progress, claims exclusive worker leases and renews heartbeats. `ip_lookup_replacement.go` atomically publishes the newest combined /24 report generation, detaches historical receipt references and deletes superseded payloads. Checks record the exact quota quantity, including refunds; financial records survive cache removal.

`ip_lookup_background_test.go` covers exclusive lease recovery, identity-free progress, free shared submissions, exact refresh-unit refunds and combined-cache replacement with preserved receipts in hosted CI.

`ip_lookup_admission_test.go` covers global FIFO admission across process owners, free cross-user same-IP joining, quote/completion races, idempotency and repair deferral without burning attempts. `ip_lookup_background.go` admits only the oldest pending report and excludes active report/live repairs. `outbox_jobs.go` excludes those repair kinds while any fresh report waits or runs, while unrelated jobs remain claimable. `retention_operations.go` protects unstarted queue entries and live leases; only admitted, abandoned IP runs enter stale settlement. No new persistence is introduced.

`ip_lookup_waiting_test.go` verifies an unstarted report and its receipt survive a long queue wait and remain eligible for admission after maintenance.

IP lookup resumes and report repair read current saved settings. `ip_lookup_current_config_test.go` verifies column removal, retained checkpoints/references/rowids, and current settings reads. Signed quotes and settlement amounts remain transactional.

- `billing_settlement_tx.go` ? Shared transactional payment credit, ledger and outbox path.
- `usdt_migration.go` ? Verified upgrade backup, dedicated connection table rebuild and FK restoration.
- `usdt_quotes.go` ? Permanent amount reservations and atomic checkout snapshots.
- `usdt_transfers.go` ? Immutable verified receiving events committed before scan progress.
- `usdt_transfer_queries.go` ? Local cursor, recovery and keyset review queries.
- `usdt_settlement.go` ? Atomic receipt claim, exact identity validation, bounded shortfall matching, audited review and ledger credit.
- `usdt_test_helpers_test.go` ? Shared hosted quote and transfer fixtures.
- `usdt_quotes_test.go` ? Concurrent allocation, permanence, exhaustion and BSC atomic-unit checks.
- `usdt_settlement_test.go` ? Delayed observation, replay, role, amount, audit and failed-batch checks.
- `usdt_migration_test.go` ? Legacy payment/FK preservation and verified upgrade backup checks.

`xnode_nodes.go` Stores node metadata, credential digests and monotonic revisions.
`xnode_configurations.go` Commits versioned base configs and ordered snippet references.
`xnode_snippets.go` Stores versioned encrypted reusable snippets.
`xnode_groups.go` Stores authoritative groups and node/inbound bindings.
`xnode_endpoints.go` Stores subscription endpoints and encrypted client overrides.
`xnode_identities.go` Stores stable proxy IDs and encrypted credential/token references.
`xnode_periods.go` Stores accounting periods and reads exact byte aggregates.
`xnode_traffic.go` Commits contiguous reports exactly once with transactional weighted-byte carry.
`xnode_customizations.go` Stores owner/node customization revisions and encrypted inputs.
`xnode_rates.go` Commits multiplier boundaries after accounting barriers.
`xnode_restrictions.go` Projects existing manual access restrictions.

- `outbox_heartbeat.go`: Entry points: `RenewOutboxJob`.

- `xnode_barriers.go`: Entry points: `UserByUsername`, `XNodePeriods`, `TrackXNodePeriodNode`, `XNodePeriodNodes`.

- `xnode_fleet_statistics.go`: Entry points: `XNodeFleetUsage`, `XNodeIdentityGrowth`.

- `xnode_imported_history.go` and `xnode_imported_usage.go` read immutable historical accounting for settlement and history views.


- `xnode_service_bindings.go`: Entry points: `XNodeServiceBindings`, `SaveXNodeServiceBinding`.

- `xnode_operation_recovery_test.go` covers durable lease recovery and stale asynchronous receipt callbacks.

## Memory budget

The authoritative database has at most four connections with a 4 MiB page cache
per connection, no memory mapping and file-backed temporary work. Bootstrap
places temporary files under DATA_DIR/tmp for read-only-root deployments. Proxy
indexes live in a separate disposable cache outside this schema and its backups.

`api_tokens.go` reads hashed credentials and bounded owned metadata. `api_token_writes.go` creates/revokes tokens with transactional audits. `api_tokens_test.go` covers hash-only storage, expiry and owner isolation.

`retired_detector.go` reads only acknowledged pre-upgrade restrictions and atomically records their release once. `retired_detector_test.go` upgrades a real pre-078 schema and verifies removed secrets/raw samples, canceled jobs, manual restrictions, financial history, ambiguous-mutation audits, foreign keys and idempotent release. Historical detector incident rows are archived; they have no member or administrator detector API.

`xnode_commerce_test.go` covers in-place cadence persistence without replacing the accounting period.

`admin_user_provider_commands.go`, `admin_user_ban_state.go` and `admin_user_audit.go` own manual restrictions, expiry restoration and audit records.

`native_rollover_version_test.go` preserves native accounting provenance through calculation and atomic renewal. `legacy_controls_retirement_test.go` verifies that upgrade retirement disables executable relink/host jobs while retaining receipts.

Native rollover calculations retain `native-term-v1` through the renewal transaction; old calculated records continue through the existing whole-term normalization. No settled financial records are rewritten.

- `xnode_compile_version.go`: Reads a small transactional compile revision and the next purchase access boundary.

- `xnode_named_services.go`: Atomically saves standalone encrypted terms, authorization and quota without rewriting usage or settled periods.

Firewall operational storage: `xnode_firewall_tx.go` owns atomic writes, bounded compressed participant bitmaps and the existing coalesced outbox. `xnode_firewall_members.go` tracks fleet versions; `xnode_firewall_participants.go` joins delivered pages; `xnode_firewall_events.go` commits monotonic replay cursors and master authorization. `xnode_firewall_page.go` reads version-bound snapshots without holding transactions over RPC. `xnode_firewall_sources.go` stores generation metadata only and prunes operational history. `xnode_firewall_retirement.go` retires disabled/deleted participants in bounded pages while offline active members remain pending. `xnode_firewall_purchase.go` atomically debits, appends the ledger, grants an exact-IP exemption and queues delivery. `xnode_firewall_test.go` covers replay, all-node expiry, concurrent purchase deduplication and retained proxy evidence.

`xnode_firewall_page.go` projects effective compact restriction prefixes after subtracting active exact-IP exemptions. It retains the underlying immutable detection/audit evidence and restores classification eligibility when the exemption ends; IP ranges are never enumerated.

`table_migration.go` reuses one guarded referenced-table rebuild path for USDT and native rate upgrades, preserving named indexes/triggers and restoring verified FK enforcement before pool reuse. `xnode_zero_migration.go` creates and verifies the pre-upgrade backup for native zero multipliers.

`xnode_zero_rate_test.go` covers raw usage on free nodes, immutable priced ranges and rejection of negative rates. Rate changes start after the latest committed cursor so late flush results cannot rewrite settled pricing.

`xnode_zero_migration_test.go` covers upgrade backups, foreign-key restoration, dependent views/indexes, retained group references and compile invalidation triggers.


`xnode_routing_plans.go` commits encrypted managed graphs and provider operations
atomically, preserves prior generations through deployment, and audits phase
changes. It keeps optimistic revisions and operation replay fingerprints distinct.
`xnode_routing_plans_test.go` covers owner receipt isolation, concurrent graph
rejection, replay, generation barriers, retained evidence and recovered attempts.

`usdt_review.go` provides payment-scoped matching evidence and transactional permanent
review dismissal. `stablecoin_test.go` covers token isolation and concurrent courtesy,
settlement, dismissal and scanner replay. Migration 085 keeps old USDT cursors and
evidence IDs, and makes credited/dismissed transfers immutable in SQLite.

`stablecoin_migration_test.go` verifies legacy cursor progress, permanent quote fingerprints, transfer IDs and token-specific outstanding quote watchers across migration 085.

`stablecoin_matching_test.go` covers exact-first matching, tolerance boundaries, overpayments and manual ambiguity rejection for both native tokens.

`xnode_firewall_admin.go` atomically writes an administrator grant, audit and provider receipt, then observes the existing fleet delivery clock. `xnode_firewall_admin_test.go` covers free idempotent grants and delivery completion. `xnode_geoip_queue.go` coalesces country preparation through the existing outbox; `xnode_geoip_queue_test.go` covers active-job coalescing and failed-job recovery. `outbox_admission_test.go` covers urgent firewall priority and capacity deferral without exhausting retries.

`member_squad_access.go` normalizes last-enabled preferences by panel owner during entitlement mutations, independent of a legacy link. `member_squad_native_test.go` covers unrelated provider jobs, malformed historical payloads, reset fencing and native preferences. `billing_purchase_addon_replay.go` and `billing_purchase_addon_replay_test.go` replay charged selections before catalog changes and preserve one debit. Add-on quotes/commits share the entitlement conflict fence; new entitlement mutation kinds must be included in member_combo_control_helpers.go, while IP lookup, media and wallet-provider jobs remain independent.

`billing_purchase_addon_quote.go` rejects non-NULL squad/add-on entitlement
overrides, including `[]`, before pricing or writes, and checks paid add-on facts
independently of effective entitlements. `billing_purchase_addon_override_test.go`
covers no-debit/no-ledger/no-adjustment rejection and normal duplicate handling.
Charged idempotent replay remains before the quote fence; previously settled
financial facts are never changed by this guard.

`xnode_routing_access.go` atomically prunes owner-scoped operational route state,
clears previous choices/superseded customizations, disables remaining choices,
increments the compile revision and appends an audit. It preserves immutable
legacy evidence and accounting. `xnode_routing_access_test.go` covers owner and
revision fences, stale apply rejection, evidence retention and node invalidation.

Access pruning also fences the authoritative compile revision and defers across
a purchase boundary. A concurrent entitlement/endpoint change rejects stale
cleanup instead of removing routes under newly granted access.

`xnode_metered_probes.go` atomically binds probe access and immutable origin
references, with revocation/barrier evidence. `xnode_probe_accounting.go` maps
exact probe/period/target samples to the owner inside normal journal transactions.
`xnode_probe_accounting_test.go` covers duplicates, multiplier carry, missing
periods, late tails and successor isolation across reset/renewal/refund boundaries.
`xnode_speedtest_operations.go` replays owner-scoped operation keys before renewed
route/access validation. No command result bytes update financial usage.

`xnode_firewall_search.go` streams active records and applies prefix overlap before
page limits, without range expansion or a persisted duplicate search index.
`xnode_firewall_release.go` releases an exact generation, retains audit/paid grant
records, and stores version evidence in existing operation items. Receipts finish
only when active firewall members acknowledge that version. Hosted regressions
in `xnode_firewall_search_test.go` and `xnode_firewall_release_test.go` cover matches
beyond the first 200 rows, IPv6 overlap, cursor paging, stale generations, replay
and delivery acknowledgment.

`xnode_diy.go` persists one encrypted override document per member under both
settings/routing revision fences, sharing the durable receipt transaction with
`xnode_routing_queue.go`. Local saves complete without routing deployment.
`xnode_diy_test.go` covers atomic rollback, owner isolation, stale revisions,
idempotency and subscription-only writes during pending routing deployment.

`xnode_firewall_detection.go` stages bounded memory-only fleet counters, committing
only after durable cursor/ban writes; `xnode_firewall_detection_test.go` covers
cross-node totals, replay, exclusion, rollback and expiry. `xnode_firewall_policy.go`
migrates encrypted node rules to one encrypted global policy.
`xnode_firewall_protected.go` releases existing managed-node false positives.
`xnode_whitelist.go` derives capacity from the effective purchase's combo;
`xnode_whitelist_test.go` checks paid-unban refusal and charged retry replay.
`xnode_diy_reconcile.go` batches free reference cleanup through the existing outbox
and uses revision fences; it never debits a wallet or allowance.

`xnode_diy_billing.go` shares `ip_lookup_allowances` and `changeBalanceTx` with IP
checks under the same write lock and transaction. Successful changed saves consume
one allowance or the configured fresh lookup fee. Unconfigured fees reject paid
changes; zero is an explicitly configured free price. Quote drift, stale revisions
and insufficient funds roll back desired settings, routing, charge and receipt.
Financial metadata and the committed revision remain under `diySave` in the
existing operation result. Deployment completion merges its result so retries
retain the exact initial outcome. No additional billing table or cache is added.

`xnode_diy_billing_test.go` covers shared allowance races, paid and free saves,
transaction rollback, stale/expired quotes and exact lost-response replay in hosted CI.

`native_retirement.go` verifies a boundary backup before bridge cleanup. `native_retirement_test.go` covers incomplete accounting refusal, exact historical facts, and translated queued access. `proxy_provisioning.go`, `proxy_provisioning_backlog.go` and `proxy_provisioning_test.go` own paid native identity references and recovery. `xnode_imported_usage.go` and `xnode_imported_history.go` expose frozen pre-native accounting facts for settlement and historical views; they provide no ingestion or synchronization API.

`outbox_admission.go` and `outbox_resources.go` derive account, node, purchase, operation and chat exclusion from existing records inside transactional claims. Independent due jobs skip occupied resources. IP Lookup repair admission stays serial.

Ordinary admission prefers non-dynamic work; dynamic firewall jobs retain their
dedicated reserved claim path and can use idle ordinary capacity. A continuously
replenished firewall backlog therefore cannot starve member/provider jobs.
`outbox_admission_test.go` checks both paths progress across repeated busy retries
without exhausting attempts. Per-resource exclusion applies to both paths.

`outbox_resources_test.go` checks independent account admission, cross-kind account ordering and ownerless standalone targets.

API-token revocation deletes the complete credential row atomically with a secret-free audit event. The owner-scoped audit makes retries idempotent without a credential tombstone.

`xnode_diy_retirement.go` pages saved member references and atomically replaces old ciphertext using user, revision and cipher fences. It does not change financial records, routing or native compilation.

Due outbox jobs with identical timestamps retain insertion order through SQLite rowid; opaque random IDs never decide account execution order.

`xnode_routing_recovery.go` wakes routing/DIY retries on authenticated node
reconnection, using the existing outbox and original receipt. It transactionally
recreates missing jobs for unfinished plans, leaves live leases and terminal
receipts untouched, and never charges or rewrites desired routing/accounting.
`xnode_routing_recovery_test.go` checks missing jobs, delayed/failed retries,
duplicate wakes, live leases, terminal receipts and retained DIY result evidence.


## Automatic PAYG

`payg_catalog.go` filters future sales and snapshots base squad references only when the filtered set differs while preserving full custom overrides and editable add-ons. `payg_rollover.go` settles reconciled rollover when conversion removes the last eligible renewal squad. Existing paid terms do not filter against live billing mode. `payg_commerce_test.go` covers grandfathered terms and empty-renewal settlement.

`xnode_payg_accounts.go` stores immutable member/squad/node attribution, its native period and atomic accounting participant reference. `xnode_payg_billing.go` appends exact GB block charges to the existing wallet ledger with immutable rate evidence. `xnode_payg_traffic.go` integrates raw usage and debits with contiguous traffic-journal commits; balances precede periods in its lock order. Connection purchases require funds, while delayed traffic may create wallet debt. PAYG never consumes combo quota or node multipliers. The first GB is purchased only after 10,240 cumulative raw bytes on that member/squad/node; idle connections and wallet funding cannot charge a first block. Initial bytes count toward the purchased GB.

`xnode_payg_jobs.go` coalesces balance-change provisioning through the existing outbox, scans missing tuples in bounded batches and prunes disposable 72-hour minute aggregates. Credentials, cumulative counters and financial evidence remain durable. Source IPs, connection observations, generated subscriptions and lease timestamps are never stored. `xnode_payg_owners.go` provides indexed service lookups, node-scoped compiler identities and credential-free activity ownership reads.

`onboarding.go` queues `xnode_access_repair` for every agreement completion and
`xnode_payg_reconcile` when PAYG is configured, in the agreement transaction.
`xnode_access_repair.go` also backfills completed members without credentials,
independently of purchases or funds. Existing paid-period repair retains its
restriction and accounting fences. `onboarding_provisioning_test.go` covers
purchase-free signup, PAYG queueing, incomplete-member exclusion and coalesced
backfill in hosted SQLite/PostgreSQL CI.

`xnode_payg_billing_test.go` and `xnode_payg_concurrency_test.go` cover automatic GB boundaries, replay, price changes, debt, failed batches, simultaneous nodes and bounded statistics in hosted SQLite/PostgreSQL CI. Local automated suites remain prohibited.

`automatic_renewal_types.go` owns renewal quote contracts. `automatic_renewal_failure.go` retains existing failed-renewal settlement; PAYG empty renewals preserve rollover separately. `billing_renewal_quote.go` isolates future-term pricing from batch mutation.


<!-- generated transaction retry map -->

Generated adapters repeat complete rolled-back PostgreSQL operations for SQLSTATE
40001/40P01, with bounded context-aware backoff. SQLite retains its writer lock.
The generator excludes methods that access SQL outside their transaction or own
mutable in-memory batches. Provider requests remain outside every transaction.

- `activity_draws_part2_retry_generated.go` wraps the transactional methods in `activity_draws_part2.go`.
- `activity_draws_retry_generated.go` wraps the transactional methods in `activity_draws.go`.
- `activity_history_part2_retry_generated.go` wraps the transactional methods in `activity_history_part2.go`.
- `activity_part2_retry_generated.go` wraps the transactional methods in `activity_part2.go`.
- `activity_raffle_entries_retry_generated.go` wraps the transactional methods in `activity_raffle_entries.go`.
- `activity_raffle_publish_retry_generated.go` wraps the transactional methods in `activity_raffle_publish.go`.
- `activity_raffle_settle_retry_generated.go` wraps the transactional methods in `activity_raffle_settle.go`.
- `activity_retry_generated.go` wraps the transactional methods in `activity.go`.
- `admin_bulk_extension_retry_generated.go` wraps the transactional methods in `admin_bulk_extension.go`.
- `admin_combo_replacement_retry_generated.go` wraps the transactional methods in `admin_combo_replacement.go`.
- `admin_coupon_grants_retry_generated.go` wraps the transactional methods in `admin_coupon_grants.go`.
- `admin_entitlement_edit_retry_generated.go` wraps the transactional methods in `admin_entitlement_edit.go`.
- `admin_entitlement_refund_retry_generated.go` wraps the transactional methods in `admin_entitlement_refund.go`.
- `admin_operation_resolution_retry_generated.go` wraps the transactional methods in `admin_operation_resolution.go`.
- `admin_user_provider_commands_retry_generated.go` wraps the transactional methods in `admin_user_provider_commands.go`.
- `administration_records_retry_generated.go` wraps the transactional methods in `administration_records.go`.
- `affiliate_config_retry_generated.go` wraps the transactional methods in `affiliate_config.go`.
- `affiliate_referrals_retry_generated.go` wraps the transactional methods in `affiliate_referrals.go`.
- `api_token_writes_retry_generated.go` wraps the transactional methods in `api_token_writes.go`.
- `automatic_renewal_plan_retry_generated.go` wraps the transactional methods in `automatic_renewal_plan.go`.
- `automatic_renewal_rollover_retry_generated.go` wraps the transactional methods in `automatic_renewal_rollover.go`.
- `automatic_renewal_state_retry_generated.go` wraps the transactional methods in `automatic_renewal_state.go`.
- `automatic_traffic_reset_retry_generated.go` wraps the transactional methods in `automatic_traffic_reset.go`.
- `billing_bounds_retry_generated.go` wraps the transactional methods in `billing_bounds.go`.
- `billing_courtesy_retry_generated.go` wraps the transactional methods in `billing_courtesy.go`.
- `billing_ledger_retry_generated.go` wraps the transactional methods in `billing_ledger.go`.
- `billing_payment_settlement_retry_generated.go` wraps the transactional methods in `billing_payment_settlement.go`.
- `billing_payments_retry_generated.go` wraps the transactional methods in `billing_payments.go`.
- `billing_purchase_addons_retry_generated.go` wraps the transactional methods in `billing_purchase_addons.go`.
- `billing_purchase_cancellation_retry_generated.go` wraps the transactional methods in `billing_purchase_cancellation.go`.
- `billing_purchases_retry_generated.go` wraps the transactional methods in `billing_purchases.go`.
- `billing_quote_retry_generated.go` wraps the transactional methods in `billing_quote.go`.
- `billing_refunds_retry_generated.go` wraps the transactional methods in `billing_refunds.go`.
- `billing_renewal_retry_generated.go` wraps the transactional methods in `billing_renewal.go`.
- `billing_retry_generated.go` wraps the transactional methods in `billing.go`.
- `browser_access_links_retry_generated.go` wraps the transactional methods in `browser_access_links.go`.
- `catalog_retry_generated.go` wraps the transactional methods in `catalog.go`.
- `compensation_config_retry_generated.go` wraps the transactional methods in `compensation_config.go`.
- `compensation_dismiss_retry_generated.go` wraps the transactional methods in `compensation_dismiss.go`.
- `compensation_observation_retry_generated.go` wraps the transactional methods in `compensation_observation.go`.
- `compensation_review_retry_generated.go` wraps the transactional methods in `compensation_review.go`.
- `connection_ip_block_completion_retry_generated.go` wraps the transactional methods in `connection_ip_block_completion.go`.
- `connection_ip_block_expiry_retry_generated.go` wraps the transactional methods in `connection_ip_block_expiry.go`.
- `connection_ip_blocks_mutations_retry_generated.go` wraps the transactional methods in `connection_ip_blocks_mutations.go`.
- `connection_ip_blocks_retry_generated.go` wraps the transactional methods in `connection_ip_blocks.go`.
- `connection_scan_lifecycle_retry_generated.go` wraps the transactional methods in `connection_scan_lifecycle.go`.
- `connection_scans_retry_generated.go` wraps the transactional methods in `connection_scans.go`.
- `continuity_retry_generated.go` wraps the transactional methods in `continuity.go`.
- `coupon_purchase_retry_generated.go` wraps the transactional methods in `coupon_purchase.go`.
- `coupons_part2_retry_generated.go` wraps the transactional methods in `coupons_part2.go`.
- `coupons_part3_retry_generated.go` wraps the transactional methods in `coupons_part3.go`.
- `destructive_retry_generated.go` wraps the transactional methods in `destructive.go`.
- `emby_operations_retry_generated.go` wraps the transactional methods in `emby_operations.go`.
- `emby_provisioning_part2_retry_generated.go` wraps the transactional methods in `emby_provisioning_part2.go`.
- `emby_provisioning_retry_generated.go` wraps the transactional methods in `emby_provisioning.go`.
- `emby_setup_retry_generated.go` wraps the transactional methods in `emby_setup.go`.
- `ip_lookup_cache_retry_generated.go` wraps the transactional methods in `ip_lookup_cache.go`.
- `ip_lookup_commands_retry_generated.go` wraps the transactional methods in `ip_lookup_commands.go`.
- `ip_lookup_completion_retry_generated.go` wraps the transactional methods in `ip_lookup_completion.go`.
- `ip_lookup_live_cache_retry_generated.go` wraps the transactional methods in `ip_lookup_live_cache.go`.
- `ip_lookup_live_stage_retry_generated.go` wraps the transactional methods in `ip_lookup_live_stage.go`.
- `ip_lookup_queries_retry_generated.go` wraps the transactional methods in `ip_lookup_queries.go`.
- `ip_lookup_settings_retry_generated.go` wraps the transactional methods in `ip_lookup_settings.go`.
- `maintenance_runs_retry_generated.go` wraps the transactional methods in `maintenance_runs.go`.
- `member_early_activation_retry_generated.go` wraps the transactional methods in `member_early_activation.go`.
- `member_operation_begin_retry_generated.go` wraps the transactional methods in `member_operation_begin.go`.
- `member_refund_commit_retry_generated.go` wraps the transactional methods in `member_refund_commit.go`.
- `member_reset_compensation_retry_generated.go` wraps the transactional methods in `member_reset_compensation.go`.
- `member_squad_access_retry_generated.go` wraps the transactional methods in `member_squad_access.go`.
- `member_squad_preferences_retry_generated.go` wraps the transactional methods in `member_squad_preferences.go`.
- `notification_admin_cancel_retry_generated.go` wraps the transactional methods in `notification_admin_cancel.go`.
- `notification_admin_finance_retry_generated.go` wraps the transactional methods in `notification_admin_finance.go`.
- `notification_events_retry_generated.go` wraps the transactional methods in `notification_events.go`.
- `notification_scans_retry_generated.go` wraps the transactional methods in `notification_scans.go`.
- `onboarding_retry_generated.go` wraps the transactional methods in `onboarding.go`.
- `operations_retry_generated.go` wraps the transactional methods in `operations.go`.
- `outbox_jobs_retry_generated.go` wraps the transactional methods in `outbox_jobs.go`.
- `outbox_retry_operations_retry_generated.go` wraps the transactional methods in `outbox_retry_operations.go`.
- `panel_entry_verification_retry_generated.go` wraps the transactional methods in `panel_entry_verification.go`.
- `payment_operation_resolution_retry_generated.go` wraps the transactional methods in `payment_operation_resolution.go`.
- `payment_operations_retry_generated.go` wraps the transactional methods in `payment_operations.go`.
- `payment_profiles_retry_generated.go` wraps the transactional methods in `payment_profiles.go`.
- `payment_refund_resolution_retry_generated.go` wraps the transactional methods in `payment_refund_resolution.go`.
- `provider_operation_lifecycle_retry_generated.go` wraps the transactional methods in `provider_operation_lifecycle.go`.
- `provider_operation_notification_completion_retry_generated.go` wraps the transactional methods in `provider_operation_notification_completion.go`.
- `provider_operations_retry_generated.go` wraps the transactional methods in `provider_operations.go`.
- `proxy_provisioning_backlog_retry_generated.go` wraps the transactional methods in `proxy_provisioning_backlog.go`.
- `purchase_sync_retry_generated.go` wraps the transactional methods in `purchase_sync.go`.
- `questionnaire_imports_retry_generated.go` wraps the transactional methods in `questionnaire_imports.go`.
- `questionnaire_operations_retry_generated.go` wraps the transactional methods in `questionnaire_operations.go`.
- `questionnaire_settlement_retry_generated.go` wraps the transactional methods in `questionnaire_settlement.go`.
- `questionnaires_part2_retry_generated.go` wraps the transactional methods in `questionnaires_part2.go`.
- `questionnaires_retry_generated.go` wraps the transactional methods in `questionnaires.go`.
- `retention_compaction_retry_generated.go` wraps the transactional methods in `retention_compaction.go`.
- `retention_retry_generated.go` wraps the transactional methods in `retention.go`.
- `retired_detector_retry_generated.go` wraps the transactional methods in `retired_detector.go`.
- `rollover_calculation_retry_generated.go` wraps the transactional methods in `rollover_calculation.go`.
- `rollover_retry_generated.go` wraps the transactional methods in `rollover.go`.
- `store_part2_retry_generated.go` wraps the transactional methods in `store_part2.go`.
- `store_retry_generated.go` wraps the transactional methods in `store.go`.
- `telegram_boost_receipts_retry_generated.go` wraps the transactional methods in `telegram_boost_receipts.go`.
- `telegram_pm_commands_retry_generated.go` wraps the transactional methods in `telegram_pm_commands.go`.
- `telegram_pm_moderation_retry_generated.go` wraps the transactional methods in `telegram_pm_moderation.go`.
- `telegram_pm_recovery_retry_generated.go` wraps the transactional methods in `telegram_pm_recovery.go`.
- `usdt_quotes_retry_generated.go` wraps the transactional methods in `usdt_quotes.go`.
- `usdt_review_retry_generated.go` wraps the transactional methods in `usdt_review.go`.
- `usdt_settlement_retry_generated.go` wraps the transactional methods in `usdt_settlement.go`.
- `usdt_transfers_retry_generated.go` wraps the transactional methods in `usdt_transfers.go`.
- `user_preferences_retry_generated.go` wraps the transactional methods in `user_preferences.go`.
- `xnode_access_repair_retry_generated.go` wraps the transactional methods in `xnode_access_repair.go`.
- `xnode_barriers_retry_generated.go` wraps the transactional methods in `xnode_barriers.go`.
- `xnode_configurations_retry_generated.go` wraps the transactional methods in `xnode_configurations.go`.
- `xnode_diy_billing_retry_generated.go` wraps the transactional methods in `xnode_diy_billing.go`.
- `xnode_diy_reconcile_retry_generated.go` wraps the transactional methods in `xnode_diy_reconcile.go`.
- `xnode_diy_retry_generated.go` wraps the transactional methods in `xnode_diy.go`.
- `xnode_firewall_policy_retry_generated.go` wraps the transactional methods in `xnode_firewall_policy.go`.
- `xnode_firewall_tx_retry_generated.go` wraps the transactional methods in `xnode_firewall_tx.go`.
- `xnode_groups_retry_generated.go` wraps the transactional methods in `xnode_groups.go`.
- `xnode_metered_probes_retry_generated.go` wraps the transactional methods in `xnode_metered_probes.go`.
- `xnode_named_services_retry_generated.go` wraps the transactional methods in `xnode_named_services.go`.
- `xnode_nodes_retry_generated.go` wraps the transactional methods in `xnode_nodes.go`.
- `xnode_payg_accounts_retry_generated.go` wraps the transactional methods in `xnode_payg_accounts.go`.
- `xnode_payg_jobs_retry_generated.go` wraps the transactional methods in `xnode_payg_jobs.go`.
- `xnode_payg_traffic_retry_generated.go` wraps the transactional methods in `xnode_payg_traffic.go`.
- `xnode_periods_retry_generated.go` wraps the transactional methods in `xnode_periods.go`.
- `xnode_rates_retry_generated.go` wraps the transactional methods in `xnode_rates.go`.
- `xnode_routing_access_retry_generated.go` wraps the transactional methods in `xnode_routing_access.go`.
- `xnode_routing_plans_retry_generated.go` wraps the transactional methods in `xnode_routing_plans.go`.
- `xnode_routing_recovery_retry_generated.go` wraps the transactional methods in `xnode_routing_recovery.go`.
- `xnode_traffic_retry_generated.go` wraps the transactional methods in `xnode_traffic.go`.
`admin_entitlements_page.go` filters display validity using the current UTC boundary while returning the untouched purchase lifecycle status. Pending rollover/renewal work remains recoverable. `admin_entitlement_validity_test.go` covers exclusive expiry, future terms, terminal states and unchanged pending settlement/balances in hosted CI.
