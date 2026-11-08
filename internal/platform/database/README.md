# Application store

Agreement publication updates content only. PostgreSQL migration 00027
restores previously accepted members to complete without rewriting their original
acceptance evidence. First-time acceptance still validates the latest revision.
`agreement_continuity_test.go` covers publication and acceptance continuity.

`pricing_readiness_test.go` verifies that published price validity survives the
calculation cadence and that expired publications become unavailable. Product
readiness exposes the calculation reason without changing settlement admission.
Unlimited provider capacity still requires demand, cost and health evidence.

`pricing_daily_actual_test.go` verifies daily traffic actuals and missing windows for pricing forecasts.
`pricing_personal_freshness_test.go` distinguishes missing and pending history
from an actually stale forecast vintage. Interactive personal forecast refreshes
share the worker's bounded compute admission and reuse completed daily features.
`pricing_summary_payment_test.go` verifies that declared provider payments use
their recording time, independently of the effective date of cost metadata.

`xnode_traffic_discard.go` records user-authorized unmatched traffic exceptions.
Reference conflicts roll back the entire report before the real report cursor
and an audit record commit. Gaps are audited while valid reports still count.
Missing-period exceptions retain the validated period reference in the audit so
an operator can diagnose a recurring agent journal without retaining traffic payloads.
Agent identity, malformed input, storage failures and accounting durability remain
mandatory; no fabricated traffic is charged. Duplicate exceptions are idempotent.
`xnode_traffic_discard_test.go` covers full-batch rollback, audit replay and
the next valid journal report on PostgreSQL.

Firewall sweeps reuse one active-member bitmap and the bounded page's participant
snapshots within the same serializable transaction. `xnode_firewall_tx.go` shares
the expiry calculation with ordinary agent events. `xnode_firewall_sources.go`
serializes sweeps, restores the page cursor on transaction failure/retry, and
prunes seven-day operational history at most once per minute. Active expiries
still run on every sweep; paid exemption evidence is retained.
`xnode_firewall_sweep_test.go` verifies shared-audience isolation, immediate
retirement between cleanup intervals and bounded history pruning on PostgreSQL.
`xnode_firewall_retirement.go` seeks the existing C-collated generation/source
primary keys and limits each bank to 256 candidates before merging one 256-row
page. The strict composite cursor still orders bans before sources; empty keys,
released generations and bank transitions preserve the original projection.
Every cycle wraps to the beginning, including startup/recovery; errors restore
the cursor through `SweepFirewall`'s existing whole-transaction retry boundary.
No admission cache, schema change, expiry shortcut or delivery-lane change is
introduced. `xnode_firewall_seek_test.go` covers projection equivalence, a full
expiry cycle and rollback/recovery after a partially processed page. See the
[production measurements](../../../docs/database-performance-2026-11-08.md).

`xnode_runtime_reads.go` narrows reconciliation to current funding candidates,
connected principals (128 IDs per query), and resettable period owners. It reuses
the authoritative PAYG joins/scanner and existing indexes on PostgreSQL.
No balances, usage, admission decisions or duplicate state are persisted/cached;
exact credit expiry and settlement remain in the X-Node service.
`xnode_runtime_reads_test.go` checks wallet/credit changes, node isolation,
bounded connection batches and retained cadence recovery on PostgreSQL.
`XNodePAYGFunding` projects only authorization fields through `xnode.PAYGFunding`.
It shares the account scanner, current price resolver and funding predicate, but
omits the historical period join. The validated account-to-period foreign key
preserves existence; historical counters never escape this projection. Full
accounting reads keep exact purchased/used bytes. The master prefers this optional
read while retaining older repository fallbacks; every renewal rereads funding,
and exact credit expiry, readiness and lease caps remain in the service.
`xnode_runtime_reads_test.go` verifies current wallet/price/disablement changes,
node isolation, nanosecond expiry and intact accounting counters.
`lockPAYGOwnersTx` selects report or observation principals in bounded batches
before locking owners in stable order. Atomic journal and financial writes keep
their existing isolation and retry rules.

`xnode_payg_node_costs.go` reads exact owner-scoped PAYG charges from immutable
ledger records for the requested history window. Account/node references retain
cost-only and retired-node entries without deriving cost from traffic;
`xnode_payg_node_costs_test.go` covers owner/window isolation, exact minor-unit
aggregation and combo-accounting independence on PostgreSQL.

`catalog_stock.go` owns bounded aggregate stock display;
`purchase_squad_membership.go` shares ownership precedence with checkout.
`catalog_stock_test.go` verifies snapshot/fallback behavior and stockless reads;
`catalog_stock_plan_test.go` emits the hosted 2,001-term PostgreSQL query plan.
`paid_member_access_test.go` checks actual PAYG spending, automatic entrance
visibility, manual overrides and independence from later combo transitions.
`xnode_cursor_test.go` races rate/report writes and checks missing-reference
failures leave the cursor untouched on PostgreSQL.

`ListSquadProductOverrides` is the stockless merchandising interface used by
native compilation, PAYG, inventory and export. `ListSquadProducts` is reserved
for catalog/stock display; it computes all limited combo squads in one aggregate
under its own two-second deadline, after closing the override rows. Both stock
and `purchaseSquadsFrom` share `purchaseSquadMembershipCTE` snapshot precedence.
Checkout still enforces capacity in its existing financial transaction. The
hosted PostgreSQL stock regression emits EXPLAIN (ANALYZE, BUFFERS); indexes are
added only when measured plans justify them, never to mask repeated queries.

`xnode_cursor.go` provides the shared transaction cursor lock for journal commits
and multiplier changes. Both paths retain SERIALIZABLE whole-operation retries
with deterministic cursor locking; rates begin at the next uncommitted sequence.
Missing period/identity references discard the entire report with a fixed audit
reason and sequence, without storing credentials or report payloads.

`xnode_endpoint_epoch_test.go` verifies endpoint insert, semantic edits and deletion
invalidate routing/bypass references while equal values and timestamp-only writes
do not. Forward migration 00014 supply the missing endpoint epoch triggers.

`xnode_ddns_changes.go` stores one desired mutation/replay cursor per node/job,
with exact decimal uint64 sequence values and a durable operation reference.
Node and compile-revision locks fence authentication/settings changes before
cursor, operation and outbox commit together. The table deliberately has no job
FK so active old routes can retain attestations after a job is removed.
`xnode_ddns_apply.go` changes only bound host addresses and advances durable
phases; the transaction advances the catalog/compile revision. No wallet write or
member configuration fee occurs. Maintenance protects referenced receipts and
pending phases. The existing routing recovery path also resumes DDNS receipts.
`xnode_ddns_changes_test.go` covers replay/order, full-width sequences, config
fences, atomic rollback and retention. `xnode_ddns_apply_test.go` covers IPv4-only,
IPv6-only/dual host mutation, epoch invalidation and reconnect recovery on the
isolated PostgreSQL contract factory.

`xnode_ddns_bindings.go` commits node discovery settings and normalized job/host
references in one transaction. Forward migration 00012 enforce same-node
host ownership, distinct family/job bindings and administrator audit references.
Reference changes invalidate the existing compile revision. Maintenance protects
the configuring user; bound hosts must be unbound before deletion or node moves.
The regression fixture checks hosted PostgreSQL atomic rollback,
credential retention and owner retention. No discovery observations are stored
in these binding tables. `xnode_ddns_bindings_test.go` exercises these constraints
through the shared PostgreSQL store factory.

