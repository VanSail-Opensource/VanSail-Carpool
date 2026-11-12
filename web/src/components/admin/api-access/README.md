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

The inventory now opens creation in a focused modal. Administrative, node and migration permissions use localized checkbox groups; one-time secrets clear at session logout as well as disposal/page hide.

`APITokenForm.test.ts` guards unique native checkbox labels and cross-group permission retention using real Nuxt UI components. The shared validation field has no outer control label; every permission has an explicit unique control ID.

Successful revocation immediately removes the row and clears its one-time secret. Invalidated list requests cannot restore deleted credentials; a failed refresh retains the filtered list.

Read only and Full administration presets explicitly select both independent administration/node scope families. Initial selection remains only `admin:read`. Labels explain that each write scope includes reads only for its own resource and database/backup tools require both families. `APITokenForm.test.ts` verifies the exact preset arrays.
