# Persistence invariants and migration risks

## Financial and commerce truth

Balances and ledger evidence commit atomically with purchases, reservations,
entitlements and their outbox effects. Signed 64-bit overflow is rejected before
SQL arithmetic. Ordinary debits cannot create debt; administrator adjustments and
provider reversals retain their existing explicit debt semantics. Exact provider
references and callback fingerprints deduplicate credits. Settled financial
history cannot be rewritten, and predecessor settlement cannot revoke a paid
successor. Reset never erases purchase-wide refund evidence.

Lock order must be documented per transaction: sorted account IDs, order IDs,
reservation keys, entitlement/period IDs. A retry repeats the whole transaction
only for serialization/deadlock failures. Network/provider calls happen after
commit through existing queues; they are never retried by a SQL statement wrapper.

## X-Node truth

An agent cursor and all accepted usage commit together before ACK. Serialize first
cursor creation and subsequent report evaluation by agent. Enforce contiguous
sequences, replay digest consistency, sorted shared-period locking, exact upload,
download, weighted usage and fixed-point remainder arithmetic. Preserve late
reports through aliases and unfinished reset barriers. Closing/review periods,
service identities and metered probes keep their ownership rules. Large browser
integers stay decimal strings. Live metrics/rates/IPs remain memory-only.

## Durable work

Business effects and outbox insertion share a transaction. Active-work partial
unique indexes and resource admission remain enforced. `SKIP LOCKED` alone does
not protect two different jobs sharing the same user or node: preserve derived
resource exclusivity. Persistent claim generations must increase through recovery,
manual retries and busy deferrals; resettable attempts are not a fencing token.
NOTIFY only wakes polling. A dedicated LISTEN connection reconnects independently.
Unknown provider effects require reconciliation before retry after cutover.

## Trigger inventory

The fixture contains 67 trigger definitions. The structured inventory preserves
each exact source definition and its table. Non-compile protections include:

- Affiliate config/tier update/delete immutability, including the narrow recorded
  identity-rekey exception for config authors.
- Notification payload immutability with the recorded owner-rekey exception.
- Terminal USDT evidence immutability.
- Purchase preference visibility before/after transitions.
- Firewall inventory counts/release versions and node membership retirement.
- Historical open-host-operation exclusion, retained only for compatibility.

Compile-revision triggers cover all source configuration/identity/period/squad
changes and conditional settings/node/user transitions. PostgreSQL implementations
must preserve WHEN predicates and update-column restrictions, including the
exhaustion boundary. Concurrent invalidation cannot lose revisions.

## Recovery and operational gates

Retain SQLite's verified file restore only for SQLite. PostgreSQL requires logical
exports plus encrypted physical backups and continuous WAL archiving. Restore and
PITR drills must verify data, encryption with the independently secured master key,
session references, cursors and unfinished jobs in a separate deployment. The
single master/scheduler rule remains until distributed ownership is implemented.

No production metrics or snapshot have been provided. Baseline latency, writer
contention, data volume, callback ingress policy, off-site storage and recovery
objectives remain deployment evidence requirements, not inferred passing results.

Before PostgreSQL production writes, rollback may use the untouched verified
SQLite snapshot. After production writes, use PostgreSQL repair/recovery or a
separately verified reconciliation; never silently reopen stale SQLite truth.
