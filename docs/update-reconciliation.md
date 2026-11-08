# Native update receipts

The existing queued node-operation worker persists an `updateExpectation` in its provider receipt before the first native mutation. Install intent records the operation ID as native transaction ID, the requested/resolved release and the original running release. An unspecified release is resolved once through the documented `update.check`; retries use the persisted version. Rollback intent refers to the original install transaction and restores its `previous` release.

Recovery polls documented `update.status` through the existing command path. Success requires the expected transaction ID, install version, previous version, nonempty deadline and final status (`confirmed` for install, `rolled_back` for rollback). The current authenticated peer must also report the expected running version, no pending upgrade, fresh ready/synchronized telemetry and the current configuration revision. Status reads and readiness are bound to the same peer. No network wait holds the master mutex.

Interrupted mutations are never repeated. A legacy in-flight operation without original intent stays under review; it cannot fabricate an earlier version after restart. Missing/mismatched transaction evidence produces `XNODE_UPDATE_UNCONFIRMED`. A confirmed transaction lacking current readiness produces `XNODE_UPDATE_READINESS_PENDING`. Offline admission and pre-mutation status/release lookup failures use resumable outbox dependency waits. Intents and sanitized transaction fields survive all final outcomes; native executable paths are excluded. Existing completed receipts remain unchanged.

A completed native install reply can prove an unchanged result when its current/target releases match the persisted intent and fresh readiness. This receipt-bound no-op exception is unavailable during interrupted-command recovery. Version strings alone never confirm a mutation.

Rollback rechecks the original transaction immediately before dispatch and pins dispatch to that peer. The pinned upstream rollback API accepts an empty argument object; it has no transaction-CAS argument. A later authorized out-of-band transaction change therefore remains uncertain unless the original matching rollback receipt is observed. Do not invent unsupported wire arguments or infer completion from a changed version.

`update_transaction_v1` gates status-based confirmation. The panel's pinned public protocol already declares `update.status`; no protocol/native-validator upgrade is required for this fix. Configuration/accounting access barriers remain independent of command result waits.

Hosted checks cover immutable intent, legacy attempts, unrelated transaction IDs, incorrect previous versions, pending receipts, reconnect, stale/unready telemetry, configuration generations, peer replacement and result sanitization. Local validation uses Go vet and static review only. Browser command fixtures show localized uncertainty/readiness states.
