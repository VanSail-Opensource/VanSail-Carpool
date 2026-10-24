# PostgreSQL schema mapping

The authoritative source is the post-startup
[schema inventory](postgresql/sqlite-inventory.json), including every column,
default, composite key, original CHECK/FK/UNIQUE definition, index and trigger.
The SQL companion preserves the full definitions for review and schema comparison.

| SQLite representation | PostgreSQL representation | Constraint |
| --- | --- | --- |
| INTEGER money/counters/IDs | BIGINT | Exact signed 64-bit values; never floating-point money |
| INTEGER 0/1 flags | Constrained integer | Preserve existing Go scanners and typed admin behavior |
| TEXT stable identifier | TEXT | Preserve bytes and case; no UUID coercion |
| BLOB secret/hash/token | BYTEA | Preserve ciphertext and associated-data identity contexts |
| TEXT canonical payload | TEXT | No JSONB recanonicalization of fingerprints, outbox or signed data |
| Timestamp text | TEXT initially | Owner selected exact preservation; defer TIMESTAMPTZ conversion |
| Date-only bucket | DATE or deliberate TEXT | Preserve the accounting timezone/date policy |
| INTEGER PRIMARY KEY | BIGINT identity or explicit stable ID | Explicit IDs remain allowed; reset sequences after import |
| Implicit row ordering | Declared generated ordering column | Import original order for caches/queues; not a general editor row identity |

PostgreSQL timestamps have microsecond precision. Existing RFC3339Nano financial,
cursor and optimistic-review values can contain nanoseconds; migration must
explicitly preserve byte-sensitive evidence and define comparison/cursor semantics
before conversion. The owner selected exact timestamp text on 2026-10-24.
No initial timestamp cast or rounding is permitted.

All 127 tables require a baseline definition. Circular foreign keys require a
second constraint phase or deferred validation during import. All 119 explicit
indexes must be mapped, including active-work partial uniqueness predicates.
SQLite table-level uniqueness must not be counted as absent merely because its
automatic index has no CREATE SQL. Tables without declared keys remain excluded
from PostgreSQL administrative mutations.

Migration tooling must copy dependency-ordered rows into an empty target, retain
encrypted data unchanged, validate FKs and reset every generated identity. The
source migration version list is an exact allowlist, not just a maximum number:
two historical migrations use the `073` prefix.
