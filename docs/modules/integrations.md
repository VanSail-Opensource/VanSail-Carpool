# External integrations module

## Boundary rule

Each integration package translates a documented external protocol into a small application-facing interface. Domain services own interfaces and business decisions; adapters own URLs, authentication, signing, transport timeouts, response decoding, and provider-specific errors. Every method that can block accepts `context.Context`. HTTP clients have explicit timeouts and bounded response bodies. Application adapters submit every X-Node and Emby call to a bounded, paced, context-owned FIFO queue before constructing or invoking the provider client; cancellation and shutdown release blocked callers.

The implementation is split into focused transport, mapping, and lifecycle files without changing the public adapter interfaces. The reference contracts remain the source of truth for every method, path, query, body, and header; no browser code calls either provider directly.

## Telegram

The Telegram adapter validates Mini App `initData`, creates and revokes join-request links, approves only an expected Telegram identity, reads canonical chat membership, configures the webhook and chat menu button, creates Stars invoice links, reconciles transactions, refunds Stars, and decodes updates. Its Bot API surface is `setWebhook`, `setChatMenuButton`, `createChatInviteLink` with `creates_join_request=true`, `approveChatJoinRequest`, `revokeChatInviteLink`, `getChatMember`, `createInvoiceLink` using XTR, `answerPreCheckoutQuery`, `getStarTransactions`, and `refundStarPayment`. Startup derives callback URLs from `PUBLIC_BASE_URL`, requests `message`, `chat_member`, `chat_join_request`, and `pre_checkout_query` updates, and verifies the secret-token header in constant time. Selecting the bot's Main Mini App remains a BotFather deployment step.

The adapter exposes raw Telegram IDs as integers internally but API boundaries serialize them as strings. It never consumes `initDataUnsafe`. Join links request approval, expire in 30 minutes, and are revoked after use. Invite names contain the Telegram user ID and a 96-bit HMAC over user, chat, and expiry within Telegram's 32-character limit; no invite row is persisted. Join-request callbacks validate signature, identity, chat, and expiry before approval and revocation. Successful payment fields, including `telegram_payment_charge_id`, are passed to billing for idempotent validation; the adapter does not credit balances.

## X-Node

The panel owns encrypted desired configurations, native principals and exact accounting. The authenticated master WebSocket is the sole proxy control plane. Public protocol, native validation and release-trust packages are pinned to the verified signed source documented under `reference/Upstream/X-Node`; there is no legacy REST control surface. `internal/xnode/README.md` documents compilation, command execution, traffic acknowledgment and capability negotiation.

Native commands use the existing durable receipts and per-node execution queue. Waiting for commands runs outside the outbound transport queue so traffic acknowledgments remain responsive. Replay retains the original operation identity, and uncertain external mutations require reconciliation before another attempt. Emby and other queued provider integrations continue through their existing executors.

Traffic multipliers are retired: deprecated node metadata stays constant `1`, and all newly accepted reports after the atomic database cutover use 1x, including offline backlog. Previously committed weighted usage, historical rates, rounding carry and financial evidence remain authoritative. The database migration fences the previous runtime and drains traffic writers before establishing each next-sequence boundary; see `internal/platform/database/README.md`. PAYG and hourly prices remain separate.

Client documents are administrator-owned encrypted templates, distinct from server Xray configuration. Subscription output is generated for its owner on demand; previews use synthetic credentials. Host-name `{price}` expands only current PAYG pricing. Imported accounting remains immutable historical evidence and cannot authorize live control-plane mutations.

## EZPay

EZPay signing follows the supplied PHP reference: omit `sign` and `sign_type`, remove empty values, sort remaining keys, concatenate `key=value` pairs with `&`, append the merchant key, and compute lowercase MD5. MD5 is retained only because it is the provider protocol and is never used for password storage or local integrity.

The checkout is a signed GET to `{base}/submit.php` containing `pid`, one of the documented `alipay`, `wxpay`, `qqpay`, `bank`, or `jdpay` types, `notify_url`, `return_url`, `out_trade_no`, `name`, exact CNY `money`, `sign`, and `sign_type=MD5`. The signed GET notification must match local order, stored rail/type, provider trade ID, payable amount, and `TRADE_SUCCESS`. The return route never settles the order.

## BEPusdt

BEPusdt creation posts JSON to `/api/v1/order/create-transaction` with local `order_id`, exact USD `amount`, `fiat=USD`, one of the ten configured USDT trade types, name, timeout, and canonical notify/redirect URLs. Its signature filters empty and `signature` values, sorts keys lexically, joins `key=value` with `&`, appends the API token without another separator, and computes lowercase MD5.

The adapter persists fiat payable amount/currency, `trade_id`, crypto `actual_amount`/currency, receiving `token`/address, and expiration as separate fields. The direct member flow renders that validated address itself, so BEPusdt's optional cashier `payment_url` is not trusted, stored, or required for a successful checkout. Signed notifications require the provider signature. The v1.19 unsigned direct-notification shape is accepted only at a callback URL containing an HMAC capability derived from the API token and local order ID; that path segment is redacted from access logs. Notifications accept status `1`/`3` as non-crediting lifecycle updates and status `2` as a candidate settlement. Order ID, USD amount, actual crypto amount, currency/token semantics, address, and stored trade ID are checked before credit. Each rail selects its channel from one shared encrypted provider profile; signed callbacks use that provider credential and attach the matched rail to the normalized event.

The bundled BEPusdt references differ on acknowledgement text: the general API document says `success`, while the endpoint-specific v1.19 notification document specifies HTTP 200 and `ok` for paid status. The adapter therefore keeps an administrator-configurable `ok`/`success` response, defaulting to `ok`, with fixtures for both. User cancellation calls the documented signed `/api/v1/order/cancel-transaction` endpoint when a direct transaction has a trade ID; a later authoritative paid notification still wins the race and credits once.

## Emby

The Emby adapter uses an encrypted base URL/token and sends `X-Emby-Token` only to that configured origin. The base URL may include the documented `/emby` prefix; appended paths preserve it. User identifiers must be canonical 32-hex or hyphenated GUID values. It calls the bundled user, password, policy, library, and parental-rating endpoints with bounded responses and typed status errors. Candidate creation is reconciled only by the exact locally persisted name after ambiguous transport failure.

Policy updates never forward a browser document. The adapter fetches the complete current user policy and writes the domain's hardened overlay, preserving remote-access and unrelated fields while disabling hidden-login exposure, remote control, transcoding/remux/conversion, and downloads. Password byte slices are never logged or included in provider error bodies.

## Failure and verification

- Non-2xx status, malformed JSON, provider-declared error, unexpected content type, oversized body, timeout, or cancelled context becomes a typed/redacted error.
- Retried requests use stable local order or user identifiers so delayed callbacks and reconciliation can target the original durable record.
- Adapter tests use `httptest.Server` and assert exact paths, methods, headers, query/body fields, signatures, fixed-decimal preservation, timeouts, redaction, and typed errors.
- Contract fixtures cover every enabled EZPay/BEPusdt rail, both BEPusdt callback/ack variants, cancellation races, Emby policy enforcement, native traffic replay and command uncertainty, current reference responses, and missing/extra fields. Unknown JSON fields are tolerated only where upstream compatibility requires it; required identifiers and amounts remain strict.