`catalog_topology_epoch_test.go` verifies native metadata changes advance the
shared revision while equal values and live agent bookkeeping do not. It runs
through the hosted PostgreSQL contract factory. Forward
migration 00011 add only invalidation triggers, with no copied datasets.

`payg_squad_state_test.go` checks unavailable admission and compatibility with
older clients omitting `paygState`. Squad merchandising stores only the durable
administrator state; live health and wallet eligibility remain derived.

`purchase_admission.go` shares exact-time automatic-renewal admission between
catalog metadata, quotes and checkout. Manual purchase, renewal and preference
changes serialize on the existing member balance row. A completed manual
successor prevents a delayed automatic renewal from charging again.
`purchase_admission_test.go` checks exact expiry, quote/transaction enforcement,
checkout replay and a delayed renewal after a manual successor.
`automatic_renewal_superseded_test.go` covers stale renewal preferences and
failure notices when a later paid term is already queued.

Maintenance preserves provider receipts referenced by native routing plans and
metered probes, including terminal receipts. These links are required desired
state and immutable billing attribution, not disposable operation history.
`retention_native_operations_test.go` covers repeated cleanup with both kinds of
references, alongside the existing retention and backup-gating regressions.

`retention_purchase_lineage.go` computes the final deletion set to a fixed point:
any retained automatic successor or renewal-batch term protects its source and
all transitive predecessors. Status/age alone cannot establish deletability;
native periods and unfinished settlements also retain terms. Tombstones and
settled financial rollups are written only for the final deletion set. Hosted
regressions cover native-retained expired chains, mixed renewal-batch chains,
repeat compaction and exact tombstone charges. `retention_purchase_lineage_test.go`
covers transitive predecessor retention for automatic-renewal and renewal-batch
chains, plus exactly-once compaction of unreferenced history. Abandoned-user
cleanup excludes every restrictive user reference, including audit actors and
desired DIY state.

`outbox_completion.go` locks the exact claim and uses PostgreSQL READ COMMITTED
for completion/busy deferral, matching queue admission. Heartbeats and unrelated
jobs can progress without serializable dependency cycles. Attempt, status and
generation fencing remain mandatory; business transactions retain SERIALIZABLE
and whole-operation retries. Hosted outbox completion regressions cover racing
heartbeats, duplicate completion and recovery generations.
`outbox_completion_retry_generated.go` retains bounded whole-operation retry;
`outbox_completion_test.go` exercises contention and stale completion in hosted
PostgreSQL without requiring serialization retries for claim completion.

`outbox_retry_policy.go` keeps desired-state routing/DIY retries between five
and thirty seconds while preserving the existing firewall and external-command
policies. `outbox_retry_policy_test.go` verifies a late retry on PostgreSQL
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
contract on PostgreSQL, including interrupted native operation attempts.

`open_configured.go` accepts PostgreSQL only and refuses a missing connection.
`transaction_retry.go` classifies whole-operation serialization/deadlock retries;
generated adapters retain existing public repository interfaces. Hosted isolated
PostgreSQL fixtures exercise business contracts, stale outbox generations and
concurrent commerce. `outbox_fencing.go` owns generation-checked completion and
heartbeats. Native SQL parameters, expressions and row types pass directly to pgx.

Forward migrations and the native schema contract live in `postgres/`. Applied
migrations remain immutable. The restricted application role verifies the schema;
the migration role performs changes. Recovery uses PostgreSQL logical/physical
archives and native read-only fingerprints with the original encryption key.

Traffic ingestion discards whole reports with explicit unmatched-accounting
conflicts, recording an audit event atomically with the actual report cursor.
Gaps are recorded without inventing missing usage; valid subsequent reports are
accounted normally. Agent identity, invalid frames and storage failures remain
rejections. `xnode_traffic_conflicts_test.go` covers these atomicity contracts.

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

Referral stubs use the decimal Telegram ID as their user ID, including before
authentication. Existing noncanonical identities and associated encrypted,
financial and accounting records remain unchanged at startup. No source database
import, identity relinking, private file backup or database swap runs at boot.
`xnode_access_repair.go` coalesces bounded paid provisioning recovery into the
existing outbox without resetting exhausted or settled purchase periods.

Native traffic aggregation streams indexed node/day groups with checked integer
sums. Backup-gated native retention prunes only expired access and superseded
acknowledged rate boundaries; native purchase/refund evidence and unsettled

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

The store uses pgx with PostgreSQL 18. Hosted CI owns all migration, race,
commerce and recovery suites; local automated suites remain prohibited.

Lucky-draw storage keeps configuration, ticket charges, reservations, and immutable outcomes local. Every X-Node effect is enqueued after the result transaction; Telegram delivery uses recorded message IDs.

Private draw coupons (`admin_visible=0`) can only be used through awarded wallet grants. Code lookup excludes them, and the shared grant lookup and wallet listing reject historical `source_type='code'` copies. Checkout and automatic renewal therefore cannot use copied grants; existing financial records remain intact. `activity_draw_coupon_security_test.go` covers these boundaries and normal public-code redemption.

- `activity_draws.go` saves and audits mode-specific configuration; `activity_draws_part2.go` settles instant results atomically, binds replays to the same member and draw, and reserves `raffle:` keys for internal settlements.
- `activity_draw_replay_security_test.go` covers cross-draw conflicts, member isolation, reserved keys, and unchanged balances/randomness on replay.
- `activity_draw_list.go` lists member-safe instant draws and admin definitions; `activity_draw_reservations.go` reconciles active raffle loss holds when reward ranges change.
- `activity_reward_apply.go` applies resolved balance, coupon, purchase, traffic, and squad effects and queues provider synchronization.
- `activity_reward_combo_order_test.go` verifies that recovery leaves later custom awards untouched, including same-time settlements, zero-price awards and multiple custom prizes. An older core change cannot justify recovering a newer custom award whose overrides are missing.
- `activity_reward_traffic.go` tracks current-term and renewal traffic separately so recurring rewards cannot preserve temporary gains or losses. Eligibility and settlement reject deductions that would exhaust either allowance. `activity_reward_traffic_test.go` covers mixed reward ordering and renewal coverage.
- `activity_raffle_entries.go` sells deduplicated seats; `activity_raffle_settle.go` refunds expired eligibility or assigns fixed stock atomically and accepts retries after a refund reopened the raffle.
- `activity_raffle_settle_retry_test.go` covers a recovered settlement job arriving after the raffle has reopened.
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
- `statistics_part2.go` continues the focused implementation from its original package module.
- `store_part2.go` continues the focused implementation from its original package module.

This package owns authoritative PostgreSQL repositories, forward migrations,
durable outbox records and recovery validation. Domain services
depend on the `Store` methods; provider network calls do not belong here.

The persistence implementation is split by domain operation. The `_part2.go` files contain continuation methods for the corresponding bounded repository module.

