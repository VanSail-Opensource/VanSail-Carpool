# Hourly commerce, renewal, and admin redesign

This document tracks the multi-task delivery for Charge by hour. Hosted
regressions are authored here; local automated suites remain prohibited.

## Requested outcomes

1. Notify a member when an hourly squad loses funding and will stop.
2. Every hourly squad has its own required-balance hour threshold.
3. Editing a fixed-term squad to another billing mode must not change an owned
   fixed-term renewal.
4. Replace the "Unreserved extra credit" and "Combined speed after allowance"
   rows with a purchased-usage allowance bar, or a throttling animation plus
   opt-in advice when extra traffic is disabled and the allowance is exhausted.
5. Hourly squads and fixed-term plans share one seat stock limit; refusals are
   explained in the member UI, while already-enabled hourly members can always
   pay the next hour.
6. Redesign browser-admin user editing for manual fixed-term grants, refunds,
   and hourly squad inspection/controls.
7. Animate the Usage details measure switch.
8. Animate and block the whole hourly page while the member holds an active
   fixed-term plan.

Two member-review rounds followed the first implementation: the locked/hourly
animations were changed to full-component visuals with less copy, then the
duplicate page title and throttled allowance caption were removed, purchased
traffic was made a standout-green wave segment, a green rocket boost was added
for enabling automatic extra purchases during throttling, and the reduced-speed
card was redesigned as a low-energy, desaturated state.

## Cross-team interfaces

- `HourlyOffer.requiredHours` (`json:"requiredHours"`, decimal JSON integer)
  is the per-offer eligibility hours. `0` inherits `policy.activationHours` for
  pre-existing rows. Effective hours = `requiredHours > 0 ? requiredHours :
  policy.activationHours`; `requiredBalance = hourlyPrice * effectiveHours`.
- Admin offer write adds required `requiredHours` (`0..2562047`). The member
  view keeps `offer.requiredHours` and `requiredBalance`.
- New durable user events: `hourly_funding_warning` (before the paid hour ends)
  and `hourly_funding_suspended` (the hour could not be funded). Facts:
  `squad`, `hourlyPriceMinor`, `balanceMinor`, `time`.
- Admin endpoints:
  - `POST /api/v1/admin/users/{userId}/entitlements` creates a zero-charge,
    audited manual fixed-term grant. Overlapping terms return `409
    ENTITLEMENT_OVERLAP`; an active/preparing hourly enrollment returns `409
    HOURLY_LEGACY_ACTIVE`. Replays remain immutable.
  - `GET /api/v1/admin/users/{userId}/hourly` returns that owner's admin hourly
    projection (policy, balance, enrollments without credentials).
  - `POST /api/v1/admin/users/{userId}/hourly/enrollments/{enrollmentId}/disable`
    queues an administrator disable without a member cooldown.
- Renewal hydration keeps owned squads/add-ons regardless of later
  `billing_mode` edits; storefront visibility still excludes Charge by traffic.
- Shared stock uses one count query (`owned_squads` union active
  `hourly_enrollments`) for both member catalog and hourly views. Settlement
  never consults stock; only new activations refuse with `capacity`.
- Migration 00047 adds `hourly_offers.required_hours`; SchemaVersion is 47.

## Delivery checklist

- [x] 01 Backend: hourly funding warning/suspension notifications.
- [x] 02 Backend: per-offer required hours (migration, API, admin form data).
- [x] 03 Backend: owned renewal selection independent of billing-mode edits.
- [x] 04 Backend: unified stock projection and next-hour continuity.
- [x] 05 Backend: admin manual entitlement, admin hourly projection/disable.
- [x] 06 Member hourly: bar replacement, throttling animation, active-plan block.
- [x] 07 Admin frontend: redesigned user editing plus hourly controls.
- [x] 08 Dashboard: animated Usage details measure switch.
- [x] 09 Static checks, browser validation, docs, commit, push, CI.

Delivery status: implementation, static gates, browser validation, documentation
and the local commit are complete. Pushing to GitHub Actions is blocked in this
execution environment: outbound HTTPS to github.com fails TLS credential
negotiation (`SEC_E_NO_CREDENTIALS`) and no git credential/token is available.
Hosted CI therefore remains pending for the commit created here.

