# Coupons

- `coupons.go` defines coupon/grant types, validation, and normalization.
- `coupons_part2.go` contains eligibility and fixed-point discount math.
- `service.go` coordinates definitions, redemptions, wallet reads, member
  soft-discard, and purchase quotes. Administrator handlers use `SaveAdmin` with
  the exact `UpdatedAt` revision loaded before merging partial fields; creation
  uses a zero revision. The store rechecks the current actor and atomically saves
  the definition and audit. Stale merges return a conflict without overwriting a
  concurrent deactivation. Coupon codes are omitted from policy audit details.

Wallet deletion writes a durable discard record rather than removing a grant. Its
redemption and purchase foreign keys remain available for history, while quotes
and checkout reject discarded grants. Administrator definition deletion remains
deactivation, so existing history is preserved there as well.
- `coupons_test.go` verifies coupon validation, arithmetic, and application behavior.
