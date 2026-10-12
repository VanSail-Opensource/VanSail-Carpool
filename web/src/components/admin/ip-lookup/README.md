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

`ProxySourceTestControl.vue` and `useProxySourceTest.ts` start full tests of saved sources and poll typed no-store diagnostics every two seconds while visible. Disposal aborts polling without cancelling backend collection. A last-saved source/resolver signature disables Test for unsaved edits. Disabled saved sources can still be tested. Results reuse the expanded IPProxySourceDetails renderer.

Chrome DevTools MCP reviewed saved disabled-source tests, in-progress/completed/partial/error diagnostics, unsaved-edit disabling and save re-enabling at 390px and 1280px. Tests did not submit the settings form. Console checks were clear in the reviewed constructed flows.
