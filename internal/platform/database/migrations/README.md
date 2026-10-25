# Database migrations

`096_payg_credit_windows.sql` preserves paid history while adding prepaid
remainder/charge-time state and bounded raw/unbilled report counters for exact
rolling 72-hour thresholds. Existing remaining credit keeps its last charge's
original timestamp; historical unbilled traffic is not charged on upgrade.

`090_telegram_member_identity.sql` introduces only upgrade compatibility aliases.
After the vault is available, `MigrateTelegramIdentities` takes a private SQLite
backup and atomically rekeys member references and encrypted X-Node/Emby desired
state, removes the assigned username column, and advances unfinished username
steps to agreements. Existing financial amounts, credentials, subscription tokens
and period IDs remain unchanged. A failed decryption or FK audit rolls everything
back; startup does not accept traffic until migration succeeds. Late native
samples are translated only against their period and participating node.
The users-table rebuild replaces the legacy username-aware compilation trigger
with onboarding-state invalidation, preserving cache updates after column removal.

`092_identity_rekey_immutable_references.sql` preserves the affiliate-version and
notification-payload immutability rules while allowing their owner columns to be
rekeyed exclusively through a completed `member_identity_aliases` mapping. All
other updates, and every update after the identity migration, still abort.

`087_native_accounting_maintenance.sql` adds native date/user/purchase and expiry
indexes without copying or rewriting accounting and financial records.

`075_xnode_control_plane.sql` adds authoritative X-Node configuration and access
references plus period accounting, immutable report cursors, rate boundaries and
resumable migration metadata. Runtime connection observations remain in memory.

`076_xnode_final_accounting.sql` retains the encrypted final source export,
its digest, operator backup reference and completion timestamp. This evidence
is required before shutting down the migration provisioning bridge.

`073_browser_admin_sessions.sql` preserves all legacy sessions as Mini App scope,
replaces the global per-user unique index with a partial Mini App-only index,
adds renewal timestamps, and creates hashed one-use browser handoff references.
Only hashed browser bearer references are persisted.

`059_member_squad_preferences.sql` stores only disabled squad references per
account; purchase ownership and upstream names are not duplicated.

Migration files are embedded and applied in lexical order by `database.go`.

- `056_restore_custom_combo_reset_cadence.sql` fills missing custom reset-strategy overrides from the latest matching award's original core combo, including live automatic-renewal successors. Explicit cadence overrides and audited entitlement edits remain untouched; repairs are audited and synchronized through the existing outbox.

- `055_restore_ordered_custom_combo_rewards.sql` follows up 054 using settled raffle ticket order to distinguish same-time prizes and reconstruct recorded temporary/recurring traffic. It verifies surviving fields against the old core-reset behavior and preserves later custom awards, administrator changes and financial records.

- `054_restore_custom_combo_after_core_reward.sql` restores erased custom terms on current purchases when immutable reward history proves a custom award followed by a core-change prize. It skips ambiguous or subsequently edited histories, records an audit event and queues provider sync without changing balances or renewal enablement. See [recovery boundaries](../CUSTOM_COMBO_RECOVERY.md).

- `030_affiliates.sql` adds referral eligibility, immutable tier versions, settlements, and awards.
- `031_user_notifications.sql` adds durable semantic notification deduplication
  and provider-success release gates.
- `032_automatic_traffic_reset.sql` adds the account-wide opt-in preference
  used by the five-minute traffic scanner.
- `033_node_compensation.sql` adds nullable revisioned policy, persistent node state, immutable outage snapshots, frozen recipients, and reviewed operation linkage.
Never edit a migration that may already have been deployed; add the next numbered
file instead.

- `051_lucky_draw_redesign.sql` removes legacy configurations while preserving outcomes and ledger evidence; it adds raffle tickets, revisions, delivery IDs, private coupons, and purchase reward overrides.

- `001_initial.sql` — core users, sessions, catalog, purchases, ledger, settings,
  outbox, audit, backup, and onboarding schema.
