# HTTP API transport

`xnode_node_settings_test.go` verifies that stored legacy-settings compatibility does not admit retired bandwidth, unrelated unknown settings or invalid known types through the strict node-write request decoder.

`xnode_endpoint_preview.go` exposes the browser-administrator read-only endpoint-draft preview. It returns derived inbound metadata, applicable fields, field errors and per-client compatibility with no credential-bearing render output. Endpoint lists and preview/save responses use private no-store headers. Authentication remains the existing browser-admin boundary.

`member_usage_breakdown.go` serves the onboarded member's private
`GET /api/v1/usage/breakdown` projection. Owner identity comes exclusively from
the authenticated session; query strings cannot select another account. The
response retains decimal strings, rational costs and explicit coverage, with
no-store headers. Administrator bearer tokens remain barred from member routes.
Physical Combo/PAYG node entries and named hourly squad entries have separate
identities; enrollment/principal identifiers and encrypted credentials stay private.
`member_usage_breakdown_fixture_test.go` reuses the signed member request helper.
`member_usage_breakdown_test.go` covers privacy, authentication, onboarding,
unsupported projection errors and monetary transport without rounding.
`member_usage_breakdown_hourly_test.go` covers owner-only named squad attribution,
decimal values above JavaScript's exact-integer range and private principal data.

`xnode_provider_zones.go` serves administrator-only saved-provider discovery at
`GET /api/v1/admin/xnode/providers/{id}/zones`, using request cancellation and
no-store responses. It delegates all credential access and queued network work
to the X-Node service and uses existing sanitized error projections.

`member_combo_controls.go` serves owner-scoped squad state and switch commands,
plus earliest-queued eligibility and typed-confirmation activation. It uses
onboarding, signed requests and the existing idempotent operation protections.
- `database_admin_part2.go` continues the focused implementation from its original package module.
- `handlers_part2.go` continues the focused implementation from its original package module.
- `server_part2.go` continues the focused implementation from its original package module.

## Core transport

- `browser_auth_routes.go` mounts public entrance, browser link/session, and
  the restricted `/admin-access` exception and selects signing-cookie scope.
- `browser_auth_sessions.go` restores identity, consumes handoffs, and manages
  independent browser cookies; `browser_auth_middleware.go` renews live admin
  sessions and rechecks eligibility on every protected browser request.
- `browser_auth_links.go` issues five-minute fragment links. Exchange, renewal,
  and logout require the configured same-origin URL.
- `browser_auth_test.go` covers signed identity restoration, cookie flags,
  stable renewal, cross-origin exchange rejection, logout, and member redirects
  in hosted CI; local automated suites remain prohibited.
- `browser_auth_recovery_test.go` verifies that transient exchange failures return
  a retryable response without cookies, preserve the handoff for retry, and still
  enforce single use after successful authentication.
- `session_recovery_test.go` verifies that Mini App session lookup outages return
  retryable availability errors, preserve the bearer, and distinguish actual
  expiration before any protected handler can run.
- `telegram_refund_recovery_test.go` injects a Stars refund receipt write failure
  after the wallet reversal and verifies that webhook redelivery repairs the
  operation and item without repeating the financial change.
- `browser_auth_scope_test.go` covers Mini App rejection from browser admin
  routes, signed-request replay, expired-cookie clearing, role revocation,
  removed login routes, and existing streamed-backup origin/device safeguards.
- `browser_auth_member_test.go` checks that ordinary signed Mini App members
  receive member redirect state while allowlisted Mini App administrators
  retain the browser landing/login entrance until they establish browser scope.
- `browser_shell_test.go` covers independent entry fallback, browser console
  framing/SDK CSP isolation, and preserved Mini App embedding policies.
- Administrator APIs require `txc_admin_session` plus signatures derived from
  `txc_admin_request_key`. Mini App cookies authorize only member APIs and
  restricted `/admin-access` routes. Shared `/me`, display-currency, and
  IP Lookup browser-admin calls additionally send `X-TXC-Session-Scope: browser`.
  Existing backup stream origin/idempotency safeguards remain enforced.

