# Operational scripts

`xnode_period_recovery.py` previews a node's missing historical period IDs from
an immutable SQLite snapshot against the current PostgreSQL state. The snapshot
SHA-256 and node UUID are required. It accepts only a unique same-owner,
same-purchase, same-boundary/strategy/quota match with no snapshot usage. It
refuses changed, settled, already-flushed or ambiguous target periods.

By default the command only prints a JSON preview. `--apply` repeats the
checks in a serializable transaction, adds missing period participants and
node-scoped mappings, and records snapshot and mapping evidence in `audit_events`.
Only run after the compatible panel schema is installed and after a verified
backup and rehearsal. The script never changes a journal cursor or a balance.

For embedded PostgreSQL, `--docker-container NAME` invokes psql inside that
container. `--database NAME` selects a restored rehearsal database; the default
is `vansail`. Otherwise it uses the caller's standard PostgreSQL environment.
Pass credentials via protected environment or PostgreSQL service configuration,
not command-line arguments. Existing mappings are idempotent.
