# Reachability-based account cleanup

The database owns purge authorization. Telegram reachability is evidence, not a
definitive account-existence lookup. `account_cleanup_health` binds checks to the
numeric bot verified through queued `getMe`; `account_cleanup_checks` stores
daily scheduling, a candidate and one pending nonce. Accepted failures, final
checks and cancellations are retained in immutable `account_cleanup_evidence`.
No token, response body, source IP or message content is copied into evidence.

Composition calls `ConfigureAccountCleanupAdmins` before starting workers. An
uninitialized set disables cleanup. Stored admin roles and configured IDs are
checked during scheduling, response completion, admission and final mutation.

`EnqueueAccountCleanupChecks` staggers initial checks over the UTC day, writes a
nonce and enqueues the existing `account_cleanup_probe` outbox job atomically.
At most 64 accounts are selected per pass. Pending jobs are coalesced by owner
and generation; an expired queued nonce can be refreshed under its pending row
lock, while processing work keeps its original nonce. `AccountCleanupProbe`
revalidates the committed nonce before the worker makes both queued methods.
The API admission lasts two minutes. When a worker loads an aged but still
matching queued nonce, it refreshes only the persisted start time before calling
Telegram; completion also matches that start time, so an older captured attempt
cannot complete the refreshed admission. No canceled row is recreated and no
external call holds SQL locks.

The worker supplies narrow response categories. A successful `sendChatAction`
cancels the candidate even if `getChat` is inconclusive. A matching private
`getChat` alone does not prove unblocked reachability. Unknown responses never
count as account failures. Generic unreachable pairs require a prior successful
private action in the same bot epoch; never-started ordinary accounts therefore
cannot be removed on generic chat-not-found replies.

Admission requires three accepted failures, each at least 24 hours after the
previous accepted failure, spanning at least seven days. A distinct subsequent
final probe must qualify. The first blocked webhook only records its ordered
status and wakes a probe; it changes no balance, benefit or lifecycle generation.

Bot verification expires after 15 minutes. A ten-minute health window counts at
most one vote per account; five observations with at least three unhealthy pairs
and at least half unhealthy pause admission for 24 hours. Unknown network/API
results can pause global admission but cannot authorize an account purge. Probes
continue during the pause so healthy evidence can still cancel candidates.

The final evidence is valid for five minutes. Waiting purges receive a separate
refresh probe after four minutes. Refreshing proof wakes the same pending purge
job without discarding its cursor, changing its receipt or resetting attempts.
Old blocked/purging rows receive no inferred grace history. A valid first
admission of an active account increments its generation; already fenced legacy
purges keep their existing generation. Final SQL guards refresh freshness from
the database clock after locks, rather than trusting an old queued timestamp.

Lock order is health, user, check, then delivery/benefit rows. Authenticated
activity uses user then check and never locks health. Completion only updates an
existing matching nonce: authentication cannot be undone by a late reply. Valid
sessions and verified initData cancel candidates without a `last_auth` column or
an idle per-request timestamp write. A returning ordinary partially purged user
remains fenced with `account_cleanup_pending`; paid facts are not reconstructed.
`ErrAccountCleanupRecoveryPending` wraps `ErrAccountUnavailable` for this verified
identity outcome so HTTP can expose recovery status rather than invalid initData.
Stored/configured administrators can authenticate and supersede an old partial
purge, while their wallet and benefit history remain unchanged.

`AccountCleanupAdmissionValid`, `AccountCleanupAdmissionReason` and the private
transaction guard protect every destructive phase. Financial records, uncertain
command receipts, native accounting, hourly proofs and replay tombstones are
retained; dependencies stay pending. New provider-operation intents lock actor
and owner lifecycle rows before insertion. Exact receipt replay is allowed after
closure and never queues another external mutation. Existing SQL deadlock retry
handles callers that already hold owner or wallet locks.
Admission also locks existing provider-operation rows and supersedes only proven
unattempted parents/items before commit. A previously claimed dispatcher paused
before `BeginAttempt` therefore cannot cross admission using a stale active
snapshot; recorded attempts and uncertain continuations remain intact.

Schema 45 also carries future `purchased_term_seconds` into purchase-history
tombstones. Existing unknown durations remain NULL; no history is backfilled.

Hosted regressions cover grace and separate-final semantics, subday/replay
rejection, nonce cancellation, administrator changes, bot epochs, transient
surges, scheduling backlog, old unproved purges, immutable evidence and provider
intent races. Local automated suites are not run.
