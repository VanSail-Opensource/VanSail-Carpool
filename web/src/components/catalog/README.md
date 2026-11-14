# Catalog components

Explore offers Charge by traffic and Charge by hour. Fixed-term purchase screens,
their drafts and their checkout-only helpers are retired. Existing owners keep
Home renewal/refund controls and the Settings add-squad flow. The shared squad
and coupon components below remain in those supported flows.

- `CatalogExperience.vue` lazily selects `PAYGPage.vue` or
  `../hourly/HourlyPage.vue`, defaults to PAYG and emits one selection haptic
  per actual change. `CatalogExperience.test.ts` covers mode switching and
  independent lazy loading. `catalogPages.ts` provides shared import identities
  for mounting and optional preloading. After mount, one idle callback warms
  both page chunks; it skips hidden documents, Data Saver and 2G connections,
  and cancels pending scheduling on disposal. Demo code warms on trigger hover
  or keyboard focus and is mounted only when requested.
- `CatalogChunkState.vue` provides localized loading and chunk-recovery UI
  while keeping the mode controls available.
- `BillingDemo.vue` opens from the current mode's icon-led demonstration button.
  It composes `TrafficBillingScene.vue` or `HourlyBillingScene.vue` using the
  synthetic snapshots in `billingDemo.ts`. The shared
  `useIllustrationPlayback` owns play/pause/replay, manual steps, visibility
  suspension and timer disposal. Reduced motion uses manual steps with static
  diagrams. Nuxt UI's modal owns focus, dismissal and accessible naming; each
  step supplies a localized screen-reader explanation alongside its short
  visual caption. Demo state never enters account caches or calls a write API.
  Example prices remain explicitly illustrative, and all copy lives in
  `locales/*/payg.json` under `payg.demo`.
- `PAYGPage.vue` polls owner-scoped availability and the authoritative 72-hour
  traffic/spending totals through the shared subscription-data lifecycle.
  `PAYGPage.test.ts` covers loading, empty results, exact totals and retained
  last-good data after a failed refresh.
- `PAYGSummary.vue` presents the 72-hour summary. `PAYGSquadCard.vue` adds
  per-GiB pricing, availability and raw traffic/spending to the shared card.
- `PAYGStatusDot.vue` exposes localized availability through a keyboard/touch
  popover. Reduced motion disables the opacity/ring animation.
- `CatalogSquadStep.vue` passes active-term add-squad selections to
  `SquadSelector.vue`, which composes `SquadPricingTable.vue` and
  `SquadPricingCard.vue`. `CatalogSquadStep.test.ts` and
  `SquadSelector.test.ts` cover prepared ordering, type groups and events.
- `SquadPricingCard.vue` renders profile facts and keyboard-accessible selection;
  PAYG and hourly use its informational variant. `SquadPricingCard.test.ts` covers profile
  types, absent profiles/nodes, hidden node names and ordered features.
- `SquadFeatures.vue` shows administrator-authored icon/title/detail rows in
  saved order through the allowlisted icon map.
- `catalogSquadPresentation.ts` orders candidates within type groups and
  preserves included/stock-held eligibility. `catalogSquadPresentation.test.ts`
  covers stable catalog order, profile grouping, included/full placement and
  stock held by the current member.
- `useCatalogSquadPresentation.ts` derives member ordering from current catalog
  data without requesting administrator-only composition statistics. The retired
  composition/share ranking and its never-populated featured badge are removed.
- `CatalogCouponStep.vue` selects an eligible wallet grant, redeems a new code
  and confirms permitted grant discards in the add-squad flow. Redemption and
  commit retry identities remain in their owning composables.
- `SquadActivationDialog.vue` collects memory-only codes for selected gated
  squads and hourly activation, with busy/error handling and Enter submission.

Cards omit individual node names and legacy traffic multipliers. Feature text is
escaped Vue text; no application Markdown parser is involved. Explore cards use
an aligned, responsive desktop grid and one column on narrow screens. The title
and price have independent header columns; long names wrap, facts use compact
icon rows, and the activity/action area stays at the bottom of each card. This
keeps row order consistent with keyboard and reading order. The 140ms mode
entrance is interruptible and disabled by reduced-motion preferences.

The traffic demonstration follows native block accounting: 100 MiB of usage in
the rolling 72-hour window triggers the first binary 1 GiB block, additional
usage buys whole blocks, a reconnect retains unexpired credit, and unused
credit expires 72 hours after the latest purchase without an idle debit.
The hourly demonstration covers prorated first-hour activation, whole-hour
renewal, shared daily allowance, reduced speed, consented decimal-GB extras,
voluntary disable/cooldown and the UTC-day reset. These visuals explain the
server rules; they never calculate a member quote or authorize access.

Hidden PAYG squads are omitted from Explore/subscription output while server
totals preserve consumption. Client presentation never substitutes for server
authorization, settlement or owned fixed-term renewal eligibility.
