# Active-term squad additions

## Scope

Members can add visible optional squads to their active VanSail Carpool purchase. The dashboard opens the mobile-safe two-step `UModal`: choose squads, then checkout and in-place confirmation. A queued successor disables the action because its term already owns the next renewal boundary.

## Contract and pricing

`POST /api/v1/purchases/{id}/addons/quote` returns the authoritative price. `POST /api/v1/purchases/{id}/addons` requires `Idempotency-Key` and applies the same server-side validation. Each squad is priced as `min(fullPrice, ceil(fullPrice * remainingTerm / originalComboValidity))`; administrator extensions therefore cannot exceed the full squad price.

The request validates owner, active unexpired term, visible upstream-present squad, duplicate or already-held squad, activation code, current stock, and absence of a separate queued term. `stockHeldByCurrentUser` is derived only in the catalog response from the active entitlement, so a member holding a stock reservation can retain it for renewal or a normal queued purchase even after the capacity is reduced or new sales are closed. The server checks the held reservation inside the transaction; other members and newly selected squads still require spare capacity.

## Durability

The commit transaction debits TXB, inserts one immutable ledger entry and an idempotent adjustment record, creates the `purchase_addons` rows, updates the purchase totals used by rollover, and enqueues `remna_sync_user`. It never calls Remnawave or Emby directly. Existing renewal planning reads the updated add-on rows, so the next cycle charges their normal full-term price.

## Ownership

- `internal/catalog/squad_additions.go` owns service-level request validation and local catalog checks.
- `internal/platform/database/billing_purchase_addon_*.go` owns calculation, replay protection, and the transaction.
- `internal/httpapi/purchase_addons.go` owns transport mapping.
- `web/src/components/squad-addition/` owns the dialog flow; `useSquadAddition.ts` owns its client state.

## Verification

CI covers rounding and caps, extension behavior, replay/no-double-debit, outbox enqueue, rollover and automatic-renewal totals, queued-term blocking, picker filtering, and owner-held full-squad selection. Local tests are intentionally not part of this release procedure.
