# Member DIY Center

This module composes client-only subscription tuning and existing managed
routing under one page Save. Drafts stay in memory; failed validation, requests
and revision conflicts retain both sections. Reset restores subscription
inheritance without touching routing. The backend owns the allowlist,
compatibility and atomic encrypted persistence; generated clients type all data.

The page uses member theme assets and no administrator resources. Client
refresh, not route deployment, activates connection tuning. Routing remains on
the existing durable fleet deployment queue and keeps its own pending status.

`DIYCenter.vue` composes the two sections and one Save. `useDIY.ts` owns drafts,
both revision fences, idempotent retry keys and durable deployment receipts.
`useDIYPreview.ts` debounces cancellable server validation with stale-response
fencing. `DIYSettingsGroup.vue` groups server-described typed fields;
`DIYFieldControl.vue` chooses native/Nuxt controls, with `DIYRangeField.vue`,
`DIYHeadersField.vue` and `DIYSocketsField.vue` for structured values.
`diyValidation.ts` and `diyStructuredValidation.ts` provide frontend Zod checks;
the backend repeats validation and checks generated Xray with the pinned parser.
`DIYCompatibility.vue` and `TuningCompatibilityList.vue` display shared server
applicability, host counts and documented client/platform restrictions.

`useDIY.test.ts` covers both draft sections through validation, same-key lost replies, revision conflicts and inheritance reset in hosted CI.
