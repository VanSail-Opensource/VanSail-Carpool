# Frontend availability, efficiency and interaction audit

Date: 2026-11-13. Baseline: merged Carpool main at
`985958f9bea54bd6145f27dd0f5b09ab0841609e`.

This is the frontend portion of the coordinated Carpool/X-Node audit. Native
queue execution, database transactions, agent restart behavior and releases are
covered by the backend/agent audit. Findings below distinguish implemented UI
changes from backend guarantees and constructed browser evidence.

## Findings and corrections

| Area | Finding | Correction and evidence |
| --- | --- | --- |
| Feature availability; failure isolation | All ten X-Node tabs depended on one six-request `Promise.all`; unrelated group/snippet failures or slow metrics disabled node recovery and independent global settings. | `workspaceTabs.ts` declares exact dependencies. `useXNodeWorkspace.ts` isolates optional metrics and waits only for required current-tab inventory. Successful resources survive partial failure; a keyed per-tab loading boundary prevents stale readiness. |
| Queue/failure UX | An unavailable metrics read was indistinguishable from an offline node; a connected runtime not ready was also labelled offline. | `nodeStatus.ts` distinguishes unavailable, stale, offline, runtime-not-ready, unsynchronized and online evidence. Recovery controls stay available during telemetry failure. Pending removal continues to expose its durable blocker/receipt workflow. |
| Database/read pressure; protocol efficiency | Workspace visited irrelevant endpoints on every refresh and polled metrics on unrelated tabs. Checkout and receipt observers kept reading from hidden documents at normal cadence during outages. | Workspace inventories are scoped by tab. Metrics poll only on visible Nodes/Metrics tabs, without overlap, and back off 5→10→20→40→60 seconds. Hidden payment/receipt polls pause; checkout failures back off 2→4→8→16→30 seconds. Foreground resumes the original read, with terminal/disposal fences. |
| Performance | Every node render repeatedly scanned the fleet metric array. | One computed metric map replaces repeated searches. Existing lazy expanded metric sections, bounded history and virtual tables remain in use. |
| Layout | Four action buttons competed with unrestricted node identities/status in one mobile grid row. | Long identities wrap, status labels can wrap, and mobile actions occupy a separate row. Native button target sizes remain unchanged. |
| Counter-intuitive navigation | Only the Global tab could be restored from the URL; selecting other tabs was lost on reload. | All known workspace tabs bind to `xnodeTab`, preserving other query parameters. Unknown values select Nodes. |
| Input/domain validation | DDNS accepted jobs without an address source, invalid/wrong-family fixed addresses, invalid auto discovery and unbounded TTL values. Relative/out-of-zone record names saved despite native runtime refusal. | `ddnsValidation.ts` mirrors native family/unicast, interface, HTTPS lookup, interval and TTL constraints, including the 2147483647-second TTL cap. It supports apex, exact owned names, IDNA, service labels, leading wildcard, Unicode dot separators and trailing dot. Field errors explain rejection. Native validation remains authoritative. |
| Feature availability | Providers with unsupported, failed or empty zone discovery had no way to enter a new DNS zone. | `XNodeDDNSZone.vue` falls back to direct zone entry while retaining discovered-list selection and retry. No extra provider call is made by validation. |
| Business/input boundary | Node edits spread server response properties back into writes; the API also retained a permissively ignored multiplier field. | Node editor and `XNodeNodeWrite` now use exactly six editable fields. Agent bindings, revisions, timestamps/removal state and obsolete multiplier input are rejected by the coordinated HTTP change. |
| Business/retry identity | New node, group and endpoint saves sent an empty ID, so retrying a committed creation after a lost response could create a second resource. | Every creation editor retains one generated ID for the existing same-ID PUT upsert. The visible draft stays in creation mode; existing-node actions and endpoint preview semantics remain unchanged. Hosted coverage verifies one stored resource across response-loss retries and a fresh identity for the next editor. |
| Business/revision recovery | Configuration/snippet name uniqueness already prevented identical-name duplicates, but an uncertain create followed by a renamed retry could target a new resource; an unchanged retry could not confirm the original success. | These editors retain a creation ID and revision 0. After an uncertain response or conflict, one fresh collection read must match that ID, exact name/document and ordered snippet IDs or snippet kind before reporting success. Changed saved state remains a conflict; revisions are never advanced automatically and existing edits keep optimistic concurrency. |
| Retired code | A standalone entitlement panel remained reachable only from an audit fixture; an implicit job retry-key map retained a fallback contract no production caller needed. | Removed that panel/import/client/locale/CSS fixture path and the fallback map. Current user-profile entitlement operations remain. Job retries require the caller's explicit durable command key. |
| Business/settings safety | Failed initial IP Lookup settings reads enabled Save for empty defaults; retained Hourly forms allowed writes during a refresh that could later replace the confirmed UI. | IP Lookup requires successful settings/catalog readiness and exposes retry; Hourly required reads/writes exclude each other and retire disposed replies. Hosted unit/browser cases cover recovery. |
| Counter-intuitive success feedback | Native node modal close invalidated `loaded` while its exit transition still displayed content, flashing a false load failure after a successful save. | Closing retires the request without resetting already-loaded content; the next edit initializes its own readiness. Reproduced and verified through the shared browser; hosted transition observation guards regression. |
| Retired DNS API | The redirect-only editor still echoed obsolete resolver mode/upstream values and public migration state. | Public DNS accepts only `redirect`. Old resolver fields and public `legacyDnsLoopback` were removed from schemas, types and fixtures. The ignored active-IP telemetry opt-out and its forced normalization are removed. Singular DDNS binding aliases are also removed; only explicit plural host lists are writable. The backend privately migrates existing encrypted fallback, keeping effective resolver availability without accepting old new-write contracts. |

