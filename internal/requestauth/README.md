# Browser request authentication

Every authenticated browser route under `/api/v1` requires an HMAC signature, including read-only `GET` requests and all administrator routes. The opaque `txc_session` cookie stays `HttpOnly`. Authentication also issues `txc_request_key`, a same-origin JavaScript-readable key derived with HMAC-SHA256 from `CONFIG_MASTER_KEY` and the opaque session token.

The browser sends:

- `X-TXC-Timestamp`: Unix seconds, accepted for five minutes.
- `X-TXC-Nonce`: a fresh 16-byte-or-longer base64url value.
- `X-TXC-Signature`: lowercase hexadecimal HMAC-SHA256.

The signed UTF-8 payload is exactly five newline-separated fields:

```text
UPPERCASE_METHOD
ESCAPED_PATH?EXACT_RAW_QUERY
TIMESTAMP
NONCE
LOWERCASE_HEX_SHA256_OF_EXACT_BODY_BYTES
```

`canonical.go` defines that contract and per-session key derivation. `verifier.go` enforces strict header grammars, constant-time comparisons, body restoration, and timestamp freshness. `replay.go` provides a mutex-protected bounded nonce cache with opportunistic expiry cleanup. `verifier_test.go` covers canonical verification, body tampering, malformed values, expiry, replay, and cleanup. `verifier_test_helpers_test.go` builds valid per-session signed request fixtures.

The signing envelope accepts up to 6 MiB so a valid 5 MiB questionnaire CSV and its multipart framing are not rejected. JSON and upload handlers keep their stricter format-specific limits.

Explicit protocol exceptions are the Telegram `initData` bootstrap, Telegram/EZPay/BEPusdt callbacks with their existing provider authentication, health and readiness probes, payment return pages/receipt projections, and static assets. Those routes do not represent an already-authenticated browser API session.

`bearer.go` reuses body and rate budgets for authenticated automation without requiring browser signatures. `bearer_test.go` covers both limits.
