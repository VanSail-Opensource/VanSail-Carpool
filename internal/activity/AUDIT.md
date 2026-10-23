# Lucky draw audit, 2026-09-29

## Scope

Reviewed instant draw selection, idempotency and ledger writes; raffle entry,
reservations, publication, cancellation and settlement; coupon and entitlement
rewards; HTTP authorization and Telegram webhook routing; and the frontend's
server-result and retry-key flow. This is a source audit, not an inspection of
production accounts or proof that historical abuse did not occur.

## Findings and fixes

| Severity | Finding | Fix |
| --- | --- | --- |
| High | Draw coupon codes were hidden from administration but still redeemable. Winners could mint an additional grant or share the code with other members. | Public code lookup excludes private coupons. Wallet, checkout and attached-renewal lookups also reject historical private grants created through code redemption. Legitimate draw grants and public coupons remain usable. |
| High | A recurring traffic reward used the current allowance as its renewal base, making earlier temporary gains or losses permanent. | Apply recurring deltas to the saved renewal base independently. Check both current and renewal allowances for instant draws, cumulative raffle exposure and settlement. |
| Medium | Reusing an instant request key for another draw returned the original draw's receipt. Caller keys also shared the raffle settlement namespace. | Require the same member and draw on replay; reject caller keys prefixed with `raffle:` before any debit or randomness. |
| Medium | Publishing raffles could be edited while their announcement was in flight. Cancellation could leave a late announcement without a cleanup job. Settling raffles released their triggers even though refunds could reopen them. | Freeze publishing edits, retain trigger ownership through settlement, record late announcement IDs and enqueue distinct cleanup work after cancellation. Treat cancelled settlement retries as complete. |
| Medium | Reward kinds accepted unrelated fields and unchecked ranges. A malformed signed range on a reset reward could panic the background worker. Fixed monetary rewards lacked the random reward limit. | Enforce kind-specific fields and a single value source, bound fixed monetary rewards, and revalidate stored specifications before sampling. |

## Existing controls reviewed

- Authenticated member routes derive the user from the session. Draw administration
  requires the admin role and configured Telegram admin identity. Telegram entry
  requires the webhook secret, configured group, sender identity and onboarding.
- Randomness comes from `crypto/rand`; weighted instant selection and raffle
  Fisher-Yates assignment happen on the server. The UI animates the returned result.
- Fees, reserves, results, ledger entries and queued reward effects share a SQLite
  transaction. Message identity and result keys provide database uniqueness guards.
- Native access effects and Telegram notifications use the existing durable queues.
  This change introduces no external API calls, dependencies or schema migration.

## Validation

Regression coverage was added for coupon sharing and historical copies, request
replay isolation, publication/cancellation interleavings, trigger collisions,
malformed payloads, mixed traffic rewards, concurrent Telegram delivery, random
source failure rollback, stock assignment and ledger reconciliation.

Local automated tests are prohibited by `AGENTS.md`. Local validation uses Go
build/vet, formatting, diff inspection and the repository structure audit. Test
execution is delegated to the existing hosted CI workflow after push. No frontend
files or API contracts changed, so browser validation is not part of this patch.

## Operational limits

Already consumed discounts and already applied traffic effects are not reversed.
Historical code copies are blocked from future use; a renewal attached to such a
copy fails closed for operator review. No production data was queried to determine
whether these cases exist. Older invalid draw specifications now return validation
errors and require correction or cancellation/refund through existing admin flows.

Telegram send outcomes remain ambiguous if a network failure occurs after delivery.
Publication keeps sales closed in that case. These fixes do not claim exactly-once
delivery across Telegram and SQLite or recovery of an unknown Telegram message ID.
