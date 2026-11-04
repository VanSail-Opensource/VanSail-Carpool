# VanSail Carpool

VanSail Carpool provides a premium-dark Telegram Mini App for member onboarding, TXB funding and shared X-Node access, plus an independent browser admin console with light/dark appearance. A Vue 3 single-page app is built into a Go binary; the binary owns the API, background work, PostgreSQL database, and static assets. Production therefore needs one container, durable application and PostgreSQL volumes, and one HTTP port.

## What is included

- Telegram member authentication with five-minute `initData` validation; independent browser admin sessions use short-lived one-use access links. Authenticated requests use replay-protected HMAC signing.
- Resumable group/channel membership, immutable username, and agreement onboarding.
- Server-priced combos, optional X-Node internal squads, renewals, queued plan changes, and a transactional TXB ledger.
- EZPay, BEPusdt, and Telegram Stars top-ups with signed, idempotent callback processing.
- Authoritative X-Node WebSocket control with per-node execution queues; queued Emby access, subscription rotation, traffic statistics and durable synchronization jobs.
- Memory-only automatic firewall leases and automatically refreshed member IP observations; bounded 72-hour per-node PAYG history. [Operation and hosted measurements](docs/automatic-firewall-leases.md).
- Multiple Activity games, daily check-ins, weighted lucky draws, and coupon wallets.
- Durable Emby account setup, encrypted temporary credentials, server-enforced restricted policies, and compensating refunds.
- Audited administration for settings, catalog, users, adjustments, entitlements, refunds, schema-aware database inspection, backups, offline recovery, and failed jobs.
- Daily verified PostgreSQL exports with seven-day retention, authenticated download, and offline recovery.

The machine-readable HTTP contract is [api/openapi.yaml](api/openapi.yaml). Module boundaries, invariants, failure behavior, and test expectations are indexed in [docs/modules/README.md](docs/modules/README.md).

## Architecture

```text
Telegram Mini App / browser admin console
      |
      v
Go HTTP server :8080 ---- embedded Vue assets
      |
      +---- configured or embedded PostgreSQL
      +---- persistent outbox and scheduler
      +---- Telegram Bot API
      +---- authenticated X-Node agent WebSockets
      +---- Emby API
      +---- EZPay / BEPusdt / Telegram Stars
```

`cmd/server` is deliberately thin and exposes two commands: `serve` and `healthcheck`. Domain packages live under `internal`; blocking operations accept `context.Context`, and provider clients are hidden behind consumer-owned interfaces. PostgreSQL enforces foreign keys, serializable write transactions, statement/lock deadlines and bounded connections. Balance changes, purchases, webhook deduplication, and outbox creation are transactional.

The Vue application lives in `web` and uses Composition API, `<script setup lang="ts">`, Vue Router, Pinia for session-wide identity only, Nuxt UI v4, Motion for Vue, external Iconify Phosphor/country icons, and Zod validation. English and Simplified Chinese ship as parity-checked locale modules. `npm run build` writes to `internal/webui/dist`; Go embeds that directory and serves the SPA with same-origin APIs.

After Telegram authentication, the server issues an HttpOnly `txc_session` cookie and a separate request-signing key. The browser signs the exact method, escaped path/query, timestamp, nonce, and body hash; the server rejects stale, replayed, malformed, or unsigned protected requests. Provider callbacks, payment returns, probes, and Telegram bootstrap retain their own documented unsigned protocols.

## Required environment

Copy `.env.example` to a secret deployment-specific file and set these four values:

| Variable | Requirement |
| --- | --- |
| `ADMIN_TELEGRAM_ID` | One or more positive Telegram user IDs separated by commas. Configured admins can open the browser console from Mini App settings and complete member signup independently; native member access begins after purchase. |
| `TELEGRAM_BOT_TOKEN` | Bot token from BotFather. It is also used for Mini App auth validation and Stars. |
| `PUBLIC_BASE_URL` | Canonical externally reachable HTTPS origin, without a trailing path. Webhooks and returns are derived only from this value. |
| `CONFIG_MASTER_KEY` | Base64 encoding of exactly 32 random bytes, for example `openssl rand -base64 32`. Keep it stable and backed up. |

Optional runtime settings are `PORT=8080`, `DATA_DIR=/data`, `TZ=UTC`, and `LOG_LEVEL=info`. `ALLOW_INSECURE_HTTP=true` exists only for isolated local development; never enable it for a Telegram deployment.

X-Node credentials, provider credentials, provider feature flags, chat IDs, rates, and catalog data are configured in the admin dashboard. Sensitive values are encrypted with AES-256-GCM under `CONFIG_MASTER_KEY`, are masked after entry, and are write-only over the API. Losing or changing the master key makes encrypted settings unreadable.

## Build and run

