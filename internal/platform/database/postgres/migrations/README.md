# Forward PostgreSQL migrations

Migrations 00001 through 00031 are immutable applied history. They remain embedded
so both fresh installations and populated PostgreSQL upgrades follow one ordered
chain. Do not rewrite old scripts or remove historical accounting/audit tables.

All applied forward migrations remain immutable. `00046_retire_traffic_multiplier.sql` retires traffic
multipliers transactionally: stop the old runtime before migrating, drain SQL
writers, install 1x at final cursor+1 and fence future policy writes. It preserves
committed history and fractional remainders; see [the cutover contract](../../multiplier_retirement.md).

`00032_native_postgresql.sql` replaces dependent JSON CHECK constraints, expression
indexes and paid-access trigger functions before dropping compatibility functions.
It preserves stored values, ciphertext, text collation and exact timestamp text.
Drops deliberately omit CASCADE so an unexpected live dependency aborts migration.

Add numbered forward migrations, update SchemaVersion and the native contract,
and extend hosted fresh/upgrade/recovery checks for future changes. Downgrades
refuse execution. Only the migration role owns schema changes; the app uses DML.

## Applied history


`00027_agreement_continuity.sql` mirrors SQLite 113 and restores prior accepted
members without changing their recorded consent or first-time onboarding.

`00023_period_aliases_source_marker.sql` records SQLite migration 109 in the
source inventory after its equivalent PostgreSQL schema has been applied.

`00022_xnode_period_aliases.sql` retains the node-scoped reference map introduced
by SQLite migration 109 for schema compatibility; traffic discard does not use it.

`00016_xnode_ddns_multi_hosts.sql` permits multiple host references per DDNS
family while retaining the unique host constraint.

`00014_xnode_endpoint_epoch.sql` mirrors SQLite 102 endpoint invalidation without
invalidating on equal values or timestamp-only updates.

`00013_xnode_ddns_changes.sql` mirrors SQLite 101 desired mutation/order state
with exact text sequences and durable operation references.

`00012_xnode_ddns_bindings.sql` matches SQLite 100 host ownership, audit references
and invalidation triggers without copying encrypted discovery settings.

`00011_catalog_topology_epoch.sql` is the forward equivalent of SQLite 099,
adding narrow metadata invalidation to the existing authoritative epoch.

`00010_payg_squad_state.sql` adds the same administrator-owned availability state
as SQLite migration 098. Existing products default to normal.

`00008_payg_credit_windows.sql` applies SQLite migration 096's prepaid expiry
state and rolling report windows with BIGINT counters and binary timestamp text.
Existing charge evidence is retained. Downgrade refuses authoritative mutation.

`00001_compatibility.sql` installs the explicitly supported text/JSON/time
compatibility functions. `00002_baseline.sql` creates the final post-startup
schema, bootstrap defaults, constraints, indexes and invariant triggers.
`00003_domain_ordering.sql` gives cache and FIFO records declared ordering keys.

`00004_outbox_fencing.sql` adds monotonic claims, transactional wake-up hints and
a sequence for the disjoint X-Node service-ID range.

`00005_cutover_state.sql` records the inactive import and verification gates and
makes foreign keys deferrable during a controlled, atomic source import.

`00006_observability.sql` adds derived aggregate views for outbox, reconciliation,
accounting barriers and logical backup age. It duplicates no authoritative rows.

`00007_payg.sql` is the forward-only PostgreSQL equivalent of SQLite PAYG migration 095, including binary text collation and compile invalidation triggers. The older baseline is unchanged.

`00009_payg_preferences.sql` is the forward-only equivalent of SQLite 097. It retains BIGINT flags, C text collation, foreign keys and narrow compile invalidation.
# Paid member access

`00015_paid_member_access.sql` updates preference transition functions to retain
paid PAYG access and backfills Show Around TX for existing commerce spenders.
The existing columns and immutable financial records remain unchanged.

- `00017_pricing.sql`: Forward PostgreSQL pricing schema corresponding to SQLite migration 105, including immutable fact triggers and compact physical forecast/actual tables.

- `00018_retire_rollover.sql`: Cancel unsettled credits without wallet changes and archive settled evidence before operational schema retirement.

- `00019_personal_pricing_decisions.sql`: Immutable terminal personalized decisions and funding outcome separation.

- `00020_host_name_overrides.sql`: Only authored owner-specific host display preferences.

- `00021_pricing_active_commands.sql`: Adds the command/status/expiry lookup index
  required by the existing source schema contract without rewriting migration 00017.

- `00024_pricing_read_indexes.sql`: Paired PostgreSQL exposure-window/entity-day indexes with authoritative downgrade refusal.

`00025_provider_cost_models.sql` adds the versioned provider charge model and capacity mode, corresponding to SQLite migration 111.

