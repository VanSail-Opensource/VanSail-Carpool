# Native PostgreSQL delivery

The application now accepts PostgreSQL only. Runtime SQL uses native parameters,
JSON operators, intervals, conflict targets and locking. pgx receives native rows
without SQL translation or row coercion. SQLite drivers, migrations, conversion
tools, source inventories and live file-swap restoration have been removed.

Applied PostgreSQL migrations 1 through 31 remain unchanged. Forward migration
32 replaces dependent checks, indexes and trigger functions before dropping live
compatibility functions. Stored timestamp text, encrypted credentials, identities
and historical financial/accounting evidence are preserved exactly.

The existing embedded PostgreSQL deployment retains its persistent volumes,
restricted application role, migration role, ownership fence, pgBackRest/WAL
backup and stopped-worker recovery. External PostgreSQL remains supported.
Missing configuration or connection failure produces a startup error.

Hosted CI uses isolated PostgreSQL fixtures for ordinary business and race tests,
plus fresh/populated schema, concurrent commerce, restricted-role export,
logical/physical restoration, PITR and actual embedded-image gates. The current
native-query repair run must finish before delivery is accepted. Local static
review and browser evidence do not substitute for successful hosted workflows.

No production database mutation or live deployment is part of this code change.
Recovery keys, repositories and production workload measurements remain
deployment-specific evidence. Operational instructions live under
[`deploy/postgres`](../../deploy/postgres/README.md) and
[`deploy/embedded`](../../deploy/embedded/README.md).
