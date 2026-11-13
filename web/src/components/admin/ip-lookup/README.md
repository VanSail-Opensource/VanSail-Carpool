# IP lookup administration

`AdminIPLookupPage.vue` owns the browser console form, schema validation and configuration/provider/quota tabs. `useAdminIPLookup.ts` loads atomic settings and clears write-only credentials after save. Provider and geolocation order use existing reorder controls; ordinary fees, refresh quota cost and legacy purchase allowances remain configurable.

The source inventory, custom resolver and active probe tabs, request helpers and diagnostics are retired. Existing settings decode without those fields. No background inventory job or scan is launched by administration.

Feature implementation and contracts: `AdminIPLookupPage.vue`, `ComboQuotas.vue`, `ProviderOrder.vue`, `ProviderSettings.vue`, `RetrySettings.vue`, `useAdminIPLookup.ts`.

All configuration fields are locked while an atomic save is pending. Saves use
a snapshot of the reviewed settings, load/save admission is exclusive, and owner
changes or workspace disposal clear credential drafts and retire pending replies.

`useAdminIPLookup.test.ts` covers immutable save snapshots, read/write admission, owner changes and disposal of private provider drafts.

Initial or refresh failure keeps the form read-only until settings and catalog
load successfully. Empty defaults can never be submitted after a failed read;
previously loaded values remain inspectable and the retry action remains
available. Hosted unit and desktop/mobile browser regressions cover failure,
recovery and the first authorized save.
