# Catalog components

Optional squads sort by ascending popularity within their existing type groups;
included and sold-out entries remain last. Combo and PAYG cards show squad names
and configured profile facts, including carriers, routes, bandwidth and region.
Individual node names, rows and multipliers are omitted.
Shared cards use compact padding and profile gaps; PAYG keeps its wallet, totals
and cards close together while preserving 44px primary controls.
At desktop widths, squad cards use content-sized masonry columns within each
type group, preserving the prepared order down each column and reducing gaps
between squads with different profile content; mobile keeps a single column.


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

`CatalogSquadStep.vue` composes `SquadSelector.vue`, `SquadPricingTable.vue` and `SquadPricingCard.vue` for localized ordering and selection. Profile facts render independently of accessible nodes. `SquadPricingCard.test.ts` covers profile types, empty node lists, missing profiles and hidden node names.

`CatalogPage.vue` owns the four-step purchase flow. `CatalogSquadStep.test.ts` covers prepared ordering.

Shared cards keep the entire selection surface keyboard accessible; profile facts have no independent selection behavior.

`CatalogExperience.vue` owns the PAYG/Combo view selection, defaulting to PAYG, and the accessible reduced-motion entry transition. `PAYGPage.vue` polls owner-scoped availability and 72-hour raw traffic/wallet spending with the existing `useSubscriptionData` lifecycle. It reuses `SquadPricingCard.vue` in informational mode, without node rows, stock gates, selection controls or traffic multipliers. The Combo flow remains in `CatalogPage.vue`; only Combo retains the automatic-renewal purchase gate.

`CatalogExperience.test.ts` guards PAYG defaults, isolation from the combo purchase gate and view switching in hosted CI.

- `CatalogModeTransition.vue` presents an 800ms viewport-wide introduction with animated icons above one complete, single-line mode phrase. Slogans appear only during this transition. One continuous content stage reserves the mode toolbar, PAYG recharge controls, visible checkout controls and navigation; it has no cutout masks or uncovered gap below the toolbar. `CatalogPage.vue` and `PAYGPage.vue` emit readiness after their initial load so bounds include the finished view's controls. Scrolling/resizing ends the transition. It never captures pointer/focus input and is omitted for reduced motion or insufficient content height. PAYG metrics/cards enter during the reveal; the wallet and buttons stay visible. Views swap immediately to avoid competing page fades.
- `CatalogModeTransition.test.ts` covers rapid scene replacement, timed disposal and reduced-motion omission in hosted CI.
- `PAYGSummary.vue` presents authoritative wallet balance, recharge navigation and one 72-hour traffic/spend summary. `PAYGSquadCard.vue` reuses the shared profile card with price/GB, availability and raw-traffic/spend metrics; neither derives financial totals or exposes node names.
- `PAYGPage.test.ts` covers initial loading, empty results, exact summary values and retained totals after a background refresh failure. All automated regressions run in hosted CI.
