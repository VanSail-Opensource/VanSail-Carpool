# Path item shards

`api-v1-browser-config.yaml` serves `/api/v1/browser/config`.
`api-v1-browser-session.yaml`, `api-v1-browser-exchange.yaml`,
`api-v1-browser-renew.yaml`, and `api-v1-browser-logout.yaml` serve the scoped
session lifecycle under `/api/v1/auth/browser`.
`api-v1-admin-access-links.yaml` exposes the sole signed Mini App administrator
exception under `/api/v1/admin-access`. It issues a short-lived one-use browser
handoff; Telegram OAuth endpoints and settings have been removed.

`api-v1-me-internal-squads.yaml` reads owner-scoped active controls;
`api-v1-me-internal-squads-uuid.yaml` queues validated access changes.
`api-v1-purchases-id-early-activation.yaml` quotes earliest-queued eligibility;
`api-v1-purchases-id-activate.yaml` requires typed confirmation before forfeiture.

Each YAML file is one complete OpenAPI Path Item Object. `../openapi.yaml` registers these files under their canonical URL paths.

- `api-v1-abuse.yaml` groups member history, node-agent upload, and administrator abuse-detector path items.

Member connection scans, purchase refund resources, operation receipts, statistics, and admin bulk/backup mutations use the same signed-session and idempotency conventions as the existing paths.