- `server.go` constructs middleware, public routes, authenticated routes, and the SPA fallback.
- `responses.go` owns compact JSON response serialization and request-correlated API errors.
- `ip_lookup_details_test.go` covers signed-route-compatible JSON/SSE negotiation, flushed completion and the lookup-specific response deadline in hosted CI.
- The SPA fallback revalidates `index.html` while serving hashed assets with immutable caching to avoid stale WebView bundles.
  Missing `/assets/` files return an uncacheable 404 instead of a successful HTML
  shell; `static_assets_test.go` covers stale hashes, real assets and admin links.
  CDN asset rules must respect these origin cache headers and avoid caching
  error responses. Overriding them can retain an HTML fallback for an obsolete
  JavaScript hash during a rollback and break later lazy page loads.
- `request_auth.go` requires the per-session HMAC contract on authenticated browser requests.
- `request_auth_test.go` tests signed-route enforcement and session-cookie visibility rules.
- `auth_rate_limit.go` bounds Telegram session exchange attempts by normalized client IP.
- `affiliates.go` exposes member referral pages and optimistic administrator tier replacement through strict, size-limited JSON decoding.
- `affiliates_input_test.go` rejects trailing documents, invalid text, and oversized bodies before affiliate configuration mutation.
- `affiliate_commands.go` accepts exact positive-decimal private referral parameters.
- `affiliate_commands_test.go` covers accepted and rejected referral parameters.
- `auth_rate_limit_test.go` covers client-IP isolation, refill, and forwarded-address handling.
- `request_validation.go` applies shared request validation, strict JSON decoding, and URL parameter normalization.
- `authentication.go` exchanges verified Telegram init data for session and request-signing cookies.
- `authentication_lifetime_test.go` covers real bootstrap configuration through
  Telegram exchange, persisted expiry, browser cookie retention and a signed
  member identity read after advancing the persistence clock in hosted CI.
- `handlers.go` serves onboarding actions, dashboard, catalog, purchases, balance, and ledger history.
  Its dashboard projection preserves ordered active squad labels and subscription
  readiness from the catalog; absent labels serialize as an empty list.
  `dashboard_response_test.go` covers the populated and empty JSON contracts.
- `purchases.go` serves the authenticated queued-purchase cancellation and TXB refund endpoint.
- `member_purchase_operations.go` serves first-term refund quotes, idempotent mutations, and owner-scoped receipts. Member manual resets are not exposed; only previously paid queued reset obligations remain supported.
- `member_connections.go` maps retained connection-service errors; current owner connection observations and grants are served by `member_connection_observations.go`.
- `member_routes.go` mounts the member connection, refund, and operation resources.
- `dashboard_usage.go` validates the authenticated member's UTC traffic range and serves the bounded per-node usage projection.
- `statistics.go` serves cached aggregate snapshots and current native node statistics with retryable availability errors.
- `billing_orders.go` creates, polls, and cancels member payment orders.
- `automatic_renewal.go` serves the owner-only automatic-renewal status and toggle resource and logs unexpected provider failures with request, user, and purchase correlation; manual renewal routes are not public.
- `onboarding.go` serves and administers localized onboarding content.
- `emby.go` serves member and administrator Emby account projections.
- `emby_commands.go` validates durable member setup/update and administrator retry commands.
- `database_admin.go` exposes guarded database inspection, backup download, restore, and streamed-upload transports.

## Community features

