# Session response cache

The HTTP client remains the network entry point. Every request still resolves with
a fresh backend response or rejects normally. Rendering loaders explicitly restore
a previous response before starting that request; Vue replaces the displayed data
when it completes, without navigation or a document reload.

## Modules

- `session.ts` owns experience/account/role/onboarding-scoped, in-memory JSON snapshots, isolated copies, an 8 MiB / 256-entry LRU bound, write timestamps, and invalidation generations. Optional maximum ages expire display snapshots without extending freshness on reads.
- Its independent, reactive identity generation fences global HTTP authentication effects across account changes and lets active views synchronously retire displayed account data. Ordinary snapshot invalidation leaves this generation unchanged, so parallel writes cannot hide valid expiry notifications.
- Session changes clear old snapshots before notifying synchronous identity observers. Surviving shared display loaders retire their state immediately and only regain live readiness through a fresh request after the session view updates.
- `policy.ts` separates read requests from mutations and builds method/URL/query/body-specific keys. Authentication, payment capabilities, action quotes, keys, funding-sheet responses, and operation/connection polling are never cached.
- Explicit `cache: 'no-store'` also disables in-memory snapshots and in-flight response sharing, preserving the caller's live or sensitive response policy.
- `transport.ts` writes successful responses immediately, shares identical unsignalled reads in flight, rejects older writes, invalidates on mutations/auth failures, and evicts forbidden or removed resources. Dashboard snapshots redact `subscriptionUrl`; the fresh caller still receives the complete response.
- `restore.ts` exposes typed snapshot-to-state helpers. They never skip the loader's normal backend request or authorize actions.
- `restore.test.ts` covers stale nested shapes, compatible empty/additive responses, and contained projection errors. Snapshot restoration validates the current OpenAPI shape before mutating state and discards incompatible entries without blocking the live request.
- `catalogOptions.ts` restores the two catalog resources used by admin filters and account actions.
- `drafts.ts` merges refreshed form fields without replacing edits made while the response was pending.
- `drafts.test.ts` covers refreshes that arrive while an administrator edits cached fields.
- `preload.ts` schedules five background reads at a time after authentication, contains individual failures, aborts on session changes, and limits each read to 20 seconds.
- `preloadRoutes.ts` inventories only initial member data, including default queries. Incomplete onboarding preloads its content; no administrator data is loaded into the Mini App.
- `session.test.ts` covers isolation, copy ownership, bounded retention, invalidation, and snapshot clearing before identity observers run.
- `transport.test.ts` covers fresh responses, overlapping reads, mutations, errors, and excluded endpoints.
- `preloadRoutes.test.ts` covers member coverage and the onboarding boundary.
- `preload.test.ts` covers member-only preloading, partial failures, concurrency and session cancellation.

## Coverage and boundaries

Member preload prioritizes Home and its usage breakdown, Explore's traffic and hourly views,
and DIY while route chunks load, then warms Activity, member Settings, affiliates,
Emby and community. Settings reads include preferences, member tags and internal
squad controls. Only incomplete onboarding preloads its localized content. Retired
node-usage reads and live connection metrics are absent. The browser console loads each section on demand and never
runs the member preload. Filters, pagination, details and administrator resources
are cached on use with their exact resource keys and a browser-scoped identity.
Home, Activity, Settings, Explore and DIY opt into 60-second rendering snapshots. Their GETs
use `no-cache` to revalidate HTTP responses while allowing the document's memory
cache; each entry still performs a fresh read. Activity actions, Settings mutations, hourly controls and DIY Save remain disabled
until a live response succeeds. Cached preferences do not grant optional navigation
eligibility; failed live checks remove that authority. Usage breakdown snapshots
must pass both response-shape and exact numeric accounting validation before rendering. DIY merges untouched
snapshot fields while preserving edits made during refresh. Read-only DIY and
routing previews do not invalidate unrelated display snapshots.
Home gets entitlement labels in its dashboard response and node
metadata from its usage summary rather than fetching catalog.

Preloading does not execute purchases, create payments, start connection scan jobs,
download backups, or enumerate every database row/account. Their existing action
and polling flows remain responsible for fresh authority and durable operations.
No data is persisted in localStorage, sessionStorage, Telegram storage, or a database.
Document reload, logout, account/role/onboarding changes discard all snapshots.
The two experiences retain independent navigation and authentication while sharing
the backend request queue and typed, live API authority.

`router/memberViews.ts` shares exact lazy loaders with navigation and warms two
member chunks per idle slice. Data Saver/2G skip this optional code preloading;
logout, captcha and session errors cancel scheduled work. No route is mounted by
preloading. Administrator components and subscription credential endpoints are
excluded; dashboard rendering snapshots redact their legacy subscription URL.

- `policy.test.ts` verifies display-only member snapshots, explicit no-store, and excluded credentials. Activity eligibility is always revalidated before actions.
- Funding-sheet `/balance` responses stay uncached because they can contain pending payment capabilities; Home balance is already present in the redacted dashboard snapshot.
- Home's statically imported components are warmed with its shared route loader. Activity and Settings also join the idle code queue. No component is mounted and no modal is opened by code preloading.
