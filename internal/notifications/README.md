# User notifications

- `facts.go` owns stable immutable payload fact keys.
- `copy.go` owns matching English and Simplified Chinese notification copy.
- `format.go`, `format_fields.go`, `format_admin.go`, and `format_new.go` render concise,
  escaped MarkdownV2 cards with one title emoji and ordered detail rows.
- Immediate combo activation, successful Add TXB settlement, and provider refunds
  enqueue private receipts after their authoritative state changes. Refund cards
  show TXB deducted and any cancelled combos; they wait for user sync when needed.
- Expiry reminders say the subscription is expiring soon because scans can run at
  any point in the 48-hour window. Renewal charges display a positive amount,
  and reset refund reasons use localized explanations.
  the frozen node, squads, observation interval, applied duration, expiry change,
  review reason, and capped-calculation indicator.
- `worker.go` delivers durable private-chat jobs through the queued Telegram sender.
- Username-conflict refunds send one localized receipt with the total returned to
  TXB and instructions to choose a new username. Shared Markdown truncation keeps
  bold spans and mention links complete; migration 049 retries only known parser
  rejections that Telegram definitively did not deliver.
- `scanner.go` evaluates the 48-hour reminder, strict 90% notice boundary, and strict-above-95% automatic-reset handoff.
- Automatic-reset success is provider-gated; insufficient balance disables the preference in the reset transaction, while definitive failure produces a localized refund notice.
- Queued purchases and renewal batches acknowledge the TXB debit without claiming access is active. Add-on success waits for user sync; cancellation, manual reset, first-term refund, and automatic-renewal failure cards report final financial or access outcomes.
- `format_new_test.go` covers both locales for every new event, dynamic MarkdownV2 escaping, and localized failure reasons.
- `format_test.go`, `automatic_reset_format_test.go`, `scanner_test.go`, and `worker_test.go` cover formatting,
  localized reset details, threshold behavior, and retryable delivery failures.
- `format_receipts_test.go` covers both locales for new receipts and reset failures.

Delivery reads current user preferences immediately before sending; suppressed jobs finish without removing financial or event audit records.
`preferences_test.go` covers pending-delivery suppression, lookup retry and re-enablement.

Private Telegram recipient-blocked/deactivated/not-found responses are terminal delivery failures, 429 respects its retry deadline, and transient or unknown errors retain bounded retries. Notification failure classification does not trigger account purge.

A bounded worker tombstone acknowledges pre-retirement outage notifications without sending or formatting them. Historical delivery facts remain available to retention/audit; no new producer exists.

Traffic scans emit usage reminders only; the retired automatic-reset preference
is never read and scans cannot create a debit. Historical paid-reset completion
and refund payloads remain readable. Old insufficient-funding notices are
acknowledged without sending retired product instructions.
