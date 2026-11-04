# Admin squad profile editor

- `AdminSquadProfileEditor.vue` owns the type selector, validation state, and preview.
- `CountryCodeField.vue` accepts a guarded uppercase ISO alpha-2 country code and restores the last valid value after an accidental blank.
- `BroadbandProfileFields.vue` edits ISP, port, access mode, and detailed location.
- `ChinaOptimizedProfileFields.vue` edits CT, CU, CM, port, and country.
- `InternationalNetworkProfileFields.vue` edits port, country, and upstream carriers.
- `SquadPortField.vue` provides the shared Mbps and unlimited-port control.
- `profileForm.ts` maps API profiles to the compact editor draft and back.
- `profileForm.test.ts` checks type-specific mapping, unlimited ports, and required fields.
- `AdminSquadFeaturesEditor.vue` edits the ordered, optional features with icon previews and move controls; `featureForm.ts` validates Unicode character limits and normalizes text for the squad product API.
- `AdminSquadFeaturesEditor.test.ts` covers feature order and removal. `featureForm.test.ts` covers Unicode limits and normalized payloads in hosted CI.

The editor emits only a complete typed profile. Incomplete drafts remain local
to the form and cannot be submitted to the admin API.
Saves omit untouched feature lists to preserve concurrent updates; an explicitly
edited empty list clears the saved features.

AdminSquadFeaturesEditor and featureForm are shared by squad and Core combo editors. Untouched features are omitted to preserve newer content; explicit empty lists clear them.