## Implementation evidence

### Backend

- `internal/platform/database/postgres/migrations/00047_hourly_offer_required_hours.sql`
  adds the per-offer threshold; `postgres/open.go` and `postgres/contract.json`
  track schema 47.
- `notification_hourly.go` emits one warning per enrollment and paid-through
  boundary inside the existing notification scan, and the settlement
  transaction emits the definitive suspension notice. Copy is localized EN +
  zh-CN and covered by hosted notification formatting tests.
- `hourly_validation.go` applies the per-offer threshold with overflow and
  range checks; the member view and admin offer API expose `requiredHours`.
- `renewalAddonsTx`, `renewalComboTx`, and owned catalog hydration retain every
  purchased squad/add-on after later merchandising edits; storefront and
  new-sale visibility are unchanged.
- `catalog_stock.go` and `hourly_stock.go` share `sharedStockRemaining` and the
  same distinct-member count; hourly stock is projected for disabled offers too.
- `admin_entitlement_create.go`, `admin_user_hourly.go`, `hourlybilling/admin.go`
  and the admin HTTP routes add the manual grant, admin hourly projection, and
  administrator disable without weakening member authorization.
- Hosted regressions were authored for the new notification, grant, stock,
  renewal-retention, and per-offer-hours behavior. They were compiled by
  `go vet` but not executed locally, per repository policy.

### Frontend

- Member hourly page: duplicate title removed, whole-page animated lock state
  for an active fixed-term plan, one purchased/allowance bar whose extra-used
  segment keeps the normal accent base with clipped standout-green sine wave
  lines, a single conditional legend row (purchased-extra-left when exhausted,
  otherwise remaining), a larger green rocket on the reduced-speed card before
  the bar swap, a low-energy reduced-speed visual, per-squad required hours,
  capacity/next-hour continuity copy, and the funding-suspended notice.
- Admin: redesigned Entitlements workspace with manual grant, edit, and refund
  actions; administrator hourly squad table with disable; per-offer required
  hours; corrected Fixed-term plan / Charge by traffic billing-mode labels.
- Dashboard: Usage details measure switches tween for 320 ms with frozen
  reduced-motion and hidden-measure states while keeping the same mounted SVG.

## Verification

Local static gates:

- `go build ./...` and `go vet ./...` pass from the repository root.
- `gofmt -l` on every changed/new Go file is clean.
- `npm run typecheck`, `npm run lint`, and `npm run build` pass from `web/`.
- `npm run generate:api` regenerated `generated.ts` and
  `responseSchemas.json`; the generated contract is committed.

Browser evidence (Vite dev server, constructed fixtures):

- Usage breakdown: `?hourly-idle` hidden/re-show, `?unallocated` attribution
  message, `?large` fractional arcs; same SVG, 220 px, ~320 ms arc tween,
  console/network clean at 390 px and 1440 px.
- Member hourly: `?out-of-allowance` (no caption, depressed animated card),
  `?extra&out-of-allowance` (accent-base purchased segment with clipped green
  sine wave lines and only "Purchased extra left"), `?extra` within allowance
  (only "Remaining"), confirm-enabling automatic extra traffic during reduced
  speed (larger rocket over the reduced-speed card, static reduced-motion flash,
  then the purchased bar), `?suspended`, `?capacity`,
  `?capacity-preparing`, `?legacy`, `?first`, `?offer-hours`, `?zh`, `?light`,
  reduced motion, 390 px and 1440 px with no horizontal overflow.
- Admin hourly: `?admin` per-offer required hours default 168 and save
  round-trip to 24 hours / 1.20 TXB required.
- Admin user profile: `validity-audit.html?profile` manual grant (success
  notice and new row), refund (queued receipt), hourly disable (queued receipt),
  `?overlap` conflict copy retained in the open dialog, EN and zh-CN at 390 px
  and 1440 px.
- Browser console and request failures were clear after each inspected flow.

These are fixture/browser checks, not hosted backend or production-data
regressions. Hosted CI remains the execution gate for the authored regression
suites.
