# Coupons module

## Ownership and interfaces

Coupons owns canonical definitions, redemption quotas, member wallet grants, purchase eligibility, discount quoting, one-time consumption, and immediate balance effects. Members list their wallet with `GET /api/v1/coupons/wallet`, soft-discard a wallet grant with `DELETE /api/v1/coupons/wallet/{id}`, and redeem a code with idempotent `POST /api/v1/coupons/redeem`. Administrators create, update, list, and deactivate definitions under `/api/v1/admin/coupons`.

Codes are trimmed, uppercased, and restricted to 4–64 ASCII letters, digits, `_`, or `-`. A coupon may expire, have global and per-user redemption limits, and optionally target combo and squad product IDs. Eligibility is checked from the server-owned catalog snapshot rather than browser labels.

## Effect and checkout invariants

The supported definition kinds are:

- `purchase_recurring`: a reusable wallet grant until expiry or quota exhaustion.
- `purchase_once`: a wallet grant consumed by one committed purchase.
- `balance_add`: immediately credits a fixed number of TXB minor units.
- `balance_multiply`: stores a basis-point factor above `10000` and credits `floor(currentBalanceMinor * factorBps / 10000) - currentBalanceMinor`.

Purchase grants use either a fixed minor-unit discount or a basis-point percentage with an optional fixed cap. A quote receives the exact member, grant, combo, selected paid add-ons, and gross server price. The discount cannot exceed gross price. Checkout accepts at most one explicitly selected grant and persists `couponGrantId`, gross price, discount, and net debit on the purchase.

The coupon applicability check, quota/consumption update, purchase insert, TXB debit, and ledger entry share one PostgreSQL transaction. A one-time grant is consumed only when that transaction succeeds. Recurring grants remain available subject to expiry and limits. Rollover snapshots use the net TXB actually debited after discount.

Immediate balance coupons use immutable semantic ledger references and idempotent redemption records. Concurrent redemption cannot exceed a global or per-user quota. Deactivated or expired definitions cannot issue new grants; an existing grant remains subject to its snapshotted/linked definition rules at use time.

Member discard is an idempotent soft state stored separately from a grant. It
removes the grant from wallet, quote, and checkout surfaces without deleting
redemption or purchase records. An administrator's DELETE action remains
definition deactivation rather than destructive deletion, preserving the same
historical references.

## Verification

- Domain tests cover canonicalization, fixed and capped-percentage formulas, factor bounds, and eligibility combinations.
- PostgreSQL tests cover global/per-user quota races, duplicate redemption, balance add/multiply atomicity, one-time checkout consumption, recurring reuse, rollback, and no-stacking enforcement.
- HTTP and Vue tests cover explicit grant selection, ineligible/expired explanations, decimal-string money, and price changes when a selected grant is removed.
