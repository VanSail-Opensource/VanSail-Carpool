# Exact database conversion

`source.go` opens a read-only consistent SQLite snapshot and checks integrity,
foreign keys, exact migrations, final schema fingerprint and stable identities.
`snapshot.go` uses the driver's native online backup API and refuses overwrites.
`report.go` streams typed row hashes in binary primary-key order, retaining exact
payloads, timestamps, blobs and insertion order. `invariants.go` reconciles balances
using arbitrary-precision sums and validates encrypted document contexts without
printing plaintext. `copy.go` streams COPY values and resets owned sequences.

`import.go` requires an inactive fresh target, verifies bootstrap-only row counts,
copies all records in FK dependency order and commits once. User triggers are
suspended only inside that transaction; deferred FKs are checked before commit.
`verify.go` compares every authoritative table and records a hashed verification
gate. `cutover-check` rechecks that hash and refuses changed data, financial
anomalies and unreconciled processing jobs/providers. The source is never repaired
or rekeyed here; finish those operations with the compatible old application.

Use migration-role credentials for schema/import/verification and the unchanged
independently secured CONFIG_MASTER_KEY. All reports are private and hashed; never
check live reports or snapshots into Git. A failed import rolls back data and
trigger changes. Previously populated or active targets are never overwritten.
Source snapshots must remain immutable until cutover verification completes.

`sequences.go` retains deleted-ID high water marks as well as live IDs.
`roundtrip_test.go` exercises hosted exact import, overwrite refusal, key failure
and changed-data readiness rejection. `roundtrip_fixture_test.go` supplies real
encrypted configuration, report cursor, multiplier carry and daily usage evidence.

`recovery.go` verifies a consistent PostgreSQL snapshot after logical, physical
or point-in-time restore. `db-migrate recovery-check --expected-hash` compares it
with secured pre-backup evidence without changing activation state.
