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
- Squad merchandising is a sparse override over the current native group inventory; default values remove the override. The X-Node domain owns durable group and node bindings. Merchandising reads revalidate that inventory instead of treating saved product metadata as authoritative node membership.
- Balance adjustment requires a bounded nonzero signed amount and reason and appends one ledger entry plus audit event.
- Telegram `/deduct <amount>` is accepted only from a configured administrator in the configured group as a reply to a known human sender. Amounts are positive human-major TXB values; the atomic debit rejects insufficient balance, uses a deterministic quoted-message reference for replay safety, and appends `telegram.balance_deduct` audit metadata for the sending administrator.
- PM-topic `/refund`, `/addtxb <value>`, and `/deducttxb <value>` target only that topic's user. Refunds use the live verified remaining-traffic quote; balance adjustments use Telegram update references, immutable ledger entries, and administrator audit metadata.
- Entitlement cancellation and payment refund append durable compensating commands; they never mutate provider state first and hope persistence follows.
- A terminal failed/expired payment may receive one locally funded courtesy credit only with a 3-500 byte reason. Its ledger entry, dedicated idempotency record, and audit event commit together; it never changes the original order to provider-paid or calls a provider.
- Activity games/draws, coupons/imports, and Emby retries use their module services so validation, transactions, and idempotency remain centralized.
- Job retry is allowed only for an eligible failed job and cannot change kind or typed payload. Successful jobs are removed at completion. Manual deletion rejects processing jobs, unfinished provider commands, live retry targets, unresolved legacy Emby setup, required restriction expiry and unfinished financial/account-cleanup lanes with `409`. Resolve or retry the underlying work first. Eligible deletion and a sanitized `outbox.delete` actor audit commit together; receipts remain available for exact replay.
- Audit events are retained durably; the bounded administrator list is a page of that history. Sensitive values are redacted before persistence.

Payment profiles are durable records per provider account. EZPay and BEPusdt
may each have multiple independent profiles, with separate endpoint,
encrypted credential, provider name, acknowledgement, and channel selection.
The member method list uses each profile's stable ID so channels from accounts
with the same provider remain independently selectable.

## Guarded database recovery editor

The administrator can inspect allowlisted PostgreSQL tables and typed, redacted
record pages. Native server catalogs supply validated table/column identifiers
and declared primary keys. Tables without keys cannot be paginated. Cursor
values are sealed and bind the filter fingerprint; pages remain bounded.

Wire values preserve exact decimal-string integers/numerics and typed booleans,
nulls and base64 binary values. Sensitive fields are masked and excluded from
broad search. Protected tables reject generic mutations. Eligible recovery edits
require an exact one-use review, a reason, explicit table/action confirmation,
a verified rescue backup and an unchanged record fingerprint. Mutation, review
consumption and actor audit commit together. Direct edits bypass domain hooks;
use the domain APIs for ordinary credential, financial and desired-state work.

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
- Database tests cover identifier injection, declared/composite-key cursors, exact large values, typed null/boolean/binary transport, sensitive masking, protected-table refusal and guarded mutation/replay boundaries.
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
Once any cancellation or refund lane has recorded credit for a purchase, fresh
entitlement edits cannot reopen or rewrite it. Its accepted command keys remain
replayable. Grant separate zero-charge access when a member needs a replacement
term, preserving the settled purchase and refund evidence.

`POST /api/v1/admin/users/{userId}/entitlements` creates a zero-charge manual
fixed-term grant. It requires an `Idempotency-Key` and a reason of 3-500 bytes,
and validates that the combo is active, every requested squad exists upstream,
the traffic snapshot is positive, the reset strategy is `DAY`/`WEEK`/
`MONTH_ROLLING`, and `validUntil` is after `validFrom`. An omitted or empty
`squadUuids` list inherits the combo's included squads. One transaction inserts
a purchase with `charged_txb_minor`, gross, core gross, coupon discount, and
ledger movement all zero; snapshots `entitlement_traffic_limit_bytes`,
`entitlement_reset_strategy`, and `entitlement_squad_uuids`; records the
`entitlement.create` actor audit; enqueues the existing `xnode_sync_user`
outbox job; and inserts a provider-gated `admin_update` member notification.
The term is `active` when `validFrom <= now` and `queued` otherwise. The wallet
is never debited and no ledger entry or debt is created. Any overlap with an
existing `activating`, `active`, or `queued` term returns `409` with code
`ENTITLEMENT_OVERLAP`. An owner holding a `preparing` or `active` hourly
enrollment for the current account generation is rejected with `409` code
`HOURLY_LEGACY_ACTIVE` before any purchase is written; replays of an accepted
grant remain immutable. Replaying the same actor, owner, key, and request
fingerprint returns the original purchase; a changed fingerprint conflicts.

## Administrator hourly workspace

`GET /api/v1/admin/users/{userId}/hourly` returns `AdminHourlyView`:
`policy`, `balance`, `extraPreference`, current UTC-day `usage`, and every
enrollment for the owner's current account generation (including `disabled` and
`closed`). Each squad row has its live group name and offer price, offer
enabled flag, status, `enrollmentId`, `paidThrough`, `cooldownUntil`, `activatedAt`,
`disabledAt`, and `createdAt`. The safe enrollment ID is the target for the
disable endpoint. The projection never exposes credentials, proxy
or period references, account generations, activation keys, or fingerprints,
and reads never create allowance rows.

`POST /api/v1/admin/users/{userId}/hourly/enrollments/{enrollmentId}/disable`
requires `Idempotency-Key` and `{"policyVersion": "<current policy version>"}`.
It validates enrollment ownership, the owner's current account generation, and
the current policy version; a superseded or terminated enrollment and a stale
policy version return `409`. The command queues the existing
`xnode_hourly_disable` provider operation with `ActorUserID` set to the
administrator and `OwnerUserID` set to the member, so the operator action is
durably attributed without weakening member authorization. It retains the paid
hour and records a plain `disabled` stop with no voluntary cooldown; it never
classifies an operator stop as a funding suspension. The response is `202` with
an `OperationReceipt`.

Bulk extension filters use inclusive OR across combo IDs and add-on squad UUIDs.
Only currently active purchases qualify, each user is targeted once, and every
queued successor shifts by the same extension duration at both boundaries so
its duration remains unchanged. The preview and job use the same target query.

The shared provider-operation worker records operation and item intent before
calling the queued X-Node adapter. A completed target is never replayed. An
interrupted attempt or uncertain network result becomes `pending_review`; mixed
bulk results become `partial`. Transient local reads before a provider attempt
remain retryable and do not fabricate provider failure.

### Repairing an operation receipt

The generic administrator resolution endpoint repairs a parent receipt only
when completed, retained item outcomes already establish the requested result.
It supports the administrator entitlement edit, credited-refund synchronization,
combo replacement, bulk extension, and retained legacy outage synchronization
kinds. Every target must have a recorded attempt and completion; all outcomes
must match `succeeded`, `failed`, or `compensated`, or fully completed mixed
outcomes must substantiate `partial`.

Unfinished or ambiguous items require the owning domain's reconciliation.
Payment/member/hourly settlement, Emby provisioning, temporary restrictions and
native X-Node command receipts cannot be force-completed through this endpoint.
Receipt repair never changes item evidence, moves money, releases notification
gates or holds, or dispatches a provider request. A repaired success clears the
parent's obsolete error code. The actor, request key and exact returned receipt
are committed with an immutable audit; replay returns that resolution's snapshot.
Historical resolutions without a snapshot can replay only while the original
receipt is unchanged.
