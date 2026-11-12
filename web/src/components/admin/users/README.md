# Admin user workflows

- `AdminBulkExtensionDialog.vue` previews inclusive-OR active targets and queues the audited durable extension job.
- `AdminDurationField.vue` and `duration.ts` provide the minute-normalized amount/unit control shared by administrative extension workflows.
- `duration.test.ts` covers deterministic minute, hour, and day conversion boundaries.
- `AdminEntitlementEditor.vue` validates the advanced optimistic-lock overwrite form.
- `AdminEntitlementRefundDialog.vue` requires an exact positive TXB-minor refund amount bounded by the original net debit.
- `AdminManualEntitlementDialog.vue` creates a zero-charge, audited fixed-term grant with explicit combo, term, bytes, cadence, optional squads and reason; 409 overlaps keep the draft visible with localized conflict copy.
- `AdminOperationResolutionDialog.vue` records an audited resolution without retrying an ambiguous provider mutation.
- `AdminUserEntitlements.vue` presents the fixed-term plan workspace: a primary manual-grant action plus labelled per-row edit-terms and refund actions, readable status/validity/price facts, and Motion transitions that honor reduced motion.
- `AdminUserHourly.vue` presents the administrator hourly projection (policy, wallet, daily usage, enrollments) with a confirmed disable action.
- `useAdminUserHourly.ts` owns the hourly projection load with abort/generation guards, the quoted policy version and idempotent disable queuing.
- `AdminUserHistory.vue` presents tabbed Emby, payment, refund, and durable-operation tables, with refund and courtesy-credit actions only for that profile's eligible payment records.
- `AdminUserIPBlocks.vue` presents active member-created blocks and exposes only the administrator unblock action.
- `AdminUserOverview.vue` presents identity, balance, synchronization, and the active combo.
- `AdminUserPM.vue` moves PM controls, topic navigation, repair, delivery history, and read status into the selected profile.
- `AdminUserCouponActions.vue` grants or historically discards active purchase-discount wallet coupons.
- `AdminUserSearchFilters.vue` supplies browser popover state, combo, and squad facets with AND/OR composition.
- `AdminUserAccountContext.vue` presents successful affiliate history without duplicating member facts.
- `AdminUserProfilePage.vue` composes the aggregate user workflow, its focused dialogs, and the administrator hourly projection.
- `AdminUserIPBlocks.test.ts` verifies that administrator profiles expose unblock without a block control.
- `AdminUserProviderActions.vue` emits the queued provider receipt to the profile shell before its reload.
- `adminUserFormat.ts` owns profile-specific formatting, status tones, date conversion, and exact positive-balance checks used to present wallet-funded PAYG status.
- `useAdminUserProfile.ts` loads the aggregate and serializes idempotent profile mutations with conflict handling, including action-specific conflict copy for entitlement overlaps and active hourly enrollments.
- Profile summaries and entitlement badges derive current validity at time boundaries and on focus/visibility refresh. Advanced edits and explicit manual refunds retain the raw lifecycle status; presentation never settles accounting or rewrites a purchase.
- Table cells and native overlays keep the profile stable during mutation review.
`useAdminRefundQuote.ts` loads server refund suggestions without overwriting manual edits or changed entitlement contexts. `useAdminRefundQuote.test.ts` covers defaults, failed loads, retries and stale responses.

The profile uses accessible tabs for entitlement, pricing, metrics, history, and
account workspaces. Only the selected workspace mounts its data-heavy content;
the tab list scrolls horizontally when necessary on small screens.
`AdminUserOverview.vue` supplies independent browser identity and metric geometry;
it does not rely on Mini App styles. Native browser navigation opens PM topics.
Provider, refund, audited resolution, coupon, and PM repair failures stay visible
inside their focused overlays; exact amounts and operation receipts remain owned
by the existing typed APIs and composables.

`AdminPaymentCreditDialog.vue` loads verified matching evidence for one payment with
abort/generation guards. Selecting evidence invokes settlement; the no-evidence option
uses the courtesy ledger. Failed evidence loads disable confirmation until retry.

`AdminUserMetrics.vue` presents native availability, rates, exact accounting and seven-day charts/tables. `useAdminUserMetrics.ts` aborts superseded loads, guards identity generations, polls only while mounted and clears snapshots/buffers at logout. `AdminUserBalanceAdjustment.vue` moves audited TXB adjustments into the selected profile.

`useAdminUserMetrics.test.ts` verifies abort/generation isolation between profiles and retirement of pending responses/samples on logout.

Balance adjustments carry an idempotency key for the exact member, signed amount
and reason. Ambiguous failures retain that key for a safe retry; confirmed
successes retire it so a subsequent intentional adjustment is distinct.
`AdminUserBalanceAdjustment.test.ts` covers these recovery boundaries.

The IP table distinguishes stale/unavailable reports from a fresh report with no recent activity. No scan action or receipt polling is needed.

`AdminUserActiveIPs.vue` presents automatically refreshed, owner-scoped sampled
IP observations, node, last activity and exact counts. `AdminUserPAYGHistory.vue`
keeps recorded historical nodes in a native Nuxt UI filter and 72-hour hourly
chart/table. `userTrafficHistory.ts` aggregates with BigInt and converts only
plot coordinates; `userTrafficHistory.test.ts` checks precision and filters.
`AdminUserTrafficHistory.vue` displays native graph counters without imported
series or columns. Canonical current/lifetime accounting totals remain unchanged.
Refreshes never overlap, pause while hidden and preserve a clearly marked stale
snapshot on error; scope changes and logout clear all volatile information.

An available but truncated empty IP list is marked limited, preserving the
distinction from a complete fresh report with no recent activity. The backend
bounds output across nodes as well as per node.

`AdminIPUnblockDialog.vue` runs administrator unblock with browser controls and durable receipts.
