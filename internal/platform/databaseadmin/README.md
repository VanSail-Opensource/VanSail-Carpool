# Reviewed PostgreSQL administration

The browser editor exposes typed records and reviewed insert/update/delete for
operational tables. Policies own table/column protection reasons and project the
same rules into metadata. Deletes cannot cascade into protected evidence or
credentials. Financial/accounting/audit evidence, secrets, generated fields and
existing keys cannot be edited.

All `hourly_*` billing and credit relations are protected, including offers and
member preferences that require versioned or consent-bearing domain commands.
`account_cleanup_*` evidence/health and `xnode_usage_*` history are managed state;
generic writes cannot manufacture purge admission or rewrite usage coverage.

`postgres.go` resolves identifiers and native types through PostgreSQL catalogs.
Record, key, value and query modules preserve exact numeric strings and redact
sensitive values. Declared primary keys are required.

Mutation modules create actor-bound ten-minute one-use reviews. Preparation
checks exact request digests and optimistic hashes. Apply requires a complete
verified rescue backup, consumes the review, locks and rechecks the record,
executes parameterized SQL, and appends a redacted audit in one transaction.
Constraints remain authoritative; domain hooks are bypassed.

Extend policies before exposing another protected subsystem. Ordinary fields
inherit native constraints; generated columns/defaults come from metadata.
Hosted regressions cover CRUD, numeric precision, actor binding, expiry/replay,
concurrent changes, protected records/keys and backup failure.

`types.go` and `types_part2.go` define typed transport. `schema.go`, `schema_part2.go`, `policies.go` and `policy_schema.go` classify fields and enforce protection. `records.go`, `record_keys.go` and `record_values.go` preserve exact redacted rows. `query.go` and `query_part2.go` bind queries. `mutations.go` and `mutations_part2.go` own reviews and backups; `mutation_prepare.go` validates/locks, `mutation_values.go` parses values for both filters and edits, and `mutation_execute.go` executes writes and audits. Native PostgreSQL boolean predicates bind Go booleans; retained integer flags still bind 0/1. Text operators reject boolean columns. `service_test.go`, `query_boolean_test.go` and `mutation_service_test.go` cover reads, native/legacy boolean filtering and reviewed CRUD.
