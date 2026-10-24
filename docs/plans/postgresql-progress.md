# PostgreSQL implementation progress

This checklist tracks delivery of `docs/postgresql-migration.md`. A checked code
deliverable is not a production acceptance claim. All operational gates require
evidence from the selected deployment.

| Step | Repository implementation | Validation / remaining gate |
| --- | --- | --- |
| 01 Inventory | Final schema, dialect/write inventory and invariants implemented | Hosted determinism passes; production size/latency/locks require target |
| 02 Infrastructure/config | PG18 embedded in the selected 1 GiB app container, persistent storage, SCRAM/roles, bounded pools and supervision; external profile retained | External-role gate passes; new embedded image gate pending |
| 03 Goose baseline | Numbered schema, triggers, C text ordering, catalog/version checks | Hosted fresh/reapply and prefix-upgrade gates pass |
| 04 pgx compatibility | SQL-aware binding, typed null/blob/bool handling and complete-operation retries | Hosted portable business contracts pass |
| 05 Domain repositories | Domains ported; generic admin writes refused while catalog reads/masking remain | Hosted portable contracts pass; live workflow rehearsal pending |
| 06 Financial concurrency | Balance locks, exact arithmetic, stock/payment/renewal races, real deadlock retry | Hosted PostgreSQL concurrency gate passes |
| 07 X-Node accounting | Cursor/period locks, exact charging/replay/digests and commit-before-ACK | Hosted journal contracts pass; live reconnect rehearsal pending |
| 08 Outbox | SKIP LOCKED, resource/fencing, dedicated LISTEN and poll fallback; bounded member admission, scoped DIY deployment and 30s retry cap | Hosted original lease/resource gates pass; added routing regressions pending |
| 09 Recovery | Logical export, encrypted pgBackRest/WAL, S3 option and offline procedures | Hosted restricted-role logical restore and all-127-table full/PITR verification pass; target repository/key drill pending |
| 10 Observability | Private exporter, aggregate views/rules, pool/retry/ACK logs, physical-backup metrics | Hosted scrape/grant gate passes; live Prometheus/Grafana and off-site integration pending; Redis deferred |
| 11 Conversion CLI | Exact inspect/export/import/verify/readiness/recovery, sequences, ciphertext and activation gates | Hosted round-trip, failure rollback and refusal paths pass |
| 12 Rehearsal/cutover | Hosted gates and operator runbooks implemented | Complete replacement release CI, protected staging/browser/upstream rehearsal, benchmark and live cutover pending |

## Deployment decisions

Hosted [PostgreSQL job in run 40065516216](https://github.com/VanSail-Opensource/VanSail-Carpool/actions/runs/40065516216)
at `eb5d7cd3` passed all database gates: fresh/reapplied/prefix-upgraded schema,
portable business and concurrency contracts, exact conversion/refusal/rollback,
restricted-role logical restoration, encrypted physical restoration and named
PITR matching all 127 table hashes, restricted-role application activation,
single ownership, and private exporter scraping. The new actual-image embedded
rehearsal caught pgBackRest's default lock path on a read-only root; its lock
path now lives on the writable recovery volume and requires a replacement run.
The selected release head must pass the entire hosted workflow before delivery.

The owner selected PostgreSQL server inside the application container and a
1 GiB memory limit. See [embedded deployment](../../deploy/embedded/README.md).
The selected profile keeps the legacy `/data` volume and a separate durable
PostgreSQL volume, uses 128 MiB shared buffers and a 384 MiB Go heap budget,
and retains full commit durability. These conservative values still need live
workload measurements; they are not a production benchmark claim.

PostgreSQL TIMESTAMPTZ stores microseconds, while current Go timestamp values can
contain nanoseconds. The proposed safe transition is to preserve exact timestamp
text initially, then perform a separately verified conversion. This changes the
plan's initial type mapping. The owner explicitly selected exact preservation on
2026-10-24; the PostgreSQL baseline retains timestamp TEXT accordingly.

Staging and production targets, protected snapshot access and backup repository
have not been selected. No live cutover or database mutation has been attempted.
