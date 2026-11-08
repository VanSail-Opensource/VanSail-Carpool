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

## Verification

CI owns migration, transaction-failure, idempotency-conflict, compensation, ambiguity, quiesce/restore, signed-handle, and queue-shifting tests. Local verification is limited to formatting, structure, lint, type/build, vet, and diff checks; provider and Telegram device behavior remains a deployed staging gate.
