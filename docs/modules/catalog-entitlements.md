# Catalog and entitlements module

## Ownership and interfaces

This module owns live combo definitions, sparse per-X-Node-squad merchandising overrides, server-side quotes/pricing, coupon-aware purchases, active/queued access terms, rollover settlement, statistics, and durable commands that make X-Node match local entitlement state. It consumes the transactional balance interface and a narrow access-synchronization interface; it does not call HTTP providers directly.

Members read `/api/v1/catalog`, quote at `/api/v1/purchases/quote`, create/list `/api/v1/purchases`, see entitlement projections on `/api/v1/dashboard`, and can read bounded per-node traffic at `/api/v1/dashboard/node-usage` with inclusive UTC `start`/`end` dates. The active ride's rollover detail is fetched on demand from `/api/v1/purchases/{id}/rollover`; it requires the authenticated owner, returns only aggregate cadence windows, and never persists raw provider statistics. The owner-only `GET` and `PUT /api/v1/purchases/{id}/auto-renewal` resource returns the authoritative one-cycle price, eligibility, dates, and toggle state. Administrators manage combos and sparse squad overrides, inspect combo/squad statistics, inspect entitlements, and issue audited cancellation commands. X-Node owns accessible-node resolution.

## Catalog and pricing invariants

- Combos use a strict minimum-remaining rollover threshold and have no configured TXB rollover cap. Forecasts use actual usage divided by elapsed term time, projected across the full term, and report the maximum allowable usage plus required total/daily reduction when ineligible.
- Activation-required squads remain visible with a localized badge. Checkout prompts sequentially for every selected gated squad, including combo-included squads; raw codes stay memory-only and the database stores only bcrypt hashes. Previously validated renewal access is reused without another prompt.

- A combo has a positive traffic limit, `DAY`, `WEEK`, or `MONTH` reset cadence, positive validity in days, a nonnegative TXB minor-unit price, included internal squads, and a strict rollover threshold without a TXB credit cap.
- "Free nodes" are represented only by included X-Node internal squads. Node IDs are not stored as a second access model.
- Squad identity/name/availability comes from every live X-Node list call. `squad_product_overrides` stores only edited Markdown, typed profile metadata, term price, and visibility keyed by upstream UUID; restoring defaults removes the row.
- A public catalog contains only active combos whose included squads are all upstream-present, plus visible upstream-present add-ons. A stale selection is revalidated in the purchase transaction before any debit. Historical records are archived rather than deleted when purchases refer to them.
- The public catalog exposes enabled node display metadata (UUID, name, country, and consumption multiplier) for Home's usage distribution bar. The purchase quote's accessible-node projection additionally carries the live provider name/favicon URL; all such metadata is read-only, display-only, and never persisted locally.
- Included squads are selected explicitly in combo administration. They are omitted from the member's optional-squad step because the flow chooses paid squads before the core combo; a combo change still immediately prunes newly included squads from the paid selection.
- Only paid add-ons whose authoritative `stockRemaining` is zero are shown as disabled gray `Full` choices in the member catalog. Combo-included squads are supplied by the selected combo rather than shown as optional choices and rely on the existing server-side validation.
- Combo and squad descriptions use Markdown with raw HTML disabled and sanitized link protocols. The closed `[text]{color=... size=...}` directive accepts only the documented six color and four size tokens and emits classes; unsupported directives remain text. Administration uses the same renderer for preview.
- The server reloads all selected records and calculates the final gross and coupon-adjusted net TXB price in one transaction. Browser totals are display-only.
- Included upstream squad UUIDs are validated JSON on `combos`; paid add-ons alone are retained in `purchase_addons`. A purchase retains charged/gross/discount facts, coupon relationship, validity, status, user, idempotency, and one live `combo_id`. Combo edits intentionally change active, queued, and historical behavior/projections and enqueue one canonical user-sync job per affected member. Referenced combos can only be hidden.

## Purchase and term behavior

