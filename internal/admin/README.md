# Admin package
- `connectivity_settings.go` validates atomic connectivity settings through the queued account adapter and cancels obsolete runs after successful writes; `connectivity_settings_test.go` covers generic writes and disabling during upstream outages.
- `settings_part2.go` continues the focused implementation from its original package module.

Audited administrative operations for catalog data, settings, balances, refunds, purchases, backups, and durable jobs live here.

## Files

- `service.go` defines admin-facing dependency contracts, service construction, backup deletion, and settings forwarding.
- `catalog.go` manages combos, typed squad profiles, and upstream squad imports; X-Node remains authoritative for node accessibility, while administrator projections initialize required node arrays without storing assignments.
  The compatibility import endpoint reads and audits the live list; the retired
  identity-mirroring refresh/mark-missing repository contracts are removed.
- `finance.go` validates reference-bound balance adjustments and deductions,
  terminal-payment courtesy credits, atomic entitlement cancellations, and backup requests.
  Payment refunds and outbox retries use the durable mutation queue.
- `settings.go` defines editable settings, encrypted storage access, masked payment profiles, safe listing, and readiness checks.
- `payment_profiles.go` validates, encrypts, resolves, and resolves provider-account profiles. Retained order references protect account identity and credentials from retargeting or deletion.
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
- `user_entitlement_commands.go` validates manual grants, full edits, and exactly-once refunds.
- `user_combo_replacement.go` recovers accepted no-charge replacement intents before current catalog validation.
- `user_bulk_commands.go` validates minute-precise inclusive-OR bulk previews and durable extension jobs.
- `user_operation_resolution.go` validates idempotent audited resolutions for ambiguous provider outcomes without dispatching another provider call.
- `user_operation_worker.go` applies exact local entitlement state through the
  shared provider-operation dispatcher and releases user messages only after a
  real successful provider item.
- `user_account_commands.go` grants and idempotently retires only purchase-discount wallet coupons.
- `service_test.go` covers settings forwarding and catalog operations.
- `mutation_operations_test.go` covers idempotent manual-maintenance queueing and its audit event.
- `finance_test.go` covers reference-bound balances, courtesy credits, atomic cancellation, and backup operations.
- `mutation_queue_test.go` covers queued refund/retry intent, replay before mutable reads, and retry failure classification.
- `refund_worker_test_helpers_test.go` supplies actual mutation-worker fixtures with observable attempt and persistence boundaries.
- `refund_worker_recovery_test.go` covers transient order reads and exact terminal item recovery after receipt persistence fails.
- `refund_worker_outcomes_test.go` covers Stars proof, definitive rejection, ambiguous outcomes, and local persistence recovery.
- `service_test_helpers_test.go` contains shared admin service test doubles.
- `settings_test.go` covers secret handling, safe listing, readiness, and validators.
- `payment_profiles_test.go` covers provider profile validation, encryption, masking, and readiness behavior.
- `zero_coverage_test.go` covers backup deletion, balance deductions, stable-ID payment-profile lookups and runtimes, legacy EZPay validation, and activity reward boundaries.
- `catalog_test.go` covers optional setting validators.
- `user_profiles_test.go` covers aggregate selection, ordering, and synchronization projection.
- `user_profiles_test_helpers_test.go` contains focused aggregate-profile repository doubles.
- `user_command_replay_test.go` covers retained edit/replacement/bulk intents through catalog and provider failures, changed payload conflicts, and fresh command validation.
- `user_command_replay_test_helpers_test.go` supplies actor, kind, key and fingerprint-bound workflow fixtures.
- `user_operation_worker_test.go` covers ambiguous provider outcomes and partial bulk completion.
- `user_operation_worker_test_helpers_test.go` contains reusable provider-operation worker fixtures.
- `README.md` documents the package layout.

Provider profiles have stable IDs, separate endpoint, credential, merchant ID,
provider name, and acknowledgement values. Channel selection only controls
which rails are offered for that account. Balance changes accept caller-owned
financial references: retries return the original immutable entry and mismatched
target, delta or reason conflict. Separate intent keys remain separate audited
operations even when their reasons are identical.

- `settings_part2.go` contains the remaining settings validation and persistence helpers.
`user_refund_quote.go` derives an editable administrator refund default from the net debit and observed whole-term weighted traffic. Native snapshots are selected by panel owner and purchase ID; compatible legacy sources retain the ID-and-range fallback. `user_refund_quote_test.go` covers discounts, custom limits, reset periods, cent rounding, queued terms, outages and ownership.

`turnstile_settings.go` and `turnstile_settings_test.go` require both keys before enabling, reject clearing an enabled site key, and use the existing encrypted secret registry. Setting writes serialize cross-key validation.

