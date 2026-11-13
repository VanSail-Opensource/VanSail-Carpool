# Imported Emby API reference

`openapi.json` is an imported OpenAPI 3.0.1 document identifying Emby Server API
version `4.1.1.0`. The import contains 356 paths, 445 operations and 201 component
schemas. Its original source revision and retrieval date were not recorded;
the document does not establish compatibility with every deployed Emby version.

The `servers` entry is the original export's third-party server address. It is
reference metadata, never a Carpool default or an authorized test target. Do not
send credentials or execute generated requests against that address. Production
connections use the separately configured, validated encrypted Emby settings.

The maintained adapter in `internal/integrations/emby` uses the documented
user creation, lookup, deletion, password, policy, selectable-folder and parental
rating operations. It sends credentials in `X-Emby-Token`, retains bounded
responses, rejects redirects and validates the exact returned identity. The
account saga and hosted contract tests own authorization, reconciliation and
financial behavior; this reference supplies no runtime implementation.

The complete JSON parses without duplicate keys, and every local `$ref` resolves.
No external schema references are required. Audit coverage for this generated
document is complete structural traversal plus review of the adapter's consumed
contracts, rather than a claim of hand-reading every repeated schema property.
The reference directory is excluded from the production image.