- `community_membership.go` maps canonical Telegram membership checks and strict one-space invite eligibility to stable Community transport errors.
- `community_shared.go` defines the shared activity timezone default and nullable JSON request fields.
- `community_helpers.go` contains required idempotency, decimal, collection, and error helpers.
- `activity_types.go` maps Activity domain games, draws, rewards, and history to responses.
- `activity_member.go` serves member Activity overview, check-in, bet, draw, and configuration behavior.
- `activity_admin.go` serves Activity settings and game administration. All five reward settings save atomically through the dedicated settings service; member and Telegram activity reads use its single snapshot. Administrative game, draw and raffle writes carry the current actor into the transactional authorization/audit boundary.
- `activity_settings_recovery_test.go` covers complete HTTP settings updates, audit-failure rollback, stale administrator rejection and explicit recovery without partial reward policy.
- `activity_draw_admin.go` saves validated instant and raffle configurations, publishes drafts, checks live catalog prize references through queued integrations, and exposes current-cost forecasts.
- `activity_types_test.go` checks member-safe prize previews and draw receipt identity mapping.
- `coupons_member.go` serves wallet, soft-discard, and redemption behavior plus coupon response mapping.
- `coupons_admin.go` serves coupon creation, updates and deactivation through the atomic administrator writer. Partial edits retain the exact revision read before merging; stale writes conflict without reactivating a concurrently disabled coupon or losing its audit.

## Administrator features

- `admin.go` registers administrator routes and delegates community administration registration.
- `admin_settings.go` lists, creates, and updates deployment settings.
- `admin_billing_limits.go` validates exact TXB minor-unit bounds before committing the singleton range and audit record atomically.
- `admin_payment_profiles.go` lists, creates, and updates masked EZPay/BEPusdt account profiles with independently enabled channels.
- `admin_catalog.go` manages combos, squad products, and statistics windows; node accessibility is read-only in member catalog quotes.
- `admin_statistics_window.go` validates administrator statistics date ranges and buckets.
- `admin_accounts.go` manages user lists, aggregate user detail, balance adjustments, and admin-as-actor block removal.
  Balance adjustments require a caller-owned `Idempotency-Key`. The actor-scoped
  reference replays the original immutable ledger entry and rejects changed
  target, amount, or reason; it cannot apply a second credit after response loss.
- `admin_inventory_page.go` validates shared cursor, limit, search, and status filters for administrative inventories.
- `admin_user_detail.go` maps the non-duplicated aggregate profile and its active IP blocks to the public response.
- `admin_user_commands.go` serves optimistic entitlement edits, entitlement refunds, and no-charge combo replacements.
- `admin_user_account_commands.go` serves idempotent profile-scoped coupon grants and retained-history discards.
- `admin_user_connections.go` scopes durable connection scans and transient IP projections to the selected profile.
- `admin_bulk_extensions.go` normalizes minute durations or deprecated day inputs for inclusive-OR previews and idempotent bulk-extension jobs.
- `admin_bulk_extensions_test.go` covers minute input, legacy-day normalization, and ambiguous duration rejection.
- `admin_operation_resolution.go` requests evidence-consistent parent receipt repair; unsupported workflows and unfinished item evidence return `OPERATION_RECONCILIATION_REQUIRED` without a provider or financial override.
- `admin_payments.go` lists administrator payment and refund projections, applies refunds, and grants terminal-payment courtesy credits.
- `admin_operations.go` manages backups, durable jobs, and audit-event listings.
- `admin_errors.go` maps administrator domain and upstream failures to transport errors.
- `xnode_routes.go` preserves safe GeoIP/network failure causes in request-correlated logs without exposing asset URLs.

## Operational and provider protocols

- `purchase_addons.go` serves owner-scoped active-term squad quote and idempotent commit routes with stable selection, stock, queued-term, balance, and activation errors.
- `purchase_addons_test.go` covers accepted, rejected and replayed purchase-addon submissions.

- `operations.go` serves liveness and readiness probes.
- `operations_shared.go` contains bounded-context and request-ID helpers.
- `telegram_webhook.go` validates and dispatches Telegram membership and Stars payment updates.
- `telegram_commands.go` checks configured-group raffle keywords and commands before ordinary commands and group-message rewards; PM topic administrator commands resolve the target from the topic. Failed entries get localized replies.
- `telegram_command_target.go` strips Telegram's optional `@botname` suffix only when it matches the cached bot identity, so commands addressed to other bots are ignored.
- `telegram_command_target_test.go` covers addressed raffle and built-in commands, other-bot rejection, and plain keywords.
- `display_currency.go` exposes the owner-scoped display preference and the configured fixed TXB-per-CNY/USD rates without changing authoritative prices.
- `telegram_command_replies.go` composes subscription/combo replies from
  existing catalog services and reads the cached statistics average for check-in copy.