A member has at most one active combo and one effective account-wide traffic budget/expiry. A first purchase starts immediately and defaults automatic renewal on. Selecting a different combo or changing add-ons creates a queued term starting exactly at the current term end and defaults automatic renewal off; there is no proration or mid-term squad replacement. While an active purchase has automatic renewal enabled, catalog quote and purchase requests are rejected to prevent conflicting terms.

The read-only quote and the creation transaction both revalidate the live combo, coupon/grant, selected add-ons, active term, effective date, and upstream squad identities. Creation then atomically commits coupon use, balance debit, purchase/add-on facts, pending extension-credit consumption, ledger entry, and canonical outbox operation. Insufficient TXB returns `409 INSUFFICIENT_BALANCE`; no purchase is visible without its matching debit and synchronization job.

The first purchase provisions the reserved X-Node username through the queued worker. A future queued term creates a disabled remote account immediately; activation later applies the term. Confirmed remote deletion repairs the same username and current entitlement. A username claimed by another Telegram identity cancels and fully refunds all current and queued paid terms, including charged add-ons, then returns the member to username onboarding.

Combo and internal-squad statistics accept a bounded date range, IANA timezone, and daily/weekly bucket. They report unique buyers, purchase count, charged/discount/add-on totals, series, and included-versus-add-on distribution from authoritative purchase facts plus current live combo references.

When a term activates, synchronization persists three phases: remove all squads to quiesce access, reset usage while access is quiesced, then replace the complete internal-squad list, apply the account-wide limit/reset strategy, and set X-Node `expireAt` to the term's local `validUntil`. A crash or ambiguous reset response repeats only a phase safe while no traffic can accrue; a retry after final apply never resets again. At final expiry, synchronization sets the upstream identity DISABLED, clears squads and traffic entitlement, and preserves the disabled-user far-future expiry; only a later active term restores ACTIVE. The local term remains authoritative.

Automatic renewal creates exactly one queued successor from the current ride's combo and paid add-ons. The scheduler runs it before expiry transitions, refreshes provider-dependent availability through the queue, and atomically debits the one-cycle current price, creates the successor, and links it to the source purchase. The unique source-successor link makes retries idempotent; normal rollover then quiesces the expiring term and activates the queued successor in the existing outbox order. Existing purchases remain opt-in after migration. Enabling validates balance, absence of a queued successor, accessible nodes, and paid add-on stock. Stock counts distinct members: an active, activating, or queued member retains an existing squad seat even if its limit is lowered below occupancy or set to zero. Only selections without a held seat must have spare capacity; expired and cancelled terms release their seats. If renewal cannot charge because of balance, combo/node availability, or unavailable capacity for a released seat, it charges nothing, disables automatic renewal, records a stable failure code for the dashboard, and normal expiry removes access. An insufficient-balance failure is special-cased: when the expiring term still has a pending or in-flight rollover whose credit could cover the renewal, the scheduler defers the attempt (records nothing, keeps automatic renewal enabled) and retries after the next rollover settlement tick; availability/stock failures disable immediately and never wait for rollover.

An attached recurring discount is persisted when the original purchase is made (and legacy rows are backfilled only from an already-linked recurring grant). Automatic renewal keeps that relationship even if the member later discards the wallet grant. It uses only the grant definition's latest discount mode, value, and percent cap; it deliberately ignores later coupon active state, expiry, quota, eligibility, and kind changes, and writes no coupon-use record. One-time coupons never attach.

Renewal quotes and debits resolve each purchased optional squad by its X-Node UUID at the current override price. Storefront visibility controls new purchases only; hiding or repricing an owned squad does not invalidate its renewal. Removing a sparse override restores the default zero price. Every retained squad must still exist upstream, and stock and current-price balance checks still apply. The debit transaction reloads prices, preserves original purchase charges, and stores the successor's actual charge. Stock-ineligible quotes still display the full current price and recurring discount.

