# Dashboard components

`EntitlementSummary.vue` labels retained purchases as grandfathered fixed-term
benefits and keeps their existing renewal and paid-successor controls reachable.

Purchase actions retain automatic renewal and eligible refunds. Manual and
automatic paid reset controls are retired. The dashboard shows usage
facts without purchase entrances; legacy Home add-on query links redirect to
Settings. This keeps dashboard actions separate from subscription settings.


- `UserHome.vue` composes Home in one balance-first flow. Ordered active squad labels arrive in its owner-scoped dashboard response, and usage summaries carry node labels and country metadata. Home no longer fetches the full Explore catalog or persists duplicate squad data. Retired traffic multiplier metadata remains constant 1 only for API compatibility and is not displayed.
- `BalanceHero.vue` opens the real funding sheet from Home and refetches a provider-returned reissue order before prepopulating it; `SubscriptionPanel.vue` is reused on Subscription for the active URL and Connections/Revoke actions on both mobile and desktop. Home exposes the subscription entrance and forwards legacy Revoke query links.

`UsagePanel.vue` retains the legacy quota meter and stale-data disclosure. The old per-node breakdown bar and reset controls are retired; detailed traffic belongs to Usage Breakdown.
- `ComingSoonLinks.vue` preserves mobile Around TX actions through Vue Router without native document links and renders the section only for an opted-in entrance preference and a server-confirmed valid combo; the desktop sidebar carries the same gated entry points.
- `ComingSoonLinks.test.ts` verifies every member-tool action reaches its route while keeping the launch document URL unchanged. The retired member TX Statistics action is absent; administrator statistics stay in the browser console.
- `UsagePanel.test.ts` verifies stale upstream data disclosure.
- `EntitlementSummary.test.ts` covers active, queued, and absent combos.

Dashboard controls distinguish copy, open, navigation, retry, confirmation, and destructive intents. Automatic renewal switches emit selection feedback only when their controlled value changes.

Language and currency controls appear on Settings. Mobile Around TX additionally requires the persisted entrance preference.

The IP Lookup entrance in `ComingSoonLinks.vue` remains available to members without a combo when the feature is enabled; only its item bypasses the existing combo/preference gates.

`MemberLiveMetrics.vue` polls owner-scoped, memory-only metrics every five seconds while visible, with incomplete coverage and localized limitation reasons.

Its Connections value is traffic-active / total open client flows. The localized
accessible label and native tooltip explain the counts and the unchanged
traffic-active limit. Total counts are decimal strings; missing totals from older
or unavailable agents remain unknown. `memberActivity.ts` projects the server's
expiring status: a fresh open flow is online even with zero sampled activity;
offline requires complete fresh coverage reporting no open flows.

`memberActivity.ts` projects native observation status using the server-relative lifetime from `fetchedAt` to `activityExpiresAt`, measured from request start with the browser's monotonic clock. Network time only shortens freshness; focus and visibility changes invalidate the prior observation until a new poll succeeds. `MemberLiveMetrics.vue` emits fresh activity, retires stale rates, and clears status on errors/unmount. `UserHome.vue` passes that status to `UsagePanel.vue`; the legacy onlineAt field no longer controls the native badge. `memberActivity.test.ts` and `MemberLiveMetrics.test.ts` cover expiry, failed or invalid timestamps, slow responses, and visibility invalidation.
`useDashboard` and `EntitlementSummary.vue` revalidate cached purchases at validity boundaries and on visibility/focus refresh. Expired predecessors disappear immediately while successor discovery refreshes quietly; display state never changes settlement records.


- `EntitlementSummary.vue` renders the active combo directly and keeps queued purchase controls; member usage forecasts are not requested.

Active purchase controls and exact traffic presentation, using the shared chart primitives for quantitative visuals: `AutoRenewalControl.vue`, `PurchaseActions.vue`, `RideSummaryFace.vue`, `TrafficNodeChart.vue`, `TrafficUsageDetails.vue`.

`UserHome.vue` always composes `UsageBreakdownPanel.vue` once the member dashboard
is available, regardless of the current wallet or purchase. The panel reuses the
visibility-aware `useSubscriptionData` lifecycle for the owner-only no-store
`GET /api/v1/usage/breakdown`. It retains an existing snapshot on read failure and
labels incomplete coverage, gaps, omitted nodes and stale data without backfill.

Chrome DevTools inspection on 2026-11-09 covered EN/Chinese, 390px/1440px,
dark/light, loading, empty, single-node, errors/retry, coverage gaps, capped nodes,
unallocated/hourly costs and large fractional totals. Twenty rapid metric clicks
retained the same SVG, shared node paths, dimensions and distinct node colors.
Keyboard metric/legend selection worked; browser mode emitted no Telegram calls.
The zero-balance Home fixture requested history without an active purchase.
No local automated suites were run; hosted regressions provide execution coverage.

`UsageBreakdownPie.vue` owns only metric selection and presentation. Traffic
includes all billing modes. Cost keeps actual charges, a clearly identified
current fixed-term estimate, and unallocated charges separate. An unknown original
purchase duration leaves actual costs visible and the estimate unavailable.
`usageBreakdownFormat.ts` rounds only displayed money, never calculation inputs.
`usageBreakdownFormat.test.ts` covers sub-minor disclosures and half-unit display
rounding while retaining the underlying rational value.

`usageBreakdownSegments.ts` preserves exact BigInt/rational totals through node
grouping, including omitted totals in Others and retaining unallocated cost as a
non-node segment. Server physical-node counts hide a selected pie with at most
one node before grouping/capping; totals and disclosures remain visible. Colors
are assigned across all node IDs before metric filtering, with a bounded 256-ID
component-local registry for refreshes. Stable accessors, IDs, 220px dimensions and
explicit zero duration on both Unovis components prevent animated mode updates;
the same mounted SVG survives even when a selected measure hides the chart.

`usageBreakdownSegments.test.ts` covers exact conservation, large/fractional costs,
independent node counts, grouping, unallocated costs and stable bounded colors.
`UsageBreakdownPie.test.ts` checks twenty mode changes without remounting, explicit
animation settings, unknown-term disclosure, tiny positive percentages and browser
feedback isolation. Browser fixtures exercise the real Unovis implementation.
