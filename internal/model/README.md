# Shared models

- `payg.go` defines PAYG availability independently of catalog visibility. Hidden
  squads retain access; unavailable squads suppress only PAYG authorization.

- `purchase_addons.go` defines the server-priced active-term squad-addition quote payload.

- `model.go` documents the dependency-free shared model package.
- `identity.go` defines Telegram identity and local user representations.
- `commerce_foundation.go` defines the global Add TXB bounds returned to members and administrators.
- `commerce_renewals.go` retains legacy internal renewal transport types separately from the automatic-renewal contract.
- `automatic_renewal.go` defines the owner-facing automatic-renewal state, current one-cycle price, and dashboard failure notice.
- `finance.go` defines ledger, payment, masked payment-profile, refund, and terminal-payment courtesy-credit payloads plus canonical TXB formatting.
- `analytics.go` defines member traffic, date-bounded per-node usage, and administrator statistics payloads.
- `operations.go` defines backup, audit, outbox, and safe setting views.
- `provider_operations.go` defines durable provider-operation receipts and item states shared across workflows.
- `statistics_dashboard.go` defines the persisted member-facing statistics dashboard projections.

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

`account_lifecycle.go` defines the private generation-fenced purge target. Lifecycle status/generation/block flags in User are internal and never serialized to browsers.

## Hourly accounting

`hourly_credit.go` defines internal owner/generation/day byte-credit obligations,
directional progress and immutable proof partitions. These contracts use exact
integers; public transports must select approved fields and encode large counts
as decimal strings. Financial obligations outlive operational account closure.

`hourly_policy.go` defines explicit administrator units, disabled-by-default
policies and independent squad offers. `hourly_commerce.go` defines activation
scopes, immutable hour/GB receipts and opt-in price consent. Principal IDs and
lifecycle generations stay private; large public counts use decimal strings.
`Purchase.PurchasedTermSeconds` records future original purchased validity;
older terms are neither inferred nor rewritten.

`hourly_validation.go` shares strict policy units and overflow-safe activation
eligibility across administration and wallet entry points. Eligibility does not
reserve or debit the threshold amount.

`hourly_view.go` defines owner-scoped catalog, readiness and committed daily
traffic projections without exposing credentials or live rate allocations.

`hourly_validation_test.go` covers disabled defaults, explicit invalid units,
duration and eligibility multiplication bounds, and exact decimal-string JSON.

`account_cleanup.go` defines private bot-health, durable probe and classified
response contracts. They carry numeric bot identity, account generation and
nonce metadata; raw Telegram responses, tokens and account-existence claims are
not persisted or exposed by these types.

`usage_breakdown.go` defines the owner Usage Breakdown contract: decimal-string
traffic and actual charges, reduced rational minor amounts, explicit unknown
legacy terms, unallocated money and truthful forward coverage. Named hourly
squads have separate group identities from physical Combo/PAYG nodes. Positive
entry counts include both before the shared response limit; the deprecated
positive-node counts retain their physical-node meaning. Account generations
and native credentials stay private.
