# Administrator API tokens

`service.go` issues 256-bit opaque `vsc_` credentials, checks expiry and the
owner's current role/administrator allowlist, and loads metadata for browser
administrators. `types.go` defines repository contracts and secret-free metadata.
Only the creation response contains a secret; SQLite retains a SHA-256 digest.

`scopes.go` confines bearer use to administrator APIs and distinguishes `admin`,
`xnode` and `migration` read/write scopes. Write includes read access. Migration
does not inherit commerce, settings or database permissions. Credential
management and the retired detector are excluded from bearer authorization.
Operation receipt reads also check resource scope and actor/owner identity.

Tokens default to 30-day absolute expiry. Creation accepts 0–365 days; zero
explicitly selects no expiry. Each administrator may hold 50 active tokens.
The browser lists up to 200 active/recent owned records and revokes only its
owner's tokens. Revocation is idempotent. Last-use timestamps update at most
once every five minutes; no request bodies or node telemetry are retained.

HTTP integration uses the existing request-auth body/rate budgets, role checks,
request queue, idempotency and provider-operation receipts. Token authentication
does not create cookies or browser/Mini App sessions. See [API usage](../../api/API_TOKENS.md).

`scopes_test.go` covers resource separation, unsafe paths and credential-management exclusions.
