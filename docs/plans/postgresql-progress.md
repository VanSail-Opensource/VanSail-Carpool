# PostgreSQL implementation progress

This checklist tracks delivery of `docs/postgresql-migration.md`. A checked code
deliverable is not a production acceptance claim. All operational gates require
evidence from the selected deployment.

| Step | Repository implementation | Validation / remaining gate |
| --- | --- | --- |
| 01 Inventory | Fixture utility, final schema, SQL-contract/source locations, mapping, invariants and risk report implemented | Static review passed; hosted determinism pending; production measurements require target access |
| 02 Infrastructure/config | Isolated PostgreSQL 18 profile, SCRAM roles/secrets, pool/deadline/policy validation, explicit activation refusal implemented | Hosted startup/authentication/privilege checks pending; application network dependency belongs to activation stage |
| 03 Goose baseline | Forward-only engine, full generated baseline, catalog contract and schema CLI implemented | Hosted fresh/reapply/immutability checks pending |
| 04 pgx compatibility | Pending | Depends on baseline and exact value/scanner contracts |
| 05 Domain repositories | Pending | Depends on backend compatibility |
| 06 Financial concurrency | Pending | Requires hosted row-lock, stock, callback and retry regressions |
| 07 X-Node accounting | Pending | Requires hosted cursor/replay/reset/concurrency regressions |
| 08 Outbox | Pending | Resource exclusion, monotonic fencing, SKIP LOCKED and notifications |
| 09 Recovery | Pending | Backup repository, full restore and PITR environment required |
| 10 Observability | Pending | PostgreSQL telemetry/exporter/alerts; Redis remains separately gated |
| 11 Conversion CLI | Pending | Schema/type policy and reconciliation contracts required |
| 12 Rehearsal/cutover | Pending | Staging/production target, verified ingress policy and recovery evidence required |

## Deployment decisions

PostgreSQL TIMESTAMPTZ stores microseconds, while current Go timestamp values can
contain nanoseconds. The proposed safe transition is to preserve exact timestamp
text initially, then perform a separately verified conversion. This changes the
plan's initial type mapping. The owner explicitly selected exact preservation on
2026-10-24; the PostgreSQL baseline retains timestamp TEXT accordingly.

Staging and production targets, protected snapshot access and backup repository
have not been selected. No live cutover or database mutation has been attempted.
