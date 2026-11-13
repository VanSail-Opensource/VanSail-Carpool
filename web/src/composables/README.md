# Composables

- `usePurchaseValidity.ts` owns a boundary timer and focus/visibility revalidation for member and browser-admin purchase displays. Consumers pass raw purchases and an optional error-handling refresh callback; the returned status/current helpers derive UI validity without changing settlement state. Timers/listeners are disposed with their consumer.
- `usePurchaseValidity.test.ts` covers offline boundary expiry, focus refresh and timer/listener disposal in hosted CI.

- `motionPresets.ts` owns the shared timing tokens and critically damped layout spring.
- `useMotionPreferences.ts` is the central reduced-motion boundary for Motion Vue components; it returns reduced-safe transitions and bounded offsets without storing animation state in components.

Page loaders restore typed session snapshots through `api/cache/restore.ts` before
their existing requests. Cached content renders during refresh; backend responses
replace it through the same state and error handling. Query and cursor keys remain
separate. Action quotes and permission decisions still await the backend.

- `useSquadAddition.ts` owns active-term optional-squad filtering, time-adjusted quote freshness, activation-code handoff, balance recovery, and idempotent commit feedback.

- `useActivity.ts` loads games, draws, and check-in actions.
- `useActivity.test.ts` checks that ambiguous draw failures reuse the same idempotency key and confirmed draws release it.
- `useAffiliateCentre.ts` loads member metrics and fixed five-row referral pages.
- `useAffiliateCentre.test.ts` covers obsolete page replies and retrying a failed
  page; only current requests update referral data or loading state.
- `useAdminDatabase.ts` coordinates protected database review and mutation flows.
- `useAdminSection.ts` provides latest-request-safe administrator list operations and cursor accumulation.

- `useCouponRedemption.ts` redeems a coupon code for the guided purchase flow.
- `useCouponRedemption.test.ts` covers guided redemption normalization and idempotency reuse.
- `useAutoRenewal.ts` owns the owner-scoped automatic-renewal status load, eligibility, and server toggle update.
- `useDisplayCurrency.ts` keeps the persisted member display choice and read-only CNY/USD rate snapshot in one reactive source of truth. Selection supersedes earlier reads; session hydration retires previous-owner replies and resets pending state. Logout clears currency/rates. Deferred-response regressions in `useDisplayCurrency.test.ts` cover these ownership boundaries.
- `useAutoRenewal.test.ts` covers the server-ineligible toggle constraint.
- `useClipboard.ts` provides safe clipboard feedback.

- `useDashboard.ts` loads owner-scoped dashboard summaries, revalidates purchase boundaries and resumes durable subscription-revocation receipts. Usage Breakdown owns detailed traffic.
- `useEmby.ts` manages Emby setup and preferences.

- `useIntroSequence.ts` drives the onboarding intro sequence.
- `useIntroSequence.test.ts` covers intro timing and completion.
- `useOnboarding.ts` coordinates intro and current-revision agreement acceptance, including recovery when agreement state changes or completion was already committed. Content uses the latest locale request; mutations remain exclusive during concurrent refreshes, submit the revision actually reviewed, and ignore replies after owner replacement or disposal. Published revision changes clear earlier consent.
- `useCommunityMembership.ts` refreshes canonical Telegram membership and strict active-combo access, opens requested-space invites, and unregisters its Telegram activation refresh handler on teardown. Overlapping reads retain only the newest result; session changes and disposal retire membership replies and pending invites.
- `useCommunityAccess.ts` shares the read-only server-confirmed valid-combo and positive-balance gate between the shell, the sidebar entrances and Home product sections. Both reads refresh together after navigation or a completed payment. Owner changes synchronously clear prior access and start fresh reads; old responses cannot restore or revoke another member's access. `useCommunityAccess.test.ts` covers shared requests, owner replacement, logout and disposal.
- `useCommunityMembership.test.ts` covers invite opening, error feedback, Telegram activation refresh, event cleanup, obsolete reads and retired invites. `useOnboarding.test.ts` covers username-free progression, owner/disposal boundaries, locale races, current-revision consent, exclusive mutations and conflict recovery.
- `usePaymentOrder.ts` coordinates durable checkout/cancellation receipts with payment-sheet presentation states. Cancellation always projects the fetched payment order: a failed receipt cannot label a still-pending checkout cancelled, and polling continues until the authoritative order becomes terminal.
- `usePaymentCountdown.ts` owns a disposal-safe one-second payment-window countdown used only for presentation and replacement prompts.
- `usePaymentConfiguration.ts` owns exact TXB bounds, display-currency amount input, one canonical TXB-minor order amount, and selected payment method state; display switches re-render the field without repricing the order, and a missing stored rate falls back to TXB entry.
- `usePaymentOrderOperations.ts` retains stable creation keys per method/amount and cancellation keys per order, then polls payment provider-operation receipts without replaying provider calls. An uncertain request can be retried unchanged without binding a different checkout to its key.
- Payment resets and disposal invalidate pending submissions, receipt lookups and
  presentation work. Superseded replies cannot reopen an earlier checkout or
  restart its polling. `usePaymentOrderOperations.test.ts` covers ignored late
  acceptance and per-order cancellation key recovery.
