# Provider operations

This package defines provider-neutral command, item, replay, and connection-scan
metadata. SQLite persists request fingerprints and sanitized lifecycle facts;
the outbox performs external calls. Raw connection IPs and signed connection
handles remain short-lived service data and are never stored in these records.

- `types.go` defines operation states, commands, receipts, items, and results.
- `connections.go` defines metadata-only connection scan inputs and progress.
- `validation.go` canonicalizes commands and bounded JSON result objects.
- `dispatcher.go` routes the one shared outbox lane to kind-specific handlers.
- `dispatcher_test.go` covers kind routing without provider calls.

`telegram_pm.go` reserves relay/profile/repair kinds in the existing durable lane. Targets store conversation and Telegram message references; no message body is part of an operation.

`admin_kinds.go` defines supported administrator command kinds. Retired mutation kinds are no longer dispatched.

`command_kinds.go` defines subscription, Emby, retry, and refund command identifiers.
