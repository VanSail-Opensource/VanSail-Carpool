# User settings

Internal squad switches return completed local preference receipts immediately;
the UI refreshes confirmed state without waiting for a provider job. Submission
disables repeated actions. Existing queued receipts still resume after reload,
and early activation retains its durable operation and progress feedback.

- `AdminAccessPanel.vue` is the Mini App's sole admin surface, displaying one-use browser entrance links.

`SettingsComboControls.vue` composes owned-squad switches, recoverable operation
feedback and the eligible early-start action. `useComboControls.ts` owns loading,
receipt resumption and cache invalidation; `EarlyActivationDialog.vue` owns typed
confirmation and date preview. Ownership remains separate from enabled access.
Add squads uses a full-width action row and the existing squad addition dialog.
Language, currency and automatic reset share the same flat settings-row rhythm;
the group tag save action sits beside its input.

`SettingsPage.vue` composes existing language, currency and traffic-reset controls.
It is the sole member navigation entrance to `/settings/diy`; the existing DIY
route and in-memory drafts remain owned by the DIY module. Mini App entrances
navigate through the Vue router instead of emitting anchors, because Telegram's
WebView treats host-level URL navigation separately from in-app history.
`NotificationSettings.vue` renders the five delivery categories.
`SettingsSwitchRow.vue` is a controlled, accessible row: changes are confirmed by
the preferences store before the displayed value changes.
`GroupMemberTagEditor.vue` edits live Telegram data, including clearing a tag.
`useSettings.ts` isolates reset automation and canonical tag load/save/error states.
The page uses the account-scoped preferences store, typed HTTP clients and queued
Telegram APIs. Optional entrances require current server-confirmed eligibility.

`useSettings.test.ts` covers immediate reset persistence and failed-save state retention.

`useComboControls.ts` refreshes temporarily blocked eligibility every five seconds while visible, stops after errors/disposal and retains receipt-driven reconciliation. The settings header also offers refresh; purchase/activation constraints and the last-enabled-squad guard remain authoritative.

`SettingsPAYGControls.vue` composes the master and sparse per-group preference switches with existing controlled rows. Its owner-scoped PAYG loader supplies squad names and loading/error states; confirmed preferences remain authoritative after save failures. The master switch preserves subordinate choices.

`SettingsNavigationRow.vue` supplies flat full-width DIY/admin entrances with localized labels, existing icons and router-only navigation. Admin visibility and one-use console access remain owned by the session and AdminAccessPanel.
