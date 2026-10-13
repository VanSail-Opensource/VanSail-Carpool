# Browser administrator access

The service owns browser-only Telegram OIDC, encrypted OAuth configuration,
rolling device sessions, and short-lived Mini App handoffs. `New` injects the
existing database/vault, public URL, allowlist, and queued HTTP client.

- `service.go` resolves, renews, revokes, and creates isolated device sessions;
  handoff exchange is atomic and stores only SHA-256 references.
- `config.go` validates and encrypts `auth.browser.telegram` as one setting.
  Secrets are write-only; an omitted secret preserves its previous value.
- `oauth.go` implements authorization-code exchange with PKCE, browser-bound
  state/nonce, and `coreos/go-oidc/v3` verification of issuer, audience, expiry,
  signature, and nonce. Telegram's numeric `id` determines administrator
  eligibility independently of its OIDC `sub`. Provider rejection, provider
  unavailability, and unverifiable responses remain distinct for safe login
  diagnostics. No userinfo call is made.
- `queued_http.go` buffers bounded token/JWKS responses inside the existing
  Telegram upstream queue. The OIDC key cache is shared by the service;
  network calls have a 15-second timeout and cannot follow redirects.
- `service_test.go`, `oauth_test.go`, and `queued_http_test.go` cover hosted
  authorization, secret, destination, and queue regression scenarios.

Browser sessions last 30 inactive days and renew at most daily. Neither their
opaque bearer nor derived request key changes during renewal. Mini App
single-session replacement and browser multi-device persistence are separate.
Every authenticated browser request rechecks the persisted role and deployment
allowlist. A verified nonadmin OAuth identity is returned only through a
five-minute encrypted result cookie; no account or browser session is created.

Configure OAuth through the restricted administrator access API. Register its
returned callback URL exactly in BotFather Login Widget settings. Keep OAuth
disabled until the client ID and secret are configured; Mini App handoff links
bootstrap access independently. Provider protocol reference:
https://core.telegram.org/bots/telegram-login

Future identity providers must use the injected queue client, preserve scope
isolation, and avoid logging codes, tokens, state, or credentials.
