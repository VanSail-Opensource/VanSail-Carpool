# Common components

- `ConfirmDialog.vue` provides the shared Nuxt UI confirmation modal with soft cancellation, rigid confirmation, heavy destructive confirmation feedback, and an opt-in truly centered header without close-control spacing. Failed actions stay inside the open modal and announce through its native alert role.
- `CountryFlag.vue` renders bundled Iconify flags for common nodes, including the Netherlands, and an ISO regional-indicator flag fallback for every other valid country code.
- `InlineNotice.vue`, `OperationStatusNotice.vue`, `SkeletonBlock.vue`, and `StatusBadge.vue` provide status feedback, including durable operation polling failures. Notice, receipt/error state swaps, and badge presence are component-owned Motion with opacity-first reduced-motion fallbacks; global styles do not animate semantic mounts.
- `InlineNotice.vue` forwards fallthrough attributes to its motion host explicitly;
  the presence wrapper renders a fragment and cannot inherit caller layout classes.
- Operation notices use simultaneous presence because a receipt, polling failure
  and retry action can coexist. Sequential single-child presence hides recovery
  controls with newer Motion versions.
- `DualCurrencyMoney.vue` renders an authoritative TXB amount with an optional second-line approximate CNY/USD amount for checkout totals.
- `SwitchField.vue` and `TxbAmountField.vue` provide domain form fields; `TxbAmountField.vue` keeps exact TXB entry and can add a bounded `USlider` for member funding without weakening minor-unit validation.
- Amount text remains a decimal string through entry and validation, so large
  values and excess precision cannot be silently rounded by a numeric input.
  Only the optional bounded slider uses numbers. `TxbAmountField.test.ts` covers
  exact large values and rejection of fractional minor units.
- `SwitchField.vue` uses a native label for its entire 52px row, so tapping the copy or surrounding space toggles the associated Nuxt UI switch once. The name and optional help remain separate accessible associations, and disabled switches retain their native guard.

`JsonDocumentField.vue` is the shared localized JSON field. External value changes reset invalid drafts; parsed edits emit update/validity events. It owns no node or administrator state.

`MetricLineChart.vue` renders bounded rate, count, byte-total, money and percentage
series with localized captions. Callers can supply a forecast label appropriate
to their metric; the default remains the usage forecast label.

- `LivePrice.vue`: Compact localized indicator beside an applicable dynamic amount.

- `PricingQuoteNotice.vue`: Localized calculating, delayed, stale and unavailable quote states, manual retry and precise validity.

Catalog descriptions use escaped Vue text and a native multiline field. Squad merchandising uses the structured feature list; no application Markdown parser or editor remains.

Short-window `MetricLineChart.vue` axes include seconds so ten-second rate
samples have distinct labels; day histories retain UTC calendar labels.

`LivePrice.test.ts` covers invalid expiry, replacement validity and rescheduling
beyond the browser timer limit; ready indicators expire at the actual deadline.

`MetricLineChart.vue` adds exact byte detail only for safe, nonnegative integer
samples. Numeric telemetry or forecasts outside that range keep their formatted
magnitude without claiming to recover lost precision; `MetricLineChart.test.ts`
covers unsafe, fractional and exact byte/rate samples.

`DemoTransport.vue` is the shared Explore/DIY film transport: Nuxt UI slider seeking,
44px playback/chapter controls and localized accessible names. It emits actions
and owns no playback clock. `DemoPointer.vue` renders the same percentage-based
interaction path as a mouse cursor on desktop and a touch/ripple on mobile;
`illustrationTimeline.ts` contains pure easing and pointer interpolation. All
motion follows the parent's clock, so pause, seeking and hidden-page suspension
also stop the simulated interactions. Reduced-motion scenes omit the pointer.

`DemoPlayer.vue` unifies the modal and playback lifecycle for all Knowledgebase,
Explore and DIY lessons. It owns focus restoration and native Back behavior.
`DemoStage.vue` provides an accessible, responsive diagram canvas;
`DemoNode.vue` renders shared icon tiles and `DemoFlow.vue` renders continuous
SVG flow from the same seekable clock. None perform network or account actions.
