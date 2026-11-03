# Administration module

## Authorization and route groups

Administration is available only to a valid Telegram session whose Telegram ID appears in the comma-separated `ADMIN_TELEGRAM_ID` environment list. The environment variable is the authorization source; request data, usernames, handles, and mutable database roles cannot grant access. Any configured administrator may configure a greenfield deployment before completing ordinary member onboarding.

All operations live below `/api/v1/admin`. The Vue router lazy-groups existing URLs into commerce, community, accounts, and system navigation without breaking bookmarks. Domain-specific endpoints remain preferred for settings, catalog, users, balances, entitlements, payments/refunds, Activity, coupons, Emby, backups, outbox jobs, and audits. A separate schema-aware editor exists for exceptional recovery work; it exposes no raw SQL.

Private-message administration is part of each user profile. Actual replies remain in the configured Telegram forum topic; the profile owns moderation, delivery history, read evidence, and repair navigation. The legacy `/admin/pm` URL redirects to the user directory.

## Settings and secrets

The fixed registry defines each known key's validator, sensitivity, display category, and readiness impact. Unknown keys are rejected. Sensitive values are encrypted with AES-256-GCM using a random nonce and the setting key as authenticated context. Reads expose an empty value plus configured/encrypted metadata; plaintext and ciphertext are never returned. An empty secret update preserves the existing value.

| Setting | Contract |
| --- | --- |
| `telegram.group_chat_id`, `telegram.channel_chat_id` | Required nonzero integer chat IDs |
| `telegram.webhook_secret` | Encrypted, URL-safe; generated at bootstrap |
| `billing.rate.txb_per_cny`, `txb_per_usd`, `txb_per_xtr` | Required positive fixed decimals; legacy inverse rates are not converted |
| `billing.ezpay.*`, `billing.bepusdt.*` | Legacy compatibility/readiness keys; new provider credentials and channel choices are edited through the provider-account payment profile endpoint |
| `billing.stars.enabled` | Boolean Stars gate |
| `emby.base_url`, `emby.api_token`, `emby.setup_price_txb` | Encrypted HTTPS origin/token and human-major setup price |
| `activity.timezone`, `activity.daily_reward_min_txb`, `activity.daily_reward_max_txb` | IANA timezone and inclusive nonnegative daily reward range; minimum may not exceed maximum |
| `activity.group_message_threshold`, `activity.group_message_reward_txb` | Nonnegative message threshold and human-major TXB reward; either zero disables the reward |

All TXB setting/editor fields accept human-major decimals with at most two fractional digits. `150` therefore stores or resolves to `15000` minor units when used financially. Admin API domain records continue to serialize money as decimal-string minor units.


Readiness checks required settings, enabled-provider completeness, and at least one active combo without calling external providers. Missing new-direction rates keep the corresponding payment method unavailable. Provider connectivity and permissions surface through their normal operations instead of leaking secrets through readiness.

## Domain operations and audit retention

- Combo records are live: changes affect active, queued, and historical purchases and enqueue deduplicated user synchronization. Referenced combos may be hidden but not hard-deleted.
- Squad merchandising is a sparse override over the live X-Node list; default values remove the override. Node assignments are revalidated and re-fetched upstream and are never persisted locally.
- Balance adjustment requires a bounded nonzero signed amount and reason and appends one ledger entry plus audit event.
- Telegram `/deduct <amount>` is accepted only from a configured administrator in the configured group as a reply to a known human sender. Amounts are positive human-major TXB values; the atomic debit rejects insufficient balance, uses a deterministic quoted-message reference for replay safety, and appends `telegram.balance_deduct` audit metadata for the sending administrator.
- PM-topic `/refund`, `/addtxb <value>`, and `/deducttxb <value>` target only that topic's user. Refunds use the live verified remaining-traffic quote; balance adjustments use Telegram update references, immutable ledger entries, and administrator audit metadata.
- Entitlement cancellation and payment refund append durable compensating commands; they never mutate provider state first and hope persistence follows.
- A terminal failed/expired payment may receive one locally funded courtesy credit only with a 3-500 byte reason. Its ledger entry, dedicated idempotency record, and audit event commit together; it never changes the original order to provider-paid or calls a provider.
- Activity games/draws, coupons/imports, and Emby retries use their module services so validation, transactions, and idempotency remain centralized.
- Job retry is allowed only for an eligible failed job and cannot change kind or typed payload. Successful jobs are removed at completion; pending, failed, and legacy done jobs may be deleted, while processing jobs return `409`.
- Every audit insertion transactionally retains the newest 200 events. Sensitive values are redacted before persistence.

