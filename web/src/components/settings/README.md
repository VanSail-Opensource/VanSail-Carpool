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
Language and currency share the same flat settings-row rhythm;
the group tag save action sits beside its input.

`SettingsPage.vue` composes existing language, currency and member access controls.
It is the sole member navigation entrance to `/settings/diy`; the existing DIY
route and in-memory drafts remain owned by the DIY module. Mini App entrances
navigate through the Vue router instead of emitting anchors, because Telegram's
WebView treats host-level URL navigation separately from in-app history.
`NotificationSettings.vue` renders the five delivery categories.
`SettingsSwitchRow.vue` is a controlled, accessible row: changes are confirmed by
the preferences store before the displayed value changes.
`GroupMemberTagEditor.vue` edits live Telegram data, including clearing a tag.
`useSettings.ts` isolates canonical tag load/save/error states.
The page uses the account-scoped preferences store, typed HTTP clients and queued
Telegram APIs. Optional entrances require current server-confirmed eligibility.

`useSettings.test.ts` covers prior-owner reads, failed-save state retention and
disposal. Tag reads cannot supersede a pending save and strand its busy state.

`useComboControls.ts` refreshes temporarily blocked eligibility every five seconds while visible, stops after errors/disposal and retains receipt-driven reconciliation. The settings header also offers refresh; purchase/activation constraints and the last-enabled-squad guard remain authoritative.

`SettingsPAYGControls.vue` composes the master and sparse per-group preference switches with existing controlled rows. Its owner-scoped PAYG loader supplies squad names, exact balance, and loading/error states; switches are disabled when balance is nonpositive or unavailable. Confirmed preferences remain visible and unchanged, including subordinate choices, while a balance notice explains the subscription restriction. The server independently checks activation requests.
`SettingsPAYGControls.test.ts` covers balance transitions, failed refreshes, and preservation of the displayed choices.

`SettingsNavigationRow.vue` supplies flat full-width DIY/admin entrances with localized labels, existing icons and router-only navigation. Admin visibility and one-use console access remain owned by the session and AdminAccessPanel.

Administrator links retire synchronously on owner, privilege or destination
changes. Link creation and clipboard feedback are generation-scoped; expired or
invalid links cannot be copied after refresh failure. `AdminAccessPanel.test.ts`
checks delayed owners, expiry recovery and obsolete copy completion.

Subscription controls and queued receipts also retire on account changes. A
failed or pending eligibility refresh blocks changes until a current read
succeeds. Early activation requires matching current/queued purchase IDs and
fresh typed confirmation when the target, eligibility or period changes.
`useComboControls.test.ts` and `EarlyActivationDialog.test.ts` cover those guards.
