# VanSail Carpool and X-Node repair delivery

This checklist tracks the October repair plan across both repositories. Existing
commits and uncommitted purchase work are included only after review. Tasks are
handled in order; implementation review, browser evidence and hosted regression
evidence are recorded separately. Production fleet upgrades are out of scope.

## Ordered work

| # | Requirement | Status |
| --- | --- | --- |
| 1 | Bounded updater locks, asynchronous restart and durable reconnect reconciliation | Reviewed; compatibility gap repaired; static vet passed; hosted pending |
| 2 | Managed geosite preparation on validation, activation, reload and recovery | Reviewed existing implementation; static vet passed; hosted pending |
| 3 | Native domain/category/attribute validation and localized country diagnostics | Reviewed; native semantics and typed errors repaired; vet/typecheck and mobile/desktop browser checks passed |
| 4 | Mode-specific node-file certificate contracts and form | Existing contracts/form reviewed; desktop/mobile mode/path/draft/empty/loading checks passed |
| 5 | Outage-based signed-upgrade mitigation runbook | Reviewed X-Node docs/upgrade-recovery.md against outage and CLI; production execution remains separately scheduled |
| 6 | Streamed disk-backed source ranges and bounded kernel population | Existing fixed-width range files, bounded sort/merge and 512-element kernel transactions reviewed; Linux vet passed; hosted pending |
| 7 | Independent source/dynamic staging and versioned atomic publication | Reviewed; dynamic replay no longer starts bulk preparation; Linux vet passed; hosted regression added |
| 8 | Priority/reserved dynamic delivery, reconnect wakeups and job coalescing | Existing priority/capacity/reconnect/coalescing reviewed; added reserved one-second dispatch using the same outbox; vet passed; hosted timing pending |
| 9 | Fleet-wide administrative IP/CIDR exemptions and durable receipts | Existing admin API, idempotency, expiry, capability and paid-grant isolation reviewed; mobile error/empty and desktop receipt flows verified |
| 10 | Bounded stage/generation/certificate/download/job cleanup | Added bounded abandoned-stage/download/certificate cleanup; protected partial runtime activation and rollback dependencies; Linux vet passed; hosted regressions added |
| 11 | Million-prefix RSS and firewall storage admission limits | Corrected free-space checks to the actual bulk filesystem; explicitly enabled hosted million-prefix RSS/native-ban timing and low-space/quota recovery cases; hosted results pending |
| 12 | Native connection scan lifecycle and typed unavailable-node errors | Existing nonce/native-identity repair and ephemeral projection reviewed; vet and desktop/mobile loading/empty/unavailable/error browser checks passed |
| 13 | Pinned command audit, outcomes, delivery deadlines and recovery access | Reviewed command/deadline/reconnect contracts; repaired pending-upgrade capability and corrupt-status fences; Linux vet and outcome browser checks passed |
| 14 | Purchase-owned rollover/reset/provider-operation reconciliation | Reviewed and included existing uncommitted owner/purchase reset/refund work; durable reset proof and uncertain retry safeguards retained; vet/contracts passed; hosted regressions pending |
| 15 | Native Add Squad and squad switching with existing commerce safeguards | Existing native ownership/charging/replay repair reviewed; vet and mobile switch/last-enabled/lost-reply addition plus desktop inactive/loading browser checks passed |
| 16 | Fresh native member activity and unavailable stale telemetry | Existing freshness/capability implementation reviewed; desktop unavailable and mobile offline/online-to-expired browser checks passed |
| 17 | Native connectivity monitoring, traffic aggregation and accounting-safe pruning | Existing native monitoring resolution/display reviewed; added indexed bounded aggregation and safe maintenance fences/pruning; vet and monitor-disabled/admin browser checks passed; hosted pending |
| 18 | Shared subscription metadata/preview/download rendering and native formats | Shared compatibility filtering/diagnostics implemented; Clash/Mihomo and Stash include VLESS XHTTP Reality; unsupported/failed host projections are omitted individually; contracts/vet/typecheck/browser passed; hosted pending |
| 19 | Explicit formats, ordered User-Agent rules, detection and fallback | Shared selection renderer added to previews; existing precedence reviewed; Go vet and browser regex/explicit/built-in/fallback/invalid-regex checks passed |
| 20 | Authorized inbound/outbound endpoint IDs and legacy route resolution | Endpoint IDs/owner scope/listener-aware proof and compilation implemented; legacy documents migrate on save; contracts/vet/typecheck and retained-host browser save verified; hosted pending |
| 21 | Selected-node route defaults, host retention and participating graph nodes | Selected node/host defaults and participant-only graph implemented; repaired nested reactive draft cloning; browser confirmed added Singapore alternate host persists in save payload without console errors |
| 22 | Readable ordered route paths, hop hosts, validation barriers and drafts | You/node/host/selector paths implemented; fixed long-label mobile grid overflow; ordering/barriers/drafts retained; typecheck/lint and desktop/mobile populated/empty/loading/error browser checks passed |
| 23 | Per-network USDT/USDC switches with outstanding-quote continuity | Explicit switches, legacy inheritance and empty-list authority implemented; issued quotes stay watched; vet/typecheck/lint and desktop/mobile signed-save payload checks passed; hosted pending |
| 24 | Exact-first settlement, tolerance/review and shared RPC concurrency | Exact-first match, 0.001 shortfall tolerance, ambiguity review and one shared queued RPC limit reviewed; hosted boundary fixtures retained; hosted pending |
| 25 | Distribution ring geometry, semantics and reduced motion | Concentric rings now bind motion geometry instead of leaking animation props, share one payment denominator, and keep final geometry with opacity-only reduced motion; typecheck/lint and desktop/mobile distribution checks passed; hosted pending |
| 26 | Shared Quality host binaries/fixtures with retained VM coverage | External X-Node deliverable; the host binaries, fixtures and VM jobs are not present in this workspace. The panel now builds against the signed v2.0.6 protocol instead of unpublished symbols |
| 27 | Contracts/docs, browser regressions, hosted Linux regressions and signed release delivery | Contracts regenerated; module READMEs and structure audit passed; desktop/mobile browser regressions passed; hosted Linux regressions and signed amd64/arm64 release delivery remain pending |

## Compatibility repair

`internal/xnode` no longer references unpublished X-Node protocol symbols. Country
prefixes travel inside the ordinary state snapshot, administrator CIDR grants keep
local canonical validation, and command deadlines stay explicitly bounded. The
module remains pinned to signed source `fa867d98c0a4` (v2.0.6); features that need
newer agents remain capability-gated.

## Delivery gates

No local automated suites are permitted. Static checks, builds, schema review and
Chrome DevTools MCP are allowed. Hosted regressions must finish successfully;
then publish and verify signed amd64/arm64 agent and CLI artifacts, pin the panel
to that release, and finish panel CI. New bulk/CIDR behavior is capability-gated.
Raw connection observations remain ephemeral. Financial facts, paid successors,
unfinished accounting and rollback dependencies must survive all repairs.
