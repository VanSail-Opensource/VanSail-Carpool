# Admin user workflows

- `AdminBulkExtensionDialog.vue` previews inclusive-OR active targets and queues the audited durable extension job.
- `AdminDurationField.vue` and `duration.ts` provide the minute-normalized amount/unit control shared by administrative extension workflows.
- `duration.test.ts` covers deterministic minute, hour, and day conversion boundaries.
- `AdminEntitlementEditor.vue` validates the advanced optimistic-lock overwrite form.
- `AdminEntitlementRefundDialog.vue` requires an exact positive TXB-minor refund amount bounded by the original net debit.
- `AdminOperationResolutionDialog.vue` records an audited resolution without retrying an ambiguous provider mutation.
- `AdminUserEntitlements.vue` presents active and queued entitlement table rows with edit and exact-refund actions; only the editor can change a combo.
- `AdminUserHistory.vue` presents tabbed Emby, payment, refund, and durable-operation tables, with refund and courtesy-credit actions only for that profile's eligible payment records.
- `AdminUserIPBlocks.vue` presents active member-created blocks and exposes only the administrator unblock action.
- `AdminUserOverview.vue` presents identity, balance, synchronization, and the active combo.
- `AdminUserPM.vue` moves PM controls, topic navigation, repair, delivery history, and read status into the selected profile.
- `AdminUserCouponActions.vue` grants or historically discards active purchase-discount wallet coupons.
- `AdminUserSearchFilters.vue` supplies browser popover state, combo, and squad facets with AND/OR composition.
- `AdminUserAccountContext.vue` presents successful affiliate history without duplicating member facts.
- `AdminUserProfilePage.vue` composes the aggregate user workflow and its focused dialogs.
- `AdminUserIPBlocks.test.ts` verifies that administrator profiles expose unblock without a block control.
- `AdminUserProviderActions.vue` emits the queued provider receipt to the profile shell before its reload.
- `adminUserFormat.ts` owns profile-specific status tones and date conversion helpers.
- `useAdminUserProfile.ts` loads the aggregate and serializes idempotent profile mutations with conflict handling.
- Table cells and native overlays keep the profile stable during mutation review.
`useAdminRefundQuote.ts` loads server refund suggestions without overwriting manual edits or changed entitlement contexts. `useAdminRefundQuote.test.ts` covers defaults, failed loads, retries and stale responses.

The profile separates entitlement, history, and account workspaces.
`AdminUserOverview.vue` supplies independent browser identity and metric geometry;
it does not rely on Mini App styles. Native browser navigation opens PM topics.
Provider, refund, audited resolution, coupon, and PM repair failures stay visible
inside their focused overlays; exact amounts and operation receipts remain owned
by the existing typed APIs and composables.

`AdminPaymentCreditDialog.vue` loads verified matching evidence for one payment with
abort/generation guards. Selecting evidence invokes settlement; the no-evidence option
uses the courtesy ledger. Failed evidence loads disable confirmation until retry.
