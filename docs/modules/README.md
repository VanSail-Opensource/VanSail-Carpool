# Module specifications

These documents describe the implemented module boundaries. They are maintenance contracts: each document states ownership, durable invariants, external interfaces, failure behavior, and the verification expected before release.

| Module | Specification |
| Native commerce, retirement and subscription template delivery | [native-commerce-delivery.md](native-commerce-delivery.md) |
| --- | --- |
| Runtime, persistence, scheduler, and backups | [platform-runtime.md](platform-runtime.md) |
| Telegram identity and onboarding | [accounts-onboarding.md](accounts-onboarding.md) |
| Catalog, purchases, access terms, and rollover projection | [catalog-entitlements.md](catalog-entitlements.md) |
| Active-term squad additions | [squad-additions.md](squad-additions.md) |
| Member connection, paid reset, and first-term refund operations | [member-operations.md](member-operations.md) |
| Typed internal squad profiles | [squad-profiles.md](squad-profiles.md) |
| TXB ledger, top-ups, and refunds | [billing-payments.md](billing-payments.md) |
| Games, daily check-ins, group-message rewards, and lucky draws | [activity.md](activity.md) |
| Coupon definitions, wallet grants, and purchase discounts | [coupons.md](coupons.md) |
| Questionnaire participation and CSV settlement | [questionnaires.md](questionnaires.md) |
| Emby account setup and restricted policy management | [emby.md](emby.md) |
| Telegram, Remnawave, EZPay, and BEPusdt adapters | [integrations.md](integrations.md) |
| Retired detector upgrade | [detector-retirement.md](detector-retirement.md) |
| Admin authorization and domain operations | [admin-operations.md](admin-operations.md) |
| Vue Mini App and generated API client | [web-ui.md](web-ui.md) |
| VanSail Carpool bug-fix adjustment contract | [tx-carpool-adjustments.md](tx-carpool-adjustments.md) |

## VanSail Carpool adjustment specification

The current mobile-first adjustment set is covered by the existing module
contracts: `web-ui.md` owns responsive catalog, renewal, traffic, admin, and
payment surfaces; `billing-payments.md` owns stable-ID provider-account profiles;
`admin-operations.md` owns stock, balance, coupon, backup, and database-editor
administration; and `integrations.md` owns shared provider credentials and
callback capabilities. No Remnawave or Emby request bypasses the upstream queue
boundary described in the integration specification.

## Source package maps

These READMEs describe the direct files in each implementation package:

