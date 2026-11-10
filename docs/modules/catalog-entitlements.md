# Catalog, accounting and entitlements

## Ownership and interfaces

The catalog module owns Combo definitions, sparse squad merchandising, server-priced quotes, coupon-aware checkout, active/queued terms and automatic renewal. Native X-Node owns proxy identities, credentials, accounting periods and desired access. Database transactions own money and immutable purchase/refund evidence. External work uses the existing durable queue.

Member interfaces include `/api/v1/catalog`, `/api/v1/purchases/quote`, `/api/v1/purchases`, purchase add-on quote/commit resources, `/api/v1/dashboard` and `/api/v1/dashboard/node-usage`. Owner-only automatic-renewal GET/PUT resources expose next-cycle amounts, dates, eligibility and preference. Admin user price/prediction resources are scoped to the browser administrator/API-token boundary.

## Catalog and checkout

Combos contain a positive traffic limit, validity, reset cadence (`DAY`, `WEEK`, `MONTH_ROLLING`), included squads and a nonnegative TXB reference. Squad records retain only authored merchandising, billing mode and profile overrides; authoritative native identities remain references. PAYG uses TXB per GB and explicit normal, hidden, unavailable and upstream-missing presentation.

Activation-required squads remain visible. Checkout validates codes for selected gated squads, including included squads; raw codes remain memory-only and stored codes are bcrypt hashes. Validated recurring access is reused for renewal. Stock counts distinct entitled members across one shared seat count: fixed-term owned squads plus active hourly enrollments. The member catalog and hourly view use the same count and the same `max(0, limit - count)` projection; the hourly view publishes `stockRemaining` whenever a limit exists (enabled or disabled offer) and applies limits independently of billing mode. The exposed `capacity` refusal reason explains a blocked new activation only.

An owned renewal selection is independent of later merchandising edits. Changing a squad product's `billing_mode` to `payg`, hiding it, or re-pricing an hourly offer never removes the squad or add-on from an already-purchased fixed-term plan: renewal hydration, `renewalComboTx` and `renewalAddonsTx` retain purchased identities and price them from the current override or inherited pricing contract. The same rule covers the combo itself: hiding it (`active=0`) retires it from new sales only, and an owned term still renews against its own combo row. Renewal resolution prefers a manual/draw full-entitlement override, then the purchase's recorded base bundle snapshot, then the live bundle; the purchase snapshot therefore keeps the purchased included-squad identities when the live bundle is edited, while a truly missing combo row still fails closed. An administrator combo replacement is the explicit exception: it clears the recorded snapshot so the replacement's live bundle becomes the owned baseline. Renewal prices remain current authoritative prices (a pricing contract, when present, preserves the purchased price and ceiling basis). Storefront visibility still excludes hidden combos and PAYG products from standalone add-ons and new sales.

Quotes and commit revalidate selections, availability, applicability, decision versions, effective dates and authoritative prices. Successful checkout atomically consumes an explicitly selected coupon, debits sufficient funds, records purchase/add-on and ledger facts, and queues access synchronization. Failed checkout changes neither money nor renewal preference. Idempotency keys and unique source-successor relationships prevent duplicate charges.

A future Combo requires terminal current-renewal and selected-future decisions. Fixed pricing or an explicit refused adjustment produces an unchanged terminal decision; pending, stale or failed work does not admit checkout. PAYG-only users have no current-renewal prerequisite. Successful queued checkout atomically disables current auto-renewal. Live squad additions remain available, use prorated coupon-aware settlement and invalidate affected renewal decisions.

## Pricing and forecasts

Numerical snapshots exclude current balance and funding eligibility. Applicable coupon scope, amount, cap, expiry, recurring applicability and cohort are model inputs. Higher gross candidates are evaluated against revenue after the normal coupon. Insufficient evidence produces an explicit unchanged decision; funding failures are censored separately from price rejection.

PAYG eligible rates refresh every minute with reusable trained models. Changes apply to future prepaid blocks; existing prepaid bytes retain their purchase basis. Pricing and forecast kernels run through the durable compute lane. Nodes return bounded validated numerical results; only master transactions publish prices or settle money. See `internal/pricing/README.md` and the pinned X-Node compute protocol.

Administrator user-detail traffic forecasts show the last two completed UTC accounting days and five complete future days. Missing history remains unavailable. Empirical intervals, observed node-day coverage and measured original-vintage calibration are distinct. These analytical views do not select an individual base price. Member Home renders purchases directly and does not request personal forecasts or display the retired prediction flip card.

## Renewal and purchase lifecycle

Enabling auto-renewal does not require sufficient balance. Availability, stock and successor constraints still apply. Settlement checks funds atomically, records one successor and ledger debit, inherits eligible recurring coupon terms and queues native activation. Insufficient funds records a funding failure and never becomes a model price rejection. Renewal settlement has no member notice-delivery dependency.

A fixed-term renewal keeps the squads and add-ons it already owns even after their products are re-marked PAYG and regardless of later hourly-offer changes. Only a missing live upstream identity or unavailable node can remove a retained squad; a later display edit never changes an owned renewal. Retiring a combo (`active=0`) has the same meaning: it closes the product to new sales but neither blocks nor rewrites an owned renewal, including when the live bundle was edited afterwards. Hourly settlement also never consults seat stock, so an already-active hourly member can always pay the next hour when the shared limit is full; only a new hourly activation can be refused for `capacity`.

Purchase expiry closes accounting periods and promotes an already paid successor through the normal lifecycle. Unfinished periods remain available for settlement; settling a predecessor must never revoke its successor. Native account-wide reset/desired-state phases and durable node receipts retain their existing idempotency guards. Purchase-wide refund calculations combine native periods with reconciled historical usage once and retain evidence across resets.

Operational unused-traffic credits have been retired. Forward PostgreSQL migration 18 cancels unsettled work without crediting wallets, archive settled audit facts, and remove operational tables, columns, settings and jobs. Historical migrations and previously settled financial/refund facts remain immutable.

## PAYG provisioning and owner exports

Onboarding, balance changes and bounded repair work ensure identities, encrypted subscription credentials, per-owner/squad/node accounts, periods and node synchronization. Reconciliation immediately reloads newly created accounts. Links and inventory do not depend on a Combo or sufficient funds; connection admission and actual purchases retain funds, prepaid-credit, readiness and restriction checks.

Owner host-name writes authorize an accessible exported host identifier. Empty input removes an override. `{price}` expands to the exported principal's current PAYG rate (`0.50 TXB/GB`) with no Combo multiplier label. Published identifiers and collision handling remain stable. Page links, QR output and supported downloads share the export boundary; applied route/split metadata is explicit.

## Validation and extension points

Hosted regressions cover forward upgrades without wallet credits, historical accounting, successor continuity, missing PAYG credentials, concurrent pricing admission, coupon scope, duplicate charging, prepaid-rate preservation, compute retries, owned-renewal retention after billing-mode/hourly-offer edits, the single shared seat count, and next-hour settlement at a full seat limit. Static Go/frontend/schema checks run locally; automated suites run only in hosted CI. Chrome DevTools validates member/admin interactions at 320 px, 390 px and desktop widths.

New billing products extend existing typed policies, immutable evidence and quote/commit boundaries. New numerical algorithms require a versioned shared kernel, bounded inputs/results and master-side validation. Never add direct provider calls, wallet features to pricing, duplicate native inventory, or an independent financial queue.