Prerequisites are Go 1.26.5, Node.js 26 with npm, and GNU Make. The application stack includes Vue 3.5.41, Tailwind CSS 4.3.3, and PostgreSQL 18 with pgx.

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
  -v vansail-postgres-data:/var/lib/postgresql \
  -e PG_EMBEDDED=true \
  vansail-carpool:local
```

The runtime image executes as UID/GID `10001`, exposes only port `8080`, persists `/data` and the embedded PostgreSQL volume, and uses `vansail-carpool healthcheck` against `/healthz`. `/readyz` remains unavailable until local storage works and the required dashboard setup is complete.
If a host bind mount is used instead of a named volume, make the mounted directory writable by UID/GID `10001` before startup.

## First-time setup

1. Deploy the HTTPS origin with the required environment and a durable `/data` volume.
2. Open the Mini App as a configured `ADMIN_TELEGRAM_ID` and use `/settings/admin-access` to obtain a short-lived, one-use browser console link. Browser administration has its own device session; member signup remains independent.
3. Configure Telegram membership, payment profiles, required `txb_per_*` rates and catalog products. Add native X-Node configurations, nodes and access groups, then generate per-node enrollment credentials. Configure the encrypted Emby token, HTTPS URL and setup price when Emby access is offered.
4. Review native access groups and their published nodes. Configure product prices, visibility, plain-text descriptions and ordered feature lists in the catalog.
5. In BotFather, select the deployed URL as the bot's Main Mini App. The service configures the Telegram webhook and chat menu button from `PUBLIC_BASE_URL`; the bot needs invite and join-request administration rights in both chats.
6. Confirm `/readyz` reports `status: ok` before sharing the Mini App.

Do not derive callback URLs from proxy request headers. The application deliberately uses only `PUBLIC_BASE_URL`; configure trusted proxying and TLS termination so that origin is reachable unchanged.

## Payment and Telegram policy warning

Payment redirects are navigation only. TXB is credited exclusively after a verified EZPay notification, a signed BEPusdt status `2` callback, or Telegram `successful_payment`. Provider transaction IDs and callback dedupe keys are unique, and callback replays cannot credit twice. The client polls the durable order until it reaches `paid`; invoice-close or browser-return events never establish payment.

**[Telegram requires Telegram Stars](https://core.telegram.org/bots/payments-stars#faq) for digital goods and services sold inside Telegram apps.** EZPay and BEPusdt are implemented behind admin feature flags because they are explicitly required for this deployment, but enabling them inside the Mini App can violate Telegram policy and can make the app unavailable on Telegram mobile clients. Keep Stars as the compliant default and obtain platform/legal approval before enabling an alternative provider.

## Operations and recovery

- `/healthz` proves HTTP and PostgreSQL liveness. `/readyz` also checks required dashboard setup readiness.
- A single context-managed scheduler handles entitlement boundaries, kind-routed outbox retries, payment reconciliation, purchase accounting closure and compute jobs, Emby provisioning, and daily online backups.
- Verified backups are written atomically under `/data/backups`; completed files older than seven days are removed. Failed backup attempts remain visible to the admin and never publish a partial database file.
- Administrators download verified PostgreSQL archives. Restore runs offline into an isolated target; verify schema, exact recovery fingerprints and role grants before resuming one application/master. The browser has no database upload or live replacement action.
- Logs are structured. Bot tokens, provider secrets, encrypted setting plaintext, session cookies, Telegram raw `initData`, and X-Node subscription URLs must never appear in logs or audit details.

The admin API exposes domain operations plus allowlisted, schema-aware read-only record pages; it never accepts raw SQL. Domain APIs keep ledger rows, refunds, audit events and provider events immutable. PostgreSQL credentials use separate migration/application roles.

## CI and release images

Pull requests enforce source line limits, per-folder file maps, locale parity, icon/control policy, generated-contract freshness, frontend lint/type/test/build checks, npm audit, Go format/vet/race/coverage checks, `govulncheck`, and a production image build. Pushes to `main` and semantic `v*.*.*` tags run the same checks before publishing SBOM/provenance-enabled `linux/amd64` and `linux/arm64` images to `ghcr.io/txyyddss/remna-user-panel` with SHA, `latest`, and semantic tags as applicable.

Dependabot checks the root Go module, `web` npm dependencies, Docker base images, and GitHub Actions weekly. Dependabot pull requests are eligible for squash auto-merge only after every reported check succeeds; enable repository auto-merge and protect `main` with the required checks for this policy to take effect.

PostgreSQL is required. Use the [embedded deployment](deploy/embedded/README.md)
with durable `/data` and `/var/lib/postgresql` volumes, or configure an external
connection using DATABASE_URL_FILE. DB_DRIVER defaults to postgres and no other
driver is accepted. Missing/unavailable databases fail startup. Run `db-schema
migrate` with the migration role, then start the restricted application role.
Keep existing PostgreSQL volumes and CONFIG_MASTER_KEY unchanged during upgrades.
See [schema policy](docs/postgresql-migration.md) and
[backup/recovery operations](deploy/postgres/recovery.md).
