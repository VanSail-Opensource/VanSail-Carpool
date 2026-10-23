# PostgreSQL compatibility inventory

## Evidence and scope

The checked-in [SQLite schema](postgresql/sqlite-schema.sql) and
[structured inventory](postgresql/sqlite-inventory.json) are captured by
`go run ./cmd/db-inventory`. This opens a new private fixture, executes every
embedded migration and completes the vault-aware Telegram startup migration.
The current final schema has **127 tables, 119 explicitly declared indexes and
67 triggers**. SQLite's automatically created uniqueness indexes are represented
by their original table constraints. Historical migrations are unchanged.

`python scripts/postgresql_inventory.py` records every identified SQL-specific
source location, direct `*sql.DB`, `*sql.Tx` and `*sql.Conn` contract, writer
serialization site, transaction entry and outbox insertion helper in
[source-inventory.json](postgresql/source-inventory.json). This is a searchable
location inventory, not evidence of PostgreSQL compatibility.

## Replacement map

| Existing dependency | Required PostgreSQL replacement |
| --- | --- |
| `database.Open`, SQLite URI pragmas | Explicit driver selection, pgx stdlib, bounded pool and connection deadlines; never availability fallback |
| Embedded historical migrations | Independent Goose baseline of the final fixture schema; reject newer schemas |
| `Store.writeMu`, `_txlock=immediate` | Account/order/period row locks, deterministic ordering and whole-transaction retries; retain SQLite serialization |
| `?` parameters | SQL-aware binding that preserves quoted strings, identifiers, comments and dollar strings |
| `INSERT OR IGNORE` | Explicit `ON CONFLICT DO NOTHING`; audit every affected-row/idempotency caller |
| `IS ?`, nullable reference equality | `IS NOT DISTINCT FROM`; retain ordinary `IS NULL` semantics |
| Scalar `MAX` / `MIN` | `GREATEST` / `LEAST`; retain aggregate forms and checked integer arithmetic |
| `json_extract`, `json_valid`, `json_set` | Explicit typed extraction/validation/update; preserve canonical text and distinguish missing, null, string and boolean values |
| `julianday`, RFC3339 text ordering | Native timestamp comparisons and interval arithmetic with updated scanners and cursors |
| `GLOB`, `COLLATE NOCASE` | Explicit pattern/ILIKE behavior; verify wildcard escaping and identity-alias rules |
| `rowid` insertion order | Declared durable ordering keys for IP cache/queue ordering; natural/composite primary keys elsewhere |
| `LastInsertId` | `INSERT ... RETURNING`; preserve explicit proxy IDs and the service ID range |
| SQLite error text | pgx SQLSTATE classification, preserving domain conflict and not-found contracts |
| `sqlite_schema`, `pragma_*` | PostgreSQL catalogs; declared primary keys only; disable generic mutations until parity is established |
| `VACUUM INTO`, WAL checkpoint | Backend-specific snapshot/export and pgBackRest/WAL recovery |
| SQLite startup identity rekey | Complete on source before export; baseline has no assigned username; preserve aliases and ciphertext contexts |
| Online file replacement restore | Offline PostgreSQL maintenance/recovery; never replace a running database |

## Domain audit order

1. Identity/settings/session contracts and encrypted associated-data contexts.
2. Financial/commerce checked balances, reservations and payment deduplication.
3. X-Node cursors, fixed-point usage, barriers, period closing and compile triggers.
4. Outbox admission, resource exclusivity, fencing and provider recovery.
5. Supporting features, administrative introspection, backup and maintenance.

The inventory includes retired historical tables and constraints because they
remain upgrade/financial evidence. Their presence does not authorize restarting
legacy integrations or adding new writers.

## Operational evidence still required

No production snapshot or metrics endpoint was supplied for this implementation.
Production database/WAL size, nonzero table counts, journal backlog, transaction
latency and lock contention must be captured from the selected deployment with
the read-only inventory command and operational metrics. Empty fixture row counts
are not a production baseline. No production cutover may pass on fixture evidence.
