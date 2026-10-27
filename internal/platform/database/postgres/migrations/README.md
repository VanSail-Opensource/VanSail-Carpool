# PostgreSQL migration history

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

The parent package embeds these forward-only Goose migrations. New changes use
new numbers; every downgrade refuses authoritative-data mutation. Original SQLite
migrations remain separate and immutable. The baseline is reproducible from the
isolated schema inventory and the reviewed generator, never from production data.

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
