# Affiliate administration

- `AdminAffiliatesPanel.vue` owns the versioned settings form and save flow.
- `AdminAffiliateTierEditor.vue` edits one tier with accessible reorder controls.
- `affiliateForm.ts` mirrors server tier and reward-union validation with Zod.
- `useAdminAffiliates.ts` loads coupon references and handles optimistic-version saves.
- `AdminAffiliateTierEditor.test.ts` covers conditional reward-union fields.

Affiliate reads and versioned saves retire older requests and stop at workspace
disposal. Saves capture a separate submitted tier list, preserve edits made while
the write is pending, and advance only the confirmed configuration version.
Repeated loads use cached snapshots only for initial hydration.
`useAdminAffiliates.test.ts` covers a late cached refresh, a newer draft during
save, and owner disposal before the write completes.
