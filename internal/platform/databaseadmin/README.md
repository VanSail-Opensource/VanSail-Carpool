# PostgreSQL administration

The browser API exposes an allowlisted, read-only PostgreSQL catalog and typed
record pages. Domain APIs own all mutations. Historical mutation request/receipt
wire types remain accepted only to return an explicit read-only error; generic
writes cannot create reviews, rescue archives or alter credentials/financial data.

- `types.go` and `types_part2.go` define decimal-string/boolean/base64 transport,
  schema metadata, paging/filter contracts and historical mutation shapes.
- `schema.go` and `schema_part2.go` classify native PG types and redact sensitive
  fields; `postgres.go` resolves validated identifiers and declared primary keys
  through server catalogs. Tables without keys cannot be paginated.
- `records.go` builds native parameterized cursor pages and redacted row hashes.
- `record_keys.go` preserves exact numeric keys and seals sensitive keys/cursors.
- `record_values.go` renders exact values without browser-number conversion.
- `query.go` and `query_part2.go` bind typed filters/search and fingerprint sealed
  query cursors. `filter_values.go` validates values using the resolved PG type.
- `mutations.go` rejects generic writes before any side effect.
- `service_test.go` covers native catalogs, declared-key pagination, large numeric
  precision and malformed input. `mutation_service_test.go` verifies secret
  masking and unchanged records when a generic write is attempted.

No user-supplied identifier reaches SQL without catalog resolution and identifier
validation. No query results, raw SQL or secret argument values enter logs.
