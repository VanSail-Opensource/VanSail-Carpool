# VanSail Carpool OpenAPI contract

- Browser access uses the `browser-auth.yaml` schemas and public browser/session
  paths. All administrator paths require browser-scoped session signatures;
  `/admin-access` is the sole Mini App administrator exception. Shared `/me`,
  display-currency, and browser IP Lookup calls explicitly select browser scope
  with `X-TXC-Session-Scope: browser`. Browser session restoration also reports
  verified member redirect state without creating member/browser privileges.
- `GET /api/v1/admin/billing/amount-limits` returns global billing limits without
  requiring member wallet access. Its shape matches the existing admin PUT.

- IP Lookup subnet addresses add optional `ports` (the existing port/task/finding contract, at most three entries) and the `suspected` status. Red remains `flagged`; older saved rows can omit probes. Existing quote fields express free shared work with zero charge/quota, and existing progress endpoints distinguish queued from executing checks.

- Active-term squad additions use `POST /api/v1/purchases/{id}/addons/quote` and idempotent `POST /api/v1/purchases/{id}/addons`; the regular purchase route remains full-term priced.

- `openapi.yaml` is the public OpenAPI 3.1 entry point. It registers all API paths, reusable components, and the authenticated browser request-signing requirement.
- `paths/` contains one bounded Path Item Object per API URL and an index mapping filenames to routes.
- Manual maintenance uses an idempotent `POST /api/v1/admin/maintenance` command that returns the shared durable operation receipt.
- `components/security-schemes.yaml` defines the session cookie plus timestamp, nonce, and HMAC signature schemes.
- `components/parameters.yaml` contains reusable path, query, and idempotency parameters.
- `components/responses.yaml` contains reusable API error responses.
- `components/schemas/` contains bounded definitions grouped into shards with a schema index.
- IP Lookup refusal kinds include inventory pool/database/abuse categories and `cloudflare_cdn`; confirmed services share one comma-separated protocol source per port. Public probe evidence preserves its existing schema, and private inventory checkpoints never enter the API response.
- Automatic renewal is represented by one owner-only `GET`/`PUT /api/v1/purchases/{id}/auto-renewal` resource. It always quotes and schedules one next cycle; manual multi-term renewal is not part of the public contract.
- Account-wide automatic traffic reset is represented by `GET`/`PUT /api/v1/me/traffic-reset-automation`; the update body contains only `enabled`.
- Member-facing display currency is represented by `GET`/`PUT /api/v1/me/display-currency`; CNY and USD use server-managed TXB-per-currency rates only for presentation.
- Community access uses `GET /api/v1/community/membership/check` for strict active-combo visibility, `POST /api/v1/community/membership/check` for canonical Telegram facts, and `POST /api/v1/community/invites/{kind}` for one requested group or channel link.
- Squad profile schemas in `schemas-02.yaml` describe local Broadband, China Optimized, and International Network metadata; they do not alter the Remnawave upstream contract.
- Each public squad product carries its required live `accessibleNodes` projection, including nullable provider names; purchase quotes retain the authoritative selected-squad union.

Generate TypeScript types from the root entry point with `npm run generate:api` in `web/`. Authenticated operations inherit all four AND-combined security schemes. Telegram authentication, provider callbacks, payment returns, operational probes, and static delivery retain their explicit unsigned protocols.

Host connectivity is administrator-only under `/api/v1/admin/host-connectivity`. Snapshot, asynchronous manual checks, exact-user resolution and stable 24-hour history return credential-free projections. Scheduling configuration remains one atomic `connectivity.config` JSON value written through the existing validated settings registry.

`API_TOKENS.md` documents scoped bearer authentication, management, expiry and migration operation polling.

`GET /api/v1/admin/xnode/service-identities/{id}/subscription` returns private
metadata and the existing bearer URL for explicitly imported standalone access.
Probe identities are excluded. Quota, expiry, enabled groups and host ACLs remain
authoritative for both standalone client output and server admission.

Subscription UA rules add optional action (omission retains template selection).
Browser/empty response selection is available only without an explicit client
format. Empty templates omit document, accept final status 200-599/default 403,
and are excluded from format defaults/metadata. Preview includes kind/status.

Active firewall bans accept query/cursor/limit; CIDRs match overlap after network
normalization. Administrator grants accept hours=0 only with an exact active
generation, releasing that ban without creating or altering exemptions. The
idempotent receipt remains pending until existing delivery versions acknowledge.
