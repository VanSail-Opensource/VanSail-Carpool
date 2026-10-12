# TX Carpool OpenAPI contract

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
