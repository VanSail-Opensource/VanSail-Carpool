# Catalog components

Optional squads sort by ascending popularity within their existing type groups;
included and sold-out entries remain last. Node controls show name, country and
consumption multiplier, including for single-node squads.
At desktop widths, squad cards use content-sized masonry columns within each
type group, preserving the prepared order down each column and reducing gaps
between squads with different node counts. Node controls can form a compact
two-column grid when their card width allows; mobile keeps the existing layout.


- `useCatalogStepPersistence.ts` owns user-scoped step restoration, scroll reset, and quote refresh after a restored or changed coupon/squad selection; it stores only the current step and keeps quote state server-authoritative.
- `useCatalogSquadPresentation.ts` starts non-blocking composition-statistics loading with the catalog page and returns featured and ordered squad IDs from the selected combo or the global composition fallback.
- `CatalogFlowProgress.vue` and `CatalogFlowControls.vue` present the current step and navigation controls; completed catalog steps use a Motion-presence check indicator, the active marker uses a shared layout ID, and the Back control uses local layout/presence. The compact progress rail fits narrow phones without forced horizontal expansion, and controls stay in normal document flow.
- `catalogFlowProgress.ts` contains the pure completed-step icon rule used by the progress component.
- `CatalogFlowProgress.test.ts` verifies one-based catalog steps map to the stepper index and completed steps use check indicators.
- `CatalogComboPricingTable.vue` lays core combos out as responsive selectable cards, preserving Markdown descriptions and server-projected values while combining traffic with reset cadence, presenting the price period as `per N days`, showing the included-squad count, and shortening rollover eligibility copy. `ComboOption.vue` is the focused option primitive covered by its existing values and selection test; its selected check uses local Motion presence. Core-combo selection uses the configured display currency directly.
- `catalogSquadPresentation.ts` orders selectable add-ons independently inside each squad type, uses aggregate global composition before a combo is selected, moves Included and Full squads not already held by the current user to the end of their type, and returns every tied positive leader per type. `catalogSquadPresentation.test.ts` covers selected-combo ordering, global fallback, type-local leaders, exclusions, and missing data.
- `CatalogCouponStep.vue` selects an eligible wallet coupon or redeems a new code while its Continue action waits for a quote matching the current selection.
- `CatalogConfirmation.vue` presents the localized post-purchase summary, uses a short one-shot Motion confirmation reveal with a reduced-motion opacity fallback, and emits the Home navigation action.
- `CatalogConfirmation.test.ts` verifies purchase details and the Home action.
- `CatalogCheckout.vue` combines authoritative review and idempotent purchase confirmation; payment funding remains in the balance sheet. Returning to review restores a missing quote before confirmation and does not leave a completed or failed quote in a calculating state.
- `SquadActivationDialog.vue` prompts sequentially for every selected gated squad, including combo-included squads; raw codes remain memory-only until one purchase request.
- `CatalogPaymentStep.vue` is retained as the route-safe payment handoff component for legacy links while the catalog review owns purchase confirmation.
- `CatalogPage.test.ts` verifies confirmed purchases do not trigger quote restoration, squad-step exit refreshes the quote, empty node unions block progress, and Coupon-step continuation stays disabled until a usable quote returns.
- `SquadSelector.test.ts` verifies non-empty type grouping, prepared order handoff, legacy-profile exclusion, and event forwarding.
- `ComboOption.test.ts` verifies stable plan values, hidden included-squad detail, and selection events.
- `CatalogPaymentStep.test.ts` verifies the add-balance action stays in Vue Router history.

Coupon purchase discounts are described as price reductions, including recurring
discounts, rather than as balance additions.
Combo, squad, and coupon feedback is emitted only when the selection changes; step navigation is soft and purchase confirmation is rigid.

`comboDisplayPrice.ts` adds selected paid squads while excluding included squads, using integer minor units and the account price-display preference.
`comboDisplayPrice.test.ts` covers included/free/duplicate squads and integer precision.

`CatalogSquadStep.vue` composes `SquadSelector.vue`, `SquadPricingTable.vue` and `SquadPricingCard.vue` for localized ordering and selection. `SquadNodeBlocks.vue` renders readable node names, flags and multipliers as informational content. Their matching `.test.ts` files cover ordering, selection and node rendering.

`CatalogPage.vue` owns the four-step purchase flow. `CatalogSquadStep.test.ts` covers prepared ordering and `SquadNodeBlocks.test.ts` covers node labels and multipliers.

Informational node rows no longer sit above the card selection button or receive pointer/focus decoration, so retiring node-image actions leaves the entire card selectable.
