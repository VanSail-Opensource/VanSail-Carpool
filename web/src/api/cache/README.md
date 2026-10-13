# Session response cache

The HTTP client remains the network entry point. Every request still resolves with
a fresh backend response or rejects normally. Rendering loaders explicitly restore
a previous response before starting that request; Vue replaces the displayed data
when it completes, without navigation or a document reload.

## Modules

- `session.ts` owns experience/account/role/onboarding-scoped, in-memory JSON snapshots, isolated copies, an 8 MiB / 256-entry LRU bound, and invalidation generations.
- `policy.ts` separates read requests from mutations and builds method/URL/query/body-specific keys. Authentication, payment capabilities, action quotes, keys, and operation/connection polling are never cached.
- `transport.ts` writes successful responses immediately, shares identical unsignalled reads in flight, rejects older writes, invalidates on mutations/auth failures, and evicts forbidden or removed resources.
- `restore.ts` exposes typed snapshot-to-state helpers. They never skip the loader's normal backend request or authorize actions.
- `restore.test.ts` covers stale nested shapes, compatible empty/additive responses, and contained projection errors. Snapshot restoration validates the current OpenAPI shape before mutating state and discards incompatible entries without blocking the live request.
- `catalogOptions.ts` restores the two catalog resources used by admin filters and account actions.
- `drafts.ts` merges refreshed form fields without replacing edits made while the response was pending.
- `drafts.test.ts` covers refreshes that arrive while an administrator edits cached fields.
- `preload.ts` schedules five background reads at a time after authentication, contains individual failures, aborts on session changes, and limits each read to 20 seconds.
- `preloadRoutes.ts` inventories only initial member data, including default queries. Incomplete onboarding preloads its content; no administrator data is loaded into the Mini App.
- `session.test.ts` covers isolation, copy ownership, bounded retention, and invalidation.
- `transport.test.ts` covers fresh responses, overlapping reads, mutations, errors, and excluded endpoints.
- `preloadRoutes.test.ts` covers member coverage and the onboarding boundary.
- `preload.test.ts` covers member-only preloading, partial failures, concurrency and session cancellation.

## Coverage and boundaries

Member preload covers Home and its default node-usage range, catalog and balance,
statistics, activity, affiliates and referral page 1, coupons, questionnaires,
Emby, abuse records, IP blocks, reset automation, community membership, and localized
onboarding content. The browser console loads each section on demand and never
runs the member preload. Filters, pagination, details and administrator resources
are cached on use with their exact resource keys and a browser-scoped identity.

Preloading does not execute purchases, create payments, start connection scan jobs,
download backups, or enumerate every database row/account. Their existing action
and polling flows remain responsible for fresh authority and durable operations.
No data is persisted in localStorage, sessionStorage, Telegram storage, or a database.
Document reload, logout, account/role/onboarding changes discard all snapshots.
The two experiences retain independent navigation and authentication while sharing
the backend request queue and typed, live API authority.

- `policy.ts` excludes activity responses from persistent snapshots because boost eligibility must be obtained live.
- `policy.test.ts` verifies live activity responses are excluded while ordinary reads retain snapshots.
