# HTTP API transport

`member_combo_controls.go` serves owner-scoped squad state and switch commands,
plus earliest-queued eligibility and typed-confirmation activation. It uses
onboarding, signed requests and the existing idempotent operation protections.
- `activity_member_part2.go` continues the focused implementation from its original package module.
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
- `abuse_agent.go` accepts bounded authenticated node log reports.
- `abuse_member.go` exposes privacy-safe member incident history.
- `abuse_admin.go` serves detector rules, records, and node-key controls; `abuse_admin_policy.go` owns the revisioned policy contract and cached-client streak compatibility.
- The SPA fallback revalidates `index.html` while serving hashed assets with immutable caching to avoid stale WebView bundles.
- `request_auth.go` requires the per-session HMAC contract on authenticated browser requests.
- `request_auth_test.go` tests signed-route enforcement and session-cookie visibility rules.
- `auth_rate_limit.go` bounds Telegram session exchange attempts by normalized client IP.
- `affiliates.go` exposes member referral pages and optimistic administrator tier replacement.
- `affiliate_commands.go` accepts exact positive-decimal private referral parameters.
- `affiliate_commands_test.go` covers accepted and rejected referral parameters.
- `auth_rate_limit_test.go` covers client-IP isolation, refill, and forwarded-address handling.
- `request_validation.go` applies shared request validation, strict JSON decoding, and URL parameter normalization.
- `authentication.go` exchanges verified Telegram init data for session and request-signing cookies.
- `handlers.go` serves onboarding actions, dashboard, catalog, purchases, balance, and ledger history.
- `purchases.go` serves the authenticated queued-purchase cancellation and TXB refund endpoint.
- `member_purchase_operations.go` serves first-term refund quotes, idempotent mutations, and owner-scoped receipts. Member manual resets are not exposed; automatic and existing queued reset executors remain supported.
- `traffic_reset_automation.go` serves the onboarded member's account-wide automatic-reset preference and strict boolean update.
- `member_connections.go` preserves scan and drop-route compatibility while the signed-handle command now queues a three-day block followed by disconnect.
- `member_ip_blocks.go` lists owner-only active blocks, queues owner unblocks, and maps ownership mismatches to not found.
- `member_routes.go` mounts the member connection, reset automation, reset, refund, and operation resources.
- `dashboard_usage.go` validates the authenticated member's UTC traffic range and serves the bounded per-node usage projection.
- `statistics.go` serves the cached aggregate snapshot, on-demand shared
  ten-second node snapshot, and image-only process-local node Geocheck result; first-load provider failures are logged with the request ID.
- `rollover.go` serves the authenticated owner's on-demand active-purchase rollover projection, maps ownership, inactive-term, and upstream failures to stable API errors, and correlates unexpected provider failures with the request ID.
- `billing_orders.go` creates, polls, and cancels member payment orders.
- `automatic_renewal.go` serves the owner-only automatic-renewal status and toggle resource and logs unexpected provider failures with request, user, and purchase correlation; manual renewal routes are not public.
- `onboarding.go` serves and administers localized onboarding content.
- `emby.go` serves member and administrator Emby account projections.
- `emby_commands.go` validates durable member setup/update and administrator retry commands.
- `database_admin.go` exposes guarded database inspection, backup download, restore, and streamed-upload transports.
- `backup_upload.go` parses one bounded multipart SQLite stream and publishes it only after hash and schema verification.
- `request_stream_upload.go` recognizes the exact streamed-upload route while retaining origin, session, administrator, and idempotency enforcement.

## Community features

- `community.go` registers member Community, Activity, coupon, and questionnaire routes.
- `community_membership.go` maps canonical Telegram membership checks and strict one-space invite eligibility to stable Community transport errors.
- `community_shared.go` defines shared community settings and nullable JSON request fields.
- `community_helpers.go` contains idempotency, bounded multipart, decimal, collection, and error helpers.
- `activity_types.go` maps Activity domain games, draws, rewards, and history to responses.
- `activity_member.go` serves member Activity overview, check-in, bet, draw, and configuration behavior.
- `activity_admin.go` serves Activity settings and game administration.
- `activity_draw_admin.go` saves validated instant and raffle configurations, publishes drafts, checks live catalog prize references through queued integrations, and exposes current-cost forecasts.
- `activity_types_test.go` checks member-safe prize previews and draw receipt identity mapping.
- `coupons_member.go` serves wallet, soft-discard, and redemption behavior plus coupon response mapping.
- `coupons_admin.go` serves coupon creation, updates, deactivation, and partial-input merging.
- `questionnaire_types.go` maps questionnaire, participation, import, and settlement responses.
- `questionnaire_member.go` serves active questionnaire, history, and participation behavior.
- `questionnaire_admin.go` serves questionnaire lifecycle and editable-input behavior.
- `questionnaire_imports.go` serves bounded CSV upload, analysis, settlement, and import lookup.