- `activity_draws_part2.go`, `activity_extensions_part2.go`, and `activity_part2.go` continue activity persistence.
- `billing_payments_part2.go`, `coupons_part2.go`, and `coupons_part3.go` continue billing and coupon persistence.

## Production files

- `billing_purchase_addon_fingerprint.go`, `billing_purchase_addon_quote.go`, and `billing_purchase_addons.go` validate, price, replay-protect, debit, record, and queue active-term squad additions in one transaction.
  repair, and atomic full refunds plus username restart on identity conflict.
  preserved account facts, and the localized conflict receipt.

- `timestamp_cursor.go` signs filter-bound timestamp/ID cursor payloads shared by administrative inventories.
- `admin_users_page.go`, `admin_entitlements_page.go`, `admin_finance_pages.go`, and `admin_jobs_page.go` provide stable filtered pages without fixed-cap truncation.
- `open_configured.go` opens PostgreSQL; `postgres/` owns migrations, native schema
  contracts, ownership fencing and recovery fingerprints.
- `store.go` — core store type, user/session persistence, membership, username,
  and X-Node recovery state.
- `store_access.go` exposes the database handle and optional structured logger.
- `community_membership.go` queries only currently active combo windows for entitlement-gated community access.
- `settings.go` — encrypted-or-plain application setting records.
- `onboarding.go` — versioned onboarding content, agreement contracts, and
  onboarding completion.
- `catalog.go` — combo inputs and transactional combo writes. Combo updates bind
  each field once, including the active flag, on PostgreSQL.
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
- `notification_new_events.go` and `notification_operation_events.go` snapshot scheduled charges, add-on activation, cancellations, renewal failures, and terminal member operation receipts with stable event keys.
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
  and participation history.
- `emby.go` — Emby setup debit/outbox creation and account reads.
- `emby_provisioning.go` — retryable provisioning-saga state transitions, refund,
  preferences, and account touch operations.
- `emby_queries.go` — provisioning scans, preference hydration, folder replacement,
  and provider-error normalization.
- `operations.go` — durable job insertion, entitlement-transition enqueueing, and
  purchase expiry.
- `outbox_jobs.go` — outbox claim, completion, retry, deletion, recovery, listing,
  scanning, and persisted-error sanitization.
- `purchase_sync.go` — entitlement synchronization and traffic-reset phase state.
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
- `payment_profiles.go` — provider-account profile masking, protected deletion, and encrypted credential persistence.
- `retention.go` — bounded cleanup of aged operational records.
- `retention_activity_rollups.go`, `retention_payment_rollups.go`, and
  `retention_purchase_rollups.go` preserve compact activity, payment, purchase,
- `retention_compaction.go` coordinates all backup-gated cleanup writes in one transaction.
- `retention_native.go` prunes aged native accounting maintenance rows without removing purchase-wide financial evidence or rollback dependencies.
- `retention_operations.go` fails stale open provider operations after 24 hours, resolves linked local state, and removes processed receipts plus their queue records.
- `retention_operation_compensation.go` refunds stale traffic-reset and Emby-setup debits before their operations are removed.
- `retention_housekeeping.go` prunes completed notification events, expired or superseded sessions, abandoned pre-username users, and old maintenance runs.
- `continuity.go` retains native paid-successor projections and retires unclaimed legacy preparation jobs without deleting history.
- `statistics.go` — catalog and activity administrator statistics.
- `product_statistics.go` calculates range-free KPIs across live and compacted
  facts, immutable spending flows, and database/WAL size.
- `product_statistics_catalog.go` calculates active combo and squad distributions with user-facing squad names when local merchandising data exists.
- `product_statistics_payments.go` combines live and compacted EZPay, BEPusdt, and Telegram Stars terminal-status facts.
- `product_statistics_usage.go` selects one current non-admin member combo for live weighted usage projection.
- `statistics_snapshots.go` persists independent last-good statistics partitions.
- `destructive.go` — audited feature deletion transactions.

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
- `emby_test.go` — Emby setup, provisioning, retry, and refund atomicity.
- `retention_test.go` — retained operational-record cleanup behavior. Audit
  retention counts include migration-created history and inspect fixture events
  separately without deleting historical evidence.
- `retention_native_test.go` covers bounded native accounting maintenance retention and preservation of settlement evidence.
- `outbox_priority_dispatch_test.go` covers reserved priority dispatch, one-second coalescing and ordinary-job ordering.
- `maintenance_runs_test.go` covers local-day locking, forced same-day history, and backup-gated maintenance state.
- `retention_provider_operations_test.go` covers retaining provider operations referenced by durable administrative and compensation records.
- `continuity_test.go` covers native successor projections without preparation jobs and idempotent legacy-job retirement.
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
- `ledger_page_test.go` — cursor pagination order and validation.
- `squad_profiles_test.go` — typed profile round trips and legacy Markdown preservation.

`payment_profiles_test.go` covers independent provider-account persistence and stable-ID lookup.

The `postgres/migrations/` directory contains the ordered native PostgreSQL history. New
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

`user_preferences.go` merges nine persisted preference flags atomically, projects active-combo eligibility and resolves legacy referral delivery recipients.
`user_preferences_test.go` covers defaults, partial merges, account isolation, eligibility loss and referral attribution.

`squad_features_test.go` covers ordered feature persistence, old-client omitted writes, explicit clearing, and bounded validated input.

`telegram_boost_receipts.go` and `telegram_boost_receipts_test.go` atomically deduplicate group/boost IDs with their outbox work, including concurrent and changed updates. No boost count is mirrored.

`panel_entry_verification.go` and `panel_entry_verification_test.go` persist only Telegram ID and a boolean. Panel visits initialize the row, verification or a disabled challenge qualifies it, and onboarding removes it in the same transaction as agreement acceptance.

`telegram_pm_commands.go` deduplicates webhook IDs and atomically queues notices or reference-only relay operations. `telegram_pm_reads.go` infers read timestamps from member replies and stores references against delivery IDs only. `telegram_pm_queries.go` joins routing and normalized user flags; `telegram_pm_pages.go` reuses stable cursors and exposes bounded delivery references with read evidence. `telegram_pm_moderation.go` audits idempotent flags/repair commands and queues profile refreshes; `telegram_pm_topics.go` claims creation and records durable topic/profile certainty. `telegram_pm_recovery.go` resumes only definitely unsent waiters after a proven repair. `telegram_pm_retention.go` bounds receipts at 30 days while preserving routing uncertainty; generic maintenance protects recent PM operations and conversation owners.
`telegram_pm_commands_test.go`, `telegram_pm_permissions_test.go`, `telegram_pm_topics_test.go` and `telegram_pm_retention_test.go` cover concurrent replay, normalized flags across destinations, unauthorized mutation, serialized topic/card phases, safe recovery, retention boundaries and foreign keys in hosted CI.
`telegram_pm_reads_test.go` covers reply-inferred read state. `admin_balance_reference_test.go` covers stable-reference replay without duplicate ledger or audit effects.



