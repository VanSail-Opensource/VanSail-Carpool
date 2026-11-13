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
pages. Admission includes active squads inside the bounded one-minute prefetch
window only while the wallet currently covers the next whole hour, so an
underfunded squad is not re-admitted on every tick and the existing
`(kind,payload)` outbox dedupe keeps at most one pending job per owner.
`delivery.go` checks at most eight squads concurrently in one 1.5-second window,
including prefetch-window squads. `settlement.go` passes those short-lived exact
compile-revision proofs to deterministic wallet settlement. Genuine unavailable
squads remain pending; the database charges only a deliverable remainder and
never missed intervals.

Inside the prefetch window the database pays exactly one whole next UTC hour
starting at the current aligned `paid_through`, so `paid_through` advances by one
hour and the compiled native `ValidUntil` is replaced (trigger-bumped compile
revision plus `xnode_sync_user`) while the member is still authorized. The
wallet debit and ledger keep transaction time, never the future interval start.
A missing readiness proof leaves the interval unpaid and retryable; insufficient
funds or an unpurchasable offer skips the prepayment without moving the paid
boundary, cancelling the paid remainder, or suspending early. Those failures are
decided only at the boundary by the existing late path, which charges the
current remainder and suspends with the definitive notice when the wallet cannot
fund it. Activation proration, late/offline recovery, one receipt per
`(enrollment_id, hour_start)` and deterministic wallet ordering are unchanged.
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
`terminal_recovery_test.go` verifies that an already recorded terminal item
completes its parent receipt before decrypting input or reloading mutable owner
state. Its reference, result and error survive interrupted receipt writes exactly.
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
`prefetch_test.go` proves readiness covers only active squads inside the bounded
prefetch window; the hosted `hourly_billing_prefetch_test.go` proves one whole
next hour before the boundary, one receipt and debit under concurrent workers,
and a funded-insufficient skip that never cancels the paid remainder or suspends
early.

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
metadata, cooldowns, readiness and committed shared-day usage. It computes
`requiredBalance` from each offer's `requiredHours` (zero inherits
`policy.activationHours`), keeping the eligibility contract per squad. Hourly
visibility is independent of grandfathered catalog visibility. GETs never create
allowance rows; unresolved reservations remain visible as pending accounting.
Public views exclude owner generations, native principal/period references and
credentials.
`preferences.go` records price-capped opt-in and preserves immediate opt-out even
when current policy cannot be decrypted. Neither path buys traffic itself.

`admin.go` queues and applies the administrator-only disable through the same
provider-operation lane with a distinct actor and a sealed administrative
target; the paid hour is retained and no member cooldown is created.
Fresh administrator admission uses the explicit transactional
`CreateAdminHourlyDisableOperation` repository entry point, which locks current
active, unblocked administrator authority before queueing. The shared operation
kind does not grant administrative authority to member commands. Accepted
replays and execution retain the original intent after later actor demotion.
`commands_test.go` covers the actor/owner split, immutable replay, and the
administrator path without weakening member authorization.
