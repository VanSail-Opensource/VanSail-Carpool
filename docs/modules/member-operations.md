# Member subscription operations

## Ownership and interfaces

`internal/connections` owns asynchronous connection scans, transient IP projections, signed drop capabilities, and selected-IP unlink reconciliation. `internal/purchaseops` owns first-term refund quotes, commands, and workers, plus settlement of historically paid traffic resets. HTTP exposes connection scan/poll/drop under `/api/v1/subscription/connections`, refund quote/mutations under `/api/v1/purchases/{id}`, and owner-scoped receipts at `/api/v1/operations/{id}`. Manual and automatic paid-reset admission, quotes, and controls are retired.

All mutations require `Idempotency-Key`. PostgreSQL stores the request fingerprint, receipt, bounded target metadata, money effects, and outbox job before accepting work. Provider calls run only after the transaction commits and always enter the shared X-Node queue. The neutral `providerops.Dispatcher` is registered once with the outbox and remains open to member, administrator, and maintenance operation kinds. Receipt reads accept only the authenticated operation actor or affected owner, allowing an administrator to poll a command they issued without exposing it to unrelated users.

## Connection privacy and reconciliation

Connection scan rows retain only owner, provider job ID, progress, status, and expiry. Completed provider IP results are returned directly to the authenticated owner and receive 15-minute HMAC handles binding owner, scan, node, IP, and expiry. Raw IPs and handles are not stored in operation tables. A drop item stores only a target hash; its handle is AES-GCM encrypted in the atomic outbox payload.

A worker records intent before starting a provider scan or drop. A scan-start interruption or provider error with no durable job ID becomes `pending_review` instead of repeating an uncertain POST. An interrupted or ambiguous drop starts a fresh user scan and checks the selected node/IP. Absence succeeds; a confirmed rejection fails; an unresolved result becomes `pending_review` and is never blindly retried.

## Historical reset settlement and refund invariants

- The compatibility reset worker requires an immutable, negative `traffic_reset_debit` owned by the operation's account before executing. No current route creates a reset debit. An unpaid queued operation fails as retired; an already-processing operation without proof remains under review.
- A definitive historical reset failure refunds its original debit exactly once. Ambiguous attempts reconcile through purchase-owned native operation evidence or remain `pending_review`. Purchased scheduled quota-reset cadence and initial purchase activation remain supported; neither erases purchase-wide refund evidence.
- Refund eligibility requires the authenticated owner's active first lineage term, a creation age strictly below 24 hours, and live used traffic equal to zero. The worker disables upstream access first, rereads usage, and restores the original exact entitlement on nonzero usage or a local state conflict.
- Successful refund cancellation, immutable original net-debit credit, receipt completion, and successor activation commit atomically. An independently purchased queued successor moves to the refund boundary with its full duration, and all later queued terms shift by the same delta.

## Member web workflow

`ConnectionsView.vue` remains a thin entrance over `components/connections`. The scan composable retains one start idempotency key after an ambiguous HTTP failure, polls only the accepted scan ID, and exposes progress, terminal failure, empty, and retry states. Completed IP rows use the server's signed handle; the browser never constructs or submits node identity alongside a drop. The confirmation overlay owns native Telegram Back and warns that provider-side selected-IP removal may interrupt other sessions behind the same public IP.

The active purchase card obtains server-owned refund quotes. Refund commands retain their idempotency keys until accepted, then poll the owner-scoped receipt. An eligible Refund action replaces automatic renewal; all other states keep the existing renewal control. `pending_review` and `partial` receipts block further member mutations and explicitly forbid retry while an administrator reconciles the provider outcome. Historical reset receipts remain owner-readable without restoring a reset action.

## Hourly activation, renewals, and usage cost

Each new hourly activation prepays its minimum enable period from the member's
wallet. The offer's `requiredHours` sets the minimum; `0` inherits
`policy.activationHours`. Preparation is unpaid. Once native access is ready, the
activation commits a debit equal to `requiredBalance` (minimum hours times the
accepted hourly price) and sets `paid_through` to the activation instant plus
those hours. No hourly renewal charges for time already covered. Disabling stops
access immediately without refunding unused prepayment; restarting is allowed immediately but requires a new activation and minimum debit.

The scheduler prepays the next interval inside a bounded one-minute lead window,
starting at `paid_through`. The first renewal after an off-hour minimum boundary
covers only the remainder of that UTC hour; later renewals cover whole hours. The
compiled-native `ValidUntil` is extended before the previous paid boundary denies
access. A duplicate or concurrent settlement still yields exactly one receipt per
`(enrollment, hour_start)` and at most one settled hour forward; the wallet debit
and immutable ledger keep transaction time, never the future interval start. A
genuine late/offline recovery keeps the prorated current-remainder path and never
back-bills missed hours or automatically resumes a suspension. Inside the lead
window an insufficient balance or unpurchasable offer is only a bounded skip: the
paid remainder stays authorized and retryable, and the enrollment is not
suspended early. At and after the boundary the existing late path decides; when
the wallet cannot fund the interval, the same transaction that changes the
enrollment to `funding_suspended` writes the definitive
`hourly_funding_suspended` notice. Before a paid hour ends, the existing
five-minute notification scan queues a `hourly_funding_warning` when the next
renewal costs more than the balance, including proration at an off-hour boundary.
A successful prepayment advances `paid_through`, so the warning event key moves
to the new boundary instead of repeating. Both
notices are durable user events with per-enrollment/per-boundary event keys,
localized EN and zh-CN copy, and facts limited to the squad name, hourly price,
balance and time. A suspended enrollment cannot resume itself; the member must
top up and start a new activation.

The rolling 72-hour usage breakdown allocates new hourly receipts to elapsed
paid time inside the window, ending at disable if it comes first. It does not
show future coverage or unused prepayment as usage cost. Allocation uses the
original debit and rounds cumulative cost down to whole TXB minor units, then
subtracts the cumulative cost before the window so adjacent windows add up.
Historical unmarked receipts still count at their original ledger timestamp.
Wallet history shows the full actual debit at transaction time in all cases.

## Verification

CI owns migration, transaction-failure, idempotency-conflict, compensation, ambiguity, quiesce/restore, signed-handle, and queue-shifting tests. Local verification is limited to formatting, structure, lint, type/build, vet, and diff checks; provider and Telegram device behavior remains a deployed staging gate.