## Administrator features

- `admin.go` registers administrator routes and delegates community administration registration.
- `admin_settings.go` lists, creates, and updates deployment settings.
- `admin_billing_limits.go` validates exact TXB minor-unit bounds before committing the singleton range and audit record atomically.
- `admin_payment_profiles.go` lists, creates, and updates masked EZPay/BEPusdt account profiles with independently enabled channels.
- `admin_catalog.go` manages combos, squad products, and statistics windows; node accessibility is read-only in member catalog quotes.
- `admin_statistics_window.go` validates administrator statistics date ranges and buckets.
- `admin_accounts.go` manages user lists, aggregate user detail, balance adjustments, and admin-as-actor block removal.
- `admin_inventory_page.go` validates shared cursor, limit, search, and status filters for administrative inventories.
- `admin_user_detail.go` maps the non-duplicated aggregate profile and its active IP blocks to the public response.
- `admin_user_commands.go` serves optimistic entitlement edits, entitlement refunds, and no-charge combo replacements.
- `admin_user_account_commands.go` serves idempotent profile-scoped coupon grants and retained-history discards.
- `admin_user_provider_commands.go` queues temporary ban, unban, and validated Remnawave identity relink commands.
- `admin_user_connections.go` scopes durable connection scans and transient IP projections to the selected profile.
- `admin_bulk_extensions.go` normalizes minute durations or deprecated day inputs for inclusive-OR previews and idempotent bulk-extension jobs.
- `admin_bulk_extensions_test.go` covers minute input, legacy-day normalization, and ambiguous duration rejection.
- `admin_compensation.go` serves revisioned policy, recipient-safe cursor history, and idempotent approve/dismiss reviews.
- `admin_operation_resolution.go` records idempotent audited resolutions for pending-review and partial operations without provider retry.
- `admin_payments.go` lists administrator payment and refund projections, applies refunds, and grants terminal-payment courtesy credits.
- `admin_operations.go` manages backups, durable jobs, and audit-event listings.
- `admin_errors.go` maps administrator domain and upstream failures to transport errors.

## Operational and provider protocols

- `purchase_addons.go` serves owner-scoped active-term squad quote and idempotent commit routes with stable selection, stock, queued-term, balance, and activation errors.

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
- `payment_callbacks.go` handles EZPay/BEPusdt callbacks, short-lived payment-return capabilities, and navigation-only payment returns/receipt polling.

## Tests

- `server_test.go` covers strict decoding, onboarding access, and SPA delivery.
- `community_test.go` covers nullable inputs, multipart bounds, decimal parsing, reward mapping, and idempotency.
- `community_membership_test.go` covers stable onboarding, membership-check, active-combo, already-joined, and invite-provider transport errors.
- `admin_catalog_test.go` covers strict decoding of typed squad profile writes.
- `operations_commands_test.go` covers Telegram deduction command parsing.
- `telegram_pm_commands_test.go` covers topic-owner targeting and refusal to infer refunds without a positive traffic quote.
- `payment_callbacks_test.go` verifies that navigation returns accept only the documented payment providers.
- `admin_user_detail_test.go` covers owner-ID restoration in nested aggregate records.
- `member_ip_blocks_test.go` covers member isolation, administrator actor attribution, and open-operation conflicts.
- `statistics_geocheck_test.go` covers cached image-only and unavailable node Geocheck responses.
- `telegram_membership_test.go` covers joins, rejoins, promotions, departures,
  bots, missing configuration, and unrelated chats through a pure transition predicate.

`README.md` is this direct-file ownership index. Unsigned routes remain limited to operational probes, Telegram authentication, provider-authenticated callbacks, the capability-limited payment return/status flow, and static assets.
- `handlers_part2.go` contains the remaining route handler implementations.
- `server_part2.go` contains authenticated request-context, static asset, and transport helper implementations.
`statistics.go` checks the optional squad context before returning a cached Geocheck image; disabled squads return `SQUAD_GEOCHECK_DISABLED`. General node statistics and shared background image collection keep their existing behavior.

