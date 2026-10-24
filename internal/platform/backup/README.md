# Backup and restore
- `restore_part2.go` continues the focused implementation from its original package module.
- `restore_startup_part2.go` continues the focused implementation from its original package module.
- `service_part2.go` continues the focused implementation from its original package module.

This package creates database backups, serves contained downloads, stages
administrator restore requests, and performs the pre-open database swap. Restore
markers and results are written atomically so process crashes remain recoverable.
Marker version 3 carries the initiating actor scope, idempotency key, and request
fingerprint into the replacement database, allowing exact command replay after restart.

## Files

The implementation is split so backup publication and restore startup remain independently reviewable.

Streamed upload ownership is split across focused files:

- `upload_types.go` defines the bounded lifecycle and 2 GiB default cap.
- `upload_receive.go` streams candidates to private staging files while hashing and enforcing the configured cap.
- `upload_finalize.go` validates regular-file identity, SHA-256, SQLite integrity, foreign keys, and migration compatibility before publication.
- `upload_records.go` persists upload and backup-run publication metadata.
- `upload_reconcile.go` fails interrupted intake or completes a verified publishing phase after restart.
- `upload_validation_test.go` covers cap, hash, schema, and upload-run metadata validation.
- `upload_reconcile_test.go` covers interrupted publication and symlink rejection.
- `upload_test_helpers_test.go` contains shared upload service fixtures and SQLite candidate builders.

- `restore_startup_part2.go` contains restore marker startup transitions.
- `service_part2.go` contains backup publication and retention helpers.

- `service.go` — service construction, backup deletion/containment, and verified
  online backup creation.
- `retention.go` — seven-day managed-file and backup-record retention, excluding
  backups with an active restore.
- `download.go` — safe opening of completed backup files for HTTP download.
- `restore.go` — restore contracts, confirmation, staging, database-path lookup,
  and pending-marker checks.
- `restore_idempotency.go` normalizes restore commands, fingerprints their payload,
  and resolves actor-scoped replay or conflict before checking the pending marker.
- `restore_snapshot.go` — snapshot copying, hashing, integrity checks, and ordered
  migration-prefix validation.
- `restore_jobs.go` — restore-job insertion, ready/failure transitions, rescue
  backup linkage, and staged-job reads.
- `restore_startup.go` — pre-open swap, SQLite sidecar handling, rollback, startup
  failure recording, and post-open completion recording.
- `restore_files.go` — bounded marker/result reads, path validation, error
  truncation, and atomic JSON file replacement.
- `service_test.go` — backup creation, retention, download containment, staged
  swap/rollback, and migration-marker verification tests.

`postgres.go` produces private pg_dump custom archives and checks their structure
before publication through the existing backup metadata API. Archive inspection
does not prove recoverability: `postgres_test.go` restores the archive into a
separate hosted database and compares all exact records and encrypted settings.
PostgreSQL online upload/file replacement is refused. Physical restore and PITR
require stopped application workers and the separate pgBackRest operator flow.