- `api-v1-auth-telegram.yaml`: `/api/v1/auth/telegram`
- `api-v1-me.yaml`: `/api/v1/me`
- `api-v1-me-display-currency.yaml`: `/api/v1/me/display-currency`, persisted member display preference plus current TXB-per-CNY/USD rates
- `api-v1-me-traffic-reset-automation.yaml`: `/api/v1/me/traffic-reset-automation`, account-wide automatic reset preference
- `api-v1-community-membership-check.yaml`: `/api/v1/community/membership/check`, with `GET` for read-only active-combo visibility and `POST` for canonical Telegram membership refresh
- `api-v1-community-invites-kind.yaml`: `/api/v1/community/invites/{kind}`, one identity-bound group or channel invite
- `api-v1-onboarding-username.yaml`: `/api/v1/onboarding/username`
- `api-v1-onboarding-agreement.yaml`: `/api/v1/onboarding/agreement`
- `api-v1-onboarding-content.yaml`: `/api/v1/onboarding/content`
- `api-v1-dashboard.yaml`: `/api/v1/dashboard`
- `api-v1-dashboard-node-usage.yaml`: `/api/v1/dashboard/node-usage`
- `api-v1-catalog.yaml`: `/api/v1/catalog`
- `api-v1-purchases-quote.yaml`: `/api/v1/purchases/quote`
- `api-v1-purchases.yaml`: `/api/v1/purchases`
- `api-v1-purchases-id-addons-quote.yaml`: `/api/v1/purchases/{id}/addons/quote`, owner-only active-term addition quote
- `api-v1-purchases-id-addons.yaml`: `/api/v1/purchases/{id}/addons`, owner-only idempotent active-term addition commit
- `api-v1-purchases-id-cancel.yaml`: `/api/v1/purchases/{id}/cancel`, owner-only queued cancellation with atomic TXB refund
- `api-v1-purchases-id-rollover.yaml`: `/api/v1/purchases/{id}/rollover`, owner-only live aggregate rollover projection
- `api-v1-purchases-id-auto-renewal.yaml`: `/api/v1/purchases/{id}/auto-renewal`, owner-only one-cycle automatic-renewal state and toggle
- `api-v1-purchases-id-refund.yaml`: `/api/v1/purchases/{id}/refund`, first-term zero-usage refund quote and mutation
- `api-v1-operations-id.yaml`: `/api/v1/operations/{operationId}`, owner-scoped durable operation receipt
- `api-v1-subscription-revoke.yaml`: `/api/v1/subscription/revoke`
- `api-v1-subscription-connections.yaml`: `/api/v1/subscription/connections`, idempotent connection scan creation
- `api-v1-subscription-connections-id.yaml`: `/api/v1/subscription/connections/{scanId}`, owner-scoped scan polling
- `api-v1-subscription-connections-drop.yaml`: `/api/v1/subscription/connections/drop`, signed-handle three-day node block followed by disconnect
- `api-v1-subscription-ip-blocks.yaml`: `/api/v1/subscription/ip-blocks`, owner-only active block list
- `api-v1-subscription-ip-blocks-id-unblock.yaml`: `/api/v1/subscription/ip-blocks/{blockId}/unblock`, owner-only durable unblock
- `api-v1-balance.yaml`: `/api/v1/balance`
- `api-v1-ledger.yaml`: `/api/v1/ledger`
- `api-v1-affiliates.yaml`: `/api/v1/affiliates`, member metrics and tier progress
- `api-v1-affiliates-referrals.yaml`: `/api/v1/affiliates/referrals`, fixed five-row registration pages
- `api-v1-statistics.yaml`: `/api/v1/statistics`, persisted 30-minute aggregate dashboard
- `api-v1-statistics-nodes.yaml`: `/api/v1/statistics/nodes`, on-demand shared 10-second node snapshot
- `api-v1-statistics-nodes-nodeuuid-geocheck.yaml`: `/api/v1/statistics/nodes/{nodeUuid}/geocheck`, cached image-only Geocheck result
- `api-v1-payments-orders.yaml`: `/api/v1/payments/orders`, idempotent durable checkout creation
- `api-v1-payments-orders-id.yaml`: `/api/v1/payments/orders/{id}`
- `api-v1-payments-orders-id-cancel.yaml`: `/api/v1/payments/orders/{id}/cancel`, idempotent durable provider cancellation
- `api-v1-emby-account.yaml`: `/api/v1/emby/account`
- `api-v1-emby-setup.yaml`: `/api/v1/emby/setup`
- `api-v1-emby-preferences.yaml`: `/api/v1/emby/preferences`
- `api-v1-emby-password.yaml`: `/api/v1/emby/password`
- `api-v1-activity.yaml`: `/api/v1/activity`
- `api-v1-activity-check-ins.yaml`: `/api/v1/activity/check-ins`
- `api-v1-activity-bets.yaml`: `/api/v1/activity/bets`
- `api-v1-activity-draws.yaml`: `/api/v1/activity/draws`
- `api-v1-coupons-wallet.yaml`: `/api/v1/coupons/wallet`
- `api-v1-coupons-wallet-id.yaml`: `/api/v1/coupons/wallet/{id}`
- `api-v1-coupons-redeem.yaml`: `/api/v1/coupons/redeem`
- `api-v1-questionnaires-active.yaml`: `/api/v1/questionnaires/active`
- `api-v1-questionnaires-history.yaml`: `/api/v1/questionnaires/history`
- `api-v1-questionnaires-id-participation.yaml`: `/api/v1/questionnaires/{id}/participation`
- `api-v1-admin-activity-settings.yaml`: `/api/v1/admin/activity-settings`
- `api-v1-admin-affiliates.yaml`: `/api/v1/admin/affiliates`, versioned tier configuration
- `api-v1-admin-activity-games.yaml`: `/api/v1/admin/activity-games`
- `api-v1-admin-activity-games-id.yaml`: `/api/v1/admin/activity-games/{id}`
- `api-v1-admin-activity-games-id-statistics.yaml`: `/api/v1/admin/activity-games/{id}/statistics`
- `api-v1-admin-lucky-draw.yaml`: `/api/v1/admin/lucky-draw`
- `api-v1-admin-lucky-draw-id.yaml`: `/api/v1/admin/lucky-draw/{id}`
- `api-v1-admin-lucky-draw-id-statistics.yaml`: `/api/v1/admin/lucky-draw/{id}/statistics`
- `api-v1-admin-lucky-draw-id-publish.yaml`: `/api/v1/admin/lucky-draw/{id}/publish`
- `api-v1-admin-lucky-draw-id-forecast.yaml`: `/api/v1/admin/lucky-draw/{id}/forecast`
- `api-v1-admin-coupons.yaml`: `/api/v1/admin/coupons`
- `api-v1-admin-coupons-id.yaml`: `/api/v1/admin/coupons/{id}`
- `api-v1-admin-questionnaires.yaml`: `/api/v1/admin/questionnaires`
- `api-v1-admin-questionnaires-id.yaml`: `/api/v1/admin/questionnaires/{id}`
- `api-v1-admin-questionnaires-id-close.yaml`: `/api/v1/admin/questionnaires/{id}/close`
- `api-v1-admin-questionnaires-id-activate.yaml`: `/api/v1/admin/questionnaires/{id}/activate`
- `api-v1-admin-questionnaires-id-imports.yaml`: `/api/v1/admin/questionnaires/{id}/imports`
- `api-v1-admin-questionnaires-id-imports-importid.yaml`: `/api/v1/admin/questionnaires/{id}/imports/{importID}`
- `api-v1-admin-questionnaires-id-imports-importid-analyze.yaml`: `/api/v1/admin/questionnaires/{id}/imports/{importID}/analyze`
- `api-v1-admin-questionnaires-id-imports-importid-settle.yaml`: `/api/v1/admin/questionnaires/{id}/imports/{importID}/settle`
- `api-v1-admin-emby-accounts.yaml`: `/api/v1/admin/emby-accounts`
- `api-v1-admin-emby-accounts-id-retry.yaml`: `/api/v1/admin/emby-accounts/{id}/retry`
- `api-v1-admin-settings.yaml`: `/api/v1/admin/settings`
- `api-v1-admin-settings-key.yaml`: `/api/v1/admin/settings/{key}`
- `api-v1-admin-payment-profiles.yaml`: `/api/v1/admin/payment-profiles`
- `api-v1-admin-payment-profiles-provider-rail.yaml`: `/api/v1/admin/payment-profiles/{id}`; the filename is retained for shard compatibility while each stable profile ID owns its enabled channels.
- `api-v1-admin-combos.yaml`: `/api/v1/admin/combos`
- `api-v1-admin-combos-id.yaml`: `/api/v1/admin/combos/{id}`
- `api-v1-admin-combos-id-statistics.yaml`: `/api/v1/admin/combos/{id}/statistics`
- `api-v1-admin-squad-products.yaml`: `/api/v1/admin/squad-products`
- `api-v1-admin-squad-products-import.yaml`: `/api/v1/admin/squad-products/import`
- `api-v1-admin-squad-products-id.yaml`: `/api/v1/admin/squad-products/{id}`
- `api-v1-admin-squad-products-id-statistics.yaml`: `/api/v1/admin/squad-products/{id}/statistics`
- `api-v1-admin-users.yaml`: `/api/v1/admin/users`
- `api-v1-admin-users-id.yaml`: `/api/v1/admin/users/{id}`
- `api-v1-admin-users-id-ip-blocks-blockid-unblock.yaml`: `/api/v1/admin/users/{userId}/ip-blocks/{blockId}/unblock`, admin-as-actor unblock only
- `api-v1-admin-users-id-balance-adjustments.yaml`: `/api/v1/admin/users/{id}/balance-adjustments`
- `api-v1-admin-users-id-entitlements-entitlementid.yaml`: `/api/v1/admin/users/{userId}/entitlements/{entitlementId}`, optimistic entitlement overwrite
- `api-v1-admin-users-id-entitlements-entitlementid-refund.yaml`: `/api/v1/admin/users/{userId}/entitlements/{entitlementId}/refund`, audited entitlement refund operation
- `api-v1-admin-users-id-coupon-grants.yaml` and `api-v1-admin-users-id-coupon-grants-grantid.yaml`: profile-scoped idempotent purchase-coupon grants and retained-history discards
- `api-v1-admin-users-id-temporary-ban.yaml` and `api-v1-admin-users-id-temporary-ban-unban.yaml`: durable temporary-ban and restoration commands
- `api-v1-admin-users-id-remnawave-id.yaml`: queued verified Remnawave identity relink
- `api-v1-admin-users-id-connections.yaml` and `api-v1-admin-users-id-connections-scanid.yaml`: profile-scoped durable scan and transient connection projection
- `api-v1-admin-users-id-combo-replacement.yaml`: `/api/v1/admin/users/{userId}/combo-replacement`, no-charge combo and add-on replacement
- `api-v1-admin-bulk-extensions-preview.yaml`: `/api/v1/admin/bulk-extensions/preview`, minute-precise deduplicated inclusive-OR target preview
- `api-v1-admin-bulk-extensions.yaml`: `/api/v1/admin/bulk-extensions`, durable minute-precise bulk extension job
- `api-v1-admin-node-compensation-config.yaml`: `/api/v1/admin/node-compensation/config`, revisioned outage settings
- `api-v1-admin-node-compensation-events.yaml`: `/api/v1/admin/node-compensation/events`, recipient-safe outage history
- `api-v1-admin-node-compensation-events-id-approve.yaml`: `/api/v1/admin/node-compensation/events/{id}/approve`, reviewed durable extension
- `api-v1-admin-node-compensation-events-id-dismiss.yaml`: `/api/v1/admin/node-compensation/events/{id}/dismiss`, idempotent dismissal
- `api-v1-admin-operations-id-resolve.yaml`: `/api/v1/admin/operations/{operationId}/resolve`, audited ambiguous-operation resolution
- `api-v1-admin-billing-amount-limits.yaml`: `/api/v1/admin/billing/amount-limits`, atomic global Add TXB bounds update
- `api-v1-admin-entitlements.yaml`: `/api/v1/admin/entitlements`
- `api-v1-admin-entitlements-id-cancel.yaml`: `/api/v1/admin/entitlements/{id}/cancel`
- `api-v1-admin-payments.yaml`: `/api/v1/admin/payments`
- `api-v1-admin-payments-id-refund.yaml`: `/api/v1/admin/payments/{id}/refund`
- `api-v1-admin-payments-id-courtesy-credit.yaml`: `/api/v1/admin/payments/{id}/courtesy-credit`
- `api-v1-admin-refunds.yaml`: `/api/v1/admin/refunds`
- `api-v1-admin-database-tables.yaml`: `/api/v1/admin/database/tables`
- `api-v1-admin-database-tables-table-rows.yaml`: `/api/v1/admin/database/tables/{table}/rows`
- `api-v1-admin-database-tables-table-query.yaml`: `/api/v1/admin/database/tables/{table}/query`
- `api-v1-admin-database-mutations-review.yaml`: `/api/v1/admin/database/mutations/review`
- `api-v1-admin-database-mutations.yaml`: `/api/v1/admin/database/mutations`
- `api-v1-admin-backups.yaml`: `/api/v1/admin/backups`
- `api-v1-admin-maintenance.yaml`: `/api/v1/admin/maintenance`, idempotent durable manual maintenance trigger
- `api-v1-admin-backups-upload.yaml`: `/api/v1/admin/backups/upload`, bounded streamed SQLite validation and staging
- `api-v1-admin-backups-id-download.yaml`: `/api/v1/admin/backups/{id}/download`
- `api-v1-admin-backups-id.yaml`: `/api/v1/admin/backups/{id}`
- `api-v1-admin-backups-id-restore.yaml`: `/api/v1/admin/backups/{id}/restore`
- `api-v1-admin-restores-id.yaml`: `/api/v1/admin/restores/{id}`
- `api-v1-admin-jobs.yaml`: `/api/v1/admin/jobs`
- `api-v1-admin-jobs-id-retry.yaml`: `/api/v1/admin/jobs/{id}/retry`
- `api-v1-admin-jobs-id.yaml`: `/api/v1/admin/jobs/{id}`
- `api-v1-admin-onboarding-content-kind.yaml`: `/api/v1/admin/onboarding/content/{kind}`
- `api-v1-admin-onboarding-content-kind-draft.yaml`: `/api/v1/admin/onboarding/content/{kind}/draft`
- `api-v1-admin-onboarding-content-kind-publish.yaml`: `/api/v1/admin/onboarding/content/{kind}/publish`
- `api-v1-admin-audit-events.yaml`: `/api/v1/admin/audit-events`
- `api-v1-webhooks-telegram.yaml`: `/api/v1/webhooks/telegram`
- `api-v1-webhooks-ezpay.yaml`: `/api/v1/webhooks/ezpay`
- `api-v1-webhooks-bepusdt.yaml`: `/api/v1/webhooks/bepusdt`
- `api-v1-webhooks-bepusdt-probe.yaml`: `/api/v1/webhooks/bepusdt/probe`, no-op acknowledgement for discovery-only probe orders
- `api-v1-webhooks-bepusdt-capability.yaml`: `/api/v1/webhooks/bepusdt/{capability}`
- `api-v1-payments-return-provider.yaml`: `/api/v1/payments/return/{provider}`
- `api-v1-payments-return-provider-orderid.yaml`: `/api/v1/payments/return/{provider}/{orderID}`
- `api-v1-payments-return-provider-orderid-status.yaml`: `/api/v1/payments/return/{provider}/{orderID}/status`; short-lived signed receipt projection for provider-return pages
- `healthz.yaml`: `/healthz`
- `readyz.yaml`: `/readyz`
`api-v1-statistics-nodes-nodeuuid-geocheck.yaml` accepts a squad context and documents disabled (403) and settings-unavailable (503) responses.

