# Administrator API tokens

`service.go` issues 256-bit opaque `vsc_` credentials, checks expiry and the
owner's current role/administrator allowlist, and loads metadata for browser
administrators. `types.go` defines repository contracts and secret-free metadata.
Only the creation response contains a secret; PostgreSQL retains a SHA-256 digest.

`scopes.go` confines bearer use to administrator APIs and distinguishes `admin`
and `xnode` read/write scopes. Write includes read access within that
resource only. General administrator scopes never authorize native X-Node
reads, enrollment credentials, mutations or operation receipts. Credential
management and the retired detector are excluded from bearer authorization.
Operation receipt reads also check resource scope and actor/owner identity.

The generic database editor and full backup APIs span both resources. Reads,
including the read-only database `POST .../query`, require both `admin:read`
and `xnode:read` (or each resource's write scope). Mutations require both
`admin:write` and `xnode:write`. This also protects the compatibility database
update route and backup downloads from bypassing native control scope.

Tokens default to 30-day absolute expiry. Creation accepts 0–365 days; zero
explicitly selects no expiry. Each administrator may hold 50 active tokens.
The browser lists up to 200 active/recent owned records and revokes only its
owner's tokens. Revocation is idempotent. Last-use timestamps update at most
once every five minutes; no request bodies or node telemetry are retained.

HTTP integration uses the existing request-auth body/rate budgets, role checks,
request queue, idempotency and provider-operation receipts. Token authentication
does not create cookies or browser/Mini App sessions. See [API usage](../../api/API_TOKENS.md).

`scopes_test.go` covers resource separation, unsafe paths and credential-management exclusions.

`cross_domain_scopes_test.go` covers database and backup resource combinations,
read-only queries, and mutation paths that must never inherit read access.

`hourly_scopes_test.go` verifies hourly policy, pricing and operation-receipt scopes, and excludes API tokens from owner commerce.
