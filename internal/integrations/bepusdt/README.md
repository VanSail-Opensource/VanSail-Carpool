# BEPusdt integration
- `transactions_part2.go` continues the focused implementation from its original package module.

This package implements the narrow BEPusdt transaction and callback surface used by VanSail Carpool. Provider-mandated MD5 signatures are isolated to the callback protocol, compared in constant time, and never placed in errors.

- `client.go` defines an API error whose formatted text contains only status codes, injectable HTTP transport, client options, and validated client construction.
- `transactions.go` validates, signs, sends, and verifies direct transaction creation. It accepts the provider cashier URL as unused response metadata because the member flow renders the validated address directly.
- `discovery.go` defines probe-order capability extraction and normalized USDT/USDC rail metadata.
- `discovery_methods.go` performs the signed probe request without cancelling the provider order and maps provider trade types into discovery results.
- `webhooks.go` parses signed and capability-authenticated callbacks and implements the provider signature algorithm.
- `wire.go` decodes BEPusdt's mixed JSON scalar representations and rejects duplicate or nested callback fields.
- `validation.go` validates callback/payment URLs and exact base-10 decimal values.
- `doc.go` supplies the package documentation.
- `bepusdt_test.go` covers signing fixtures, transaction requests, callback authentication, and response validation.
- `expiration_test.go` verifies overflow rejection for returned expiry durations and requested timeouts before arithmetic or provider work.
- `diagnostic_test.go` contains the explicitly opt-in, redacted live create/cancel diagnostic.
- `redirect_test.go` verifies provider redirects cannot receive signed request bodies.
- `README.md` documents the package layout and security boundary.
- `transactions_part2.go` contains callback verification and normalized transaction mapping.

Provider messages and request identifiers are untrusted metadata and can reflect
signed bodies or callback capabilities. They are excluded from formatted errors,
public operation receipts, and live diagnostic logs. Timeout and expiration
values retain the upstream seconds contract; values exceeding the maximum
representable Go duration fail validation before conversion.

`wire_integer_test.go` verifies exact integer metadata, including values beyond
float64 precision and the maximum supported expiry. Provider signature number
formatting remains separate from integer field decoding.