- `api-v1-activity-check-ins.yaml` documents personal-boost eligibility, half-up reward scaling, and distinct 403 and 503 refusal codes.

`api-v1-me-preferences.yaml` defines preference reads and partial updates; `api-v1-me-group-member-tag.yaml` defines canonical Telegram tag reads and writes.

`api-v1-me-captcha.yaml` defines authenticated first-entry verification and its fail-closed provider errors.

`api-v1-admin-pm.yaml`, `api-v1-admin-pm-id.yaml`, `api-v1-admin-pm-id-deliveries.yaml` and `api-v1-admin-pm-id-topic.yaml` define panel-admin conversation, idempotent controls, review and recovery routes.

`api-v1-admin-host-connectivity.yaml` exposes administrator-only progress and current results. `api-v1-admin-host-connectivity-checks.yaml` accepts or returns an active manual run. `api-v1-admin-host-connectivity-test-user-resolve.yaml` resolves an exact test username, and `api-v1-admin-host-connectivity-history.yaml` exposes host-filtered 24-hour cursor history with a default page of 50 and maximum of 200.

`api-v1-connectivity-summary.yaml` declares the onboarded member's credential-free 24-hour summary. `api-v1-subscription.yaml` declares live owner-specific native links and current-squad histories. Both prohibit persistent caching and return empty member collections when no combo is active.
- `api-v1-admin-ip-lookup.yaml` describes the signed IP Lookup operation and its typed responses.
- `api-v1-ip-lookup-checks-id.yaml` describes the signed IP Lookup operation and its typed responses.
- `api-v1-ip-lookup-checks.yaml` describes the signed IP Lookup operation and its typed responses.
- `api-v1-ip-lookup-quote.yaml` describes a read-only signed quote with an automatic exact-IP or IPv4 `/24` cache preview; previews do not create operations or consume allowance.
- `api-v1-ip-lookup.yaml` describes the signed IP Lookup operation and its typed responses.
`api-v1-ip-lookup-details.yaml` defines GET /api/v1/ip-lookup/details and source-independent partial outcomes without financial operations.

