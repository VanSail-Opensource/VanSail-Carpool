# Admin package
- `connectivity_settings.go` validates atomic connectivity settings through the queued account adapter and cancels obsolete runs after successful writes; `connectivity_settings_test.go` covers generic writes and disabling during upstream outages.
- `settings_part2.go` continues the focused implementation from its original package module.

Audited administrative operations for catalog data, settings, balances, refunds, purchases, backups, and durable jobs live here.

## Files

- `service.go` defines admin-facing dependency contracts, service construction, backup deletion, and settings forwarding.
- `catalog.go` manages combos, typed squad profiles, and upstream squad imports; Remnawave remains authoritative for node accessibility, while administrator projections initialize required node arrays without storing assignments.
- `finance.go` validates balance adjustments, deductions, refunds,
  terminal-payment courtesy credits, and entitlement cancellations; production
  persistence commits their audit and user notification atomically.
- `settings.go` defines editable settings, encrypted storage access, masked payment profiles, safe listing, and readiness checks.
- `payment_profiles.go` validates, encrypts, resolves, and safely deletes provider-account profiles when no open order references them.
- `payment_profile_channels.go` discovers BEPUSDT currency/network rails through the queued readiness probe and disables profiles whose probe fails.
- `payment_profile_readiness.go` maps enabled provider profiles to stable readiness diagnostics.
- `setting_validators.go` contains the individual setting-value validators.
- `mutation_operations.go` creates idempotent administrator outbox-retry, payment-refund, and manual-maintenance receipts.
- `mutation_worker.go` dispatches administrator mutation commands from the shared provider-operation lane.
- `mutation_lifecycle.go` marks administrator operation items and receipts through one bounded lifecycle.
- `refund_worker.go` reconciles Telegram Stars outcomes before applying one local payment refund.
- `user_profiles.go` builds the user aggregate from identity, balance, active and queued entitlements, Emby, payments, refunds, and open operations.
- `user_command_types.go` defines the complete entitlement, combo replacement, and minute-normalized bulk-extension command inputs.
- `user_command_validation.go` normalizes IDs and squads and fingerprints idempotent administrator commands.
- `user_entitlement_commands.go` validates full edits, exactly-once refunds, and no-charge combo replacements.
- `user_bulk_commands.go` validates minute-precise inclusive-OR bulk previews and durable extension jobs.
- `user_operation_resolution.go` validates idempotent audited resolutions for ambiguous provider outcomes without dispatching another provider call.
- `user_operation_worker.go` applies exact local entitlement state through the
  shared provider-operation dispatcher and releases user messages only after a
  real successful provider item.
- `user_account_commands.go` grants and idempotently retires only purchase-discount wallet coupons.
- `service_test.go` covers settings forwarding and catalog operations.
- `mutation_operations_test.go` covers idempotent manual-maintenance queueing and its audit event.
- `finance_test.go` covers balance, refund, cancellation, backup, and retry operations.
- `service_test_helpers_test.go` contains shared admin service test doubles.
- `settings_test.go` covers secret handling, safe listing, readiness, and validators.
- `payment_profiles_test.go` covers provider profile validation, encryption, masking, and readiness behavior.
- `zero_coverage_test.go` covers backup deletion, balance deductions, stable-ID payment-profile lookups and runtimes, legacy EZPay validation, and activity reward boundaries.
- `catalog_test.go` covers optional setting validators.
- `user_profiles_test.go` covers aggregate selection, ordering, and synchronization projection.
- `user_profiles_test_helpers_test.go` contains focused aggregate-profile repository doubles.
- `user_operation_worker_test.go` covers ambiguous provider outcomes and partial bulk completion.
- `user_operation_worker_test_helpers_test.go` contains reusable provider-operation worker fixtures.
- `README.md` documents the package layout.

Provider profiles have stable IDs, separate endpoint, credential, merchant ID,
provider name, and acknowledgement values. Channel selection only controls
which rails are offered for that account. Balance adjustments create unique
ledger references so repeated reasons remain separate audited operations.

- `settings_part2.go` contains the remaining settings validation and persistence helpers.
`user_refund_quote.go` derives an editable administrator refund default from the net debit and observed whole-term weighted traffic. `user_refund_quote_test.go` covers discounts, custom limits, reset periods, cent rounding, queued terms, outages and ownership.

`turnstile_settings.go` and `turnstile_settings_test.go` require both keys before enabling, reject clearing an enabled site key, and use the existing encrypted secret registry. Setting writes serialize cross-key validation.

`pm_settings.go` validates the default-off PM toggle and forum destination. Composition injects a queued forum/permission validator; enabling and changing an enabled destination require a usable forum and bot manage-topics rights.
`pm_settings_test.go` covers fail-closed enabling, destination validation and disable-before-clear behavior.
- `ip_lookup_settings.go` validates atomic IP Lookup settings, preserves independent check/geolocation ordering and protects write-only credentials. `ip_lookup_settings_test.go` covers encrypted credentials and ordering round trips without altering saved configuration on invalid input.
- `ip_lookup_settings_test.go` covers credential encryption, keeping, clearing and generic-route rejection for CI.

- `usdt_settings.go` ? Encrypted USDT configuration registration and validation.

`xnode_settings.go` Registers encrypted fleet configuration with typed validation.

`user_provider_commands.go` validates manual temporary restrictions; `user_provider_worker.go` executes queued changes. `user_account_commands_test.go` covers coupon, refund and restriction command validation.
