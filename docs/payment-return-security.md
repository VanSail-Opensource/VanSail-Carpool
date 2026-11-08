# Checkout receipt access

`billing.checkoutReturnURL` issues a domain-separated `v2` capability only while
building an authenticated checkout request. It binds provider, immutable order
ID and the expiry at order creation plus 30 minutes. Retries cannot extend it.
The injected master key is copied into the service; missing keys fail closed.
No additional persistent secret, token table or payment state is introduced.

Both public navigation routes only forward an already-valid capability to the
same-origin result page. Arbitrary IDs and old publicly-issued capabilities
cannot mint access. Status reads accept either a valid checkout capability or
the ordinary signed session/scoped bearer identity with an owner-filtered
repository lookup. Provider mismatches and foreign ownership return not found.
The narrow receipt excludes owner, checkout URL, QR, trade and provider payloads.
Neither navigation nor receipt reads perform settlement or external requests.

The browser return page signs with its own browser session when available; it
does not borrow a Mini App key. Ordinary members can reopen Telegram for their
authenticated payment history. Authorization failures erase stale receipt
details and stop polling. Missing receipts and transient failures have distinct
localized states. Link navigation never renews expired access.

Hosted regressions cover checkout issuance, immutable expiry, provider/order
binding, old format rejection, navigation replay, anonymous denial, signed owner
access, foreign ownership, projection privacy, unchanged settlement and polling
cessation. The browser fixture mounts the actual result view with constructed
checkout, legacy, expired, unavailable, missing and loading responses.