`api-v1-ip-lookup-progress.yaml` documents the authenticated no-store exact-IP work projection. A null response means no check is running; reading progress never creates a receipt or consumes quota.

`api-v1-admin-ip-lookup-source-test.yaml` defines admin diagnostic starts (202); `api-v1-admin-ip-lookup-source-test-id.yaml` defines no-store progress/result reads. Existing report contracts gain optional source diagnostics, task outcomes and bounded DNS evidence.

- `api-v1-admin-ip-lookup-sources.yaml`: admin-only read of shared source cache phase, unique-IP counts and freshness/scheduling metadata; single-source refresh retains the existing test operation routes.

- `api-v1-admin-usdt-check.yaml` ? Dedicated administrator USDT check API contract.
- `api-v1-admin-usdt-resolution.yaml` ? Dedicated administrator USDT resolution API contract.
- `api-v1-admin-usdt-review.yaml` ? Dedicated administrator USDT review API contract.
- `api-v1-admin-usdt-status.yaml` ? Dedicated administrator USDT status API contract.
- `api-v1-admin-usdt.yaml` ? Dedicated administrator USDT  API contract.

`api-v1-admin-xnode-*.yaml` defines browser-scoped node, config, snippet, group, endpoint, enrollment, validation and metrics operations.

`api-v1-admin-xnode-configurations-id.yaml` defines the matching browser-scoped X-Node resource operation.
`api-v1-admin-xnode-configurations-validate.yaml` defines the matching browser-scoped X-Node resource operation.
`api-v1-admin-xnode-configurations.yaml` defines the matching browser-scoped X-Node resource operation.
`api-v1-admin-xnode-endpoints-id.yaml` defines the matching browser-scoped X-Node resource operation.
`api-v1-admin-xnode-endpoints.yaml` defines the matching browser-scoped X-Node resource operation.
`api-v1-admin-xnode-global.yaml` defines the matching browser-scoped X-Node resource operation.
`api-v1-admin-xnode-groups-id.yaml` defines the matching browser-scoped X-Node resource operation.
`api-v1-admin-xnode-groups.yaml` defines the matching browser-scoped X-Node resource operation.
`api-v1-admin-xnode-metrics.yaml` defines the matching browser-scoped X-Node resource operation.
`api-v1-admin-xnode-nodes-id-enrollment.yaml` defines the matching browser-scoped X-Node resource operation.
`api-v1-admin-xnode-nodes-id.yaml` defines the matching browser-scoped X-Node resource operation.
`api-v1-admin-xnode-nodes.yaml` defines the matching browser-scoped X-Node resource operation.
`api-v1-admin-xnode-snippets-id.yaml` defines the matching browser-scoped X-Node resource operation.
`api-v1-admin-xnode-snippets.yaml` defines the matching browser-scoped X-Node resource operation.

