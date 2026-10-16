# Dashboard components

Purchase actions retain automatic renewal and eligible refunds. Member manual
traffic reset, its control and dialog are removed; automatic reset settings use
the strict >95% five-minute scanner threshold and normal balance-backed pricing.

Add squads lives in Settings. The ride faces display ownership and rollover
facts without purchase entrances; legacy Home add-on query links redirect to
Settings. This keeps dashboard actions separate from subscription settings.

- `EntitlementSummary.vue` relays the active ride's visible 44px Add squad action to the mobile-safe two-step `squad-addition` dialog and accepts the desktop sidebar's Home query action. `RideSummaryFace.vue` keeps its bold, filled green treatment beside the combo name while `RolloverFlipCard.vue` retains the independent rollover action.

- `UserHome.vue` composes the ordered Home experience and its request states in one balance-first flow that remains visually identical on phones and desktop. It resolves active squad UUIDs against the current catalog without persisting duplicated squad data.
- `BalanceHero.vue` opens the real funding sheet from Home and refetches a provider-returned reissue order before prepopulating it; `SubscriptionPanel.vue` is reused on Subscription & uptime for the active URL and Connections/Revoke actions on both mobile and desktop. Home replaces this block with the gated summary from the subscription module and forwards legacy Revoke query links.

`UsagePanel.vue` also displays upstream `lastTrafficResetAt` in the existing localized date/time format, with a null fallback and the existing stale-snapshot disclosure. `UsagePanel.test.ts` covers timestamp presence and absence alongside cached-data warnings.
- `UsagePanel.vue` uses Nuxt UI `UProgress`, a single percentage summary, and one used/remaining detail pair for the current-term limit, and always renders unmultiplied traffic in `TrafficUsageBar.vue`. Nodes below 5% are grouped into the other-traffic segment, and the former sparkline/detail popover is not rendered. `TrafficUsageBar.vue` keeps labels hidden until hover, focus, or activation, then exposes node name, country, multiplier, bytes, and share through `TrafficUsageBarDetails.vue`. `TrafficUsageBarDetails.vue` is the focused, motion-owned semantic detail surface; it has no selection state of its own. `TrafficUsageDetails.vue` and `TrafficNodeChart.vue` remain the Telegram-viewport-bounded, date-filtered detail building blocks for future dashboard entry points. `EntitlementSummary.vue` composes the active ride, purchase operations, and owner-only queued cancellation. `PurchaseActions.vue` keeps automatic renewal available alongside any eligible warning-colored Refund action and owns first-term refund quote confirmation and receipt presentation. `TrafficResetAutomationControl.vue` is the controlled account-wide `USwitch`, reused on Settings and saved immediately using the automatic scanner policy. `AutoRenewalControl.vue` owns the localized one-cycle quote, eligibility notice, and `UModal`/`USwitch` toggle surface. `RolloverFlipCard.vue` owns an accessible, size-safe face transition, `RideSummaryFace.vue` centers every current-plan fact, and `RolloverDetailFace.vue` presents localized rollover status plus predicted credit, maximum daily usage, or N/A. `useRolloverDetail.ts` owns on-demand loading, retry, reset, and localized errors.
- `ComingSoonLinks.vue` preserves mobile Around TX actions through Vue Router without native document links and renders the section only for an opted-in entrance preference and a server-confirmed valid combo; the desktop sidebar carries the same gated entry points.
- `ComingSoonLinks.test.ts` verifies every member-tool action reaches its route while keeping the launch document URL unchanged.
- `UsagePanel.test.ts` verifies stale upstream data disclosure.
- `EntitlementSummary.test.ts` verifies the catalog action stays in Vue Router history.
- `TrafficResetAutomationControl.test.ts` verifies the switch remains controlled while emitting an immediate account preference update.

Dashboard controls distinguish copy, open, navigation, retry, confirmation, and destructive intents. Automatic renewal and traffic-reset switches emit selection feedback only when their controlled value changes.

Language, currency and automatic traffic reset controls now appear on Settings. Mobile Around TX additionally requires the persisted entrance preference.

The IP Lookup entrance in `ComingSoonLinks.vue` remains available to members without a combo when the feature is enabled; only its item bypasses the existing combo/preference gates.

`MemberLiveMetrics.vue` polls owner-scoped, memory-only metrics every five seconds while visible, with incomplete coverage and localized limitation reasons.
