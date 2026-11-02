# Backup administration

`BackupInventory.vue` renders PostgreSQL archive metadata with download and
reviewed delete actions. Restore is an offline operator operation, described by
the localized page copy. Tables remain horizontally contained on mobile.
`JobInventory.vue` renders durable job attempts, errors and retry/delete actions.
`MaintenanceTrigger.vue` tracks the existing durable maintenance command and
refreshes backup/job lists after completion. `MaintenanceTrigger.test.ts` covers
confirmation, pending receipts, completion refresh and queue failures in hosted CI.
The parent `AdminBackupsPanel.vue` owns API actions, errors and workspace tabs.
