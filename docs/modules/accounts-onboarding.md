# Accounts and onboarding module

## Ownership and interfaces

This module owns Telegram Mini App session exchange, local users/sessions, versioned localized onboarding content, stateless signed join-request links, canonical community membership verification, and local agreement acceptance. Completed onboarding queues native subscription credential repair and applicable PAYG provisioning; paid combo synchronization separately owns purchased access.

Public operations are `POST /api/v1/auth/telegram`, `GET /api/v1/me`, and the `/api/v1/onboarding/*` operations defined in OpenAPI. Administrator authorization reuses the validated session and compares the Telegram ID to the configured comma-separated `ADMIN_TELEGRAM_ID` list; no browser role is trusted.

## State and invariants

- Raw `Telegram.WebApp.initData` is authenticated with Telegram's HMAC construction and constant-time comparison. `auth_date` must be no older than five minutes and future-skewed data is rejected.
- A random opaque session is stored only as a hash, expires after seven days, and is delivered in a Secure, HttpOnly, SameSite cookie. Raw init data and session values are never logged.
- Current onboarding advances from `intro` to `agreement` and then `complete`. Historical `membership` and `username` states can advance to agreement without reserving a new service username. Account closure cannot complete onboarding or enqueue new credentials.
- Group/channel invites request approval and expire after 30 minutes. Their Telegram name fits the 32-character limit and carries base-36 user identity plus a 96-bit HMAC over user, chat, and expiry. No invite row/link is persisted; webhooks validate signature, identity, chat, expiry, and the current signing secret before approval and revocation.
- Membership events improve responsiveness, but `getChatMember` for both configured chats is canonical. Community eligibility is checked against the current active-combo window independently of onboarding and stored membership history.
- The service account name is derived from the verified numeric Telegram ID. Historical username fields do not authorize reassignment, account adoption, refunds or a username-restart flow.
- Draft and published welcome/agreement bundles have independent revisions. `en` and `zh-CN` are mandatory and must contain matching stable IDs/order (and matching whitelisted agreement icons). Welcome durations are derived at read time from 220 Latin words/minute plus 300 CJK characters/minute, 600 ms transition allowance, clamped to 1.8–12 seconds.
- Publishing changed agreements increments the agreement revision and routes every completed user back to `agreement`. Acceptance requires the exact current revision and every active agreement ID; stale or partial submissions return `409`. Completion stores the accepted revision and credential/PAYG outbox work atomically, with no direct agent call or wallet debit.

## Native identity and compatibility repair

Authentication of a completed member with a compatibility proxy reference can recheck that exact identity. The production adapter reads the authoritative native identity store; it does not query a retired external account API. Confirmed absence can queue the existing paid repair path. Lookup failures do not turn otherwise valid Telegram launch data into invalid authentication.

Native credential repair also serves completed members without a combo. Credential availability is distinct from paid connection admission: repair does not create free entitlements, reset usage, debit the wallet or alter settled financial history. Generation and active-owner checks protect identity creation, credential export and current purchased access.

Historical generic provisioning helpers require an exact verified Telegram match. A mismatched or unverified identity remains a conflict. The retired `ResolveProvisioningConflict` mutation returns a conflict and preserves paid purchases, balance, accepted agreements and local identity; it cannot automatically refund terms or restart username selection.

## Failure and reconciliation

Malformed, stale, or incorrectly signed Telegram launch data returns the same unauthenticated envelope. A Telegram outage never infers membership from a browser claim.

Native credential repair and purchased activation are resumable through the durable outbox. Temporary agent failures retain the paid purchase and original operation evidence. Identity mismatches do not authorize adoption, automatic refunds or the retired username flow.

Invite creation/approval failures leave local membership false. Signature rotation intentionally invalidates names created with the old secret. The designated administrator can enter system setup before completing ordinary onboarding; user product operations remain guarded until that flow completes.

## Verification

- Table-driven init-data tests cover canonical sorting, percent encoding, missing fields, bad hashes, stale/future timestamps, and accepted boundary age.
- Repository tests cover current agreement progression, exact revision acceptance, closed-account refusal and credential/PAYG queue atomicity without a combo debit.
- Telegram fake-server tests cover invite expiry, wrong-user requests, duplicate join events, partial membership, and canonical recheck.
- Native and compatibility tests cover exact identity ownership, ambiguous/mismatched principals, missing-reference repair, retained paid access and failure without destructive identity mutation.
- Recovery integration tests assert identity, balance, ledger, purchase, Activity, and Emby records remain attached to the same local user.
- Handler tests verify cookie flags, session expiry, resumable transitions, recovery reason, temporary upstream mapping, administrator bootstrap, and redacted logs.
