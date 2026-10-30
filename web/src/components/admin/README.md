# Admin components

The squad editor exposes normal/hidden/unavailable PAYG state separately from
Combo merchandising. Hidden affects Explore and subscription output without
cutting existing connections; unavailable suspends
PAYG authorization without changing Combo entitlements or accounting history.

Administrative views render exclusively inside the independent browser console.
The console owns navigation, identity, appearance, and the desktop workspace.
Feature panels use Nuxt UI tables, focused side editors, tabs, and compact
toolbars; contained table scrolling supports smaller browsers.

Localized route-level panels and shared admin-only Nuxt UI.

`workspace.css` owns reusable table, editor, section, and profile geometry. It is
imported by the browser stylesheet and does not depend on Mini App styles or
Telegram viewport/safe-area tokens.

Page loaders restore session response snapshots before refreshing. Editable settings,
payment profiles, activity configuration, and onboarding/affiliate drafts preserve
edits made while refreshed data is pending. See `../../api/cache/README.md`.

Admin section labels are locale-owned, use Nuxt UI controls, and avoid visual separator literals in component code.
`AdminSectionState.vue` uses Vue's native sequential fade for loading, errors and
content, avoiding external presence bookkeeping during immediate reloads.
Administrative controls use browser feedback and native overlay focus management.
Errors remain visible inside the overlay that initiated a failed mutation, and
forms preserve their drafts so an administrator can correct or retry the action.

- `AdminActivityPanel.vue` manages games and lucky draws; calendar and reward settings are in `AdminSettingsPanel.vue`.
- `AdminAuditPanel.vue` filters the audit trail.
- `AdminBackupsPanel.vue` manages backup and restore jobs; its localized manual-maintenance control shares the durable operation status flow and refreshes both inventories after success.
- `backups/` contains focused staged-restore and streamed-upload controls.
- `AdminBillingAmountLimits.vue` loads the global inclusive Add TXB range and atomically saves exact minor-unit bounds.
- `AdminCatalogEditor.vue` edits combo terms.
- `AdminCatalogPanel.vue` manages combos and squads.
- `AdminCompensationPanel.vue` composes the revisioned outage policy and recipient-safe event review flow, with configuration and event tabs, frozen-recipient event tables, and focused review dialogs.
- `AdminCouponsPanel.vue` manages coupon definitions.
- `AdminDatabasePanel.vue` browses protected database tables.
- `AdminEmbyPanel.vue` monitors Emby provisioning.
- `AdminEntitlementsPanel.vue` manages entitlements.
- `AdminOnboardingPanel.vue` loads, saves, and publishes visual onboarding drafts.
- `onboarding/` contains bilingual content indexes with focused editors and text previews.
- `AdminPaymentProfiles.vue` lists and edits masked provider accounts, keeps BEPUSDT channels discovery-owned, deletes profiles through a protected confirmation, and contributes remaining drafts to the parent settings save.
- `AdminReasonDialog.vue` collects audited reasons and keeps action failures visible in the dialog.
- `AdminSectionState.vue` renders loading and error states.
- `AdminSettingsPanel.vue` composes runtime, payment, and activity tabs while preserving one coordinated Save action for runtime/payment/activity drafts; payment interventions stay scoped to individual user histories.
- `AdminSquadEditor.vue` edits sparse squad overrides; node access remains a live X-Node projection.
- `admin/squad-profile/` contains the typed Broadband, China Optimized, and International Network profile editor modules.
- `AdminStatisticsPanel.vue` presents resource statistics with contained table scrolling and exact server-calculated figures.
- `AdminUsersPanel.vue` searches users with responsive state/combo/squad facets and adjusts balances.
- `users/` contains the aggregate profile, wallet, provider-account, entitlement workflow, operation-resolution, and bulk-extension modules.
- `compensation/` contains focused configuration, event-card, review-modal, and display-format modules.

The editor also provides dropdowns for coupon-eligible combos and squads, a stock
limit field for squad products, a table-name search for database recovery work,
and mobile-safe backup controls. Balance adjustments use the shared audited
admin endpoint for both credits and deductions.
- `AdminSectionState.vue` is the shared loading/error/content presence boundary. Panel roots stay still; focused child workflows own their local Motion state.

`connectivity/` composes exact-account configuration, progress, latest results and 24-hour cursor history. Its exact-account configuration and run/history workflow remain independent; the raw `connectivity.config` registry entry is excluded from generic settings fields and writes.

Combo editing accepts the API-supported empty description and zero price. Its Zod submission schema enforces exact decimal money/traffic bounds, validity and quotas, with visible localized feedback for invalid drafts. Busy or failed saves retain the draft.

`AdminSquadEditor.vue` edits automatic PAYG mode and exact positive per-GB pricing alongside the existing Combo fields. Stock, visibility and activation-code controls apply to Combo; conversion preserves existing paid terms and excludes future renewals. Browser editor state and theme remain independent of Mini App initialization.

`AdminSectionState.vue` keeps successful slot content mounted through background refresh and errors, announces one loading status, and reserves three 5rem placeholders only before first content. `AdminSectionState.test.ts` covers draft retention, retry, and successful empty results.
