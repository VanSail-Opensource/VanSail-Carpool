# Stablecoin administration

`AdminUSDTPage` is composed by the independent browser console at `/admin/usdt`. It uses
the shared API client, Nuxt UI forms, theme tokens and English/Chinese localization.
Configuration, network, and transfer review workspaces use desktop tabs.
One network is edited at a time; receiving addresses and ordered endpoints remain
editable through typed controls. Transfer evidence opens in a focused side panel. Credentials
are write-only and remain blank after saving. Endpoint order expresses failover
priority. Every RPC and WS server shares the concurrency limit in general settings.

Status and review reads never trigger chain work. Explicit probes use the backend
queues. Credit resolution consumes persisted verified evidence once. Auto-credit
requires an exact amount or one unique quote with a shortfall of at most 0.001 of the same token;
ambiguous matches remain under review. It can
override the original payment window but not the token or address. The payment sheet and statistics chart
remain shared billing components. Composables guard stale responses after unmount.

## Files

- `AdminUSDTPage.vue` ? Thin page composition, network selection, readiness and form wiring.
- `configForm.test.ts` ? Hosted contract checks for masked credentials, protocols and concurrency bounds.
- `configForm.ts` ? Typed defaults and localized Zod validation using existing UUID utilities.
- `USDTEndpointEditor.vue` ? Write-only URLs/credentials and WS configuration.
- `USDTGeneralSettings.vue` ? Provider enablement, exact TXB/USDT rate, payment window and shared concurrency.
- `USDTNetworkSettings.vue` ? Receiving address pools and ordered server entries.
- `USDTReview.vue` ? Paginated verified transfer review table and evidence side panel.
- `USDTReviewItem.vue` ? Verified evidence and permanent audited dismissal input.
- `useUSDTAdmin.ts` ? Independent loading, saves, probes, review and lifecycle cleanup.

Command failures remain visible beside transfer-credit inputs in the evidence
side panel. Browser administration does not load Telegram controls or haptics.

The single token-mode selector defaults to USDT. USDC is hidden on TRON/BSC/TON.
Readiness and probes show one token rail at a time. Evidence settlement lives only
in the profile payment-credit popup; the review workspace exposes permanent dismissal.
Loads abort on replacement/disposal, generations guard responses, and failed loads
disable Save until retry succeeds. All transport retains shared request signing.

`useUSDTAdmin.test.ts` covers aborted/superseded loads, disabled writes after failed loads, successful retry and disposal guards in hosted CI.
