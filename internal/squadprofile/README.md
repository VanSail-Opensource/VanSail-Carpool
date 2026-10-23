# Squad profiles

- `profile.go` defines the three local squad profile shapes and JSON fields.
- `normalize.go` validates, normalizes, and decodes persisted profile JSON.
- `countries.go` contains the ISO alpha-2 allowlist for country-only profiles.
- `profile_test.go` covers profile validation and persisted JSON normalization.

This package owns local squad metadata only. X-Node remains authoritative for
squad identity, name, and availability. Profile values are normalized before
they reach SQLite and are never used to make upstream provider requests.
