# IP Lookup
- `useAdminIPLookup.ts` loads and saves atomic admin settings without retaining saved credentials.
- `AdminIPLookupPage.vue` composes the standalone settings form, validation and save feedback.
- `ProviderSettings.vue` edits provider switches, MaxMind account ID and write-only credential changes.
- `ProviderOrder.vue` supplies accessible 44px up/down controls for two independent lists: the provider execution sequence and geolocation priority.
- `ComboQuotas.vue` edits the next-term included quota for each core combo.
- `RetrySettings.vue` edits the shared retry switch, attempt limit and delay through the existing settings model, localized controls and bounded numeric inputs.

The feature uses Vue Composition API, existing dark-theme controls and English/Chinese localization. Route views only compose feature components. Async results are scoped to the current member, and scope disposal cancels future polling and quote updates. Browser fixtures exercise these real components through Chrome DevTools MCP. No local automated test suite is required or permitted.

Provider array order defines queue execution order. Geolocation priority defaults to IP2Location, IPAPI, MaxMind, Scamalytics, AbuseIPDB and resolves comparable quality ties or missing quality metadata. Reordering preserves each provider's enabled flag, account ID and credential edit by stable provider ID. Saving uses the existing atomic settings endpoint; each run retains its original configuration snapshot.

Chrome DevTools MCP reviewed the real form at 390px and 1280px in English and
Chinese on 2026-10-10. A constructed save retained independent reordered lists,
2.50/3.50 TXB fees, two/zero quotas, account fields and blank credential edits.
All ordering controls measured 44px; no overflow or console errors appeared.
These are browser/constructed-handler results, not a production settings write.
ProviderSettings.vue also manages the optional vaulted cloudflare_radar token with configured/keep/replace/clear behavior, independent of the risk-provider order.

`ProxyProbeSettings.vue` edits source names/URLs, resolver addresses, additional ports and the quota cost of refreshes inside the existing atomic settings form. Server defaults include all supplied inventories; credentials remain write-only.

`ProxySourceTestControl.vue` presents cache status/counts and Refresh cache or Retry for one saved source. `useProxySources.ts` owns one no-store polling stream for every row (two seconds during collection, fifteen when idle), rejects late account results and aborts reads on disposal. Unsaved source/resolver edits disable refresh. Disabled saved sources can be refreshed manually, but never contribute to the active inventory. `useProxySources.test.ts` covers selected-source admission, startup joins, retained counts after errors and account/disposal boundaries in hosted CI.

Chrome DevTools MCP reviewed saved disabled-source tests, in-progress/completed/partial/error diagnostics, unsaved-edit disabling and save re-enabling at 390px and 1280px. Tests did not submit the settings form. Console checks were clear in the reviewed constructed flows.

The revised shared-cache controls were reviewed through Chrome DevTools MCP at 390px and 1280px on 2026-10-12: startup/in-flight/completed/error states, unique counts, Refresh/Retry, unsaved-edit disabling and save recovery, retained cached counts after failure, and UTF-8 Chinese labels. Constructed refresh/save handlers made no paid check or quote; reviewed console results were clear.

Source categories use the existing USelect/UFormField and Zod settings validation. Pool, database and abuse labels are localized; missing legacy categories normalize to Proxy Pool. Category/name edits reuse source caches. Inventory totals label canonical IP/CIDR entries without expanding networks. Mobile source rows stack fields; desktop rows retain compact aligned controls.

Chrome DevTools MCP reviewed this change on 2026-10-12 using constructed handlers: 320/390px mobile and 1280px desktop, category selection/save/readback, source warming/partial/status-error, loading/empty/error views, matched category reasons, selected neighbor attribution, and English/Chinese labels. Reviewed layouts had no horizontal overflow, console warnings/errors or observed failed requests. Backend tests and memory benchmarks run only in hosted CI.

The browser admin page now separates general settings, provider credentials,
proxy inventories/probes, and combo quotas into desktop tabs. Source controls and
combo quotas use editable Nuxt tables with contained mobile scrolling. Saving
admin settings does not refresh the Mini App availability endpoint. Atomic writes,
write-only credentials, source-cache status polling, and billing policies remain
owned by the existing APIs and composables.
