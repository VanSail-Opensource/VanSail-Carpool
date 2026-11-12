# PostgreSQL exports and recovery

`Service.Run` creates a permission-restricted pg_dump custom archive, validates
its archive structure, publishes it atomically and commits backup metadata.
The existing maintenance queue owns invocation and retry; handlers only enqueue
work. The database URL is protected process input and is never logged.

- `service.go` owns deletion, path containment and service dependencies.
- `service_part2.go` owns publication, verified completion and managed-file pruning.
- `postgres.go` runs installed PostgreSQL client tools with bounded contexts.
- `download.go` opens contained regular completed files for authenticated download.
- `paths.go` limits download and deletion to managed direct children of the
  configured directory, rejecting symlinked parent paths and unrelated files.
- `retention.go` prunes current `.dump` archives/metadata and preserves older
  recovery evidence. Active historical restore references protect their archives.
- `restore.go` protects archives referenced by historical recovery records.
  Historical rows remain intact; there is no upload, pre-open file replacement,
  sidecar handling or restore marker executor.
- `service_test.go` covers retention and download containment. `postgres_test.go`
  restores a real archive into an isolated hosted database, reapplies role grants
  and compares every authoritative table and identity-sequence digest through
  the restricted application role, including encrypted evidence.

Archive readability alone does not prove recovery. Follow
[offline physical recovery and PITR](../../../deploy/postgres/recovery.md), stop
application workers, restore to an isolated volume/database and verify evidence
with `db-schema verify` and `db-schema fingerprint` before resuming ownership.
Keep CONFIG_MASTER_KEY and repository cipher keys independently backed up.

Canceled exports persist their failure through a separate five-second context
before workers finish shutting down. Both visible and historical hidden deletion
stages expire through normal retention; recovery references protect their original
archive paths. `cancellation_test.go`, `paths_test.go` and the retention fixtures
cover these boundaries in hosted CI. Abrupt process or database failure can still
require operator reconciliation; this is not a claim of cross-filesystem/database
transaction atomicity.