- `ip_lookup_refunds.go` restores never-attempted costs and closes interrupted shared runs during existing operation maintenance. Attempted calls remain charged and cacheable; IP receipts are excluded from generic compensation after domain settlement.
- `ip_lookup_cache.go` serves free compact quote previews, preferring exact-IP evidence before the newest completed IPv4 /24 report. IPv6 stays exact-only; parsed completion timestamps avoid RFC3339 text ordering errors. Quote previews create no receipt, debit, or upstream request.
- Check metadata retains the requested address and cache match independently of the evaluated report address, including after reload. Signed submission fields exclude the display-only cached report.
- `ip_lookup_compact.go` converts legacy report payloads in bounded migration batches through the core versioned codec. Consolidated facts and minimal pending-stage markers replace per-provider snapshots; historical statuses, receipts, ledger entries, and refunds remain intact.
- `ip_lookup_cache_test.go` covers free previews, exact-IP precedence, IPv4 /24 boundaries, IPv6 isolation, nanosecond completion ordering, cache arrival during submission, and display fields outside quote signatures in hosted CI.
- `ip_lookup_fixture_test.go` supplies isolated lookup configuration, balances, purchase allowances, signed submissions, and terminal report fixtures for the database regression files.
- `ip_lookup_maintenance_test.go` covers durable receipts and the distinct settlement of never-attempted jobs versus interrupted, chargeable provider calls in hosted CI.
- `ip_lookup_policy_test.go` covers free cache reads with available quota, charged partial/refused/all-error reports, repeat settlement, and fractional completion timestamps in hosted CI.
`telegram_pm_content.go` encrypts operation-bound pending outbound text using the existing application vault, reads live envelopes, and distinguishes content-free expiry/settlement markers from legacy reference-only jobs. `telegram_pm_content_test.go` covers duplicate updates, encryption/AAD binding, outbox privacy, transactional settlement erasure and 24-hour repair expiry. Generic operation completion clears settled ciphertext; PM retention clears expired content while operation pruning cascades the marker.

## Persistent realtime IP evidence

- `ip_lookup_live_cache.go` stores compact shared snapshots and selects exact or IPv4 /24 evidence using the same timestamp/record-order ordering as quality reports. Snapshots do not expire or depend on current credentials. Storing a partial snapshot transactionally schedules its first background section retry through the shared outbox using the current settings policy.
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

IP lookup resumes and report repair read current saved settings. `ip_lookup_current_config_test.go` verifies column removal, retained checkpoints/references/record order, and current settings reads. Signed quotes and settlement amounts remain transactional.

- `billing_settlement_tx.go` ? Shared transactional payment credit, ledger and outbox path.
- `usdt_quotes.go` ? Permanent amount reservations and atomic checkout snapshots.
- `usdt_transfers.go` ? Immutable verified receiving events committed before scan progress.
- `usdt_transfer_queries.go` ? Local cursor, recovery and keyset review queries.
- `usdt_settlement.go` ? Atomic receipt claim, exact identity validation, bounded shortfall matching, audited review and ledger credit.
- `usdt_test_helpers_test.go` ? Shared hosted quote and transfer fixtures.
- `usdt_quotes_test.go` ? Concurrent allocation, permanence, exhaustion and BSC atomic-unit checks.
- `usdt_settlement_test.go` ? Delayed observation, replay, role, amount, audit and failed-batch checks.

`xnode_nodes.go` Stores node metadata, credential digests and monotonic revisions.
`xnode_configurations.go` Commits versioned base configs and ordered snippet references.
`xnode_snippets.go` Stores versioned encrypted reusable snippets.
`xnode_groups.go` Stores authoritative groups and node/inbound bindings.
`xnode_endpoints.go` Stores subscription endpoints and encrypted client overrides.
`xnode_identities.go` Stores stable proxy IDs and encrypted credential/token references.
`xnode_periods.go` Stores accounting periods and reads exact byte aggregates.
`xnode_traffic.go` commits actual report cursors and valid weighted usage, or an audited discard.
`xnode_customizations.go` Stores owner/node customization revisions and encrypted inputs.
`xnode_rates.go` Commits multiplier boundaries after accounting barriers.
`xnode_restrictions.go` Projects existing manual access restrictions.

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

`xnode_commerce_test.go` covers in-place cadence persistence without replacing the accounting period.

`admin_user_provider_commands.go`, `admin_user_ban_state.go` and `admin_user_audit.go` own manual restrictions, expiry restoration and audit records.

- `xnode_compile_version.go`: Reads a small transactional compile revision and the next purchase access boundary.

- `xnode_named_services.go`: Atomically saves standalone encrypted terms, authorization and quota without rewriting usage or settled periods.

Firewall operational storage: `xnode_firewall_tx.go` owns atomic writes, bounded compressed participant bitmaps and the existing coalesced outbox. `xnode_firewall_members.go` tracks fleet versions; `xnode_firewall_participants.go` joins delivered pages; `xnode_firewall_events.go` commits monotonic replay cursors and master authorization. `xnode_firewall_page.go` reads version-bound snapshots without holding transactions over RPC. `xnode_firewall_sources.go` stores generation metadata only and prunes operational history. `xnode_firewall_retirement.go` retires disabled/deleted participants in bounded pages while offline active members remain pending. `xnode_firewall_purchase.go` atomically debits, appends the ledger, grants an exact-IP exemption and queues delivery. `xnode_firewall_test.go` covers replay, all-node expiry, concurrent purchase deduplication and retained proxy evidence.

`xnode_firewall_page.go` projects effective compact restriction prefixes after subtracting active exact-IP exemptions. It retains the underlying immutable detection/audit evidence and restores classification eligibility when the exemption ends; IP ranges are never enumerated.

`xnode_zero_rate_test.go` covers raw usage on free nodes, immutable priced ranges and rejection of negative rates. Rate changes start after the latest committed cursor so late flush results cannot rewrite settled pricing.

`xnode_routing_plans.go` commits encrypted managed graphs and provider operations
atomically, preserves prior generations through deployment, and audits phase
changes. It keeps optimistic revisions and operation replay fingerprints distinct.
`xnode_routing_plans_test.go` covers owner receipt isolation, concurrent graph
rejection, replay, generation barriers, retained evidence and recovered attempts.

`usdt_review.go` provides payment-scoped matching evidence and transactional permanent
review dismissal. `stablecoin_test.go` covers token isolation and concurrent courtesy,
settlement, dismissal and scanner replay. Migration 085 keeps old USDT cursors and
evidence IDs, and makes credited/dismissed transfers immutable in PostgreSQL.

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
`xnode_diy_reconcile.go` batches free reference cleanup through the existing outbox
and uses revision fences; it never debits a wallet or allowance.

`xnode_diy.go` commits encrypted settings, routing revisions and durable operation
receipts atomically without changing a balance or IP Lookup allowance. Deployment
completion merges its result so retries retain the original committed outcome.
`xnode_diy_free_test.go` covers unchanged allowance, balance and ledger evidence,
as well as lost-response replay, in hosted CI.

`outbox_admission.go` and `outbox_resources.go` derive account, node, purchase, operation and chat exclusion from existing records inside transactional claims. Independent due jobs skip occupied resources. IP Lookup repair admission stays serial.

Ordinary admission prefers non-dynamic work; dynamic firewall jobs retain their
dedicated reserved claim path and can use idle ordinary capacity. A continuously
replenished firewall backlog therefore cannot starve member/provider jobs.
`outbox_admission_test.go` checks both paths progress across repeated busy retries
without exhausting attempts. Per-resource exclusion applies to both paths.

