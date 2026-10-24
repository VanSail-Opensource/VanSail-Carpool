# PostgreSQL migration history

`00001_compatibility.sql` installs the explicitly supported text/JSON/time
compatibility functions. `00002_baseline.sql` creates the final post-startup
schema, bootstrap defaults, constraints, indexes and invariant triggers.
`00003_domain_ordering.sql` gives cache and FIFO records declared ordering keys.

The parent package embeds these forward-only Goose migrations. New changes use
new numbers; every downgrade refuses authoritative-data mutation. Original SQLite
migrations remain separate and immutable. The baseline is reproducible from the
isolated schema inventory and the reviewed generator, never from production data.