- `telegram_membership.go` refreshes membership and independently welcomes
  genuine non-bot joins in the configured group with a localized safe mention.
- `payment_callbacks.go` handles verified EZPay/BEPusdt callbacks.
- `payment_return.go` forwards checkout-issued capabilities without minting and serves narrow receipts through a capability or signed ownership; see `docs/payment-return-security.md`.
- `payment_return_test.go` exercises checkout issuance, navigation replay, anonymous denial, signed ownership, projection privacy and unchanged settlement.

## Tests

- `server_test.go` covers strict decoding, onboarding access, and SPA delivery.
- `community_test.go` covers nullable inputs, decimal parsing, reward mapping, and required idempotency.
- `community_membership_test.go` covers stable onboarding, membership-check, active-combo, already-joined, and invite-provider transport errors.
- `admin_catalog_test.go` covers strict decoding of typed squad profile writes.
- `telegram_pm_commands_test.go` covers topic-owner targeting and refusal to infer refunds without a positive traffic quote.
- `payment_callbacks_test.go` verifies that navigation returns accept only the documented payment providers.
- `admin_user_detail_test.go` covers owner-ID restoration in nested aggregate records.
- `member_ip_blocks_test.go` verifies administrator actor attribution for retained IP-block removals.
- `telegram_membership_test.go` covers joins, rejoins, promotions, departures,
  bots, missing configuration, and unrelated chats through a pure transition predicate.

`README.md` is this direct-file ownership index. Unsigned routes remain limited to operational probes, Telegram authentication, provider-authenticated callbacks, the capability-limited payment return/status flow, and static assets.
- `handlers_part2.go` contains the remaining route handler implementations.
- `server_part2.go` contains authenticated request-context, static asset, and transport helper implementations.

- `activity_boost_test.go` covers `GROUP_BOOST_REQUIRED` (403) and `GROUP_BOOST_UNAVAILABLE` (503). Activity overview exposes live `groupBoost` state and effective amounts while preserving unrelated activities during verification failures.
- `telegram_deduction.go` handles administrator deductions with a chat/message-scoped
  financial reference, so Telegram redelivery cannot deduct twice. It rechecks
  both configured administrator identity and the current stored administrator role.
  `telegram_deduction_idempotency_test.go` covers replay, changed amounts, distinct
  messages and role removal; `admin_balance_idempotency_test.go` exercises the
  authenticated HTTP retry contract and verifies final wallet balances.

`user_preferences.go` exposes authenticated partial preference updates. `group_member_tag.go` serves owner-scoped live Telegram tag operations.
`user_preferences_test.go` verifies authenticated-principal updates, unknown identity rejection and onboarding gating.
`admin_user_refund_quote.go` serves administrator-only live suggestions with a retryable unavailable-usage error.

`telegram_boost.go` accepts authenticated active boost updates only for the configured group, then atomically queues appreciation; storage failures keep the webhook retryable.

`captcha.go` gates authenticated protected routes before business handlers. Only bootstrap, verification and display currency controls bypass the entry gate. Server construction supplies a fail-closed fallback; app composition injects the queued verifier. CAPTCHA_REQUIRED triggers a fresh browser bootstrap.
`captcha_test.go` covers protected reads, onboarding mutations, allowed bootstrap controls, admin access and rejected/unavailable token failures.

`telegram_pm.go` delegates authenticated ordinary message/callback updates to PM; payment/command paths retain their routing. `telegram_pm_commands.go` applies `/refund`, `/addtxb`, and `/deducttxb` only to the authorized topic owner. `admin_pm.go` retains PM conversation inventory, idempotent moderation and topic repair APIs for the per-user profile controls. `admin_pm_deliveries.go` exposes bounded delivery references and read evidence, including ambiguous sends.

`member_subscription.go` serves the onboarded owner subscription with no-store headers and no raw configuration disclosure.
- `ip_lookup.go` mounts signed member and admin IP Lookup routes with owner scoping and safe errors.
`ip_lookup_details.go` serves authenticated, non-billed, no-store live IP enrichment with feature/onboarding guards and member admission errors.