`outbox_resources_test.go` checks independent account admission, cross-kind account ordering and ownerless standalone targets.

API-token revocation deletes the complete credential row atomically with a secret-free audit event. The owner-scoped audit makes retries idempotent without a credential tombstone.

`xnode_diy_retirement.go` pages saved member references and atomically replaces old ciphertext using user, revision and cipher fences. It does not change financial records, routing or native compilation.

Due outbox jobs with identical timestamps retain insertion order through the monotonic `record_order` identity; opaque random IDs never decide account execution order.

`xnode_routing_recovery.go` wakes routing/DIY retries on authenticated node
reconnection, using the existing outbox and original receipt. It transactionally
recreates missing jobs for unfinished plans, leaves live leases and terminal
receipts untouched, and never charges or rewrites desired routing/accounting.
`xnode_routing_recovery_test.go` checks missing jobs, delayed/failed retries,
duplicate wakes, live leases, terminal receipts and retained DIY result evidence.

## Automatic PAYG

`xnode_payg_accounts.go` stores immutable member/squad/node attribution, its native period and atomic accounting participant reference. `xnode_payg_billing.go` appends exact GB block charges to the existing wallet ledger with immutable rate evidence. `xnode_payg_traffic.go` integrates raw usage and debits with contiguous traffic-journal commits; balances precede periods in its lock order. Connection purchases require funds, while delayed traffic may create wallet debt. PAYG never consumes combo quota or node multipliers. A GB is purchased for pending usage only when the rolling 72-hour raw window reaches 104,857,600 bytes (100 MiB) on that member/squad/node; idle connections and wallet funding cannot charge a first block. Unbilled bytes in that window count toward the purchased GB. Remaining credit expires 72 hours after its charge.

`xnode_payg_jobs.go` coalesces balance-change provisioning through the existing outbox, scans missing tuples in bounded batches and prunes disposable 72-hour minute aggregates. Credentials, cumulative counters and financial evidence remain durable. Source IPs, connection observations, generated subscriptions and lease timestamps are never stored. `xnode_payg_owners.go` provides indexed service lookups, node-scoped compiler identities and credential-free activity ownership reads.

`onboarding.go` queues `xnode_access_repair` for every agreement completion and
`xnode_payg_reconcile` when PAYG is configured, in the agreement transaction.
`xnode_access_repair.go` also backfills completed members without credentials,
independently of purchases or funds. Existing paid-period repair retains its
restriction and accounting fences. `onboarding_provisioning_test.go` covers
purchase-free signup, PAYG queueing, incomplete-member exclusion and coalesced
backfill in hosted PostgreSQL CI.

`xnode_payg_billing_test.go` and `xnode_payg_concurrency_test.go` cover automatic GB boundaries, replay, price changes, debt, failed batches, simultaneous nodes and bounded statistics in hosted PostgreSQL CI. Local automated suites remain prohibited.

`user_payg_preferences.go` reads sparse squad opt-outs and merges validated group choices. Explicit activation checks for a positive wallet balance under the wallet lock; other preference changes do not lock it. Preference changes and PAYG reconciliation enqueue atomically; compile triggers and account projections prevent stale leases and connection purchases. Delayed journal traffic still settles independently of access.

Terminal failed PAYG provisioning/recovery jobs share the three-day failed synchronization window and explicit-retry protection. Financial journal state remains independent.

`xnode_payg_credit_window.go` and `xnode_payg_credit_window_test.go` use platform-neutral names so Go includes the rolling billing helpers and regressions on Linux as well as Windows.

## Compute and personal pricing entry points

Numerical calculations use immutable wallet-independent snapshots and the durable compute lane. Master transactions alone publish prices and settle money. Coupon cohorts and decision versions fence future checkout; live add-ons remain available. Native prepaid credit bases and historical refund/ledger facts are immutable. Owner names persist only authored display overrides.

Purchase, renewal, reward and native accounting lifecycle: `activity_reward_combo.go`, `automatic_renewal_failure.go`, `automatic_renewal_settlement.go`, `automatic_renewal_types.go`, `billing_renewal_quote.go`, `notification_purchases.go`, `payg_catalog.go`, `payg_empty_renewal.go`.

Owner display preferences and prospective PAYG labels: `host_names.go`.

Immutable analytical inputs, forecast vintages and measured calibration: `pricing_actuals.go`, `pricing_calibration.go`, `pricing_forecast_id.go`, `pricing_forecasts.go`, `pricing_history.go`, `pricing_models.go`, `pricing_personal_forecast.go`, `pricing_product_forecasts.go`, `pricing_user_actual.go`, `pricing_user_features.go`, `pricing_user_forecasts.go`.

Versioned pricing preparation, queued kernels, applicability and publication: `pricing_addon_basis.go`, `pricing_addon_prepare.go`, `pricing_cache.go`, `pricing_catalog.go`, `pricing_change_notice.go`, `pricing_commands.go`, `pricing_compute_kernels.go`, `pricing_contract_snapshot.go`, `pricing_contracts.go`, `pricing_coupon_inputs.go`, `pricing_discounts.go`, `pricing_evaluation.go`, `pricing_fixed_renewal.go`, `pricing_funding_outcome.go`, `pricing_grant_scopes.go`, `pricing_incident_notice.go`, `pricing_incidents.go`, `pricing_offer_inputs.go`, `pricing_offers.go`, `pricing_optimization_inputs.go`, `pricing_override_activate.go`, `pricing_override_validation.go`, `pricing_payg_schedule.go`, `pricing_personal.go`, `pricing_personal_reads.go`, `pricing_personal_snapshot.go`, `pricing_publication_freeze.go`, `pricing_receipts.go`, `pricing_resolve.go`, `pricing_revenue_batch.go`, `pricing_scope_version.go`, `pricing_summary.go`, `pricing_time.go`, `pricing_user_offers.go`, `pricing_validation_gate.go`, `pricing_views.go`.

Bounded, scoped analytical exports and redaction: `pricing_export_redaction.go`, `pricing_export_relations.go`, `pricing_export_rows.go`, `pricing_export_scope.go`.

Renewal contracts, numerical snapshots, coupon economics and settlement: `pricing_renewal_budget.go`, `pricing_renewal_candidates.go`, `pricing_renewal_economics.go`, `pricing_renewal_model.go`, `pricing_renewal_reads.go`, `pricing_renewal_schedule.go`, `pricing_renewal_settlement.go`, `pricing_renewal_snapshot.go`.

Bounded retention of settled or summarized operational records: `retention_node_events.go`, `retention_sync_jobs.go`, `retention_timestamps.go`.

