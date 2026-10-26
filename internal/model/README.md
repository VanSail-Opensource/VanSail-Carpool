# Shared models

- `payg.go` defines PAYG availability independently of catalog visibility. Hidden
  squads retain access; unavailable squads suppress only PAYG authorization.

- `purchase_addons.go` defines the server-priced active-term squad-addition quote payload.

- `model.go` documents the dependency-free shared model package.
- `identity.go` defines Telegram identity and local user representations, including the private account-wide automatic-reset flag.
- `commerce_foundation.go` defines the global Add TXB bounds returned to members and administrators.
- `commerce_renewals.go` retains legacy internal renewal transport types separately from the automatic-renewal contract.
- `automatic_renewal.go` defines the owner-facing automatic-renewal state, current one-cycle price, and dashboard failure notice.
- `finance.go` defines ledger, payment, masked payment-profile, refund, and terminal-payment courtesy-credit payloads plus canonical TXB formatting.
- `analytics.go` defines member traffic, date-bounded per-node usage, and administrator statistics payloads.
- `operations.go` defines backup, audit, outbox, and safe setting views.
- `provider_operations.go` defines durable provider-operation receipts and item states shared across workflows.
- `statistics_dashboard.go` defines the persisted member-facing statistics dashboard projections.
- `traffic_reset_automation.go` defines the public account-wide automatic-reset preference projection.

`telegram_boost.go` defines the transient identity payload used by queued boost appreciation.

`telegram_pm.go` defines reference-only PM conversation, delivery, relay, moderation, repair and generated-guidance payloads. Routing joins canonical user metadata rather than persisting another profile dataset.
The `Statistics` projection in `analytics.go` carries nullable upstream `lastTrafficResetAt` without deriving or persisting a reset timestamp.
- `txb_parse.go` owns the shared exact TXB parser used by billing and IP Lookup without package cycles.
`telegram_pm_content.go` defines stable expired/invalid pending-content errors; PMRelayInput carries transient JSON excluded from serialized fingerprints and outbox data.

`SquadProduct` carries optional billing mode and an exact per-GB TXB price without changing existing combo price fields. PAYG public billing models live beside the native service; all money and traffic transports retain decimal strings.

- `personal_forecast.go`: Typed complete-day predictions with exact-byte strings, observed coverage and measured calibration.

- `pricing_decision.go`: Immutable terminal or pending personalized price decision contract.

- `user_pricing.go`: Paginated user-specific price, coupon, and renewal projections.

Canonical catalog, purchase and monetary contracts: `commerce.go`.
