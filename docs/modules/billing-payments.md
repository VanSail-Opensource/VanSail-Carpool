# Billing and payments module

## Ownership and interfaces

Billing owns TXB balances, the immutable ledger, fixed-decimal exchange rates, durable payment orders, provider event deduplication, successful top-up credit, manual adjustments, refunds, and debt enforcement. Provider adapters create external checkouts and validate callbacks; billing decides whether durable business state may change.

User operations are `GET /api/v1/balance`, `GET /api/v1/ledger`, `POST /api/v1/payments/orders`, order polling by ID, and user-owned cancellation. Create and cancel require `Idempotency-Key`, return `202` with a durable operation receipt and payment-order ID, and expose read-only receipt polling through `GET /api/v1/operations/{operationId}`. Payment creation receives a stable `methodId` such as `ezpay:<profileId>:alipay`, `bepusdt:<profileId>:usdt.trc20`, or `stars`; the provider name is never sufficient to select a rail or account. Each configured provider account contributes its own methods, display name, and enabled channels. Administrative operations append balance adjustments, refunds, and terminal-payment courtesy credits, and inspect payments/refunds. No endpoint edits a balance, ledger row, or provider event in place.

## Amount invariants

- One TXB is 100 integer minor units. Database and API arithmetic uses integers for TXB.
- Required rates are `txb_per_cny`, `txb_per_usd`, and `txb_per_xtr`. Each is a validated positive fixed decimal. Legacy inverse values are not reciprocated automatically; a method remains unavailable until an administrator explicitly enters its new-direction rate.
- The server computes `ceil(requestedTxbMinor / txbPerCurrency)` at the provider's currency precision and stores the requested TXB, method, provider, rail, payable value, rate value, and rate-direction/version snapshots. Existing pending legacy orders retain `currency_per_txb` snapshots and validate without reinterpretation.
- Successful settlement credits exactly the requested `txbMinor`, not a value recomputed from a later rate.
- Each provider transaction/charge ID is globally unique where the provider guarantees uniqueness. A webhook dedupe record and its ledger credit commit together.
- The first authoritative transition to paid also commits one immutable Telegram announcement job containing the provider key, settlement-time administrator provider name, channel, TXB amount, stored payable amount/currency, and resolved username. Callback replay cannot enqueue a second job; legacy queued jobs without `providerName` use the provider fallback.

## Provider order lifecycle

Orders move from `creating` to `pending`, then to `paid`, `expired`, `failed`, `cancelled`, or `refunded`. The priced `creating` order, operation receipt, target item, and outbox job commit atomically before any provider call. The worker durably marks its single attempt before creating checkout state; an interrupted attempt is reconciled from the stored order, and an outcome that cannot be proven becomes `pending_review` rather than being retried. A verified paid callback remains authoritative and resolves the matching create receipt to `succeeded`. The client receives the exact payable value plus independently nullable payment URL, QR payload, receiving address, actual crypto amount/currency, and expiry fields.

EZPay exposes five ordered administrator-selectable rails: Alipay, WeChat Pay, QQ Wallet, bank/UnionPay, and JD Pay. BEPusdt exposes ten ordered USDT rails: TRC20, ERC20, Polygon, BEP20, Aptos, Solana, X-Layer, Arbitrum, Plasma, and TON. A saved rail must still be enabled when a new order is created. The signed EZPay callback `type` must equal the order's rail.

EZPay checkout uses a signed CNY URL and signed GET notification. BEPusdt calls `/api/v1/order/create-transaction` using USD fiat and records the returned `actual_amount`, token/address, trade ID, and payment URL separately from fiat payable amount. A callback token carrying address semantics must exactly match that stored checkout address; the reference-compatible literal `USDT` token is accepted as a currency marker. Signed callbacks require the documented signature. Unsigned v1.19 direct-transaction notifications require an unguessable per-order HMAC capability embedded in the callback path; access logs redact that segment. Stars creates a single-price XTR invoice link, opened with `Telegram.WebApp.openInvoice`. QR images are generated locally only from an explicit server-returned QR payload.

`POST /api/v1/payments/orders/{id}/cancel` atomically marks one live owner-scoped order cancelled and queues one durable provider command. For BEPusdt, the worker calls signed `/api/v1/order/cancel-transaction` when a trade ID exists and stores a sanitized provider-cancellation status. Unsupported cancellation succeeds locally; an uncertain provider result becomes `pending_review`. A later verified paid event may still transition the locally cancelled order to paid and credit it once.

Redirects and invoice-close UI events are navigation signals only. Settlement requires verified EZPay `TRADE_SUCCESS`, signed BEPusdt status `2`, or Telegram `successful_payment`. Order polling closes the payment sheet only after persisted state is `paid`.

