# Hourly commerce persistence

This module owns wallet-backed activation, renewal, voluntary stops and decimal-GB
funding. It performs SQL only. Vault operations, native preparation, native
readiness, presentation and the existing job executor belong to their services.
Migration 43 introduces empty hourly relations; no legacy purchase converts.

`PrepareHourlyActivation` creates an unpaid `preparing` enrollment. The native
service then attaches its immutable principal and `NO_RESET` period.
`CommitHourlyActivation` rechecks the exact tuple, reviewed encrypted-policy
fingerprint, offer revision, existing squad activation code, stock, legacy
benefits, cooldown and activation balance threshold before the first debit.
Activation codes are never stored by this module or included in fingerprints.
Queued services must keep them encrypted until the operation finishes.

Every financial path locks the user lifecycle before the wallet. Existing stock
locks and ordered enrollment rows precede shared-day accounting. Settings stay
encrypted: a validated policy snapshot supplied by the service is compared to
the locked stored ciphertext fingerprint. Services must not accept snapshots or
delivery evidence from member request bodies.

`SettleHourlyOwnerAvailable` consumes fresh server-native delivery guards and
charges due squads in immutable activation-time, then group-ID order. Missing,
expired or changed-revision evidence skips only that squad. PostgreSQL time after
lock waits determines the charged interval, rounded upward in integer minor
units; missed hours are never charged. Paid authorization ends at the receipt's
whole UTC hour even when workers are delayed. A transaction crossing its entire
paid window or readiness TTL rolls back. Receipt replay never grants a terminal
activation again. The compile revision is checked once before own mutations,
since those same mutations advance the global native input revision.
The same revision row serializes a global 8,192-current-owner admission limit.
Ongoing overdue/offline active owners retain their place; existing owners can
add squads without consuming another account slot. Historical generations do
not occupy a current place. `hourly_billing_capacity.go` owns this gate and
`hourly_billing_capacity_test.go` covers its boundary with a small test limit.

`DisableHourlyEnrollment` retains the already-paid charge and adds cooldown only
for a voluntary stop of a paid activation. `CancelHourlyPreparation` additionally
proves there is no paid receipt before cancelling an unpaid intent.
`DueHourlyOwnersPage` lets the existing scheduler rotate past offline owners.
Owner-credit retirement must be guarded against a newer paid benefit under the
same lifecycle lock; it must not run as an unqualified late disable callback.

Extra traffic defaults off. `SetHourlyExtraPreference` captures current-price
consent. `FundHourlyExtraGB` requires an active paid squad, fresh positive payload
demand and confirmed exhaustion of both free and previously funded credit, with
no reserved or unknown remainder. The wallet debit, immutable receipt and exactly
1,000,000,000 newly funded bytes commit together. A higher price requires new
consent; a lower price charges the actual lower price. Credit belongs to its UTC
day and expires at the next midnight; disconnect never refunds unknown bytes.

Implementation entry points:

- `hourly_billing.go`, `hourly_billing_read.go` and
  `hourly_billing_delivery.go` define bounded inputs, reads and readiness checks.
- `hourly_billing_prepare.go`, `hourly_billing_activation.go` and
  `hourly_billing_cancel.go` manage the two-phase unpaid/paid transition.
- `hourly_billing_settlement.go`, `hourly_billing_receipts.go`,
  `hourly_billing_proration.go` and `hourly_billing_disable.go` own billing windows.
- `hourly_billing_preferences.go`, `hourly_billing_extra.go` and
  `hourly_billing_extra_read.go` own consent and atomic funded-byte purchases.
- `hourly_billing_legacy.go` and `hourly_billing_stock.go` preserve reciprocal
  legacy admission and shared capacity. New checkout/manual/automatic renewal
  inserts snapshot only `purchased_term_seconds`; historical NULLs remain NULL.

Hosted regressions are in `hourly_billing_fixture_test.go`,
`hourly_billing_extra_fixture_test.go`, `hourly_billing_activation_test.go`,
`hourly_billing_admission_test.go`, `hourly_billing_settlement_test.go`,
`hourly_billing_disable_test.go`, `hourly_billing_delivery_test.go`,
`hourly_billing_paging_test.go`, `hourly_billing_receipts_test.go`,
`hourly_billing_legacy_test.go`, `hourly_billing_extra_test.go`,
`hourly_billing_extra_boundaries_test.go`, `hourly_billing_wallet_test.go`,
`hourly_billing_renewal_test.go` and `hourly_billing_proration_test.go`.
