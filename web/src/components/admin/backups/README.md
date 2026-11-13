# Backup administration

`BackupInventory.vue` renders PostgreSQL archive metadata with download and
reviewed delete actions. Restore is an offline operator operation, described by
the localized page copy. Tables remain horizontally contained on mobile.
`JobInventory.vue` renders durable job attempts, errors and retry/delete actions.
`MaintenanceTrigger.vue` tracks the existing durable maintenance command and
refreshes backup/job lists after completion. `MaintenanceTrigger.test.ts` covers
confirmation, pending receipts, completion refresh and queue failures in hosted CI.
The parent `AdminBackupsPanel.vue` owns API actions, errors and workspace tabs.

`AdminBackupsPanel.test.ts` covers target retention, duplicate-submit suppression,
explicit deletion retry and retirement of archive downloads after unmount. Hosted
Chromium fixtures check the real pending/error/empty states on desktop and mobile.

Successful deletion also records a workspace-local removed ID before inventory
refresh. Stale snapshots and failed refreshes cannot restore the deleted row or
its actions. These volatile markers disappear with the owning workspace; they
do not replace server retention or recovery evidence. Browser checks inject a
failed post-delete read and recover without replaying the deletion.