`ip_lookup_progress.go` serves authenticated exact-IP background progress with no member identity or billing fields. Both progress and combined-cache live-details responses are no-store.


- `usdt.go` ? Admin-only configuration, local status, queued probes and idempotent verified-transfer credit.
- `usdt_test.go` ? Hosted authorization, encrypted credentials and masked read/write contract checks.
`browser_legacy_entry.go` identifies old Telegram iframe launches for access-panel forwarding without granting authentication or exposing admin pages.

`xnode_routes.go` Mounts browser-scoped administration and safe error/global/metric responses.
`xnode_nodes.go` Serves node policy, enrollment and group/endpoint mutations.
`xnode_configurations.go` Serves config/snippet CRUD and realtime native validation.

- `xnode_members.go`: Entry points: `mountXNodeMembers`, `xnodeMemberMetrics`, `xnodeOwnedSubscription`, `xnodeRoutingTemplates`.


- `xnode_operations.go`: Entry points: `xnodeQueueCommand`, `xnodeDeleteNode`, `xnodeDeleteGroup`, `xnodeDeleteEndpoint`.

`api_token_auth.go` authenticates scoped bearer requests without browser cookies. `api_tokens.go` mounts browser-only token management and actor-scoped asynchronous receipt reads. `api_tokens_test.go` covers scopes, role removal, revocation, metadata privacy and session isolation in hosted CI.

Generic database and backup routes require both administrator and X-Node resource
scopes because they expose state from both domains. Database queries sent by POST
remain read-only; compatibility row updates and mutation review/apply require
both write scopes. `api_token_database_scope_test.go` exercises this boundary
through the actual bearer middleware, including full backup downloads.

`admin_user_provider_commands.go` exposes manual temporary-ban and restoration commands.

`xnode_subscription_templates.go` serves administrator-only template reads/writes and synthetic previews with private response headers.

`subscription_content.go` negotiates the copied public landing URL between HTML and native client content. `subscription_content_test.go` covers explicit formats, supported client agents, Accept quality and private browser responses.

Command results normalize large integer counters for exact browser presentation, including historical numeric speedtest results.

`xnode_service_subscription.go` exposes existing standalone subscription metadata
and bearer URLs only under the administrator router, with private no-store
headers. It excludes probe identities and does not create or rotate credentials.

- `xnode_named_services.go`: Admin-only service identity CRUD, subscription rotation and native routing endpoints.

`xnode_firewall.go` exposes administrator-only ban and detection-evidence searches. Paid member firewall purchases are retired; current owner connection grants use their separate lease endpoint.


`xnode_routing_plans.go` exposes owner-scoped typed plans/previews and browser admin
matcher templates. Unsafe cycles and uncertain contexts return bounded diagnostic
codes. Raw member routing/outbound HTTP writes have been retired; service identity
administrator customization remains separate.

`xnode_routing_plans_test.go` guards bounded owner rule references in actionable routing diagnostics.

`usdt_review.go` owns payment-scoped native-token evidence listing, audited settlement and permanent review dismissal over existing browser-signed administrative routes.

`admin_user_metrics.go` serves browser-scoped native per-profile metrics. Node multipliers are retired and normalized to one; obsolete request fields cannot alter current billing. Statistics reads use the existing administrator router.

`xnode_node_write_test.go` verifies that obsolete multiplier values are ignored and normalized for compatibility.

`xnode_subscription_selection.go` previews the draft template policy using the same bounded Go matcher as bearer requests. Both public subscription paths negotiate explicit format, admin regex, built-in detection, then Base64; a regex overrides HTML negotiation. Generated output and synthetic previews remain no-store.

`xnode_firewall_admin.go` handles administrator-only queued IP/CIDR grants under the existing signed browser/API-token middleware; it returns the shared durable operation receipt and never charges a wallet.

`xnode_logs.go` serves admin-only, no-store selected-node volatile log snapshots. The metrics route accepts summary=true to omit buffered logs while preserving default response compatibility.