`00026_unlimited_zero_capacity.sql` permits zero finite capacity when the current provider-cost row is marked unlimited.

Migration 114 / PostgreSQL 00028 retires questionnaires. It archives settled
reward evidence in audit_events, fails pending receipts with FEATURE_RETIRED,
removes retired jobs, and drops operational tables without changing wallets or
ledger rows. The hosted forward-upgrade rehearsal verifies these boundaries.

`00028_retire_questionnaires.sql` mirrors SQLite 114 and archives financial evidence before removing operational tables.

`00029_account_lifecycle.sql` mirrors SQLite 115, including database guards for new purchases, payments, sessions and onboarding completion.

`00030_account_timestamp_collation.sql` restores the required C collation on the
account purge timestamp without changing its stored evidence or prior migrations.

`00031_squad_features.sql` mirrors SQLite 116 for local squad merchandising features.

`00033_volatile_traffic_streams.sql` adds boot-scoped cursor tombstones and rate
boundaries plus settlement stream references. It never stores report buffers or
source IPs and does not rewrite historical usage, financial facts or legacy IDs.

`00034_payg_node_history.sql` adds 72-hour directional minute summaries for
PAYG display. It is forward-only, with no historical reconstruction or changes
to committed accounting. Expiry runs in bounded existing PAYG retention batches.

`00035_retire_host_connectivity.sql` drops disposable host-check attempts and its obsolete setting; accounting, audit and earlier migrations remain intact.

`00036_retire_member_firewall.sql` releases grants, retires jobs and queues cleanup. `00037_member_first_sync.sql` adds first-sync evidence. `00038_node_retirement.sql` adds retirement and binding guards. `00039_combo_features.sql` adds empty-default features.

Member-firewall retirement explicitly drops the exclusive quota compile trigger/function before dropping the obsolete column; broad CASCADE is not used.

`00040_node_removal_admission.sql` adds a null-default durable removal fence and extends binding admission guards. No purchase, financial entry or existing account is converted. The agent reconnect credential survives until accounting drains.

`00041_period_closure_generation.sql` adds a nonnegative, zero-default durable
closure generation. Existing closing periods remain valid at generation zero;
the next open-to-closing transition increments it atomically. Reopen/reclose
cannot reuse a prior participant barrier even when timestamps match. Purchase,
traffic and financial records retain their original values.

`00042_hourly_credit_obligations.sql` adds empty daily allowance/funded-credit
projections, agent-global issue highwaters and scoped byte reservations. Foreign
keys retain user/node accounting subjects; immutable-scope/proof and monotonic
counter guards preserve unresolved obligations and final receipts. Recovery can
advance its confirmed-use projection while keeping the original proof digest.
No customer, purchase, native accounting period or wallet entry is converted.
Hourly sales and paid-extra funding remain unavailable until their commerce and
verified-agent compatibility gates are delivered.

`00043_hourly_commerce.sql` adds independent offers, unpaid activation intents,
immutable UTC-hour/decimal-GB receipts and explicit opt-in preferences. Typed
PostgreSQL timestamps avoid lexical RFC3339 fraction ordering at hour boundaries.
Future fixed-term purchases record immutable original validity; existing NULL
snapshots remain unknown. No existing customer, purchase or wallet is converted.

`00044_hourly_usage_history.sql` adds forward-only raw traffic and coverage
evidence. The existing journal transaction and cursor deduplicate accepted
samples. It also invalidates disposable native compilation when hourly access
changes; minute history never becomes an additional debit or quota source.

`00045_account_cleanup_evidence.sql` adds bot-epoch health, scheduled nonce checks, and immutable sanitized cleanup evidence. Existing blocked flags confer no grace proof. It also preserves future known purchased-term duration in financial tombstones; old terms remain unknown.

`00047_hourly_offer_required_hours.sql` adds a per-offer funding threshold
`required_hours` (`0..2562047`); zero inherits the account-wide
`activationHours`, so existing offers keep their previous eligibility. It is
additive metadata: no purchase, wallet entry, settlement receipt or native
principal is converted.

`00048_telegram_star_reconciliation.sql` adds a protected per-bot cursor, monotonic page generation, exact boundary hash and scheduled continuation time. Existing balances, payments and historical receipts remain unchanged. The cursor is part of the native schema/backup contract; restoring an older snapshot safely replays idempotent provider history.

`00049_native_first_sync_reconfirmation.sql` clears historical member first-sync
proof once because earlier acknowledgments did not bind the current owner
lifecycle and credential material. Fresh authenticated configuration membership
reconfirms readiness; offline members wait for an applied acknowledgment. Native
credentials, periods, purchases, balances and immutable financial evidence remain
unchanged. The forward-only migration does not run again on ordinary startup.