- `api-v1-admin-xnode-migration-import.yaml` defines the typed X-Node contract for this resource.

- `api-v1-admin-xnode-migration-nodes-id-cutover.yaml` defines the typed X-Node contract for this resource.

- `api-v1-admin-xnode-migration-nodes-id-rollback.yaml` defines the typed X-Node contract for this resource.

- `api-v1-admin-xnode-migration-preview.yaml` defines the typed X-Node contract for this resource.

- `api-v1-admin-xnode-migration.yaml` defines the typed X-Node contract for this resource.

- `api-v1-admin-xnode-nodes-id-commands.yaml` defines the typed X-Node contract for this resource.

- `api-v1-xnode-metrics.yaml` defines the typed X-Node contract for this resource.

- `api-v1-xnode-routing-id-outbounds.yaml` defines the typed X-Node contract for this resource.

- `api-v1-xnode-routing-id-rules.yaml` defines the typed X-Node contract for this resource.

- `api-v1-xnode-routing-id.yaml` defines the typed X-Node contract for this resource.

- `api-v1-xnode-routing-templates.yaml` defines the typed X-Node contract for this resource.

- `api-v1-xnode-subscription.yaml` defines the typed X-Node contract for this resource.

- `api-v1-subscriptions-token-metadata.yaml` defines owner-scoped bearer subscription metadata.