| Package | File map |
| --- | --- |
| Internal package boundary | [internal/README.md](../../internal/README.md) |
| Accounts | [internal/accounts/README.md](../../internal/accounts/README.md) |
| Activity | [internal/activity/README.md](../../internal/activity/README.md) |
| Administration | [internal/admin/README.md](../../internal/admin/README.md) |
| Application composition | [internal/app/README.md](../../internal/app/README.md) |
| Billing | [internal/billing/README.md](../../internal/billing/README.md) |
| Catalog | [internal/catalog/README.md](../../internal/catalog/README.md) |
| Member connections | [internal/connections/README.md](../../internal/connections/README.md) |
| Member purchase operations | [internal/purchaseops/README.md](../../internal/purchaseops/README.md) |
| Coupons | [internal/coupons/README.md](../../internal/coupons/README.md) |
| Emby domain | [internal/emby/README.md](../../internal/emby/README.md) |
| Entitlements | [internal/entitlements/README.md](../../internal/entitlements/README.md) |
| HTTP API | [internal/httpapi/README.md](../../internal/httpapi/README.md) |
| Shared models | [internal/model/README.md](../../internal/model/README.md) |
| Onboarding content | [internal/onboarding/README.md](../../internal/onboarding/README.md) |
| Outbox payload helpers | [internal/outbox/README.md](../../internal/outbox/README.md) |
| Provider operation contracts | [internal/providerops/README.md](../../internal/providerops/README.md) |
| Questionnaires | [internal/questionnaires/README.md](../../internal/questionnaires/README.md) |
| Purchase rollover | [internal/rollover/README.md](../../internal/rollover/README.md) |
| Embedded web UI | [internal/webui/README.md](../../internal/webui/README.md) |
| Provider integrations boundary | [internal/integrations/README.md](../../internal/integrations/README.md) |
| BEPusdt integration | [internal/integrations/bepusdt/README.md](../../internal/integrations/bepusdt/README.md) |
| Emby integration | [internal/integrations/emby/README.md](../../internal/integrations/emby/README.md) |
| EZPay integration | [internal/integrations/ezpay/README.md](../../internal/integrations/ezpay/README.md) |
| Remnawave integration | [internal/integrations/remnawave/README.md](../../internal/integrations/remnawave/README.md) |
| Telegram integration | [internal/integrations/telegram/README.md](../../internal/integrations/telegram/README.md) |
| Platform boundary | [internal/platform/README.md](../../internal/platform/README.md) |
| Backups and restore | [internal/platform/backup/README.md](../../internal/platform/backup/README.md) |
| Runtime configuration | [internal/platform/config/README.md](../../internal/platform/config/README.md) |
| Database | [internal/platform/database/README.md](../../internal/platform/database/README.md) |
| Database migrations | [internal/platform/database/migrations/README.md](../../internal/platform/database/migrations/README.md) |
| Reviewed database administration | [internal/platform/databaseadmin/README.md](../../internal/platform/databaseadmin/README.md) |
| Identifier generation | [internal/platform/ids/README.md](../../internal/platform/ids/README.md) |
| Durable outbox worker | [internal/platform/outbox/README.md](../../internal/platform/outbox/README.md) |
| Secret vault | [internal/platform/secret/README.md](../../internal/platform/secret/README.md) |
| Upstream queues | [internal/platform/upstreamqueue/README.md](../../internal/platform/upstreamqueue/README.md) |
| Request authentication | [internal/requestauth/README.md](../../internal/requestauth/README.md) |
| Request validation | [internal/validation/README.md](../../internal/validation/README.md) |
| Command boundary | [cmd/README.md](../../cmd/README.md) |
| Server command | [cmd/server/README.md](../../cmd/server/README.md) |

The split HTTP contract is indexed from [api/README.md](../../api/README.md), and the frontend file maps start at [web/README.md](../../web/README.md) and [web/src/README.md](../../web/src/README.md).

Cross-module rules:

- HTTP request/response compatibility is defined by `api/openapi.yaml`; generated frontend types must not be hand-edited.
- All blocking repository and provider calls accept `context.Context`. Background work stops through the application context.
- SQLite is the source of truth for identity, money, purchases, callback deduplication, and pending work. Provider redirects and browser events are never authoritative.
- IDs are opaque strings, timestamps are RFC3339 UTC strings, TXB uses integer hundredths, provider decimals use fixed-decimal parsing, and traffic byte counts never pass through floating point.
- Secret and bearer values are redacted at the logging boundary. Sensitive dashboard settings are AES-256-GCM encrypted and write-only through the API.
- Domain APIs treat ledger entries, refunds, webhook receipts, Activity results, questionnaire awards, rollover results, and audit events as immutable; corrections use compensating records. The explicitly warned break-glass database editor can bypass domain hooks only after diff review, typed confirmation, rescue backup, and audit.
- Production frontend and backend code is limited to 200 physical lines per file; tests, generated clients, migrations, API documents, and embedded build output are exempt. Every split implementation file is indexed by its package README.
- Telegram WebApp lifecycle is initialized before Vue Router construction, waits for delayed WebApp context, synchronizes safe areas and theme colors, and gives the most recent visible overlay ownership of native BackButton navigation.
`user-settings.md` documents preference ownership, notification delivery, Telegram tags and constructed browser evidence.
`admin-refund-defaults.md` describes live traffic-based administrator suggestions, manual overrides and browser evidence.
