# Administrator API access

`AdminAPITokens.vue` composes the browser-only management page and its metadata
table. `APITokenForm.vue` validates names, permission presets and absolute expiry
with Zod and Nuxt UI. `APITokenSecret.vue` provides the one-time secret/copy flow.
`useAdminAPITokens.ts` owns requests, stale-response guards, revocation and
in-memory secret disposal on navigation/pagehide.

`api/adminAPITokens.ts` is the typed no-store client. Locale copy is in
`api-tokens.json`. The Mini App has no route/preload for this page, and these
components do not initialize Telegram controls. There is no persisted response
cache or local-storage credential state.

`useAdminAPITokens.test.ts` covers secret disposal, late responses and stale metadata in hosted CI.