- `002_activity_coupons_questionnaires.sql` — community activity, coupon, and
  questionnaire aggregates.
- `003_emby.sql` — Emby account and provisioning saga tables.
- `004_platform_payments_rollover_restore.sql` — expanded payment, rollover, and
  restore lifecycle records.
- `005_database_admin_reviews.sql` — database administration review support.
- `006_community_contract_fields.sql` — additional community contract fields.
- `007_purchase_idempotency.sql` — purchase request fingerprinting and replay keys.
- `008_group_message_rewards.sql` — Telegram group-message reward tracking.
- `009_expansion_and_cleanup.sql` — current expansion, normalization, and obsolete
  snapshot cleanup.
- `010_clear_subscription_cache.sql` — clears legacy X-Node subscription bearer
  values while retaining the nullable column for backup/restore schema compatibility.
- `011_minimize_payment_payloads.sql` — clears terminal provider display payloads
  that are not required for settlement.
- `012_minimize_questionnaire_imports.sql` — removes settled CSV payloads and
  aligns exhausted questionnaire jobs with the retry lifecycle.
- `013_coupon_discards_and_courtesy_credits.sql` — adds durable wallet-discard
  evidence and exactly-once terminal-payment courtesy-credit records.
- `014_goal_completion.sql` adds optional squad stock, renewal batches, bounded
  rollover aggregates, and the temporary per-rail payment profile seed.
- `015_payment_provider_profiles.sql` consolidates each provider into one
  profile row with independently enabled channels and removes the temporary
  per-rail table.
- `016_multiple_payment_profiles.sql` allows multiple independent EZPay and
  BEPusdt accounts while preserving the existing provider profile rows.
- `017_squad_profiles.sql` adds nullable normalized local metadata for the
  three customer-facing internal-squad profile types without duplicating the
  generated description.
- `018_automatic_renewal.sql` adds opt-in automatic-renewal state, a durable
  due-cycle failure notice, recurring-discount attachment, and a unique
  source-successor link while leaving legacy renewal records intact.
- `019_rollover_activation_codes.sql` removes the persisted rollover cap and
  adds bcrypt-backed squad activation metadata.
- `020_release1_commerce.sql` converts monthly cadence to `MONTH_ROLLING`, snapshots immutable core price, and adds billing limits, provider receipts, connection scans, upload metadata, and replay facts.
- `021_admin_workflows.sql` adds purchase-level entitlement overrides and the durable backup-upload publication saga.
- `022_release1_rollups.sql` adds cleanup leases, compact activity/payment/purchase rollups, and timestamped statistics partitions.
- `023_payment_callback_tombstones.sql` keeps compact provider-callback replay
  evidence after terminal payment detail is pruned.
- `024_operation_durability.sql` allows review-required connection scans and
  preserves actor-scoped staged-restore replay identity across a database swap.
- `025_mutation_durability.sql` adds review-required Emby state and prevents concurrent open setup commands.
- `026_group_message_facts.sql` stores deduplicated raw configured-group non-command message facts for cumulative statistics.
- `027_host_operation_guard.sql` prevents a new host remark mutation while the same host still has an unresolved operation.
- `028_connection_ip_blocks.sql` stores only active encrypted node-scoped IP
  blocks and their scheduled three-day cleanup references.
- `029_security_and_history_indexes.sql` bounds each user to one current session
  and indexes stable administrative inventories plus questionnaire history.
- `034_abuse_qps_detector.sql` adds encrypted node credentials, privacy-safe QPS samples, detector state, incidents, delivery records, and temporary-ban restoration facts.
- `035_abuse_warning_cooldown.sql` adds the revisioned administrator warning-record cooldown.
- `036_purchase_addon_adjustments.sql` records idempotent, prorated active-term squad additions without duplicating user or provider data.
- `037_abuse_pending_qps_index.sql` indexes detector-state continuation scans so completed sample windows are not repeatedly evaluated.
- `038_durable_abuse_processing.sql` adds configurable streak duration, durable
  normalized event claims, 30-minute QPS rollups, replay-safe incident facts,
  and completion evidence for safely pruning detailed abuse records.
