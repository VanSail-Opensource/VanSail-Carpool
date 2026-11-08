# Product retirement

Retirement removes current producers, execution, API contracts, localization and navigation together. Immutable migration definitions, purchased terms, historical audit facts and financial/accounting evidence remain intact.

## Outage Compensation, old usage bar and node bandwidth

- The outage observation service, scheduler, review APIs and browser console section are removed. The existing generic administrator worker may finish previously committed entitlement synchronization; it cannot create new outage decisions.
- The notification worker acknowledges old outage payloads without sending them. Historical approved extensions remain attached to their purchases.
- `UsagePanel` retains its quota meter, exact used/remaining bytes and cached-data warning. The old per-node bar and reset timestamp are removed. Usage Breakdown owns detailed node traffic.
- Node settings no longer accept, persist or compile per-node bandwidth overrides. Historical encrypted settings ignore that field on decode and omit it on the next save. Native user policy support is retained for commerce.
- Shared catalog inbound IDs preserve their exact deterministic UUID derivation after the outage adapter is removed.

The HTTP route regression rejects all retired outage endpoints. Hosted retention checks verify historical extensions and their committed synchronization remain valid. The native settings regression verifies retired bandwidth data cannot reappear.

Validation: Go vet, frontend typecheck, ESLint, structure/localization audit and production Vite build. Chrome DevTools checked console navigation and node editing at desktop/mobile sizes, successful node save, and quota loading/populated/empty/error/retry states with cached data. No local automated suites were run.

## Remaining retirement work

The broader retirement task still includes IP sources/probes and subnet sharing, PERSONAL pricing, discretionary traffic resets and cascading node removal. These are delivered as separately audited changes because they have distinct accounting and lifecycle dependencies.