Successful-payment announcements use the optional `telegram.payment_announcement_chat_id` setting. A missing setting skips delivery without affecting settlement; Telegram failures retain the outbox job for retry. The Chinese MarkdownV2 receipt shows the stored payable amount/currency, administrator provider label, commerce-localized channel, TXB amount, and username. EZPay and BEPusdt announce their stored CNY and USD snapshots respectively, while Stars truthfully announces its stored XTR amount and uses `Telegram Stars` as both provider fallback and channel label.

Failed or expired member orders retain their amount and method selection in the
payment sheet. Its localized **Reissue payment** action creates a distinct
replacement order; it never revives or rewrites the terminal original. A
provider-return page exposes that action only for a durable failed or expired
order, passes its order ID to Home, and Home refetches the owner-scoped record
before restoring its valid TXB amount and still-enabled payment method.

## Refund and debt behavior

A refund appends one reversal against one paid order and cannot be applied twice. If reversal makes the TXB balance negative, reconciliation cancels queued purchases first, then active purchases newest-first, appending compensating ledger records and durable X-Node revocation jobs. Any debt left after cancellable value is exhausted remains visible and blocks new purchases; it never rewrites previous entries.

Administrator payment refund requires `Idempotency-Key` and first stores a `payment_refund` receipt. Telegram Stars is refunded upstream before the local reversal; an interrupted attempt queries authoritative Stars transactions, and an unprovable outcome becomes `pending_review`. A refunded-payment webhook or scheduled transaction reconciliation completes only open matching receipts. Non-Stars providers use the existing local reversal because their enabled adapters expose no upstream refund mutation.

Manual adjustments require an administrator, a nonzero bounded delta, and a non-empty reason. A negative resulting balance blocks subsequent purchases; payment refunds additionally run the queued-first/active-newest entitlement cancellation policy described above.

`POST /api/v1/admin/payments/{id}/courtesy-credit` is restricted to failed or
expired orders and requires a 3-500 byte administrator reason. It credits the
order's immutable TXB amount exactly once, commits a linked ledger entry and
audit event in the same transaction, and leaves the order failed or expired.
Once present, the courtesy record blocks a late provider callback from changing
that order to paid or adding a second credit. It makes no provider call; use
the refund path only for an already provider-paid order.

## Failure behavior

- Unknown order, wrong provider, amount mismatch, user/payload mismatch, invalid signature, and non-success state cannot credit funds.
- Callback replay returns the provider's success acknowledgement after confirming the stored dedupe result, but does not append another credit.
- A callback arriving before the checkout HTTP response can still settle the already-durable `creating` order.
- Provider redirects arriving first leave the order pending. The return redirect carries a short-lived HMAC capability; a regular browser may poll only the matching provider/order receipt projection, while Telegram retains owner-scoped full-order polling. Browser return pages receive only order ID, amount, provider, channel, status, and timing metadata; owner, checkout URL, QR, trade, and provider payload data remain private.
- EZPay returns `success` only for a valid accepted/replayed notification. BEPusdt returns configurable acknowledgement text (`ok` or `success`, default `ok`) to support both bundled reference variants; invalid notifications deliberately do not receive it.
- Stars reconciliation queries durable Telegram charge/order state after lost updates without trusting the client.
- Backup-gated daily compaction may remove terminal payment detail older than seven days. Immutable ledger entries, callback tombstones, refunds still needed for lineage, and every live or open-operation order remain retained; order creation never prunes evidence opportunistically.

Balance funding also exposes a `coupon` method. It redeems only `balance_add`
and `balance_multiply` coupons; purchase-discount grants remain catalog-only.
EZPay and BEPusdt settings are stored as multiple independent encrypted,
masked profiles (or one profile) per provider, each with an administrator-
defined provider name, endpoint/credential, and independently enabled channels.
Automatic renewal uses the current ride's combo and paid add-ons, current
prices, one atomic debit, and a unique source-successor link for exactly one
next term. It preserves an attached recurring-discount relationship under the
catalog policy; retained legacy batch records are not a public member flow.

## Verification

- Table-driven decimal tests cover all three currencies, non-terminating rates, round-up boundaries, huge inputs, zero/negative requests, and snapshot stability.
- Concurrency tests settle and refund the same order from many goroutines under the race detector and assert one credit/reversal.
- Callback tests cover signatures, reordered parameters, empty-value rules, EZPay subtype tampering, fiat/crypto/recipient mismatches, both BEPusdt callback shapes and acknowledgements, status transitions, replay, and callback-before-response.
- Method fixtures cover every enabled EZPay/BEPusdt rail in administrator order, cancellation signatures, cancellation/paid races, and secret/capability redaction.
- Refund tests cover queued-first/active-newest cancellation, partial debt recovery, remaining debt, retry after X-Node failure, and immutable history.
- Handler tests ensure users cannot access another order or ledger, redirects never credit, and provider/bearer secrets are absent from responses and logs.
