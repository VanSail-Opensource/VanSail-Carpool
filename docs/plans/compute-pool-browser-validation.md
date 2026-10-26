# Compute and pricing browser validation

Chrome DevTools MCP exercised the real Vue components with constructed owner/admin data on localhost. This is browser evidence, not a hosted backend or production-data regression. Fixtures make loading, missing history and failure states reproducible without production writes.

## Observed flows

- Personal predictions at 320 px, 390 px and 1440 px: two observed days and five forecast days, missing-history state, delayed initial load, pending/stale status, localized error/retry and Chinese copy. Arrow/Home/End inspection and touch selection expose values and interval endpoints. No horizontal page overflow in inspected views.
- Active Combo Home: flip to predictions and back, visible-face sizing and inert hidden face, focus movement and retained renewal controls. PAYG-only Home shows predictions directly.
- Subscription host list at 320 px, 390 px and desktop: edit dialog/input/preview/save, empty reset, save failure retaining input, long names, unavailable links, copied/native URI consistency and QR generated with the saved name. Applied route and added split-XHTTP badges are explicit.
- User pricing analysis: public/personal/campaign/net bases, coupon scope/cap/expiry, decision/probability/calibration/version/freshness and renewal facts remain expanded. Explicit next-page loading changed two displayed products to three. User/product/operations surfaces fit narrow layouts with contained tables and no collapsed analytical sections.
- Admin catalog: PAYG `0.50 TXB/GB` and distinct available, hidden, unavailable and upstream-missing states. A 640 px table scrolls within its 269 px mobile container rather than widening the page.
- Shared statistical/metric charts: Unovis line, interval, stacked bars, donut and concentric donut composition; legend selection, exact displayed values and touch/keyboard readouts. Light/dark browser appearance and Mini App theme variables remain independent.
- Legacy renewal-lock projection does not block browsing Combo selection. Backend dual-decision admission remains a hosted regression gate.

## Corrections made during inspection

Unovis interval areas require interval width above their baseline. Missing observations are split into separate area datasets instead of zero-valued history. Horizontal stacked bars require swapped axis domains. Concentric labels are positioned over the correct ring center. Missing localized labels and a fixture-only unmocked refund request were corrected.

Final inspected populated flows had no console warnings/errors or unexpected failed requests. Explicit error scenarios intentionally return controlled fixture failures. Source/API/schema, build, lint and structure checks are recorded separately in the delivery progress document. No local automated suites were run.
