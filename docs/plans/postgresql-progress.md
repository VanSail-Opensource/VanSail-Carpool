# PostgreSQL implementation progress

This checklist tracks delivery of `docs/postgresql-migration.md`. A checked code
deliverable is not a production acceptance claim. All operational gates require
evidence from the selected deployment.

| Step | Repository implementation | Validation / remaining gate |
| --- | --- | --- |
| 01 Inventory | Final post-startup schema, dialect/write inventory and invariants implemented | Hosted schema determinism passes; production size/latency/lock measurements require target access |
| 02 Infrastructure/config | Isolated PG18, SCRAM/secrets, separate roles, bounded pools and secure opt-in application overlay implemented | Hosted authentication/privilege checks pass; target deployment pending |
| 03 Goose baseline | Faithful numbered schema, triggers, C text ordering and catalog/version checks implemented | Fresh/reapply/immutability checks pass; latest complete CI pending |
| 04 pgx compatibility | SQL-aware binding, typed null/blob/bool handling and whole-operation retries implemented | Hosted domain regressions actively resolving remaining differences |
| 05 Domain repositories | All domains ported; PostgreSQL generic admin writes refused while read/masking use system catalogs | Portable hosted contracts plus SQLite upgrade contracts retained; full green gate pending |
| 06 Financial concurrency | Balance locks, exact arithmetic, stock/payment/renewal races and real deadlock regression implemented | Latest hosted concurrency run pending |
| 07 X-Node accounting | Cursor/period locking, ordered charging, replay/digest invariants and commit-before-ACK preserved | Existing journal regressions run on PG; full latest gate pending |
| 08 Outbox | SKIP LOCKED, resource exclusion, monotonic claim fencing, dedicated LISTEN and polling fallback implemented | Hosted generation/exclusion/session-loss checks pending on latest code |
| 09 Recovery | Logical exports, encrypted pgBackRest/WAL, S3 option, offline procedure and isolated full/PITR drills implemented | Recovery drills pending successful round-trip fixture; live storage/key drill requires target |
| 10 Observability | Private exporter, aggregate views/rules, pool/retry/ACK logs and pgBackRest textfile metrics implemented | Hosted exporter scrape pending; production Prometheus/Grafana/off-site integration external; Redis deferred |
| 11 Conversion CLI | Inspect/export/import/verify/readiness/recovery, exact hashes, sequences, encryption and activation gates implemented | Hosted round-trip and refusal paths pending latest CI |
| 12 Rehearsal/cutover | Hosted conversion/recovery/application gates and complete operator runbooks implemented | Access-controlled staging, browser/upstream rehearsal, benchmarks and live cutover remain external gates |

## Deployment decisions

Hosted run [40063479916](https://github.com/VanSail-Opensource/VanSail-Carpool/actions/runs/40063479916)
at `b27f217e` passed fresh/reapplied schema, exact conversion, logical restoration,
encrypted physical restoration, named-target PITR and single-owner application
bootstrap. Its PostgreSQL job failed overall because the limited-stock fixture
reused a globally unique seed reference and the exporter profile omitted its
database dependency. Subsequent changes fix those gates and extend generation-
aware queue contracts, failed-import rollback and service-sequence evidence.
The selected release head must pass the entire hosted workflow before delivery.

PostgreSQL TIMESTAMPTZ stores microseconds, while current Go timestamp values can
contain nanoseconds. The proposed safe transition is to preserve exact timestamp
text initially, then perform a separately verified conversion. This changes the
plan's initial type mapping. The owner explicitly selected exact preservation on
2026-10-24; the PostgreSQL baseline retains timestamp TEXT accordingly.

Staging and production targets, protected snapshot access and backup repository
have not been selected. No live cutover or database mutation has been attempted.