- `pricing_attribution.go`: Normalizes machine native usage between fulfilling products; preserves unallocated overhead.
- `pricing_basket.go`: Complete prospective basket and best eligible owned coupon, applied once for estimates.
- `pricing_dataset_page.go`: Opaque, scope-bound keyset pagination with visible truncation.
- `pricing_dataset_page_test.go`: Hosted cursor ties, scope binding and typed product isolation.
- `pricing_fixed_personal.go`: Fixed decisions without numerical work or redundant persisted snapshots.
- `pricing_model_support.go`: Immutable strategy-specific observed price and coupon-cohort ranges.
- `pricing_product_readiness.go`: Batched publication readiness for administration; no settlement authorization.
- `pricing_purchase_replay.go`: Successful purchase receipt before mutable catalog and quote validation.
- `pricing_quote_version.go`: Authoritative basket, evidence, coupon/campaign, exact totals, validity and renewal digest.
- `pricing_quote_version_test.go`: Hosted stale review, expiry and replay regression.
- `pricing_scope_products.go`: Typed combo/addon/PAYG scope in analytical reads and exports.
- `pricing_strategy_test.go`: Hosted shared-cost conservation and basket coupon-cap regression.

- `pricing_capacity_test.go`: Hosted authoritative GiB, model/lead/freshness isolation, overlapping commitments and per-node capacity regressions.
- `pricing_lead_calibration_test.go`: Prevents one-day validation from validating all five forecast leads.
- `pricing_renewal_strategy_test.go`: Exercises the pinned numerical receipt path and verifies single probability weighting of renewal and relationship value.
- `pricing_checkout_concurrency_test.go`: Revalidates coupon and campaign mutations before debit, consumption or paid-predecessor changes.
- `pricing_performance_test.go`: Hosted fixed-catalog benchmarks at one and 1000 products and regression against redundant compute/persisted decisions.

Provider cost metadata uses PostgreSQL forward migration 25. Cycle-based records retain their original meaning; hourly and per-GiB rates use integer millionths of TXB. Capacity-unlimited nodes retain an internal placeholder solely for compatibility with the original positive-capacity constraint. Pricing and renewal snapshots carry cost versions, while immutable cost versions retain actual payments.

- `pricing_provider_models_test.go`: Hosted coverage of legacy and hourly provider costs, unlimited capacity and immutable payment history.

Blocked-account lifecycle: `account_block.go` consumes ordered private bot events and revokes admission; `account_purge_queries.go` resumes durable generations and permits fresh authentication only after an unblock. `account_purge.go` closes balances with compensating ledger entries after all native periods close. `account_purge_credit.go` archives forfeited prepaid quantities, and `account_purge_material.go` removes personal material while preserving financial facts. `account_fences.go` rejects stale receipts; `account_payment_review.go` archives late payments for review. `account_rejoin_payg.go` atomically rotates PAYG credentials without changing historical charges. `account_lifecycle_test.go` and `account_native_lifecycle_test.go` cover ordering, administrators, closure, late payments, final traffic, and fresh rejoining.

Access-group deletion: `xnode_group_usage.go` derives distinct current/queued paid and funded PAYG members from the shared entitlement snapshots. `xnode_group_delete.go` serializes with pricing publication and checks the compile revision plus membership inside one transaction; `xnode_group_delete_retry_generated.go` retries serialization failures. `xnode_group_catalog.go` freezes legacy purchase fallbacks before removing live combo references. Historical prices, offers, contracts, charges, identities, and usage remain evidence; live policies, personal quote caches, preferences, and product interventions are removed or retired. Dormant unfunded PAYG tuples remain for late accounting and cannot recreate catalog products. `xnode_group_delete_test.go` covers occupied and dormant groups, exact credit expiry, stale revisions, history and pricing isolation.

<!-- generated transaction retry map -->

Generated adapters repeat complete rolled-back PostgreSQL operations for SQLSTATE
40001/40P01, with bounded context-aware backoff. Serializable retries retain the complete operation boundary.
The generator excludes methods that access SQL outside their transaction or own
mutable in-memory batches. Provider requests remain outside every transaction.

- `account_block_retry_generated.go` wraps the transactional methods in `account_block.go`.
- `account_purge_retry_generated.go` wraps the transactional methods in `account_purge.go`.
- `account_rejoin_payg_retry_generated.go` wraps the transactional methods in `account_rejoin_payg.go`.
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
- `coupon_purchase_retry_generated.go` wraps the transactional methods in `coupon_purchase.go`.
- `coupons_part2_retry_generated.go` wraps the transactional methods in `coupons_part2.go`.
- `coupons_part3_retry_generated.go` wraps the transactional methods in `coupons_part3.go`.
- `destructive_retry_generated.go` wraps the transactional methods in `destructive.go`.
- `emby_operations_retry_generated.go` wraps the transactional methods in `emby_operations.go`.
- `emby_provisioning_part2_retry_generated.go` wraps the transactional methods in `emby_provisioning_part2.go`.
- `emby_provisioning_retry_generated.go` wraps the transactional methods in `emby_provisioning.go`.
- `emby_setup_retry_generated.go` wraps the transactional methods in `emby_setup.go`.
- `host_pricing_retry_generated.go` wraps the transactional methods in `host_pricing.go`.
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
- `outbox_completion_retry_generated.go` wraps the transactional methods in `outbox_completion.go`.
- `outbox_jobs_retry_generated.go` wraps the transactional methods in `outbox_jobs.go`.
- `outbox_retry_operations_retry_generated.go` wraps the transactional methods in `outbox_retry_operations.go`.
- `panel_entry_verification_retry_generated.go` wraps the transactional methods in `panel_entry_verification.go`.
- `payment_operation_resolution_retry_generated.go` wraps the transactional methods in `payment_operation_resolution.go`.
- `payment_operations_retry_generated.go` wraps the transactional methods in `payment_operations.go`.
- `payment_profiles_retry_generated.go` wraps the transactional methods in `payment_profiles.go`.
- `payment_refund_resolution_retry_generated.go` wraps the transactional methods in `payment_refund_resolution.go`.
- `pricing_compute_retry_generated.go` wraps the transactional methods in `pricing_compute.go`.
- `pricing_costs_retry_generated.go` wraps the transactional methods in `pricing_costs.go`.
- `pricing_export_retry_generated.go` wraps the transactional methods in `pricing_export.go`.
- `pricing_feature_snapshot_retry_generated.go` wraps the transactional methods in `pricing_feature_snapshot.go`.
- `pricing_forecast_storage_retry_generated.go` wraps the transactional methods in `pricing_forecast_storage.go`.
- `pricing_health_retry_generated.go` wraps the transactional methods in `pricing_health.go`.
- `pricing_member_catalog_retry_generated.go` wraps the transactional methods in `pricing_member_catalog.go`.
- `pricing_offer_coupons_retry_generated.go` wraps the transactional methods in `pricing_offer_coupons.go`.
- `pricing_optimization_retry_generated.go` wraps the transactional methods in `pricing_optimization.go`.
- `pricing_optimization_snapshot_retry_generated.go` wraps the transactional methods in `pricing_optimization_snapshot.go`.
- `pricing_overhead_retry_generated.go` wraps the transactional methods in `pricing_overhead.go`.
- `pricing_override_cancel_retry_generated.go` wraps the transactional methods in `pricing_override_cancel.go`.
- `pricing_overrides_retry_generated.go` wraps the transactional methods in `pricing_overrides.go`.
- `pricing_payg_offers_retry_generated.go` wraps the transactional methods in `pricing_payg_offers.go`.
- `pricing_personal_calibration_retry_generated.go` wraps the transactional methods in `pricing_personal_calibration.go`.
- `pricing_policy_retry_generated.go` wraps the transactional methods in `pricing_policy.go`.
- `pricing_purchase_prepare_retry_generated.go` wraps the transactional methods in `pricing_purchase_prepare.go`.
- `pricing_read_batch_retry_generated.go` wraps the transactional methods in `pricing_read_batch.go`.
- `pricing_renewal_quotes_retry_generated.go` wraps the transactional methods in `pricing_renewal_quotes.go`.
- `pricing_retention_retry_generated.go` wraps the transactional methods in `pricing_retention.go`.
- `pricing_unchanged_product_retry_generated.go` wraps the transactional methods in `pricing_unchanged_product.go`.
- `pricing_worker_state_retry_generated.go` wraps the transactional methods in `pricing_worker_state.go`.
- `provider_operation_lifecycle_retry_generated.go` wraps the transactional methods in `provider_operation_lifecycle.go`.
- `provider_operation_notification_completion_retry_generated.go` wraps the transactional methods in `provider_operation_notification_completion.go`.
- `provider_operations_retry_generated.go` wraps the transactional methods in `provider_operations.go`.
- `proxy_provisioning_backlog_retry_generated.go` wraps the transactional methods in `proxy_provisioning_backlog.go`.
- `purchase_expiry_retry_generated.go` wraps the transactional methods in `purchase_expiry.go`.
- `purchase_sync_retry_generated.go` wraps the transactional methods in `purchase_sync.go`.
- `retention_compaction_retry_generated.go` wraps the transactional methods in `retention_compaction.go`.
- `retention_payg_retry_generated.go` wraps the transactional methods in `retention_payg.go`.
- `retention_retry_generated.go` wraps the transactional methods in `retention.go`.
- `retired_detector_retry_generated.go` wraps the transactional methods in `retired_detector.go`.
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
- `xnode_ddns_apply_retry_generated.go` wraps the transactional methods in `xnode_ddns_apply.go`.
- `xnode_ddns_bindings_retry_generated.go` wraps the transactional methods in `xnode_ddns_bindings.go`.
- `xnode_ddns_changes_retry_generated.go` wraps the transactional methods in `xnode_ddns_changes.go`.
- `xnode_diy_reconcile_retry_generated.go` wraps the transactional methods in `xnode_diy_reconcile.go`.
- `xnode_diy_retry_generated.go` wraps the transactional methods in `xnode_diy.go`.
- `xnode_firewall_policy_retry_generated.go` wraps the transactional methods in `xnode_firewall_policy.go`.
- `xnode_firewall_tx_retry_generated.go` wraps the transactional methods in `xnode_firewall_tx.go`.
- `xnode_group_delete_retry_generated.go` wraps the transactional methods in `xnode_group_delete.go`.
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

