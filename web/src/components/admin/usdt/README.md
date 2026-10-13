# USDT administration

`AdminUSDTPage` is composed by the existing admin shell at `/admin/usdt`. It uses
the shared API client, Nuxt UI forms, theme tokens and English/Chinese localization.
One network is edited at a time; quota limits live in a native disclosure. Credentials
are write-only and remain blank after saving. Endpoint order expresses failover
priority. Group IDs bind servers sharing one provider-account budget.

Status and review reads never trigger chain work. Explicit probes use the backend
queues. Credit resolution consumes persisted verified evidence once and can only
override the original payment window, not token/address/amount identity. Amount
mismatches use existing manual adjustments. The payment sheet and statistics chart
remain shared billing components. Composables guard stale responses after unmount.

## Files

- `AdminUSDTPage.vue` ? Thin page composition, network selection, readiness and form wiring.
- `configForm.test.ts` ? Hosted contract checks for masked credentials, protocols and quotas.
- `configForm.ts` ? Typed defaults and localized Zod validation using existing UUID utilities.
- `USDTEndpointEditor.vue` ? Write-only URLs/credentials, shared group and WS configuration.
- `USDTGeneralSettings.vue` ? Provider enablement, exact TXB/USDT rate and payment window.
- `USDTNetworkSettings.vue` ? Receiving address pools and ordered server entries.
- `USDTQuotaSettings.vue` ? Provider-account limits and local usage/cooldown visibility.
- `USDTReview.vue` ? Paginated verified transfer review list.
- `USDTReviewItem.vue` ? Transfer evidence and explicit audited order-credit input.
- `useUSDTAdmin.ts` ? Independent loading, saves, probes, review and lifecycle cleanup.