## Coverage inventory

The review followed production entry points and their request/validation paths,
not fixture reachability alone.

| Requested area | Frontend review boundary and conclusion |
| --- | --- |
| Queue failure and high pressure | `useOperationReceipt`, `useDurableCommand`, payment operations, node commands/removal, certificate lifecycle, backup retry. Read retries preserve the same accepted operation; no UI polling path creates a replacement provider task. Failure/backoff and visibility gaps corrected above. Worker scheduling is in the backend audit. |
| Feature availability | Browser/Mini App route tables, console navigation, session gates, workspace dependencies, DDNS provider/zone discovery, endpoint preview, certificate lifecycle, global settings and subscription templates. Optional telemetry and provider discovery no longer unnecessarily block the identified configuration flows. |
| Failed X-Node tasks | Node commands and removal retain queued/processing/review/failed outcomes; unavailable/not-ready observations no longer masquerade as offline. Native independence of failed tasks is assessed by the agent audit. |
| Database writes/reads | Inspected API transport/cache/session boundaries, page loaders and all polling declarations in composables/admin components. Changes reduce avoidable inventory reads, hidden polling and outage retries. No new backend persistence or write-through cache introduced. |
| Unnecessary storage | Live metric maps, snapshots, DDNS discovery and polling state remain memory-only. Existing bounded metric buffers and response-cache exclusions preserve sensitive ownership boundaries. Removed the unused retry-key map; retained durable command recovery context contains no certificate keys/PEM. |
| Master outage/restart | UI separates unavailable telemetry from configuration/recovery and preserves accepted receipts. Existing session/owner generation fences and component disposal retire obsolete responses. Agent execution/config retention is assessed separately. |
| Protocol efficiency | Current-tab resource list, summary metrics, no overlapping polls, exact editable NodeWrite DTO. Existing debounced endpoint preview/native validation, lazy metrics details and selected-node logs retained. |
| Input validation/domain coverage | DDNS domain/address/TTL/source checks; node/DNS write schema retirement; reviewed certificate hostname/PEM/file validation, endpoint typed previews, firewall port/range/family checks, exact payment amount/quote boundaries and admin grants. Server validation remains authoritative for business decisions. |
| Layout | Shared admin loading boundary and long node identities/actions corrected; inspected established member/admin root separation, mobile form columns, contained tab scrolling and bounded metric tables. New desktop/mobile fixture cases cover changed flows. |
| Counter-intuitive UX | Correct status evidence, retained tab URL, direct zone entry, visible validation errors, retained failed-save drafts, read failure separate from accepted-write success. |
| Performance/efficiency | Removed repeated node-to-metric linear searches, unrelated inventory fetches, unused metric polling and hidden status polling. No performance benchmark claim; request reductions are derived directly from the dependency list. |
| Business loopholes | Confirmed frontend status is advisory, quote/action identity remains server-owned and explicit retry keys are used. Narrow node writes reject server-owned inputs; node/group/endpoint creation retries retain the same upsert identity. Configuration/snippet recovery compares the same stable ID without bypassing revisions; named-service uniqueness remains authoritative. Current grant/refund/entitlement and accounting flows retained. Backend audit owns accounting/admission enforcement. |
| Retired/backward code | Removed only proven unused UI/client paths and coordinated live node/DNS contracts. The unused database PUT update alias was removed by the coordinated backend change and generated contracts updated. Active certificate/provider and geodata conversion state, singleton firewall ports and financial/service history remain where still needed by current native/persisted state; they are not claimed as dead code. |

