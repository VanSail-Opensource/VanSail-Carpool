# Hourly commerce coordination

`worker.go` resumes encrypted provider-operation targets, validates the account
generation and fingerprint, and advances existing item/receipt phases.
`activation.go` discovers accepted purchases before reading mutable pricing,
prepares dormant credentials, obtains bounded native readiness evidence, commits
exact billing, and checks native application. Known unpaid business failures
cancel only an unpaid preparation. Uncertain commits or application keep the same
operation pending; queue retry counts never manufacture a financial outcome.
`disable.go` retains the paid hour and applies cooldown once, then synchronizes
absence and drains only its terminal enrollment. New paid/preparing benefits
protect owner admission through the native lifecycle's locked target guard.

`schedule.go` performs rotating bounded keyset admission to the existing outbox.
The application invokes it every second, including whole UTC hours and startup.
Its cursor advances after each accepted enqueue, including partially timed-out
pages. `delivery.go` checks at most eight squads concurrently in one 1.5-second
window. `settlement.go` passes those short-lived exact compile-revision proofs
to deterministic wallet settlement. Genuine unavailable squads remain pending;
the database charges only a deliverable remainder and never missed intervals.
`application.go` bounds native application to four callers within one four-second
window, using the established per-node executor and retaining uncompleted work.
`rotation.go` keeps bounded least-recently-used owner cursors so blocked head
participants cannot repeatedly starve healthy tails in either native phase.
`rotation_test.go` covers readiness/application head blocking across retry turns.

`funding.go` is the master-only extra-traffic buyer. It keeps at most 8192
memory-only immutable receipt/terms/timestamp intents, serializes concurrent
funding for each account generation/day, and replays uncertain outcomes before
new pricing. It never turns a reconnect, timeout or opt-out into refunded credit.
The database still verifies original fresh positive demand, consent, price cap,
UTC day, future paid access, exact exhaustion and balance before buying one
decimal GB. A day rollover discards retry metadata only; immutable purchases and
all unresolved byte obligations remain in their owning repositories.

`fixture_test.go` owns isolated hosted PostgreSQL and injectable-native fixtures.
`worker_test.go` covers interrupted paid application, historical disabled replay,
unpaid cancellation and encrypted code/idempotency boundaries. `funding_test.go`
covers a lost funding reply and immutable replay/consent terms. `delivery_test.go`
covers a blocked readiness call beside a healthy squad and unexhausted dependency
waits. These suites run in hosted CI, not locally.
`generation_test.go` verifies queued and processing commands terminate after a
known account-generation change without touching the current wallet or access.
`legacy_test.go` covers active/accepted zero-price queued fixed-term conflicts
and preservation of transient repository errors in disable admission.
`capacity_test.go` confirms global owner-capacity rejection uses the same
no-debit, no-cooldown unpaid-cancellation lifecycle and a terminal receipt.
`retry.go`/`retry_test.go` distinguish fresh revision contention (one second,
matching the existing outbox/admission minimum) from offline/expired proofs
(five seconds). Payment SQL is bounded to four seconds; every new attempt obtains
new proofs and repeats all original financial and native fences.
`settlement_scope.go` excludes unchanged skipped rows from native work; their
already-expired native paid boundary remains authoritative during contention.

`service.go` composes the wallet repository, encrypted policy reader and native
control plane. `commands.go` records member activation/disable intents through
the existing provider-operation queue. Activation codes stay encrypted in the
temporary outbox payload and never enter public receipts or fingerprints.

Wallet and receipt transactions live in `internal/platform/database`; native
principals, apply proofs, speed grants and byte reservations live in
`internal/xnode`. This module coordinates those owners without introducing a
second request queue or storing copied native catalogs. Live demand is never a
financial authority: the wallet transaction repeats ownership, current policy,
paid access, consent and funds checks before any debit.

`view.go` publishes owner-generation-scoped hourly offers, current native display
metadata, cooldowns, readiness and committed shared-day usage. Hourly visibility
is independent of grandfathered catalog visibility. GETs never create allowance
rows; unresolved reservations remain visible as pending accounting. Public views
exclude owner generations, native principal/period references and credentials.
`preferences.go` records price-capped opt-in and preserves immediate opt-out even
when current policy cannot be decrypted. Neither path buys traffic itself.
