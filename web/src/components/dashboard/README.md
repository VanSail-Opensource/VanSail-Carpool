# Dashboard components

Purchase actions retain automatic renewal and eligible refunds. Member manual
traffic reset, its control and dialog are removed; automatic reset settings use
the strict >95% five-minute scanner threshold and normal balance-backed pricing.

facts without purchase entrances; legacy Home add-on query links redirect to
Settings. This keeps dashboard actions separate from subscription settings.


- `UserHome.vue` composes Home in one balance-first flow. Ordered active squad labels arrive in its owner-scoped dashboard response, and usage summaries already carry node labels/country/multipliers. Home no longer fetches the full Explore catalog or persists duplicate squad data.
- `BalanceHero.vue` opens the real funding sheet from Home and refetches a provider-returned reissue order before prepopulating it; `SubscriptionPanel.vue` is reused on Subscription for the active URL and Connections/Revoke actions on both mobile and desktop. Home exposes the subscription entrance and forwards legacy Revoke query links.

`UsagePanel.vue` retains the legacy quota meter and stale-data disclosure. The old per-node breakdown bar and reset controls are retired; detailed traffic belongs to Usage Breakdown.
- `ComingSoonLinks.vue` preserves mobile Around TX actions through Vue Router without native document links and renders the section only for an opted-in entrance preference and a server-confirmed valid combo; the desktop sidebar carries the same gated entry points.
- `ComingSoonLinks.test.ts` verifies every member-tool action reaches its route while keeping the launch document URL unchanged. The retired member TX Statistics action is absent; administrator statistics stay in the browser console.
- `UsagePanel.test.ts` verifies stale upstream data disclosure.
- `EntitlementSummary.test.ts` covers active, queued, and absent combos.
- `TrafficResetAutomationControl.test.ts` verifies the switch remains controlled while emitting an immediate account preference update.

Dashboard controls distinguish copy, open, navigation, retry, confirmation, and destructive intents. Automatic renewal and traffic-reset switches emit selection feedback only when their controlled value changes.

Language, currency and automatic traffic reset controls now appear on Settings. Mobile Around TX additionally requires the persisted entrance preference.

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

Active purchase controls and exact traffic presentation, using the shared chart primitives for quantitative visuals: `AutoRenewalControl.vue`, `PurchaseActions.vue`, `RideSummaryFace.vue`, `TrafficNodeChart.vue`, `TrafficResetAutomationControl.vue`, `TrafficUsageDetails.vue`.

When no combo is active and the balance is positive, `UserHome.vue` composes
`PAYGUsagePanel.vue` in place of the quota meter. It uses the existing visibility
aware subscription-data reader against GET /api/v1/payg/history. PAYGUsagePie uses
ChartContainer/Unovis and the existing chart-selection helper. Exact bytes remain
strings/BigInt; only bounded proportional shares are numbers. Native legend
buttons and pointer/touch slices expose identical name/traffic/percentage detail.
Empty history does not claim historical zero use; stale/errors and truncated
attribution remain explicit. No history is reconstructed from financial totals.

`paygHistorySegments.ts` derives bounded drawing shares from exact totals and groups nodes below 1% into one localized Others segment per metric; `paygHistorySegments.test.ts` checks precision, zero use, threshold behavior and total preservation. `PAYGUsagePie.vue` provides chart and legend selection, Nuxt UI metric tabs and Telegram selection haptics on metric changes.
