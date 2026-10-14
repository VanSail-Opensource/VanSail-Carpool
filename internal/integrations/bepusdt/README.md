# BEPusdt integration
- `transactions_part2.go` continues the focused implementation from its original package module.

This package implements the narrow BEPusdt transaction and callback surface used by VanSail Carpool. Provider-mandated MD5 signatures are isolated to the callback protocol, compared in constant time, and never placed in errors.

- `client.go` defines the sanitized API error, injectable HTTP transport, client options, and validated client construction.
- `transactions.go` validates, signs, sends, and verifies direct transaction creation. It accepts the provider cashier URL as unused response metadata because the member flow renders the validated address directly.
- `discovery.go` defines probe-order capability extraction and normalized USDT/USDC rail metadata.
- `discovery_methods.go` performs the signed probe request without cancelling the provider order and maps provider trade types into discovery results.
- `webhooks.go` parses signed and capability-authenticated callbacks and implements the provider signature algorithm.
- `wire.go` decodes BEPusdt's mixed JSON scalar representations and rejects duplicate or nested callback fields.
- `validation.go` validates callback/payment URLs and exact base-10 decimal values.
- `doc.go` supplies the package documentation.
- `bepusdt_test.go` covers signing fixtures, transaction requests, callback authentication, and response validation.
- `diagnostic_test.go` contains the explicitly opt-in, redacted live create/cancel diagnostic.
- `redirect_test.go` verifies provider redirects cannot receive signed request bodies.
- `README.md` documents the package layout and security boundary.
- `transactions_part2.go` contains callback verification and normalized transaction mapping.
