# VanSail Carpool OpenAPI contract

Native configuration deletion exposes 204, missing-document 404 and protected-use
409 outcomes. Existing administrator authentication/signing remains mandatory;
current and historical node references continue to prevent deletion.

Usage Breakdown separates fixed-term/PAYG nodes from named hourly squads with
recorded traffic and costs. The two arrays share a 128-entry cap and combined
`positiveEntries`; physical-only `positiveNodes` remains deprecated compatibility.
Exact omitted totals and shared Unallocated charges preserve conservation without
historical repricing or estimated hourly allocations.

Administrator endpoint previews accept incomplete drafts and return derived
native metadata, expanded inbound choices, applicable typed fields, field errors
and per-client compatibility. Endpoint read metadata is never a writable native
protocol selector. Encrypted `overrides.clientOptions` uses its generated
administrator allowlist; member DIY types/permissions remain independent.

Node traffic multipliers are retired. `XNodeNodeWrite` accepts exactly the six
editable fields `id`, `name`, `enabled`, `countryCode`, `providerName` and `configId`.
The node response no longer exposes `multiplier`; obsolete multiplier inputs and
server-owned agent, revision, removal and timestamp fields are rejected on write.
Catalog and statistics responses retain their separate historical accounting
contracts. Existing recorded usage and financial facts remain unchanged;
PAYG/hourly prices and reward multipliers have independent contracts.

Certificate lifecycle reuses node commands with `name=certificates.run`,
`profileId`, `action=issue|renew` and `expectedRevision`. Delivery uses existing
durable receipts; execution is bounded to five minutes. GET on the same node
command path with `profileId` recovers the latest safe receipt/intent across
administrator devices, including pending/uncertain outcomes. Optional
`certificateProgress` in node metrics is advisory and memory-only. Certificate
status/results expose public certificate facts, never keys/provider credentials.

Node DNS writes expose only `redirect`; obsolete resolver `mode`, `upstreams`
and public `legacyDnsLoopback` inputs are rejected. Effective DNS uses native
Xray `dns.servers`. The backend privately migrates existing encrypted node
settings so historical loopback fallback remains effective until explicit native
resolver settings replace it; that migration state is never a writable API field.

Node firewall settings retain legacy individual ports and add inclusive TCP,
UDP and SSH ranges. Forwarding adds optional source/target end ports: a source
range maps either to one target port or an equal-length target range. Range
configuration requires the `native_firewall_port_ranges_v1` agent capability;
unsupported agents retain their previous applied configuration.

- Browser access uses the `browser-auth.yaml` schemas and public browser/session
  paths. Administrator resource paths accept signed browser sessions or their
  explicit API-token scopes; token management requires a signed browser session.
  `/admin-access` is the sole Mini App administrator exception. Shared `/me`,
  display-currency, IP Lookup and payment-return status calls explicitly select
  browser scope with `X-TXC-Session-Scope: browser` when using browser credentials.
  Browser session restoration also reports verified member redirect state without
  creating additional member or administrator privileges.
- `GET /api/v1/admin/billing/amount-limits` returns global billing limits without
  requiring member wallet access. Its shape matches the existing admin PUT.

- IP Lookup exposes reputation and network context for an exact address. Signed
  quotes preserve their original charge/quota terms across same-intent retries;
  authenticated progress reads report shared work without creating an operation.
  Legacy inventory, subnet-probe and administrator source-diagnostic routes are
  retired.

- Active-term squad additions use `POST /api/v1/purchases/{id}/addons/quote` and idempotent `POST /api/v1/purchases/{id}/addons`; the regular purchase route accepts historical receipt replay only after new fixed-term sales close.

- `openapi.yaml` is the public OpenAPI 3.1 entry point. It registers all API paths, reusable components, and the authenticated browser request-signing requirement.
- `paths/` contains one bounded Path Item Object per API URL and an index mapping filenames to routes.
- Manual maintenance uses an idempotent `POST /api/v1/admin/maintenance` command that returns the shared durable operation receipt.
- `components/security-schemes.yaml` defines independent Mini App and browser
  sessions, request-signing headers, scoped API tokens and checkout capabilities.
- `components/parameters.yaml` contains reusable path, query, and idempotency parameters.
- `components/responses.yaml` contains reusable API error responses.
- `components/schemas/` contains bounded definitions grouped into shards with a schema index.
- Automatic renewal is represented by one owner-only `GET`/`PUT /api/v1/purchases/{id}/auto-renewal` resource. It always quotes and schedules one next cycle; manual multi-term renewal is not part of the public contract.
- Member-facing display currency is represented by `GET`/`PUT /api/v1/me/display-currency`; CNY and USD use server-managed TXB-per-currency rates only for presentation.
- Community access uses `GET /api/v1/community/membership/check` for strict active-combo visibility, `POST /api/v1/community/membership/check` for canonical Telegram facts, and `POST /api/v1/community/invites/{kind}` for one requested group or channel link.
- Squad profile schemas in `schemas-02.yaml` describe local Broadband, China Optimized, and International Network metadata; they do not alter the X-Node upstream contract.
- Each public squad product carries its required live `accessibleNodes` projection, including nullable provider names; purchase quotes retain the authoritative selected-squad union.

Generate TypeScript types and runtime response shapes from the root entry point
with `npm run generate:api` in `web/`. The default member authorization combines
its cookie, timestamp, nonce and HMAC signature. Each explicit security entry is
an alternative authorization path; browser credentials and API tokens apply only
where listed. Provider-return status requires an existing checkout capability or
signed owner authentication; its navigation routes are public.


`API_TOKENS.md` documents scoped bearer authentication, management, expiry and
scoped operation polling. Administrator and native scopes remain independent;
generic database and backup mutations require both write families.

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

Emby connection changes use administrator GET/PUT `/api/v1/admin/emby-connection`. Both secrets remain write-only, updates compare the opaque version, and changing the URL requires a fresh token. Generic settings writes cannot change the two connection keys.

Provider-return status accepts a checkout-issued v2 capability or signed owner
authentication. Capabilities expire 30 minutes after order creation; public
navigation forwards but never issues them. Legacy links require ownership.

Hourly commerce uses independent offers and enrollments; no legacy purchase is converted. Activation/disable return existing durable operation receipts, and whole-hour renewals preserve exact paid-through authorization. All large byte and monetary accounting integers remain decimal strings.

Native telemetry accepts only sampling/report intervals. The inert
`include_active_ips` field is rejected on new writes; active-IP reporting remains
mandatory. Historical encrypted global settings are read through a private
storage adapter and no longer expose that obsolete flag.
