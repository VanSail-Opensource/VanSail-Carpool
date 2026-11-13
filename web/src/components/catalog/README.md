# Catalog components

Explore offers Charge by traffic and Charge by hour. Fixed-term purchase screens,
their drafts and their checkout-only helpers are retired. Existing owners keep
Home renewal/refund controls and the Settings add-squad flow. The shared squad
and coupon components below remain in those supported flows.

- `CatalogExperience.vue` lazily selects `PAYGPage.vue` or
  `../hourly/HourlyPage.vue`, defaults to PAYG and emits one selection haptic
  per actual change. `CatalogExperience.test.ts` covers mode switching and
  independent lazy loading.
- `CatalogChunkState.vue` provides localized loading and chunk-recovery UI
  while keeping the mode controls available.
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
  PAYG uses its informational variant. `SquadPricingCard.test.ts` covers profile
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
escaped Vue text; no application Markdown parser is involved. PAYG cards use
content-sized desktop columns and one column on mobile. The 140ms mode entrance
is interruptible and disabled by reduced-motion preferences.

Hidden PAYG squads are omitted from Explore/subscription output while server
totals preserve consumption. Client presentation never substitutes for server
authorization, settlement or owned fixed-term renewal eligibility.