Representative reviewed modules include `src/browser/{router,session,main,
BrowserApp,BrowserConsole,navigation}`, `src/router`, API cache/preload/transport,
`useStatistics`, `useActivity`, `useCommunityAccess`, `useSubscriptionData`, payment
and purchase composables, pricing datasets/workspace, administrator hourly,
activity, coupons, user profiles, and the X-Node configuration, global, service,
certificate, endpoint, firewall, lease, metrics and command modules.

## Validation

- Repository policy prohibits local automated suites. None were run.
- Final `npm run generate:api`, `npm run lint` (zero warnings), and
  `npm run build` (Vue typecheck followed by Vite production build) passed on
  the completed frontend/schema tree, including all creation-retry corrections.
  The final Vite build took 8.50 seconds;
  existing large chart/map/graph chunks still trigger its size warning.
  `git diff --check` passed. Earlier typecheck attempts were killed with exit
  137 during shared resource pressure; the final sequential gate completed
  successfully after disposable tool caches were relocated.
- Added hosted unit coverage for optional/required dependency failures, delayed
  metric requests, visibility/backoff, stale replies, teardown, exact writable
  node/DNS fields, DNS/address boundaries and immutable node saves.
- Added hosted `browser-xnode-availability.mjs` desktop/mobile cases for metrics
  failure and successful edit, independent tab navigation/reload, stale status,
  loading/empty state, direct zone entry and bounded TTL feedback.
- Added hosted configuration/snippet creation cases for a committed write with a
  lost response and unavailable read, changed-draft conflict, exact-content
  recovery, one retained resource and a fresh identity for the next editor.
- Shared-desktop Chrome inspection verified desktop unavailable-metrics warning,
  long-name wrapping/actions, scoped group failure, independent Global settings
  and URL reload, loading state, direct DNS-zone entry, FQDN guidance and TTL
  rejection/recovery. It reproduced the successful-save close flash and verified
  its correction. Additional desktop checks confirmed Online becomes Not current
  after a failed metrics poll, failed node saves retain the edited name, failed
  node reads expose retry without a writable form, and empty inventory explains
  why Add node is disabled. IP Lookup's failed initial read leaves Save and fields
  disabled with retry available. Hourly refresh retains its populated forms with
  writes disabled and a read-loading indicator. Chrome policy blocks
  DevTools/mobile emulation; the existing
  hosted browser gate exercises both 1440×1000 and 390×844 and retains screenshots.
  Record the final hosted CI/mobile artifact outcome before declaring delivery
  complete. No live agents, provider issuance or payments were exercised.

### Final creation-recovery browser check

Manual desktop inspection of the constructed configuration fixture reproduced a
committed save with a lost response and failed recovery read. The editor retained
its original draft and error. A renamed retry retained the changed draft and
reported the revision conflict. Restoring the submitted name reconciled the exact
saved resource, closed the dialog without a false error, and displayed exactly
one new revision-1 configuration. The hosted browser gate additionally covers
snippet recovery and both desktop/mobile screenshots.
