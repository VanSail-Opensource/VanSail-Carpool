# Backup admin

- `BackupUploadPanel.vue` streams a bounded SQLite candidate with optional SHA-256 verification into the staged restore workflow.
- `MaintenanceTrigger.vue` confirms and tracks the idempotent manual maintenance command, emitting a completion event for backup and job refresh.
- `MaintenanceTrigger.test.ts` covers confirmation, queued/loading state, receipt polling, completion refresh signaling, and queue errors.
- `RestoreBackupDialog.vue` confirms a server-reviewed restore operation.
- `BackupUploadPanel.vue`, `MaintenanceTrigger.vue`, and `RestoreBackupDialog.vue` limit Motion to upload, receipt, and confirmation state changes.

`BackupInventory.vue` renders verified backups as a desktop table with signed-download, staged-restore and delete actions; `JobInventory.vue` renders durable jobs, attempts, errors and guarded retry/delete actions.