<!-- end generated transaction retry map -->

- `pricing_capacity_test.go`: Hosted authoritative GiB, model/lead/freshness isolation, overlapping commitments and per-node capacity regressions.
- `pricing_lead_calibration_test.go`: Prevents one-day validation from validating all five forecast leads.
- `pricing_renewal_strategy_test.go`: Exercises the pinned numerical receipt path and verifies single probability weighting of renewal and relationship value.
- `pricing_checkout_concurrency_test.go`: Revalidates coupon and campaign mutations before debit, consumption or paid-predecessor changes.
- `pricing_performance_test.go`: Hosted fixed-catalog benchmarks at one and 1000 products and regression against redundant compute/persisted decisions.

Provider cost metadata uses PostgreSQL forward migration 25. Cycle-based records retain their original meaning; hourly and per-GiB rates use integer millionths of TXB. Capacity-unlimited nodes retain an internal placeholder solely for compatibility with the original positive-capacity constraint. Pricing and renewal snapshots carry cost versions, while immutable cost versions retain actual payments.

- `pricing_provider_models_test.go`: Hosted coverage of legacy and hourly provider costs, unlimited capacity and immutable payment history.

`xnode_access_restore.go` implements serializable, conditional access-pointer restoration with entitlement, operation, restriction and allowance fences. `xnode_access_repair.go` excludes settled/ambiguous/operation-owned recovery candidates. Outbox busy deferrals retain a sanitized reason and accept a bounded `RetryDelay` without consuming delivery attempts.

`xnode_compile_members.go` batches users, purchase ownership, periods and access preferences into one disposable compilation scope. Existing scan functions and purchase ownership CTEs define its contract. `xnode_observed_owners.go` resolves observed owners plus idle sibling principals using existing indexes. `xnode_command_defer.go` conditionally releases a definitely rejected command attempt; `xnode_command_defer_test.go` protects replay and attempt fencing.

`xnode_compile_benchmark_test.go` compares individual and batched reads over the same synthetic member set in hosted CI, including query count and allocations.

Payment settlement checks compact callback tombstones inside the settlement
transaction before claiming a webhook. Replays retain order/trade/charge binding;
new callback keys for terminal payments are compacted without another credit.
`billing_payment_test.go` covers both paths and rejects cross-order/charge reuse.
Group-deletion regression checks compare historical squad membership as a set,
preserving paid evidence independently of catalog serialization order.

`xnode_firewall_records.go` merges bounded volatile detection windows with active
master bans; matching bans suppress overlapping reports. `xnode_firewall_record_heap.go`
keeps pagination memory bounded and binds cursors to the filter. The hosted
`xnode_firewall_records_test.go` covers pagination and report/ban transitions.

`provider_operation_results.go` applies RFC 7396 to locked DIY operation results in Go, preserving exact numeric payloads and completion fences. All authored SQL uses PostgreSQL parameters; historical engine fixtures/importers are retired in favor of native populated forward-upgrade and recovery gates.

`provider_operation_results_test.go` protects exact large integers, nested receipt preservation, null deletion and array replacement during completion.

Native report state acknowledgments use GREATEST on accepted/applied revisions. Compensation history numbers dynamic filters, cursor predicates and limits as one query; interval durations use epoch seconds. Inventory temporary-table batches use bounded scalar index probes rather than repeatedly scanning a growing anti-join input. Hosted compensation pagination and routing barriers cover these contracts.

`compensation_pagination_test.go` covers combined status/cursor/limit pages.
`database_lock_test.go` checks concurrent native snapshot/write behavior.
`postgres_concurrency_test.go` covers debit contention and stale outbox generations.
`xnode_traffic_sample.go` validates exact fixed-point usage after sorted period locks.

## Native storage regression coverage

- `activity_reward_cadence_test.go`, `activity_reward_combo_test.go`,
  `activity_reward_recovery_fixture_test.go` and `historical_reward_fixture_test.go`
  protect reward snapshots, cadence inheritance and retained historical facts.
  `CUSTOM_COMBO_RECOVERY.md` documents the old repair boundaries.
- `admin_entitlement_validity_test.go` and `billing_purchase_addons_test.go`
  cover exact validity boundaries, pending settlement and atomic squad additions.
- `ip_lookup_test.go` checks durable lookup settlement and report lifecycles.
- `payg_commerce_test.go` covers metered purchases and wallet/accounting evidence.
- `pricing_admission_test.go`, `pricing_compute_test.go`, `pricing_export_test.go`,
  `pricing_financial_test.go`, `pricing_forecast_test.go`, `pricing_live_coupon_test.go`,
  `pricing_policy_test.go` and `pricing_storage_test.go` exercise bounded compute,
  exact financial inputs, exports, forecasting and published pricing policy.
