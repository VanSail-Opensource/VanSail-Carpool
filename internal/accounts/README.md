# Accounts package

`account_cleanup.go` composes durable probe/evidence repositories with the queued
Telegram adapter. `account_cleanup_identity.go` suspends admission before bot
verification and checks the configured numeric identity. `account_cleanup_worker.go`
reloads the committed nonce before both bounded Bot API methods; a nonblocking
single-probe gate keeps other existing workers available. It passes paired
classifications to the repository rather than authorizing deletion itself.
`account_cleanup_fixture_test.go`, `account_cleanup_worker_test.go` and
`account_cleanup_identity_test.go` cover nonce-before-network ordering, protected
accounts, transient/contradictory evidence, identity suspension and concurrency.
The evidence grace and retention contract is documented in
[`docs/account-cleanup.md`](../../docs/account-cleanup.md).

Trusted Telegram authentication and resumable account onboarding live here. Network and persistence dependencies are expressed as package-level interfaces so the application layer can supply adapters.

## Files

- `service.go` defines service contracts, authentication, session lookup, and service construction.
- `auth_limiter.go` bounds verified Telegram-ID authentication exchanges before account or session writes.
- `membership.go` refreshes canonical group and channel membership facts without changing onboarding state.
- `community_invites.go` creates one requested-space identity-bound invite for a current active combo or a member with actual PAYG spending, rechecks eligibility before approval, and declines plus revokes an ineligible request. Wallet funding and free traffic do not qualify.
- `onboarding.go` advances directly to agreements and completes them locally.
  Agreement acceptance atomically queues native subscription credentials and
  applicable PAYG provisioning through the existing durable outbox. A combo is
  required only for combo access, never for a member's subscription link.
- `service_test.go` covers authentication and session behavior.
- `session_recovery_test.go` distinguishes absent/expired credentials from storage
  and context failures. Only confirmed missing sessions are authentication errors;
  transient lookups retain their cause so the HTTP boundary can return a retryable
  response and preserve the member's existing cookie.
- `auth_limiter_test.go` covers per-identity isolation and token refill behavior.
- `membership_test.go` covers community membership facts, one-space invite eligibility, and approval-time entitlement rechecks.
- `onboarding_test.go` covers provider-free agreement completion without a username.
- `service_test_helpers_test.go` contains package-wide repository and upstream test doubles.
- `README.md` documents the package layout.

`member_tag.go` exposes canonical group-tag reads and validated writes without persisting Telegram data.

`first_entry.go` exposes authenticated bootstrap state and verification. Agreement acceptance remains the permanent trust evidence; first-entry rows only qualify never-onboarded users. Admins remain able to configure the challenge.
`first_entry_test.go` covers enabled/disabled bootstrap, provider identity, idempotent verified retries, configuration/provider failure refusal and configured-admin exemptions in hosted domain coverage.

`purge.go` owns the generation-fenced outbox cleanup worker. `purge_test.go` covers blocked/uncertain upstream cleanup and replay. Bot blocks enter the evidence grace policy; they do not immediately forfeit an ordinary member's balance or access. Administrators remain protected. Upstream failures remain pending. Rejoining after completed cleanup creates fresh credentials and benefits while canonical accounting references remain archived.

Purge dependency waits retain the same durable claim without consuming delivery attempts. An uncertain Emby creation remains a review failure. Each worker turn has an eight-second bound and resumes fair native cursors; offline participants are never discarded. `purge_guard_test.go` covers admission cancellation between remote, native and final phases, protected administrators, explicit payment dependencies and fencing before a busy claimed mutation.
