# EZPay integration
- `ezpay_part2.go` continues the focused implementation from its original package module.

This package implements the narrow signed redirect and payment-notification contract used by VanSail Carpool.

- `doc.go` supplies the package documentation.
- `ezpay.go` validates checkout input, builds signed redirect URLs, verifies notifications, and parses exact payment results.
- `ezpay_test.go` verifies the signing fixture, checkout URL contract, callback parsing, duplicate-field rejection, and validation boundaries.