- `039_abuse_record_retention_and_ip_bans.sql` preserves detector records for
  the configured retention period, removes irrelevant punishment durations,
  and persists asynchronous IP-ban scans across retries.
- `040_admin_user_provider_commands.sql` adds durable administrator temporary-ban state and scheduled restoration linkage.
- `041_admin_coupon_discard_commands.sql` records administrator coupon-discard idempotency without deleting historical grants.
- `042_community_membership.sql` maps legacy membership onboarding users to username when unnamed or agreement when named.
- `043_maintenance_run_history.sql` removes the one-row-per-date restriction while retaining active lease serialization and run history.
- `045_abuse_outbound_tag.sql` persists the administrator-selected Xray outbound tag used for abuse-log filtering.
- `046_abuse_outbound_tags.sql` migrates the abuse filter to a validated comma-separated outbound-tag list.
- `047_display_currency.sql` adds the TXB-default member display-currency preference constrained to TXB, CNY, or USD.
- `048_rollover_calculated_settlement.sql` adds the calculated rollover state so traffic can be measured before an atomic automatic-renewal settlement.
- `049_retry_rejected_telegram_markup.sql` requeues only failed Telegram jobs whose
  stored error proves a Markdown entity rejection; ambiguous sends stay failed.
- `050_restore_missing_remna_signup.sql` returns members stranded by the former
  missing-user agreement flow to completed local onboarding; startup queues paid
  unlinked accounts for reconciliation.
- `052_retry_safe_telegram_draw_jobs.sql` requeues transactional raffle settlement, definitively rejected Telegram parse jobs, and safe raffle edit errors after the recovery fixes; ambiguous sends remain failed.
- `053_retry_draw_payload_decode.sql` resets failed or delayed raffle-update and settlement jobs rejected by the former string-only payload decoder. It requires the matching decode error, a string draw ID, and an integer revision, and keeps at most one active job per payload. Duplicate failed rows, other failures, and processing jobs retain their state.

- `058_require_boosted_group_messages.sql` resets unfinished message progress and its counted flags once; paid rewards and retained message identities are preserved. Version 057 is reserved for the concurrent preferences change.

`057_user_preferences.sql` adds per-user choices and clears optional entrances on purchase eligibility transitions and later purchases.

`060_telegram_boost_receipts.sql` keeps only group/boost identity and receipt time to prevent repeat appreciation.

`061_panel_entry_verification.sql` creates the two-column temporary verification table; previously accepted agreements need no migrated row.

`062_telegram_pm.sql` adds two normalized user moderation flags, per-user/per-destination routing references, durable certainty and bounded update tombstones. A partial unique index serializes profile publication and repair without duplicating message bodies.

`063_retention_scan_indexes.sql` indexes event creation and final failed-job timestamps for bounded maintenance scans without additional retained data.

`064_telegram_pm_read_status.sql` adds reference-only explicit and reply-inferred read evidence for successful PM deliveries; deleting a delivery cascades its read state.

`065_host_connectivity_attempts.sql` adds one indexed, credential-free connectivity attempt table. Starts define a rolling 24-hour diagnostic window; latest results are derived without a second table or mirrored host data. A nullable host reference records setup failures, and terminal transitions retain the original start timestamp.
- `066_ip_lookup.sql` adds combo quotas, activation allowances, shared reports and paid check references.
- `067_ip_lookup_compact.sql` indexes parsed cache completion times. Its transactional Go hook removes legacy per-provider snapshots, preserves minimal pending attempt markers, and leaves historical statuses and accounting unchanged.
`068_telegram_pm_attribution.sql` adds bounded encrypted pending envelopes with content-free retry markers and attaches separately tracked footer phases to legacy unsent outbound messages.

- `069_ip_lookup_live_snapshots.sql` adds compact persistent realtime snapshots and private per-report refresh recovery state; existing quality reports default to no realtime phase.

