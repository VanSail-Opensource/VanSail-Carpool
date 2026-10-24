# PostgreSQL migration history

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
