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

The broader retirement task still includes PERSONAL pricing and discretionary traffic resets. These are delivered as separately audited changes because they have distinct accounting and lifecycle dependencies.

## IP sources, probes and subnet sharing

The optional inventory cache and download/DNS workers, source diagnostics, active port/protocol scan execution, neighbor scan and AbuseIPDB block check are removed. Member and administrator UI, routes and OpenAPI no longer expose these controls or scan fields. The disposable cache configuration is retired.

Reputation providers and queued RPKI/BGP/ASN/PTR/InternetDB/transfer enrichment remain. Read-only interrupted enrichment resumes using the legacy checkpoint key. Existing stored reports and frozen verdicts are historical facts; their public projection omits retired scan fields. Cache reads and refresh replacement now use the canonical exact IP, including old rows whose legacy cache key described a subnet. Receipt, allowance and monetary settlement remain unchanged.

The memory gate continues to stress USDT scanners, transfer proofs and cancellation under the original memory budget. Inventory-only benchmarks are retired with the implementation. Its test package is now `internal/usdt`.

IP retirement validation: Go vet, frontend typecheck/lint, structure/localization audit and Vite production build pass. Chrome DevTools inspected member/admin loading, unconfigured/empty, populated and error states; exact-IP preview; admin save; English/Chinese; desktop and 390px mobile. No reviewed console errors or API request failures. Hosted regressions remain the execution gate.

Hosted follow-up retains the original durable live-stage claim/attempt/snapshot recovery path for enrichment. Exact-address refresh checks reject neighbor-only evidence. The scanner memory gate joins mock HTTP handlers before retained-heap measurement; memory and leak thresholds are unchanged.

## Cascading node removal

The existing `node.remove` operation now enters a persistent admission fence. Its transaction prunes typed routing choices, group/service bindings, endpoint references, obsolete DDNS work and executable node customizations. Diagnostic credentials are revoked while their accounting references remain. Empty groups and empty routing plans are disabled. Existing purchases, PAYG credit, ledger entries, traffic and archived ciphertext remain intact.

Encrypted revocation evidence includes both saved and previously applying routes. Pending routing plans reject edits until every affected participant acknowledges removal of their private credentials. Only then does retirement discard that operational proof and revoke enrollment. Surviving choices reconcile through the existing master; existing DIY cleanup removes invalid client split references. No upstream call or second queue is introduced.

Schema 40 adds a null-default removal timestamp and extends binding/compile guards. An already disabled node still recompiles to stop its DDNS jobs. During a drain, old agent authentication permits reconnection for accounting; new enrollment, configuration changes and bindings are denied. Final retirement revokes the credential.

Offline participants and unfinished accounting keep the same queued receipt with a specific dependency. Attempts stay monotonic so an older callback cannot complete a recovered operation. Historical traffic, financial facts and uncertain native command receipts are preserved.

Node-removal validation: Go vet, typecheck, ESLint, structure/localization audit and production Vite build pass. Chrome DevTools exercised constructed bound-node removal through a succeeded receipt and refreshed list, queued accounting dependencies, pending controls, loading/empty/error/retry, English/Chinese, 390px mobile and 1440px desktop. No reviewed console or API-request failures. Hosted database/routing regressions validate runtime behavior; no local automated suites were run.

Traffic-mode account registration takes the same node admission lock before creating its period participation. This closes the separate registration race without preventing late traffic journal ingestion.

## PERSONAL dynamic pricing

Removed targeted campaign creation, policy controls, per-user model selection,
PAYG discount previews and new discount reservations. Public dynamic pricing and
ordinary coupons remain. Checkout derives public prices under its existing
wallet transaction and version binding; coupons apply once to the basket.
Dynamic renewals follow the public price within their purchased limits rather
than an owner's behavior or expected profitability. Existing fixed terms and
attached recurring coupons retain their settlement behavior.

Historical personal decisions, already consumed discounts, purchase receipts and
reserved renewal obligations are retained. No customer or purchase conversion is
performed. The deprecated decision receipt field remains wire-compatible and is
no longer displayed as a personal offer.

Validation for PERSONAL retirement: Go vet, frontend typecheck, ESLint,
structure/localization audit and production Vite build passed. Chrome DevTools
checked EN/ZH policy loading, populated, load-error and save-error states, ordinary
coupon estimates and empty member analysis, public checkout labels, keyboard
focus and the intervention menu, mobile 390px and desktop 1440px layouts. No
unexpected console or request failures were observed. Hosted regressions cover
retired policy flags, command rejection, coupon scope changes, purchase replay,
public renewal bounds and historical reserved-discount settlement.