`xnode_route_speedtests.go` serves authenticated owner-only starts/results at
`/api/v1/xnode/routing/speedtests` and `/{id}`. Start returns a durable receipt;
results retain pair progress and distinguish the aggregate estimate. Credentials,
raw agent errors and command byte counters are excluded from result projections.

`subscription_response.go` applies browser/empty response selections only after
bearer identity and active access validation. Browser matches serve the public
page at the landing URL and redirect raw downloads to it; empty templates send
the selected final status with zero body bytes. `subscription_response_test.go`
covers both delivery paths and status boundaries in hosted CI.

`xnode_diy.go` exposes authenticated owner-scoped DIY reads, draft previews and atomic saves with existing idempotency headers; request owners always come from the authenticated session.

`payg.go` exposes authenticated `GET /api/v1/payg` for the current onboarded member only, with private/no-store headers. The existing squad-product write contract accepts billing mode and integer per-GB pricing.

`payg_test.go` verifies owner isolation even when another user ID is supplied in the query string and confirms no-store financial responses.

- `pricing_policy.go`: Admin-only pricing routes, explicit policy saves and stable domain errors.
- `pricing_views.go`: Bounded overview/data views and direct reasoned intervention/cancellation commands.
- `pricing_history.go`: Original horizon/model-filtered history, version preconditions and provider inputs.
- `pricing_renewal.go`: Read-only owned renewal information; no terms acceptance or owner mutation endpoint.
- `pricing_export_auth.go`: Single-use expiring actor/browser/scope-bound authorization for sensitive analytical exports.
- `pricing_export.go`: Signed direct streaming, bounded lifetime, cancellation, truncation failure and export audit.

- `host_names.go`: Owner-authorized host display preference write handler.

- `personal_forecast.go`: Administrator-scoped user forecasts and pricing analysis.

- `pricing_user_response.go`: Reuses canonical coupon response conversion, including decimal amounts and explicit nulls.

`community.go` mounts the retained membership, activity, and coupon routes.

`telegram_bot_status.go` handles only authenticated private-chat my_chat_member block/unblock transitions. It protects configured/stored administrators, uses the private chat ID as the member ID, and durably enqueues cleanup before acknowledging. Group membership events do not purge members.

Admin group GET includes derived memberCount/canDelete; DELETE rechecks server eligibility and returns 409 when membership or the catalog changed.

`request_origin.go` validates browser same-origin requests and normalized default ports; no file-upload signature exception remains.

`xnode_automatic_leases.go` exposes bounded volatile lease search/pagination at
`GET /api/v1/admin/xnode/firewall/leases`. Existing X-Node read authorization and
private response headers apply; settings writes use the validated global
interface and its write scope. No IP persistence is introduced.

`admin_ip_block_errors.go` maps administrator errors. `member_connection_observations.go` exposes owner observations and rate-limited grants. `member_payg_history.go` exposes private directional history.

`retired_product_routes_test.go` verifies removed administrative outage controls return 404; no transport or dependency can recreate those decisions.

- `api_token_resource_scope_test.go`: Hosted explicit resource checks for native subscription metadata, enrollment mutations and shared receipt polling; general administrator scopes never act as an X-Node superscope.

- `client_ip.go` resolves authentication clients from the trusted right edge of `X-Forwarded-For`, with a single-IP `X-Real-IP` fallback only when that chain is absent. Only configured proxy peers can supply either. The TCP peer remains in `RemoteAddr`; unknown headers do not confer trust. `client_ip_test.go` covers spoofing, malformed prefixes, multiple lines, IPv6, canonical mapped addresses and independent limiter buckets.

`admin_emby_connection.go` exposes administrator-only GET/PUT `/emby-connection`: configuration flags plus an opaque version, with typed conflict/fresh-token errors. It never returns either connection secret or calls Emby. Member readiness uses the same validated atomic credential snapshot as the queued adapter.

`admin_emby_connection_test.go` exercises the actual authentication/scope middleware, secret masking, atomic connection setup, stale-version rejection and both generic-setting bypass routes on PostgreSQL.

