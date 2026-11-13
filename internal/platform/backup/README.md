# PostgreSQL exports and recovery

`Service.Run` creates a permission-restricted pg_dump custom archive, validates
its archive structure, publishes it atomically and commits backup metadata.
Scheduled maintenance invokes `Run`. The manual administrator endpoint invokes
`RunForActor`, which requires the attempt and actor audit to commit together
before export starts. Failed exports remain attributed. The database URL is
protected process input and is never logged.

- `service.go` owns deletion, path containment and service dependencies.
- `delete_actor.go` locks and checks the current eligible administrator before
  staging a deletion; the fence remains held through metadata and audit commit.
  `delete_actor_test.go` checks demotion, cleanup, blocked and missing actors
  cannot alter the archive, metadata or audit history.
- `service_part2.go` owns publication, verified completion and managed-file pruning.
- `manual.go` separates scheduled runs from mandatory audited manual admission;
  `manual_test.go` verifies audit failure prevents export and failed exports retain
  the actor-bound attempt.
- `postgres.go` runs installed PostgreSQL client tools with bounded contexts.
- `publication.go` syncs the renamed archive's parent directory before completed
  metadata is allowed. A directory-sync failure preserves the visible archive
  while reporting uncertain durability. `publication_test.go` covers publication,
  directory failure and rename failure with actual filesystem evidence.
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