Members may cancel only their own queued purchase through `POST /api/v1/purchases/{id}/cancel`. The local transaction requires `status='queued'`, marks the purchase cancelled, credits the snapshotted charged TXB amount, and appends one `purchase_cancellation` ledger entry. Because the entitlement has not reached X-Node, this path performs no provider call and no upstream job is needed; cancellation also releases the local stock reservation while retaining immutable purchase history.

## Rollover ordering and formula

Expiry first queues `rollover_finalize` and blocks renewal activation. The worker quiesces old access before fetching the upstream reset strategy, last reset timestamp, and bounded daily sparkline usage. It retains only aggregate allocation, used bytes, eligible unused bytes, and the algorithm version.

- A zero traffic limit awards zero.
- Remaining percentage must be strictly greater than the snapshotted `rolloverMinRemainingBps` threshold.
- An eligible award is `roundHalfUp(netPaidMinor * remainingBytes / allocatedBytes)` to the nearest TXB cent, with no configured TXB cap.
- The traffic inputs, result, old-term expiry, optional ledger credit, and next activation command commit atomically. Only then may reset/activation run.
- Transient X-Node failures retry without resetting traffic. A confirmed missing user records a zero-credit exception instead of assuming all traffic was unused.
- Purchase ID is the rollover credit's unique semantic reference; replay cannot credit twice.
- Cadence-aware settlement derives `DAY`, `WEEK`, `MONTH`, and `MONTH_ROLLING` allowances and weighted usage from bounded per-node daily buckets, then sums both across the entire combo. Reset intervals affect those traffic totals only. The strict remaining threshold is evaluated once over the full term; when it passes, `eligibleUnusedBytes` is the full remaining allocation. Already credited historical records are unchanged, while calculated but uncredited records are normalized during renewal.
- The live rollover projection forecasts from actual usage over elapsed term time, reports maximum allowable usage, maximum daily usage when the current total is still eligible but the forecast exceeds the limit, and total/daily reduction when needed. It calculates money from the immutable `charged_txb_minor` fact. When automatic renewal is disabled it returns a localized warning state without requesting provider usage.

## Failure behavior

- Invalid, hidden, archived, duplicate, or upstream-missing selections fail before debit.
- A failed due automatic renewal leaves no successor or debit, records a member-visible failure reason, and lets the ordinary expiry path disable access.
- SQLite contention follows the configured busy timeout and returns a retryable conflict without a partial ledger entry.
- X-Node failure after commit leaves the purchase activating/queued or rollover pending and retries through the durable outbox. Access is never reported active merely because the browser request succeeded.
- Replayed activation/expiry jobs converge by replacing the full upstream squad set and desired limit; they do not accumulate squads.
- A queued term cannot be promoted until rollover inputs/result and old-term closure have committed.
- Administrator cancellation appends an audit event and compensating operation. Historical snapshots remain immutable.

## Verification

- Pricing tests cover included squads, paid add-ons, coupon selection, duplicate IDs, archived records, integer overflow, and tampered client totals.
- Transition tables cover first purchase, early/late same-plan renewal, plan/add-on changes, exact boundary, expiry, cancellation, pending extensions, and concurrent purchase attempts.
- Transaction tests inject failures between coupon use, debit, purchase, squads, rollover, ledger, and outbox writes and assert all-or-nothing behavior.
- Fake X-Node tests assert rollover quiesce/read/credit ordering before reset, strict-threshold/no-cap math, confirmed 404 versus transient failure, complete squad replacement, account-wide limit/cadence, idempotent retries, and expiry cleanup.
- Restart tests confirm queued boundaries and failed rollover/activation jobs resume from SQLite without duplicate debits or credits.
- Automatic-renewal tests cover migration defaults and legacy backfill, owner toggle eligibility, concurrent/due retry idempotency, failure-to-expiry behavior, paid-only stock rechecks, the recurring-coupon attachment policy, and the insufficient-balance deferral that waits for rollover settlement (pending vs settled vs unreadable-lookup fail-safe).
