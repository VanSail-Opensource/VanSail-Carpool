# Dashboard components

Purchase actions retain automatic renewal and eligible refunds. Member manual
traffic reset, its control and dialog are removed; automatic reset settings use
the strict >95% five-minute scanner threshold and normal balance-backed pricing.

facts without purchase entrances; legacy Home add-on query links redirect to
Settings. This keeps dashboard actions separate from subscription settings.


- `UserHome.vue` composes Home in one balance-first flow. Ordered active squad labels arrive in its owner-scoped dashboard response, and usage summaries already carry node labels/country/multipliers. Home no longer fetches the full Explore catalog or persists duplicate squad data.
- `BalanceHero.vue` opens the real funding sheet from Home and refetches a provider-returned reissue order before prepopulating it; `SubscriptionPanel.vue` is reused on Subscription & uptime for the active URL and Connections/Revoke actions on both mobile and desktop. Home replaces this block with the gated summary from the subscription module and forwards legacy Revoke query links.

`UsagePanel.vue` also displays upstream `lastTrafficResetAt` in the existing localized date/time format, with a null fallback and the existing stale-snapshot disclosure. `UsagePanel.test.ts` covers timestamp presence and absence alongside cached-data warnings.
- `ComingSoonLinks.vue` preserves mobile Around TX actions through Vue Router without native document links and renders the section only for an opted-in entrance preference and a server-confirmed valid combo; the desktop sidebar carries the same gated entry points.
- `ComingSoonLinks.test.ts` verifies every member-tool action reaches its route while keeping the launch document URL unchanged. The retired member TX Statistics action is absent; administrator statistics stay in the browser console.
- `UsagePanel.test.ts` verifies stale upstream data disclosure.
- `EntitlementSummary.test.ts` verifies the catalog action stays in Vue Router history.
- `TrafficResetAutomationControl.test.ts` verifies the switch remains controlled while emitting an immediate account preference update.

Dashboard controls distinguish copy, open, navigation, retry, confirmation, and destructive intents. Automatic renewal and traffic-reset switches emit selection feedback only when their controlled value changes.

Language, currency and automatic traffic reset controls now appear on Settings. Mobile Around TX additionally requires the persisted entrance preference.

The IP Lookup entrance in `ComingSoonLinks.vue` remains available to members without a combo when the feature is enabled; only its item bypasses the existing combo/preference gates.

`MemberLiveMetrics.vue` polls owner-scoped, memory-only metrics every five seconds while visible, with incomplete coverage and localized limitation reasons.

`memberActivity.ts` and `memberActivity.test.ts` project native observation status and expiry. `MemberLiveMetrics.vue` emits fresh activity from its existing poll, retires stale rates and clears status on errors/unmount. `UserHome.vue` passes that status to `UsagePanel.vue`; the legacy onlineAt field no longer controls the native badge.
`useDashboard` and `EntitlementSummary.vue` revalidate cached purchases at validity boundaries and on visibility/focus refresh. Expired predecessors disappear immediately while successor discovery refreshes quietly; display state never changes settlement records.


- `ForecastFlipCard.vue`: Keeps the active Combo face transition and isolates the interactive prediction reverse.

- `PersonalPredictions.vue`: Loads the last two complete UTC accounting days and next five daily forecast windows for a graph with an uncertainty band. Loading, stale, error and missing states remain visible. Dashed connectors bridge unavailable days and the historical-to-forecast transition without creating observations.

Active purchase controls and exact traffic presentation, using the shared chart primitives for quantitative visuals: `AutoRenewalControl.vue`, `PurchaseActions.vue`, `RideSummaryFace.vue`, `TrafficNodeChart.vue`, `TrafficResetAutomationControl.vue`, `TrafficUsageBar.vue`, `TrafficUsageBarDetails.vue`, `TrafficUsageDetails.vue`.
