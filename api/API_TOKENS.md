# Administrator API access

Open **API access** in the browser administrator console. Name a token, choose
permissions and expiry, then copy the one-time secret. Tokens belong to their
creator; removing that administrator's role or allowlist entry disables access.
Revoke a token in the same page to prevent further requests.

| Scope | Access |
| --- | --- |
| `admin:read` | Read administrator data |
| `admin:write` | Read and change administrator data, including commerce/system controls |
| `xnode:read` | Read nodes, configuration, groups, endpoints and metrics |
| `xnode:write` | Also configure X-Node and submit commands |
| `migration:read` | Read migration status |
| `migration:write` | Also preview/import, cut over, roll back, export final accounting and finalize |

The migration-assistant preset grants `xnode:write` and `migration:write`.
Write includes read access within its resource. Token management endpoints
require a signed browser session. Bearer tokens do not authenticate member,
browser-login, Mini App access-link, subscription-bearer or agent endpoints.

Supply the token through `Authorization: Bearer`. Browser cookies and HMAC
headers are unnecessary. Keep TLS enabled on the panel. Store the token in your
automation's secret environment; the server cannot redisplay it.

```sh
curl --fail-with-body \
  -H "Authorization: Bearer $API_TOKEN" \
  "$PANEL_URL/api/v1/admin/xnode/nodes"

curl --fail-with-body \
  -H "Authorization: Bearer $API_TOKEN" \
  "$PANEL_URL/api/v1/admin/xnode/migration"
```

Mutation endpoints retain existing typed bodies and `Idempotency-Key`
requirements. Reuse the same key only when retrying the same action/body.
Node handovers still perform configuration, credentials, accounting, backup and
signed-agent checks; a token does not bypass them.

```sh
curl --fail-with-body -X POST \
  -H "Authorization: Bearer $API_TOKEN" \
  -H "Idempotency-Key: $OPERATION_KEY" \
  "$PANEL_URL/api/v1/admin/xnode/migration/nodes/$NODE_ID/cutover"

curl --fail-with-body \
  -H "Authorization: Bearer $API_TOKEN" \
  "$PANEL_URL/api/v1/admin/operations/$OPERATION_ID"
```

Queued actions return their durable receipt immediately. Poll the administrator
operation endpoint until it reaches a terminal state; `pending_review` requires
review/recovery rather than assuming cutover succeeded. Receipt access is scoped
to the current actor/owner and the token's resource permissions. Revoking a token
does not cancel an action already accepted into the queue.

The existing request budget applies per token (512 requests per five-minute
window), with bounded normal bodies and separately bounded backup streaming.
No token is accepted from URLs or query parameters. Creation defaults to 30 days;
`expiresInDays: 0` explicitly creates a non-expiring token. The management API
lists at most 200 active/recent owned metadata records; 50 active tokens per
administrator are permitted. Secret values/hashes never appear in list responses
or token audit events.
