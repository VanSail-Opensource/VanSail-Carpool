# Browser pricing workspace

These components belong exclusively to the browser administrator router. They
inherit its saved/system appearance and desktop layout with contained tables
and stacked mobile controls. No Telegram controls or initialization are imported.

- `PricingWorkspace.vue` composes overview/product/user/operations/export routes.
- `usePricingWorkspace.ts` cancels obsolete loads, limits parallel reads to three,
  and refreshes live dashboard health without polling every analytical dataset.
- `ProductPricingSettings.vue` integrates explicit audited mode saves into the
  existing product editors. Calculated bounds are read-only; no approval step.
- `PricingProducts.vue` renders paginated product modes and detail links.
- `PricingSummary.vue` separates recorded commerce results/provider payments
  from next-day unvalidated forecasts, explains missing node cost coverage and
  provider payment evidence, and links to the existing cost controls.
- `PricingHistory.vue` reads original immutable vintages, one model/lead band,
  actual/forecast/interval toggles, events, provider/calendar dates and scoped export.
  Daily history connects unavailable points visually without filling actuals or intervals.
- `PricingResponseCurve.vue` renders stored candidate probabilities and contribution
  with an accessible table; prior results are explicitly diagnostic.
- `PricingCalibration.vue` groups original probability forecasts into observed
  frequency buckets for one model, using sample-weighted sums.
- `PricingOperations.vue` shows memory-only CPU/memory series and health. Missing
  telemetry is unavailable, never a zero-load observation.
- Provider cost editing is in the X-Node node settings, using the same versioned
  pricing API; analytical datasets remain available to exports and diagnostics.
- `PricingInterventionForm.vue` sends typed, scoped, versioned, expiring commands.
- `PricingInterventions.vue` shows the timeline and reasoned cancellation. Edits
  and material discounts take effect directly and send queued notifications.
- `PricingRecordList.vue` displays bounded read-only traces and source facts.
- `PricingExports.vue` streams ZIP data through the native file picker, offers
  typed CSV/JSONL, cancellation, scope filters and sensitive-export authorization.
  Its all-category selector uses a nonempty UI value and sends the API's empty
  category only when constructing the export scope.

Public interfaces live in `api/pricing.ts`, `api/pricingAnalysis.ts` and
`api/pricingExports.ts`; labels live in both `pricing` locale domains. Analytical
caches never authorize a debit. Existing member renewal controls use a separate
read-only notice; dynamic pricing requires no terms approval.

- `PricingDecisionFacts.vue`: Expanded decision versions, reasons, probability, model and validity facts.

- `UserPricingAnalysis.vue`: Embeds all owner-specific product bases, coupon terms and forecasts directly on the user page.

- `useUserPrices.ts`: Cancels obsolete reads and appends explicit bounded pages without silently dropping products.

- `PricingDataset.vue`: Progressively renders one disclosed dataset with retry, visible truncation and cursor pagination.
- `PricingDiagnostics.vue`: Mounts independent response, model and decision panels only after disclosure.
- `PricingHistorySection.vue`: Loads original forecasts and event overlays on demand; diagnostic failures leave forecasts visible.
  The overview can switch between node traffic and the displayed product page's
  revenue forecasts without combining different forecast models or horizons.
- `usePricingDataset.ts`: Owns scoped cursor requests, cancellation, late-response rejection and retained successful pages.
- `usePricingWorkspace.test.ts`: Hosted regressions for route request budgets and obsolete polling.

The overview starts with one dashboard read, operations with one memory-only
health read, product with one policy read, and member analysis with two reads.
Optional native disclosures own their data lifetimes and independently retry.
Only foreground health is polled; completed daily forecasts refresh on focus,
UTC rollover or explicit refresh. Each lead retains its own calibration.
Only the selected node mounts operational charts. Dataset cursors preserve typed
product scope and show additional retained pages explicitly.

Checkout keeps the selected basket on a quote-version conflict and requires the
existing confirm control to submit the newly displayed total. Calculations retry
with bounded backoff, pause while hidden and expose manual retry after two
minutes. No analytical cache authorizes money, coupon consumption or preferences.
