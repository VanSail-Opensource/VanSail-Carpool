# VanSail Carpool

Administrators can configure built-in host connectivity checks in **Settings**.
Resolve an existing X-Node monitoring username, save the configuration, then
run a manual check or enable scheduling. Checks use that account's accessible
enabled hosts, including hidden hosts; every attempt is retained for 24 hours.
The engine tests authenticated HTTPS traffic and reports status, TTFB, and safe
diagnostics. [Module details](internal/connectivity/README.md) describe supported
configurations, lifecycle, retention, and extension points.

VanSail Carpool is a Telegram Mini App for onboarding members, funding a TXB balance, buying shared X-Node access, and administering the service from one premium-dark web interface. A Vue 3 single-page app is built into a Go binary; the binary owns the API, background work, SQLite database, and static assets. Production therefore needs one container, one persistent volume, and one HTTP port.

## What is included

- Telegram-only authentication with five-minute `initData` validation, seven-day HttpOnly sessions, and replay-protected HMAC signing on every authenticated API request.
- Resumable group/channel membership, immutable username, and agreement onboarding.
- Server-priced combos, optional X-Node internal squads, renewals, queued plan changes, and a transactional TXB ledger.
- EZPay, BEPusdt, and Telegram Stars top-ups with signed, idempotent callback processing.
- Queue-first X-Node and Emby access, subscription rotation, traffic statistics, and persistent synchronization jobs.
- Multiple Activity games, daily check-ins, weighted lucky draws, coupon wallets, and one-active questionnaire CSV settlement.
- Durable Emby account setup, encrypted temporary credentials, server-enforced restricted policies, and compensating refunds.
- Audited administration for settings, catalog, users, adjustments, entitlements, refunds, schema-aware database editing, backups, staged restore, and failed jobs.
- Daily verified SQLite backups with seven-day retention, authenticated download, and pre-open atomic restore recovery.

The machine-readable HTTP contract is [api/openapi.yaml](api/openapi.yaml). Module boundaries, invariants, failure behavior, and test expectations are indexed in [docs/modules/README.md](docs/modules/README.md).

## Architecture

```text
Telegram Mini App
      |
      v
Go HTTP server :8080 ---- embedded Vue assets
      |
      +---- SQLite WAL at /data/tx-carpool.db
      +---- persistent outbox and scheduler
      +---- Telegram Bot API
      +---- X-Node API
      +---- Emby API
      +---- EZPay / BEPusdt / Telegram Stars
```

`cmd/server` is deliberately thin and exposes two commands: `serve` and `healthcheck`. Domain packages live under `internal`; blocking operations accept `context.Context`, and provider clients are hidden behind consumer-owned interfaces. SQLite enables foreign keys, WAL, a busy timeout, and bounded connections. Balance changes, purchases, webhook deduplication, and outbox creation are transactional.

The Vue application lives in `web` and uses Composition API, `<script setup lang="ts">`, Vue Router, Pinia for session-wide identity only, Nuxt UI v4, Motion for Vue, external Iconify Phosphor/country icons, and Zod validation. English and Simplified Chinese ship as parity-checked locale modules. `npm run build` writes to `internal/webui/dist`; Go embeds that directory and serves the SPA with same-origin APIs.

After Telegram authentication, the server issues an HttpOnly `txc_session` cookie and a separate request-signing key. The browser signs the exact method, escaped path/query, timestamp, nonce, and body hash; the server rejects stale, replayed, malformed, or unsigned protected requests. Provider callbacks, payment returns, probes, and Telegram bootstrap retain their own documented unsigned protocols.

## Required environment

Copy `.env.example` to a secret deployment-specific file and set these four values:

| Variable | Requirement |
| --- | --- |
| `ADMIN_TELEGRAM_ID` | One or more positive Telegram user IDs separated by commas. Each configured identity starts in the admin dashboard and can optionally complete signup; X-Node creation begins after a combo purchase. |
| `TELEGRAM_BOT_TOKEN` | Bot token from BotFather. It is also used for Mini App auth validation and Stars. |
| `PUBLIC_BASE_URL` | Canonical externally reachable HTTPS origin, without a trailing path. Webhooks and returns are derived only from this value. |
| `CONFIG_MASTER_KEY` | Base64 encoding of exactly 32 random bytes, for example `openssl rand -base64 32`. Keep it stable and backed up. |

Optional runtime settings are `PORT=8080`, `DATA_DIR=/data`, `TZ=UTC`, and `LOG_LEVEL=info`. `ALLOW_INSECURE_HTTP=true` exists only for isolated local development; never enable it for a Telegram deployment.

X-Node credentials, provider credentials, provider feature flags, chat IDs, rates, and catalog data are configured in the admin dashboard. Sensitive values are encrypted with AES-256-GCM under `CONFIG_MASTER_KEY`, are masked after entry, and are write-only over the API. Losing or changing the master key makes encrypted settings unreadable.

## Build and run

Prerequisites are Go 1.26.5, Node.js 26 with npm, and GNU Make. The application stack includes Vue 3.5.41, Tailwind CSS 4.3.3, and `modernc.org/sqlite` 1.56.0.

```sh
make frontend-install
make build

# Export values from your secret environment file, then:
./bin/vansail-carpool serve
```

Useful targets:

| Command | Purpose |
| --- | --- |
| `make api-types` | Regenerate `web/src/api/generated.ts` from the OpenAPI document. |
| `make frontend-audit` | Lint, type-check, test, audit dependencies, and build the SPA. |
| `make go-audit` | Check formatting, vet, lint, race-test with coverage, and run `govulncheck`. |
| `make ci` | Run both audits and validate a production container build. |
| `make run` | Build the SPA and run `cmd/server serve`. |
| `make docker-build` | Build `vansail-carpool:local` (override with `IMAGE=...`). |

The combined coverage gate for `internal/accounts`, `internal/admin`, `internal/billing`, `internal/catalog`, and `internal/entitlements` is 80%; override `COVERAGE_MIN` only for local investigation, not CI. The complete Go package graph is still exercised with the race detector.

### Container

```sh
docker build -t vansail-carpool:local .
docker run --rm \
  --env-file .env \
  -p 8080:8080 \
  -v tx-carpool-data:/data \
  vansail-carpool:local
```

The runtime image executes as UID/GID `10001`, exposes only port `8080`, persists only `/data`, and uses `vansail-carpool healthcheck` against `/healthz`. `/readyz` remains unavailable until local storage works and the required dashboard setup is complete.
If a host bind mount is used instead of a named volume, make the mounted directory writable by UID/GID `10001` before startup.

## First-time setup

1. Deploy the HTTPS origin with the required environment and a durable `/data` volume.
2. Open the Mini App as one of the configured `ADMIN_TELEGRAM_ID` values. It opens directly to the admin dashboard; authorization is the validated Telegram session plus a configured environment ID. Select **Set up user account** there when the admin also needs normal user-side access. A X-Node identity is created after a combo purchase.
3. Configure the target Telegram group and channel, X-Node endpoint and token, enabled payment methods and required `txb_per_*` rates, and at least one combo. Configure the encrypted Emby token, HTTPS URL, and setup price only when Emby access is offered.
4. Review the live X-Node internal squads, then add only the local descriptions, TXB prices, and visibility overrides you need. Upstream-owned squad identities are not duplicated in SQLite.
5. In BotFather, select the deployed URL as the bot's Main Mini App. The service configures the Telegram webhook and chat menu button from `PUBLIC_BASE_URL`; the bot needs invite and join-request administration rights in both chats.
6. Confirm `/readyz` reports `status: ok` before sharing the Mini App.

Do not derive callback URLs from proxy request headers. The application deliberately uses only `PUBLIC_BASE_URL`; configure trusted proxying and TLS termination so that origin is reachable unchanged.

