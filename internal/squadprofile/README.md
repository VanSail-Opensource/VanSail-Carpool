# Squad profiles

- `profile.go` defines the three local squad profile shapes and JSON fields.
- `normalize.go` validates, normalizes, and decodes persisted profile JSON.
- `countries.go` contains the ISO alpha-2 allowlist for country-only profiles.
- `profile_test.go` covers profile validation and persisted JSON normalization.
- `features.go` owns the bounded ordered merchandising feature list and safe icon allowlist. A nil list serializes as an empty array; feature copy is administrator-authored.

This package owns local squad metadata only. X-Node remains authoritative for
squad identity, name, and availability. Profile values are normalized before
they reach PostgreSQL and are never used to make upstream provider requests.

The shared feature palette accepts only built-in Phosphor identifiers. Squads and Core combos share the 12-feature, 80-character title and 160-character detail limits. Extend backend validation, frontend icon mappings, API enum and both picker locales together.
