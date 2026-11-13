# Administrator API access

Create tokens in the browser administrator console's API access page. Tokens
belong to their creator and require that creator's current administrator role
and allowlist eligibility. Only the creation response exposes the secret.

| Scope | Access |
| --- | --- |
| `admin:read` | Read administrator data |
| `admin:write` | Read and change administrator data, including commerce/settings |
| `xnode:read` | Read nodes, configuration, groups, endpoints and metrics |
| `xnode:write` | Configure X-Node and submit node commands |

Write includes read access within its resource. `admin:read` and `admin:write`
do not include X-Node access; grant an explicit `xnode:read` or `xnode:write`
scope for that resource. Combining `admin:write` with `xnode:read` still cannot
submit X-Node mutations. Native operation receipts retain the X-Node scope
requirement on the shared operation-polling endpoint. Token management requires a
signed browser session. Bearer tokens do not authenticate member, browser-login,
Mini App handoff, subscription-bearer or agent endpoints.

Generic database and backup operations span both administrator and X-Node data.
Their read endpoints, including `POST /admin/database/tables/{table}/query`,
require read access to **both** resources. Their mutations require write access
to both resources. An `admin:*` token alone cannot use these generic operations
to bypass the native scope boundary. Signed browser administrator sessions keep
their existing access policy.

```sh
curl --fail-with-body -H "Authorization: Bearer $API_TOKEN" "$PANEL_URL/api/v1/admin/xnode/nodes"
```

Use HTTPS and keep the token in an automation secret. Typed mutation bodies and
`Idempotency-Key` requirements remain in force. Reuse a key only for the same
action and body. Queued mutations return durable operation receipts; poll the
administrator operation endpoint until terminal completion. Receipt reads also
check actor/owner and resource scope. Revocation prevents subsequent requests
without cancelling already accepted work.

Administrator balance adjustments also require `Idempotency-Key`. Keep the same
key when retrying an unchanged adjustment after a lost response; use a new key
for a different amount, member or reason. The key is scoped to the acting
administrator and replays the existing ledger entry without another balance
change or notification.

The existing request budget applies per token. Tokens are never accepted from
URLs or query parameters. Creation defaults to 30 days; `expiresInDays: 0`
selects no expiry. Each administrator may hold 50 active tokens. Migration
endpoints, scopes, and source credentials are retired. Native catalog contracts
use `groupId`; previous provider-specific wire aliases are removed.

Revoking a token deletes its credential record. Earlier revoked records are purged on upgrade. Secret-free audit events remain, and repeat revocation by the same owner is idempotent.