Payment profiles are durable records per provider account. EZPay and BEPusdt
may each have multiple independent profiles, with separate endpoint,
encrypted credential, provider name, acknowledgement, and channel selection.
The member method list uses each profile's stable ID so channels from accounts
with the same provider remain independently selectable.

## Read-only database catalog

The administrator can inspect allowlisted PostgreSQL tables and typed, redacted
record pages. Native server catalogs supply validated table/column identifiers
and declared primary keys. Tables without keys cannot be paginated. Cursor
values are sealed and bind the filter fingerprint; pages remain bounded.

Wire values preserve exact decimal-string integers/numerics and typed booleans,
nulls and base64 binary values. Sensitive fields are masked and excluded from
broad search. Generic mutations return a read-only error before creating any
review or backup. Domain APIs own changes to credentials, finance and desired
state, preserving their transaction and synchronization contracts.

## PostgreSQL archive administration

Export creation runs pg_dump and structural archive verification. Download takes
an opaque stored run ID; browser paths cannot select files. Lists expose only a
basename and decimal-string size. Deletion preserves path confinement and audit.
The browser supports refresh, creation, download and guarded deletion.

Recovery is an offline PostgreSQL operation into a separate volume with workers
and the master stopped. Native fingerprints and the original encryption key
verify restored evidence. The live API has no upload or staged file-swap restore.
See [the recovery runbook](../../deploy/postgres/recovery.md).

## Failure behavior and verification

All failures use the request-ID envelope. Unauthorized responses do not reveal target existence. Validation gives safe field messages. Provider/setup failure leaves prior settings intact. HTTP timeout never proves a committed command rolled back; clients refetch the returned resource before retrying.

- Authorization tests cover missing/expired sessions, non-admin identity, forged roles, exact administrator ID, and bootstrap setup.
- Settings tests cover human-major TXB conversion, ordered rail validation, write-only secret rotation, nonce uniqueness, and redacted audits.
- Database tests cover identifier injection, declared/composite-key cursors, exact large values, typed null/boolean/binary transport, sensitive masking and mutation refusal without side effects.
- Hosted recovery checks cover restricted-role exports, isolated logical/physical restoration, PITR, exact table/sequence fingerprints and encryption invariants.

## Aggregate user workflows

`GET /api/v1/admin/users/{id}` composes identity, balance, synchronization,
active and queued entitlements, the optional Emby account, payments, refunds,
and open provider operations. Purchases and add-ons remain the source of truth;
the projection does not copy subscription state onto `users`. Bulk operations
appear for a member through their durable operation item even though the job
itself has no single owner.

Every provider mutation requires `Idempotency-Key`. A full entitlement edit
uses the prior response's `updatedAt` as its optimistic `version`, writes only
the exposed mutable configuration, and preserves IDs, charged/gross/core price
facts, ledger history, and creation time. Refund credit, cancellation, audit,
provider receipt, and outbox insertion commit together. No-charge replacement
changes the active combo and add-on override without appending a TXB ledger row.

Bulk extension filters use inclusive OR across combo IDs and add-on squad UUIDs.
Only currently active purchases qualify, each user is targeted once, and every
queued successor shifts by the same calendar-day count at both boundaries so
its duration remains unchanged. The preview and job use the same target query.

The shared provider-operation worker records operation and item intent before
calling the queued X-Node adapter. A completed target is never replayed. An
interrupted attempt or uncertain network result becomes `pending_review`; mixed
bulk results become `partial` and require audited resolution rather than a
blind provider retry.