## Hourly command authorization

`hourly_command_boundary_test.go` checks that general native-command HTTP, queued
service and public master entry points reject financial credit commands before
replay, persistence or peer lookup. Only the internal durable credit coordinator
can issue their exact authenticated arguments through the node execution queue.

`admin_hourly.go` exposes typed policy and offer CAS through the existing browser
administrator/session scopes. Configuration versions and explicit decimal units
are validated without exposing encryption material or changing legacy pricing.
Automation needs the resource-specific X-Node read/write scope for both hourly
policy and offers; general administrator write scope cannot change rate policy.

`member_hourly.go` and `member_hourly_preferences.go` expose the signed Mini App
hourly snapshot, queued activation/disable and extra-traffic consent. All actions
derive owner/generation from the session. `member_hourly_errors.go` validates
canonical decimal strings and emits stable localized error codes. The optional
Hourly dependency leaves existing deployments/fixtures unavailable until wired;
member routes do not preload or call browser administrator resources.
Hosted `member_hourly_fixture_test.go`, `member_hourly_test.go`,
`member_hourly_preferences_test.go` and `member_hourly_view_test.go` cover signed
sessions, owner/generation isolation, large integers, malformed units, private
queued intents, independent offer visibility, cooldowns and opt-out on policy
failure. These tests do not claim real native or Telegram device validation.

`admin_hourly_test.go` verifies resource-specific authorization, disabled defaults, explicit integer units, stale policy conflicts, and validation failures.
`admin_hourly_input.go` requires every replacement field explicitly, preserving
the distinction between false/zero and omitted/null settings.

`legacy_sales_http_test.go` verifies closed member checkout and quote entrances
cannot create purchases or change the wallet. Immutable checkout replay remains
owner-bound through the catalog; owned renewal and reward entry points stay separate.

`authentication_recovery_test.go` covers verified sign-in during partial account
cleanup. `authentication.go` returns `409 ACCOUNT_RECOVERY_PENDING` without a new
session: authenticated activity cancels deletion evidence, while an administrator
must review any earlier irreversible changes before access can resume.

`admin_user_entitlements.go` serves the audited zero-charge manual grant at
`POST /api/v1/admin/users/{userId}/entitlements`, mapping overlap and active
hourly conflicts to explicit 409 codes without a wallet movement.
`admin_user_hourly.go` serves the owner-scoped administrator hourly projection
and the administrator disable receipt endpoint; neither exposes credentials,
proxy/period references, activation keys, or Telegram controls.

Payment profile retargets with a blank or masked credential return HTTP409 with
`PAYMENT_PROFILE_FRESH_CREDENTIAL_REQUIRED`; the dedicated profile service and
store validate the account identity. Profile mutation audit events commit with
the underlying storage changes, and deletion passes the authenticated actor
through the profile manager to that transaction.

Payment status reads preserve actual missing/foreign-order responses and return a
retryable server error when persistence is temporarily unavailable.
`payment_read_errors_test.go` covers the owned order, signed-owner return and valid
checkout-capability return handlers without exposing storage diagnostics.
`xnode_error_log_test.go` verifies native subscription failures redact bearer paths
through the shared request-path helper, including metadata and browser landing URLs.

`xnode_named_service_errors_test.go` verifies that standalone credential read
failures remain service errors, valid probes remain absent, and failed reads never
rotate or expose subscription credentials.

Native administrator writes in `xnode_configurations.go`, `xnode_nodes.go`,
`xnode_operations.go` and `xnode_named_services.go` carry the authenticated actor
through the service into the atomic current-administrator/audit storage boundary.
This includes deletion, enrollment and subscription rotation, standalone routing,
and metadata-only edits. Resource authorization remains on the existing routes;
a role or lifecycle change after that check cannot authorize a later commit.

`connection_read_errors_test.go` exercises the owner-scoped administrator scan
poll handler. Missing or foreign scans remain 404; transient storage reads return
503 `CONNECTION_SCAN_UNAVAILABLE` with a retry message and no private diagnostics.
