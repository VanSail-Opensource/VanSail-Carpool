# Billing package
- `events_part2.go` continues the focused implementation from its original package module.

Payment-method discovery, exact decimal arithmetic, checkout creation, provider event validation, and settlement live here.

## Files

- `service.go` defines settings/repository contracts, service construction, and checkout creation.
- `checkout.go` builds server-owned provider requests and validates checkout responses before persistence.
- `payment_operations.go` creates idempotent payment create/cancel commands with their order targets.
- `operation_worker.go` performs each provider mutation once after its durable attempt marker.
- `operation_worker_reconcile.go` resolves interrupted attempts from stored payment state or leaves them pending review.
- `operation_worker_errors.go` extracts bounded provider rejection copy for member-safe failed operation receipts.
- `payment_callback_operations.go` lets authoritative paid callbacks resolve matching checkout operations.
- `payment_announcement.go` delivers immutable successful-payment snapshots to
  the optional standalone Telegram channel through the durable outbox.
- `payment_announcement_format.go` renders the fixed Chinese receipt layout with the administrator provider snapshot, localized rails, shared MarkdownV2 escaping, and Telegram length bounding.
- `payment_announcement_bounds_test.go` covers the Telegram limit after dynamic MarkdownV2 escaping.
- `amount_bounds.go` loads the global inclusive Add TXB range and rejects checkout amounts outside it.
- `amount_bounds_test.go` covers configured inclusive payment boundaries.
- `payment_operations_test.go` covers atomic command queueing, idempotency conflicts, ambiguous outcomes, and callback resolution.
- `payment_announcement_test.go` covers the exact receipt layout, every
  localized rail, legacy fallbacks, MarkdownV2 escaping, blank configuration,
  and retryable Telegram failures.
- `gateway_contracts.go` defines the server-owned provider checkout/event contracts and gateway interfaces.
- `payment_methods.go` builds the configured rail list, including coupon funding and independently selectable provider-account profiles.
- `payment_channel_cache.go` stores the current process-local BEPUSDT currency/network rails discovered by queued readiness probes.
- `events.go` validates and authorizes provider events, settles orders, exposes the narrow signed-return receipt projection, and handles cancellation.
- `payment_profile_runtime.go` gates channels from stable-ID profiles and derives each BEPusdt callback capability from the selected account credential.
- `methods.go` defines and validates canonical payment method identifiers.
- `method_rails.go` owns provider rail names, ordering, and enabled-channel validation.
- `decimal.go` parses and computes exact TXB and provider-currency amounts without floating point.
- `service_test.go` covers checkout creation and provider failure paths.
- `events_test.go` covers event validation, authorization, settlement, and BEPUSDT compatibility.
- `service_test_helpers_test.go` contains shared billing repository, settings, and gateway doubles.
- `methods_test.go` covers documented method identifiers and rail ordering.
- `decimal_test.go` covers decimal parsing, equivalence, and rounding.
- `methods_service_test.go` covers configured method availability, cancellation behavior, and BEPusdt callback capabilities.
- `payment_profile_runtime_test.go` covers account-scoped BEPusdt callback capabilities.
- `zero_coverage_test.go` covers provider channel ordering, channel validation, and EZPay/BEPusdt method-list validation.
- `README.md` documents the package layout.

Exact TXB parsing in `decimal.go` delegates to `model.ParseTXBMajor`; this keeps the same arithmetic available to queued IP Lookup billing without introducing a database/package dependency cycle.

- `builtin_usdt.go` ? Local built-in availability/pricing contract and deterministic create rejection.
- `builtin_usdt_test.go` ? Hosted pricing, rail, callback exclusion and external-profile coexistence checks.