- `proxy_provisioning.go` and `proxy_provisioning_test.go` preserve queued native
  provisioning identity and retry contracts.
- `retention_node_events_test.go`, `retention_payg_test.go`,
  `retention_sync_jobs_test.go` cover bounded pruning
  while preserving unfinished work and financial evidence.
- `rollover_retirement_test.go` verifies retained historical rollover facts.
- `user_payg_preferences_test.go` checks opt-outs, activation funds and traffic
  settlement independent of access preferences.
- `xnode_traffic_test.go` protects atomic report replay, exact multiplier carry,
  ownership and known-usage gaps.

## Volatile agent traffic streams

`xnode_traffic_streams.go` serializes authenticated boot registration with node
ownership, permanently closes replaced streams and preserves the last actual
committed cursor. `xnode_stream_rates.go` owns independent sequence-based rate
boundaries and known-usage settlement seals. `xnode_traffic.go` atomically commits
usage, digest/cursor progress, gap evidence and financial effects before ACK.
Legacy cursors/rates retain their original identities. Closed or reordered stream
reports cannot charge again. No live IPs or pending report payloads are stored.
`xnode_traffic_apply.go` resolves rate/accounting owners and deterministic locks
through the existing PAYG/native accounting paths.

`xnode_traffic_streams_test.go` covers duplicate/conflicting/reordered reports,
restart closure, explicit buffer loss, legacy coexistence, unchanged committed
prices, known-usage seals and rollback/retry without duplicate usage.

## Combined traffic transactions

`xnode_traffic_batch.go` accepts at most 256 decoded reports, preserves stable
per-stream sequence order and retries complete serializable transactions at most
three times. Nodes, journal cursors, wallet owners and accounting periods lock
in that order, sorted within each bank. Individual malformed reports roll back
to their own savepoints; recognized accounting-reference conflicts retain the
existing explicit discard policy. A failed shared write rolls back every usage,
financial effect and cursor before any report can be acknowledged.

- `xnode_traffic_batch_read.go` bulk-loads node bindings, locks distinct wallet
  owners and skips financial work for already committed or closed streams.
- `xnode_traffic_batch_cursors.go` prepares ordered legacy/boot cursors and bounded
  indexed rate lookups. `xnode_traffic_batch_write.go` flushes dirty cursors once.
- `xnode_traffic_batch_state.go` owns transaction-private immutable read snapshots
  and staged cursor changes. No wallet/admission state survives a transaction.
- `xnode_traffic_buffer.go` stages native usage independently per report, so a SQL
  savepoint rollback also discards that report's Go counters.
- `xnode_traffic_bulk.go` loads locked periods and daily counters in 512-row
  chunks; `xnode_traffic_bulk_write.go` combines native daily/period writes.
- `xnode_traffic_arithmetic.go` preserves exact fixed-point remainders and rejects
  overflow. PAYG charges, credits and immutable ledger facts keep their existing
  durable path under the shared wallet/period locks. PAYG probe attribution stays
  on its existing immediate financial path.
- `xnode_traffic_batch_test.go` covers malformed/discarded/replayed reports, exact
  remainder carry, whole-write rollback and retry. `xnode_traffic_batch_finance_test.go`
  covers rolled-back charges, shared-wallet fleet contention and durable replay.
- `xnode_traffic_batch_benchmark_fixture_test.go` prepares private native data;
  `xnode_traffic_batch_benchmark_test.go` compares individual and combined writes
  for 32 nodes and 64 owners. Hosted CI publishes latency, allocations and CPU
  profiles. Local automated suites remain prohibited.

## Versioned firewall projections

`xnode_firewall_cache.go` shares immutable dynamic-bank pages across node delivery
workers. A single indexed committed-clock read precedes each lookup; a newer
version invalidates prior pages, and delayed older readers cannot publish into
the new generation. Cold loads coalesce and failures are never cached. Returned
bank slices are private so snapshot IDs, pagination flags and caller changes
cannot alter another node's payload. No financial or admission decisions use
these cached projections.

The LRU holds at most 128 pages and an estimated 8 MiB, with a five-minute lifetime
below the seven-day operational-history retention. Existing absolute agent
deadlines remain authoritative. This cache adds no tables or write transactions.
`xnode_firewall_cache_test.go` covers committed detection visibility, immutable
results, stale reader races, expiry, errors and capacity. New mutable projection
banks require explicit cloning and size accounting before caching.

`xnode_automatic_leases.go` releases only unexpired exact-IP bans through the
existing serializable firewall write/outbox path. It does not insert lease IPs,
remove broader ranges or modify manual/paid exemptions or ledger records.
`xnode_automatic_leases_test.go` covers replay, expired grants, retained IPv6
ranges, unchanged paid/manual grants and an unchanged clock on no-op replay.

`xnode_payg_node_buffer.go` collects exact directional PAYG samples per owner,
node and capture minute. Report-local forks merge only after successful accounting;
512-row sorted UNNEST upserts share the existing batched traffic transaction and
cursor/financial commit. PAYG probes retain immediate financial counters while
staging their raw display samples. Samples use their captured Through minute.
Records older than 72 hours or more than five minutes ahead are skipped; accepted
clock-skew minutes remain stored and become visible when their time arrives.
`xnode_payg_node_history.go` reads at most 128 node IDs and 73 hourly buckets
per node, with explicit truncation. Historical node IDs remain readable after
node deletion, and absent history is never inferred from group totals.
`xnode_payg_node_history_test.go` covers savepoint discard, replay, financial
rollback, owner isolation, removed nodes, retention and read bounds.

Migration 38 adds node retirement and parent-locking binding guards. xnode_node_removal.go fences final deletion/retirement with row locks and the compile revision, keeps historical references and audit evidence, and excludes retirement from active inventory without deleting accounting.

Migration 39 adds combos.features_json with an empty array default. Combo writes validate shared feature limits; omitted features preserve stored order/content and an explicit empty list clears them.

`combo_features_test.go` covers ordered feature preservation/clearing. `firewall_history_fixture_test.go` seeds historical financial grants. `xnode_first_sync.go` and `xnode_first_sync_test.go` persist and verify owner proof. `xnode_node_removal.go` and `xnode_node_removal_test.go` guard dependencies, retirement and history.

`account_purge_resume.go` recovers one durable purge job per account generation under the account row lock; review failures require explicit operator evidence. `TrackXNodePeriodNode` serializes grants with the period status row before network delivery. Closing-period scans rotate in memory, and final closure preserves the original closing boundary. Generated adapters cover `account_purge_resume.go` and transactional admission registration in `xnode_barriers.go`.

`account_purge_recovery_test.go` covers same-job dependency waits, exhausted-job recovery, generation isolation and terminal review. `xnode_period_admission_test.go` covers post-close admission refusal, stale closure fencing and rotating settlement scans. `account_purge_resume_retry_generated.go` retries the complete recovery transaction.
`xnode_period_finalize.go` atomically finalizes only the observed closing boundary; reopen and replacement boundaries reject stale network completion without changing the general period-update contract.
