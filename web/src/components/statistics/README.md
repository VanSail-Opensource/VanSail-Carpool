# Statistics components

Node user counts require usersOnlineAvailable as well as fresh node activity.
Unknown counts display unavailable while independently known traffic rates remain visible.


- `StatisticsPage.vue` is the thin page orchestrator for loading, stale, empty, and partial-error states, with separate Overview, Nodes, Seven-day traffic, Distributions, and Squad composition tabs. Overview also contains both source-freshness panels.
- `StatisticsFreshness.vue` exposes the X-Node and database snapshot timestamps as independent Overview panels with separate stale state.
- `StatisticsFreshness.test.ts` covers independent partition timestamps and stale/current disclosure.
- `StatisticsNodes.test.ts` covers the card action, open-intent feedback marker, and removal of version/multiplier rows.
- `StatisticsTrafficChart.vue` renders the seven-day per-node traffic stack with hover, focus, keyboard, tap, and phone drag selection plus exact-value details.
- `StatisticsShareCharts.vue` composes member and payment proportions.
- `StatisticsDistribution.vue` switches between squad-first and combo-first normalized SVG stacked bars with interactive segment details.
- `StatisticsDonut.vue` is the interactive reusable SVG donut-with-text and legend primitive.
- `StatisticsPie.vue` renders interactive accessible SVG pies with either outside labels or a compact label list.
- `StatisticsConcentricDonut.vue` owns the interactive reusable outer/inner ring geometry and grouped legends. Series weights let payment rings share one denominator, so arc lengths compare volume; reduced motion keeps final geometry and only fades opacity.
- `StatisticsMembershipDonut.vue` combines subscription states and active combo choices with the active-member count at its center.
- `StatisticsPaymentDonut.vue` renders EZPay outside BEPusdt, excludes failed or refunded payment facts, and centers the included payment count.
- `StatisticsCharts.test.ts` covers donut, labeled-pie, and normalized stacked-bar semantics.
- `StatisticsPaymentDonut.test.ts` covers concentric payment-ring order, excluded payment states, and the explicit no-payment state.
- `statisticsFormat.ts` owns stable chart colors plus localized number, percentage, date, and byte formatting.
- `statisticsFormat.test.ts` covers UTC weekday buckets and invalid chart fact filtering.
- `StatisticsChartDetail.vue` provides the stable exact-value readout shared by every chart.
- `useStatisticsChartSelection.ts` centralizes hover, focus, and pinned tap selection state, emitting one semantic pulse only when a selection is committed or cleared.
- Chart components animate data geometry, rankings, selected detail, and freshness only. `StatisticsPage.vue` stays static so refreshes never replay the whole route.

`StatisticsNodes.vue` and `StatisticsNodes.test.ts` retain readable live node rates and health without retired image actions. `StatisticsPage.test.ts` covers tab-panel separation.

TX Statistics is browser-administrator-only, under `/admin/statistics`. Its API routes use `/api/v1/admin/statistics`; Mini App routes, navigation, preloads, Telegram controls and haptic imports are removed. Live node reads use only native telemetry; historical aggregates use existing exact native/reconciled source accounting.

Observed statistics summary without retired credit projections: `StatisticsOverview.vue`.
