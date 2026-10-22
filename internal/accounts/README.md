# Accounts package

Trusted Telegram authentication and resumable account onboarding live here. Network and persistence dependencies are expressed as package-level interfaces so the application layer can supply adapters.

## Files

- `service.go` defines service contracts, authentication, session lookup, and service construction.
- `auth_limiter.go` bounds verified Telegram-ID authentication exchanges before account or session writes.
- `membership.go` refreshes canonical group and channel membership facts without changing onboarding state.
- `community_invites.go` creates one requested-space identity-bound invite only for a strict active combo, rechecks entitlement before approval, and declines plus revokes a lapsed request.
- `onboarding.go` advances directly to agreements and completes them locally; remote
  creation starts only when a combo has been purchased.
- `service_test.go` covers authentication and session behavior.
- `auth_limiter_test.go` covers per-identity isolation and token refill behavior.
- `membership_test.go` covers community membership facts, one-space invite eligibility, and approval-time entitlement rechecks.
- `onboarding_test.go` covers provider-free agreement completion without a username.
- `service_test_helpers_test.go` contains package-wide repository and upstream test doubles.
- `README.md` documents the package layout.

`member_tag.go` exposes canonical group-tag reads and validated writes without persisting Telegram data.

`first_entry.go` exposes authenticated bootstrap state and verification. Agreement acceptance remains the permanent trust evidence; first-entry rows only qualify never-onboarded users. Admins remain able to configure the challenge.
`first_entry_test.go` covers enabled/disabled bootstrap, provider identity, idempotent verified retries, configuration/provider failure refusal and configured-admin exemptions in hosted domain coverage.
