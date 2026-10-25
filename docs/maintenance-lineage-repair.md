# Maintenance foreign-key follow-up

Production revision `7778ac23` passed the original routing receipt pruning
failure. Authorized maintenance `80defeda-8e57-4dcd-8d0f-247948450d4a` then failed
in purchase compaction on `purchases_auto_renew_source_purchase_id_fkey`.
Its PostgreSQL backup `b4edcc54-25ce-40e3-b435-49a301c5a66a` completed first.
Compaction rolled back as one transaction; no partial financial rollups committed.

The old selector protected recent/active successors but missed old expired terms
retained by native accounting, unfinished jobs or another lineage relationship.
The final candidate set now excludes referenced predecessors repeatedly until
stable, covering both automatic renewal and renewal batches. Internal links are
detached only when both purchases are actually compacted, after tombstoning.

The foreign-key audit also covered restrictive receipt links (temporary bans,
compensation, routing plans, metered probes and IP checks), purchase links
(native periods, automatic successors, batches and consumed activity extensions),
and abandoned-user references. Cascading detail and explicitly detached/SET NULL
references retain their existing policies. Restrictive user references preserve
actors, desired state and financial attribution rather than deleting their facts.

Go vet and static schema review are permitted locally. Regressions run on hosted
SQLite/PostgreSQL jobs; successful production rerun remains the acceptance gate.