`pm_settings.go` validates the default-off PM toggle and forum destination. Composition injects a queued forum/permission validator; enabling and changing an enabled destination require a usable forum and bot manage-topics rights.
`pm_settings_test.go` covers fail-closed enabling, destination validation and disable-before-clear behavior.
- `ip_lookup_settings.go` validates atomic IP Lookup settings, preserves independent check/geolocation ordering and protects write-only credentials. `ip_lookup_settings_test.go` covers encrypted credentials and ordering round trips without altering saved configuration on invalid input.
- `ip_lookup_settings_test.go` covers credential encryption, keeping, clearing and generic-route rejection for CI.

- `usdt_settings.go` ? Encrypted USDT configuration registration and validation.

`xnode_settings.go` Registers encrypted fleet configuration with typed validation.

`user_provider_commands.go` validates manual temporary restrictions; `user_provider_worker.go` executes queued changes. `user_account_commands_test.go` covers coupon, refund and restriction command validation.

`settings_emby.go` owns the write-only Emby connection interface, fresh-token retarget validation, encrypted pair updates, redacted auditing and validated runtime snapshots. Generic writes reject both connection keys. `settings_emby_test.go` covers retargeting, stale drafts, token rotation and unavailable atomic storage. Extend the dedicated interface when adding coupled Emby credentials.

`hourly_policy.go` exposes validated encrypted policy snapshots and CAS writes;
generic settings cannot bypass that interface. `hourly_offers.go` configures
separate squad hourly prices and checks eligibility arithmetic without changing
legacy merchandising. `hourly_policy_test.go` and `hourly_offers_test.go` cover
strict decoding, paused-sales enforcement terms, stale drafts and invalid prices.
Persistence commits each configuration audit atomically. Provider composition,
units and wallet version fences are documented in `docs/hourly-administration.md`.

`user_hourly.go` builds the administrator hourly projection through narrow
optional reads and exposes no credentials, proxy/period references, generations,
activation keys, or fingerprints. `user_entitlement_commands_test.go` covers the
manual grant, entitlement edits, exact refunds, combo replacement, and the
active-hourly conflict fence.

`payment_profile_credentials.go` accepts profile-specific authenticated encryption,
and only the exact provider-named migrated ciphertext with its retained encrypted
legacy setting may use the historical encryption context. Custom or corrupted
profiles fail closed. Blank/masked credentials may preserve an unchanged account;
changing provider, endpoint or merchant requires an explicit replacement. Once any
order references an account, rotate by creating a new profile and disabling the
old one, so late callbacks remain verifiable. Storage commits profile writes,
deletions and their audit events together. Generic writes to migrated legacy
account endpoints and credentials are refused once that provider has profiles.
`payment_profile_credentials_test.go` covers migration proof, account isolation,
retarget validation and protection of the retained legacy settings.

Administrator entitlement and restriction workers pass the panel owner to the
native adapter. They do not skip native-only members or address native state by a
historical proxy link. Temporary unban completion is bound to its operation ID,
so an older delivery cannot release a later restriction on the same member.

Balance commands require the actor-attributed atomic persistence interface and a
caller-owned stable reference. Test-only random-reference entry points and the
legacy non-atomic ledger/audit fallback are removed. `finance_test.go` and
`zero_coverage_test.go` exercise the same reference-bound interface as HTTP and
Telegram; database regressions verify immutable replay and atomic audit storage.

Cancellation requires the actor-attributed `CancelAdminPurchase` transaction;
there is no separate cancellation/audit fallback. The unused synchronous refund
and retry service entry points are retired. Refund work validates its local target
before claiming a fresh provider attempt, so a temporary order-read failure stays
retryable. A terminal item restores its recorded receipt status and metadata
without rereading mutable payment state or issuing another refund. Existing
processing Stars attempts still require provider proof before local settlement.

Administrator list and detail synchronization use the same bounded native
projection. Active, fully onboarded owners with current credential material are
pending until generation-bound deployment proof exists; synchronized describes
that historical proof and does not assert current agent health or traffic access.
Missing native material is not_provisioned, and retained recovery reasons remain
failed. Legacy proxy links and wallet funding never stand in for native proof.

Entitlement edits, combo replacements and bulk extensions check the existing
actor-scoped operation key and unchanged normalized request fingerprint before
mutable catalog or provider reads. An exact accepted replacement or extension
returns its receipt; an exact edit uses the store's protected replay path to
hydrate its purchase. New intents still validate current combos and squads, and
bulk previews still read current targets. Replay-storage errors never admit new
work. Missing squad validation capability fails closed.

Queued maintenance, outbox retries and payment refunds require the database
`CreateAdminMutation` contract: the command, outbox job and actor audit commit in
one transaction. Audit failure prevents admission, and replay adds no second job
or audit. The service no longer appends a separate post-admission audit.
