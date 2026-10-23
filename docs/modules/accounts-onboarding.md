# Accounts and onboarding module

## Ownership and interfaces

This module owns Telegram Mini App session exchange, local users/sessions, versioned localized onboarding content, stateless signed join-request links, canonical membership verification, username reservation, and local agreement acceptance. Paid combo synchronization owns X-Node provisioning and confirmed-missing repair.

Public operations are `POST /api/v1/auth/telegram`, `GET /api/v1/me`, and the `/api/v1/onboarding/*` operations defined in OpenAPI. Administrator authorization reuses the validated session and compares the Telegram ID to the configured comma-separated `ADMIN_TELEGRAM_ID` list; no browser role is trusted.

## State and invariants

- Raw `Telegram.WebApp.initData` is authenticated with Telegram's HMAC construction and constant-time comparison. `auth_date` must be no older than five minutes and future-skewed data is rejected.
- A random opaque session is stored only as a hash, expires after seven days, and is delivered in a Secure, HttpOnly, SameSite cookie. Raw init data and session values are never logged.
- The resumable state machine is `intro -> membership -> username -> agreement -> complete`. A confirmed username collision during paid provisioning is the one recovery transition to `username`.
- Group/channel invites request approval and expire after 30 minutes. Their Telegram name fits the 32-character limit and carries base-36 user identity plus a 96-bit HMAC over user, chat, and expiry. No invite row/link is persisted; webhooks validate signature, identity, chat, expiry, and the current signing secret before approval and revocation.
- Membership events improve responsiveness, but `getChatMember` for both configured chats is canonical. Membership is enforced during signup.
- Usernames match `^[a-zA-Z0-9_-]{3,36}$` and are locally unique. They remain reserved unless a confirmed provider identity collision triggers a full refund and username restart. Local lookup plus X-Node preflight provide fast feedback; X-Node `A019` resolves an upstream duplicate race at provisioning.
- Draft and published welcome/agreement bundles have independent revisions. `en` and `zh-CN` are mandatory and must contain matching stable IDs/order (and matching whitelisted agreement icons). Welcome durations are derived at read time from 220 Latin words/minute plus 300 CJK characters/minute, 600 ms transition allowance, clamped to 1.8–12 seconds.
- Publishing changed agreements increments the agreement revision and routes every completed user back to `agreement`. Acceptance requires the exact current revision and every active agreement ID; stale or partial submissions return `409`. Completion stores the accepted revision locally and makes no X-Node call.

## Missing linked-user recovery

Every authentication of a completed linked member rechecks the exact X-Node user ID. A confirmed provider 404 clears the stale link and queues repair only when an unexpired paid combo exists. Timeouts, transport failures, and 5xx responses leave the local account unchanged and still permit Telegram authentication; provider-backed operations report their own temporary failure.

Confirmed-missing repair preserves the completed onboarding state, accepted agreements, username, TXB balance and ledger, purchases, Activity results, Questionnaire history, and Emby linkage. The queued worker reconciles exact username and Telegram ID before recreating or adopting a remote identity and reapplying current access.

If the username now belongs to another Telegram identity, one transaction cancels and fully refunds all unexpired current and queued paid combos, clears the unusable username, and sets `recoveryReason=proxy_identity_conflict` with `onboardingState=username`. The member chooses a new username and accepts the current agreement before buying again; previous balance and history remain.

## Failure and reconciliation

Malformed, stale, or incorrectly signed Telegram launch data returns the same unauthenticated envelope. A Telegram outage never infers membership from a browser claim.

Username reservation and purchase-triggered upstream provisioning are resumable. Temporary X-Node failures retain the paid purchase and retry through the durable outbox. Provisioning adopts only an exact username and Telegram match; a confirmed collision takes the refund path above.

Invite creation/approval failures leave local membership false. Signature rotation intentionally invalidates names created with the old secret. The designated administrator can enter system setup before completing ordinary onboarding; user product operations remain guarded until that flow completes.

## Verification

- Table-driven init-data tests cover canonical sorting, percent encoding, missing fields, bad hashes, stale/future timestamps, and accepted boundary age.
- Repository tests race two members for one username and assert one reservation.
- Telegram fake-server tests cover invite expiry, wrong-user requests, duplicate join events, partial membership, and canonical recheck.
- X-Node tests cover the exact initial payload, `A019`, timeout-after-create reconciliation, ambiguous matches, linked-user 404 recovery, and 5xx/timeout no-mutation behavior.
- Recovery integration tests assert identity, balance, ledger, purchase, Activity, Questionnaire, and Emby records remain attached to the same local user.
- Handler tests verify cookie flags, session expiry, resumable transitions, recovery reason, temporary upstream mapping, administrator bootstrap, and redacted logs.
