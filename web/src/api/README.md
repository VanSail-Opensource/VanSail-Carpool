# Frontend API

`comboControls.ts` reads active ownership and enabled squads, obtains early
activation eligibility, and submits owner-scoped idempotent switch/activation
commands. Its mutable state and quotes always bypass response snapshots.

- `purchaseAddons.ts` owns protected active-ride squad-addition quote and idempotent commit requests.
- `community.ts` owns canonical community membership checks and one-space Telegram invite requests.
- `displayCurrency.ts` owns the member display-currency preference and server-managed CNY/USD conversion-rate requests.

- `adminBilling.ts` owns the atomic administrator update for global Add TXB bounds.
- `browserAccess.ts` owns browser identity, access-link exchange, renewal/logout, and one-use Mini App handoffs with separate signing cookies and scope.
- `adminOperations.ts` owns aggregate user profiles, facet filtering, coupon wallet actions, exact entitlement refunds, queued provider-account actions, transient connection scans, bulk extensions, operation resolution, and streamed backup upload.
- `client.ts` exposes typed member and administrator API operations, including idempotent payment, subscription-revoke, refund, outbox-retry, and manual-maintenance commands, connection scans, renewal batches, payment profiles, dashboard usage, and cached product statistics.
- `client.test.ts` covers client request behavior.
- `compensation.ts` owns revisioned configuration, cursor history, and idempotent approve/dismiss requests.
- `features.ts` exposes feature-specific endpoints and contract types, including durable Emby and questionnaire commands, the browser-safe payment-return receipt projection, and member coupon soft-discard.
- `features.test.ts` verifies feature request construction.
- `generated.ts` contains the compact generated OpenAPI contract.
- `http.ts` provides authenticated HTTP and error handling primitives and records eligible responses through the session cache.
- `cache/README.md` specifies session isolation, snapshot restoration, invalidation, and member/admin preload coverage.
- `memberOperations.ts` exposes generated-type connection scans, signed-handle blocks, active block listing/removal, the account-wide reset automation resource, reset/refund quotes, and durable operation receipts.
- `request-signing.ts` signs every protected request with Web Crypto or the audited pure-JS HMAC/SHA-256 fallback; it never sends an unsigned fallback.
- `request-signing.test.ts` covers signing, nonce behavior, and missing-Web-Crypto behavior.
- `types.ts` exports stable aliases over generated schema types, including payment-operation envelopes, connection scans, durable operation receipts, reset automation, statistics snapshots, member mutation quotes, and typed squad profile read/write unions.
- `catalog-normalization.ts` normalizes catalog and purchase-quote payloads by coercing missing or null `combos`, `addons`, `nodes`, `includedSquads`, and `accessibleNodes` values into empty arrays so downstream UI code can safely iterate them.
- `catalog-normalization.test.ts` verifies recursive catalog and quote normalization for nullable or omitted nested collections, ensuring null API fields are converted into safe empty arrays.

`preferences.ts` provides typed account preference and live group-tag clients.

`client.ts` verifies first-entry tokens via the signed /me/captcha endpoint. AuthState carries optional server-owned CAPTCHA state; caches and background preload suspend protected reads until verification.

`pm.ts` exposes typed admin moderation, topic repair and reference-only delivery reads, using existing signed requests and idempotency headers.

`connectivity.ts` uses generated OpenAPI types for administrator-only snapshots, exact-account resolution, asynchronous manual checks and cursor history. Read requests bypass browser caching; configuration writes serialize one credential-free `connectivity.config` value through the existing settings endpoint.

`subscription.ts` exposes live authenticated member uptime and native import links. These endpoints are excluded from persistent rendering snapshots; neither test-account credentials nor raw resolved configurations are returned.
- `ipLookup.ts` provides generated typed IP Lookup clients with fresh capability/receipt reads and quote-only cached report previews. Submission explicitly selects the signed action fields and excludes preview metadata; `ipLookup.test.ts` covers this strict transport shape for hosted CI.

- `ipLookupStream.ts` decodes bounded UTF-8 SSE frames through the ordinary signed request transport, supports JSON fallback and requires explicit completion.
- `ipLookupStream.test.ts` covers split Unicode frames, refresh, JSON fallback, address mismatch and truncated streams in hosted CI.

- `usdt.ts` ? Typed dedicated admin endpoints using the existing signed request client.

`xnode.ts` exposes typed node administration and live metrics with nonpersistent secret/realtime reads.

- `xnodeMember.ts` provides typed member metrics, routing and subscription APIs plus cookie-free public bearer metadata reads.

`adminAPITokens.ts` provides typed no-store browser management of administrator API credentials.

- `openapiContract.test.ts` compile-checks reset automation, required squad-node groups, nullable provider fallback and predicted-rollover ownership.

`firewall.ts` owns typed shared-ban reads and idempotent paid exact-IP exemptions. It preserves browser/member session separation through the existing request client.
