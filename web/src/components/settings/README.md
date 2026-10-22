# User settings

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
route and in-memory drafts remain owned by the DIY module.
`NotificationSettings.vue` renders the five delivery categories.
`SettingsSwitchRow.vue` is a controlled, accessible row: changes are confirmed by
the preferences store before the displayed value changes.
`GroupMemberTagEditor.vue` edits live Telegram data, including clearing a tag.
`useSettings.ts` isolates reset automation and canonical tag load/save/error states.
The page uses the account-scoped preferences store, typed HTTP clients and queued
Telegram APIs. Optional entrances require current server-confirmed eligibility.

`useSettings.test.ts` covers immediate reset persistence and failed-save state retention.

`useComboControls.ts` refreshes temporarily blocked eligibility every five seconds while visible, stops after errors/disposal and retains receipt-driven reconciliation. The settings header also offers refresh; purchase/activation constraints and the last-enabled-squad guard remain authoritative.
