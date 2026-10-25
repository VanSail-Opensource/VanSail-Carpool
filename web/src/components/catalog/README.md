# Catalog components

`PAYGStatusDot.vue` gives each squad an accessible semantic status marker;
reduced-motion users receive a static dot. Hidden squads are filtered only from
Explore cards and subscription output, while server totals include consumption
and server authorization preserves existing connections.
Card traffic/spending occupy opposite bottom corners; funding remains on Home.

`CatalogBlocked.vue` presents the automatic-renewal prohibition for every member
role without redirecting or blocking the PAYG mode switch. The catalog's optional
`autoRenewalBlockedUntil` is refreshed on focus and expiry by
`useCatalogAdmission.ts`; quotes and transactional checkout enforce the same
time window. `useCatalogAdmission.test.ts` covers expiry and authoritative refresh.

`PAYGSummary.vue` centers the 72-hour label and traffic/spending metrics on one
row, with contained wrapping when available width is insufficient.

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

Explore is a lazy router entry. Its PAYG and Combo components are independent
Vue async imports; only the rendered mode loads its resources. `CatalogChunkState.vue`
uses shared skeleton/notice components for import loading and recovery, preserving
mode controls. API errors retain each mode's existing retry flow. Catalog/coupon
reads are excluded from session preload and Home; financial decisions still make
fresh requests through their existing loaders and checkout paths.

- `CatalogModeTransition.vue` presents an 800ms viewport-wide introduction. Animation completion releases the content gate; a 900ms fallback handles suppressed WebView events. Readiness remeasures controls without restarting the scene. Combo content mounts immediately but stays hidden/inert until completion, then enters over 200ms. Only mode navigation and application navigation sit outside its continuous cover; PAYG additionally reserves its wallet. Scroll/viewport changes remeasure without ending the scene. Reduced motion skips both animations. Generation checks discard stale scenes on rapid switching.

- `CatalogModeTransition.test.ts` covers rapid scene replacement, timed disposal and reduced-motion omission in hosted CI.

PAYG receives an `entranceReady` gate from actual scene completion (or reduced
motion), rather than a fixed animation delay. Cards and populated 72-hour totals
pause their CSS entrance at time zero until that gate opens. A slow response
keeps ordinary loading skeletons visible and starts its entrance on arrival;
background refreshes do not restart existing card animations. The wallet and
mode controls remain available. `PAYGPage.test.ts` and `CatalogExperience.test.ts`
cover data arriving before scene completion; Chrome samples verify CSS timing.
- `PAYGSummary.vue` presents authoritative wallet balance, recharge navigation and one 72-hour traffic/spend summary. `PAYGSquadCard.vue` reuses the shared profile card with price/GB, availability and raw-traffic/spend metrics; neither derives financial totals or exposes node names.
- `PAYGPage.test.ts` covers initial loading, empty results, exact summary values and retained totals after a background refresh failure. All automated regressions run in hosted CI.

Mode changes emit exactly one selection haptic; tapping the active mode is silent. PAYG recharge and retry controls use the existing delegated semantic feedback. Informational PAYG cards remain non-interactive.