- `activity_boost_test.go` covers `GROUP_BOOST_REQUIRED` (403) and `GROUP_BOOST_UNAVAILABLE` (503). Activity overview exposes live `groupBoost` state and effective amounts while preserving unrelated activities during verification failures.
- `telegram_deduction.go` contains the unchanged administrator deduction command handling split out to keep the affected command module below 200 lines.

`user_preferences.go` exposes authenticated partial preference updates. `group_member_tag.go` serves owner-scoped live Telegram tag operations.
`user_preferences_test.go` verifies authenticated-principal updates, unknown identity rejection and onboarding gating.
`admin_user_refund_quote.go` serves administrator-only live suggestions with a retryable unavailable-usage error.

`telegram_boost.go` accepts authenticated active boost updates only for the configured group, then atomically queues appreciation; storage failures keep the webhook retryable.

`captcha.go` gates authenticated protected routes before business handlers. Only bootstrap, verification and display currency controls bypass the entry gate. Server construction supplies a fail-closed fallback; app composition injects the queued verifier. CAPTCHA_REQUIRED triggers a fresh browser bootstrap.
`captcha_test.go` covers protected reads, onboarding mutations, allowed bootstrap controls, admin access and rejected/unavailable token failures.

`telegram_pm.go` delegates authenticated ordinary message/callback updates to PM; payment/command paths retain their routing. `telegram_pm_commands.go` applies `/refund`, `/addtxb`, and `/deducttxb` only to the authorized topic owner. `admin_pm.go` retains PM conversation inventory, idempotent moderation and topic repair APIs for the per-user profile controls. `admin_pm_deliveries.go` exposes bounded delivery references and read evidence, including ambiguous sends.

`admin_connectivity.go` exposes the administrator-only host snapshot, asynchronous
manual check admission, exact test-user resolution, and rolling 24-hour history.
Routes inherit signed-session, entry-verification, and configured-administrator
middleware. Configuration uses the existing atomic settings resource. The HTTP
layer delegates scheduling/probes to `connectivity.Service` and bounded history
queries to the store; resolved proxy credentials never enter response DTOs.
`admin_connectivity_test.go` covers actual administrator route middleware,
snapshot/resolution, asynchronous deduplication, and history validation;
`admin_connectivity_helpers_test.go` provides a real SQLite fixture and a
cancellable fake probe for hosted execution without external network traffic.

`member_subscription.go` serves authenticated onboarded GET `/api/v1/connectivity/summary` and `/api/v1/subscription` through the catalog authorization facade. Both return `Cache-Control: no-store`; only the subscription response contains that member's native links. Neither exposes monitoring account identity, raw configurations, latency or HTTP diagnostic numbers.
- `ip_lookup.go` mounts signed member and admin IP Lookup routes with owner scoping and safe errors.
`ip_lookup_details.go` serves authenticated, non-billed, no-store live IP enrichment with feature/onboarding guards and member admission errors.

`ip_lookup_progress.go` serves authenticated exact-IP background progress with no member identity or billing fields. Both progress and combined-cache live-details responses are no-store.

`ip_lookup_proxy_sources.go` provides admin-only saved-source diagnostic starts (POST 202) and no-store memory-only progress/result reads. Existing shared queues execute source downloads and DNS calls.

- `usdt.go` ? Admin-only configuration, local status, queued probes and idempotent verified-transfer credit.
- `usdt_test.go` ? Hosted authorization, encrypted credentials and masked read/write contract checks.
`browser_legacy_entry.go` identifies old Telegram iframe launches for access-panel forwarding without granting authentication or exposing admin pages.

`xnode_routes.go` Mounts browser-scoped administration and safe error/global/metric responses.
`xnode_nodes.go` Serves node policy, enrollment and group/endpoint mutations.
`xnode_configurations.go` Serves config/snippet CRUD and realtime native validation.

- `xnode_members.go`: Entry points: `mountXNodeMembers`, `xnodeMemberMetrics`, `xnodeOwnedSubscription`, `xnodeRoutingTemplates`.

- `xnode_migration.go`: Entry points: `xnodeMigrationStatus`, `xnodeMigrationPreview`, `xnodeMigrationImport`, `xnodeMigrationCutover`.

- `xnode_operations.go`: Entry points: `xnodeQueueCommand`, `xnodeDeleteNode`, `xnodeDeleteGroup`, `xnodeDeleteEndpoint`.
