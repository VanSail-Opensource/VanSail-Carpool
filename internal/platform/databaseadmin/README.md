# Reviewed PostgreSQL administration

The browser editor exposes typed records and reviewed insert/update/delete for
operational tables. Policies own table/column protection reasons and project the
same rules into metadata. Deletes cannot cascade into protected evidence or
credentials. Financial/accounting/audit evidence, secrets, generated fields and
existing keys cannot be edited.

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