- `070_ip_lookup_background.sql` adds recovery heartbeats/leases, cache identity and exact quota consumption. It makes receipt report references nullable and clears old completed quality/live cache payloads without removing ledger or allowance history. In-flight report checkpoints survive and finish under the new worker.

- `071_ip_lookup_current_config.sql` drops per-report config_json without replacing rows or removing checkpoints, receipt references, quotas or ledger records. Execution/retry settings are read from current settings.

- `072_builtin_usdt.sql` ? Widens payment-provider identity and adds compact quotes, transfers, cursors and legacy RPC admission records.
- `073_remove_usdt_rpc_quotas.sql` removes the obsolete daily/minute usage and cooldown tables after RPC work moves to one in-memory shared concurrency queue.

- `074_vansail_brand_and_link_auth.sql` removes retired browser OAuth credentials and renames untouched seeded welcome copy. Existing device sessions, handoffs, customized documents, and agreement revisions are preserved.

`077_admin_api_tokens.sql` stores hashed scoped administrator credentials and their lifecycle timestamps, independently of browser and Mini App sessions.

- `078_retire_abuse_detector.sql` removes executable detector state and credentials, fails pending detector jobs, and queues only proven historical temporary-ban releases. Unacknowledged mutations receive audit review entries. Incident facts, financial records, manual restrictions and audit history remain intact.

- `079_retire_legacy_controls.sql` disables outstanding relinking and legacy host-remark operations while preserving their history; removes the retired squad display flag.

`044_squad_geocheck.sql` is retained as immutable upgrade history; its display flag is removed by migration 079.

- `080_xnode_compile_revision.sql`: Adds desired-state invalidation metadata; routine traffic and acknowledgment writes do not invalidate compiled state.
- `081_xnode_identity_routing.sql`: Migrates routing ownership to native identity references while preserving existing ciphertext scopes.

`082_xnode_firewall.sql` persists active enforcement generations, compressed participants and expiry acknowledgments, per-namespace report cursors, immutable exemption grants and delivered versions. It does not persist connection telemetry or mirror external proxy inventories.

`083_xnode_zero_multiplier.sql` allows explicit zero node/rate multipliers while preserving all rows and historical rate boundaries. Startup verifies a boundary backup and applies the rebuild with FK checks and schema-object restoration.

`084_xnode_routing_plans.sql` stores encrypted owner route plans, preserved conversion evidence and durable deployment barriers, with compile revision invalidation.

`085_stablecoin_tokens.sql` adds token cursor keys without resetting USDT progress, preserves quote fingerprints and transfer IDs, and records immutable audited dismissals.

`086_admin_firewall_exemptions.sql` distinguishes uncharged administrator grants from immutable member payments and invalidates effective inventory projections when an exemption is issued or released.

`088_xnode_metered_probes.sql` stores minimal immutable attribution references for
isolated route-test identities, access revocation and traffic barrier evidence.
Measurements reuse existing operation items; generated configurations are absent.

`089_xnode_member_diy.sql` stores only member ID, settings revision and encrypted client overrides. Generated subscriptions and credentials are never duplicated; subscription-only writes do not invalidate server compilation.

`091_member_whitelist.sql` adds the per-combo whitelist capacity (default five),
a detecting-IP index for managed-node ban releases and quota cache invalidation.
Whitelist lists remain inside the encrypted DIY document, not a mirrored table.

`093_native_only.sql` requires completed final accounting, translates pending native entitlement work, removes obsolete bridge secrets/checkpoints, renames native group/account fields, and retains historical usage without repricing it. Old migrations stay immutable.

`094_delete_revoked_api_tokens.sql` purges credentials from earlier soft revocations while retaining immutable audit events.

`095_payg.sql` adds sparse billing configuration, base-term squad references, native PAYG attribution, immutable ledger charge metadata and bounded minute aggregates. It preserves prior migrations and existing combo entitlements.

097 adds default-on PAYG access and sparse member/group opt-outs with narrow compile-revision triggers. Existing historical migrations remain unchanged.

`097_payg_preferences.sql` adds the PAYG preference and sparse group opt-outs without changing historical migrations.
