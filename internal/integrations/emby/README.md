# Emby integration
- `client_part2.go` continues the focused implementation from its original package module.

- `client.go` implements the typed HTTP adapter and maps Emby payloads into the domain model.
- `validation.go` validates the configured server URL and Emby GUID identifiers.
- `client_test.go` verifies request paths, authentication, payloads, response mapping, and error handling.
- `boundary_validation_test.go` rejects malformed user identifiers returned by list and create operations.
- `redirect_test.go` verifies provider redirects cannot receive the API token or password body.
- `doc.go` documents the package boundary.
- `client_part2.go` contains response decoding, error classification, and domain mapping.

`delete.go` implements documented DELETE /Users/{Id}; the queued adapter treats a confirmed 404 as an already completed deletion. Other failures retain cleanup for retry.
