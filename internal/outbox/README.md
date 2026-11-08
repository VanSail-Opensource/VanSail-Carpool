# Outbox payload helpers

- `payload.go` extracts typed target identifiers and validates immutable
  successful-payment announcement snapshots. The optional `providerName`
  field preserves the administrator label while legacy queued payloads retain
  the provider fallback.
  `TargetID` validates only the requested string identifier; numeric revisions,
  booleans, and nested metadata keep their original JSON types. Invalid or
  missing target identifiers still fail before a handler performs work.
- `payload_test.go` covers mixed-type draw job payloads, invalid identifiers,
  and new and legacy payment-announcement JSON payloads.
- `kinds.go` owns shared job-kind constants used by persistence and handlers,
  including durable payment announcements, scheduled purchase receipts,
  terminal reset and refund outcomes.
- `user_notification.go` validates immutable, locale-aware private-chat event
  snapshots used by the durable user-notification worker.
- `user_notification_test.go` covers canonical payload encode/decode behavior.
