# Baseline and migration-risk report

## Measured repository baseline

The generated fixture captures 95 historical migration filenames, 127 tables,
119 explicit indexes and 67 triggers. Each table's row count, columns, declared
type, primary-key position, nullability and default are in
`postgresql/sqlite-inventory.json`. The source-location manifest records direct
SQL contracts, transaction boundaries, writer locks, outbox insertions and
SQLite-specific operations. Generated SQL strips platform line endings so hosted
Linux and Windows inventories can be compared.

The fixture command completes the real migration/rekey path; it is not a
production workload benchmark. Production volume, row counts, queue age, journal
backlog, transaction percentiles and lock-wait statistics are **not measured**.
Capture them before choosing production pool limits or estimating outage time.

## Highest risks and gates

| Risk | Existing evidence | Required gate |
| --- | --- | --- |
| Lost timestamp precision | `stamp` emits RFC3339Nano; PostgreSQL timestamps are microsecond precision | Explicit preservation policy before baseline type conversion |
| Startup schema differs from migrations | Vault-aware rekey removes assigned usernames and rewrites references/ciphertexts | Use final fixture, require source startup completion before export |
| Lost debit/stock/cursor serialization | Shared `writeMu` and SQLite immediate transactions | Row-lock and whole-transaction retry scenarios in hosted PostgreSQL CI |
| Stale worker completion | Retry attempts can decrease/reset | Independent monotonic durable claim generation |
| Weakened editor identity | Generic editor uses SQLite metadata and implicit row IDs | Catalog implementation and declared keys; mutations disabled until verified |
| Unrecoverable backup | SQLite VACUUM/file replacement is backend-specific | Physical+WAL recovery integration and demonstrated restore/PITR |
| Callback loss during outage | Provider retry behavior is provider-specific | Verified retry/durable-ingress policy before maintenance |

## Delivery evidence

Inventory capture and Go static analysis are permitted local checks. Full Go
regressions and PostgreSQL role/authentication checks run in hosted Actions.
The optional infrastructure creates no host port and uses separate SCRAM roles.
Application activation refuses PostgreSQL until the remaining compatibility work
is implemented; it cannot silently open SQLite in response to an explicit request
for PostgreSQL. No production settings or database files have been changed.