## Payment and Telegram policy warning

Payment redirects are navigation only. TXB is credited exclusively after a verified EZPay notification, a signed BEPusdt status `2` callback, or Telegram `successful_payment`. Provider transaction IDs and callback dedupe keys are unique, and callback replays cannot credit twice. The client polls the durable order until it reaches `paid`; invoice-close or browser-return events never establish payment.

**[Telegram requires Telegram Stars](https://core.telegram.org/bots/payments-stars#faq) for digital goods and services sold inside Telegram apps.** EZPay and BEPusdt are implemented behind admin feature flags because they are explicitly required for this deployment, but enabling them inside the Mini App can violate Telegram policy and can make the app unavailable on Telegram mobile clients. Keep Stars as the compliant default and obtain platform/legal approval before enabling an alternative provider.

## Operations and recovery

- `/healthz` proves HTTP and local SQLite liveness. `/readyz` also checks required dashboard setup readiness.
- A single context-managed scheduler handles entitlement boundaries, kind-routed outbox retries, payment reconciliation, purchase accounting closure and compute jobs, questionnaire settlement, Emby provisioning, and daily online backups.
- Verified backups are written atomically under `/data/backups`; completed files older than seven days are removed. Failed backup attempts remain visible to the admin and never publish a partial database file.
- An administrator can download a verified backup. Restore accepts only a stored snapshot, creates a rescue backup, verifies integrity and migration compatibility, stages an adjacent copy, records a marker, returns `202`, and exits gracefully. Startup performs the atomic pre-open swap, rolls back a failed swap, records the result, and requires clients to reauthenticate.
- Logs are structured. Bot tokens, provider secrets, encrypted setting plaintext, session cookies, Telegram raw `initData`, and X-Node subscription URLs must never appear in logs or audit details.

The admin API exposes domain operations plus an allowlisted, schema-aware table editor; it never accepts raw SQL. Generic edits use typed values, cursor pagination, optimistic hashes, diff review, typed confirmation, automatic rescue backup, and audit records. The UI warns that direct table edits bypass domain synchronization hooks. Ordinary domain APIs keep ledger rows, refunds, audit events, and provider event records append-only; the break-glass editor can bypass those hooks only through its reviewed, rescued, and audited workflow.

## CI and release images

Pull requests enforce source line limits, per-folder file maps, locale parity, icon/control policy, generated-contract freshness, frontend lint/type/test/build checks, npm audit, Go format/vet/race/coverage checks, `govulncheck`, and a production image build. Pushes to `main` and semantic `v*.*.*` tags run the same checks before publishing SBOM/provenance-enabled `linux/amd64` and `linux/arm64` images to `ghcr.io/txyyddss/remna-user-panel` with SHA, `latest`, and semantic tags as applicable.

Dependabot checks the root Go module, `web` npm dependencies, Docker base images, and GitHub Actions weekly. Dependabot pull requests are eligible for squash auto-merge only after every reported check succeeds; enable repository auto-merge and protect `main` with the required checks for this policy to take effect.

Existing deployments retain `/data/tx-carpool.db` and the `tx-carpool-data` volume.

The image also embeds PostgreSQL 18 server and pgBackRest. The selected opt-in
[single-container deployment](deploy/embedded/README.md) has a 1 GiB memory
budget and persistent PostgreSQL storage. Existing SQLite installations must
complete the verified offline migration before changing the authoritative backend.
The product and executable names change without relocating persistent data.

## Adaptive pricing

Pricing remains fixed unless an administrator enables a product. Purchase, renewal and campaign modes are independent. Enabled renewals apply automatically with notifications; no owner terms approval or second administrator is required. Calculated bounds, health/capacity gates, original forecast history and retained-data exports are documented in [the pricing module](internal/pricing/README.md). Delivery evidence and empirical rollout gates are tracked in [BAPO 5.1](docs/plans/bapo-5.1-progress.md).
