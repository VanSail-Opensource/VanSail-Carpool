# IP lookup administration

`AdminIPLookupPage.vue` owns the browser console form, schema validation and configuration/provider/quota tabs. `useAdminIPLookup.ts` loads atomic settings and clears write-only credentials after save. Provider and geolocation order use existing reorder controls; ordinary fees, refresh quota cost and legacy purchase allowances remain configurable.

The source inventory, custom resolver and active probe tabs, request helpers and diagnostics are retired. Existing settings decode without those fields. No background inventory job or scan is launched by administration.

Feature implementation and contracts: `AdminIPLookupPage.vue`, `ComboQuotas.vue`, `ProviderOrder.vue`, `ProviderSettings.vue`, `RetrySettings.vue`, `useAdminIPLookup.ts`.