- `usePaymentTarget.ts` opens accepted provider targets and read-only polls authoritative payment status.
- `paymentOrderHelpers.ts` validates exact payment bounds and replacement candidates, classifies terminal orders, and renders address-first provider QR payloads.
- `usePaymentOrder.test.ts` covers payment-order state transitions.
- `usePaymentReturn.ts` polls a provider-returned payment order until it is confirmed or terminal.
- `usePaymentReturn.test.ts` covers durable payment-return confirmation and pending polling.
- `useOperationReceipt.ts` performs bounded, read-only polling for durable member operation receipts and stops on every terminal or review state.
- `useDurableCommand.ts` retains stable UUID keys for ambiguous command retries, invalidates disposed submissions, and delegates accepted receipt polling to `useOperationReceipt.ts`.
- `usePurchaseOperations.ts` owns authoritative first-term refund quotes, conflict refresh, mutation idempotency and receipt state. Paid reset controls are retired.
- `usePurchaseOperations.test.ts` covers quote conflict refresh, idempotency reuse, server-owned refund eligibility and stale purchase isolation.
- `useStatistics.ts` independently loads the 30-minute aggregate snapshot and
  ten-second node cache while preserving last-good data across partial failures.
- `useTelegramBackButton.ts` coordinates one native Telegram BackButton across route and overlay owners, adds soft navigation feedback, and removes every handler on teardown.
- `useTelegramProtection.ts` reference-counts active destructive, busy, or dirty workflows and enables capability-aware native closing confirmation only while needed. It leaves Telegram swipe gestures unchanged; gesture suppression requires a separately reproduced host conflict. Browser administration never registers an owner.
- `useTelegramProtection.test.ts` covers overlapping owners, unmount/value cleanup, older SDKs, browser isolation and the absence of swipe calls.

- `useActivity.ts` reads eligibility live, tracks refresh state and hides reward controls when verification refresh fails; a rejected check-in refreshes boost state while retaining its localized refusal.

`useTelegramSettingsButton.ts` owns the native Settings button with SDK discovery, an optional visibility getter that hides it for members without a reachable Settings page, and lifecycle cleanup. Paid reset automation is retired.
- `useIPLookupAvailability.ts` shares account-scoped feature availability across shell and Around TX entrances.

`operationReceiptLoader.ts` defines the experience-owned receipt loader injection contract. `useOperationReceipt.ts` and `useDurableCommand.ts` accept injected/explicit loaders while retaining interval compatibility, disposal and retry limits. BrowserApp provides the signed admin operation reader; Mini App callers retain member-scoped reads. No session or signing state is shared between experiences.

`operationReceiptLoader.test.ts` verifies experience-injected polling and disposal without falling back to the member operation reader. It runs in hosted frontend regressions.

`useMetricSamples.ts` owns memory-only displayed-series buffers, capped at 300 samples and retired on disposal/session clearing/expiry.

`useMetricSamples.test.ts` guards 300-point bounds, obsolete timestamp rejection, disposal and logout retirement.

`useSquadAddition.test.ts` covers retained request keys after a lost purchase reply. `useSquadAddition.ts` keeps the same key for an unchanged purchase/selection/activation attempt until a confirmed response, using memory only.

- `usePricingQuote.ts`: Version-bound checkout preview lifecycle: request cancellation, bounded two-minute backoff, hidden-page pause and expiry; successful idempotency replay remains independent.

- `usePricingQuote.test.ts`: Hosted regression for bounded pending retries, hidden-page pause, stale totals, obsolete responses and disposal.

- `usePendingStatus.ts` replaces one live status after five seconds of pending work; Vue watcher cleanup cancels its timer. It never controls readiness or requests.

`useAutoRenewal.ts` and `usePurchaseOperations.ts` capture purchase identity and invalidate pending reads/writes on reset or scope disposal. Late replies cannot replace another purchase's eligibility, renewal setting or receipt. Refund retry keys remain bound to the original purchase across a reset; hosted regressions cover replacement while requests are pending.

`useSquadAddition.ts` retains one retry key per purchase/squad/coupon selection, including activation-code corrections, and rejects late load/mutation results after reset. A pending retry only bypasses quote readiness for that same selection.

`useAdminDatabase.ts` allows newer read queries to supersede pending searches while keeping mutations exclusive. Query generations and abort signals prevent stale filters/table responses from replacing the latest rows. `useAdminDatabase.test.ts` covers out-of-order filters and table changes. `useAutoRenewal.test.ts`, `usePurchaseOperations.test.ts` and `useSquadAddition.test.ts` also cover stale purchase responses and retained retry identity after reset.

`useClipboard.ts` fences feedback by copy attempt, exposes an owner reset and
cancels its feedback timer on scope disposal. `useClipboard.test.ts` covers
out-of-order results, owner replacement and timer cleanup in hosted CI.
