# Frontend maintenance scripts

- `generate-response-schemas.mjs` resolves OpenAPI response references and generates structural read contracts without server-owned value restrictions. It verifies Zod can compile every generated schema.

- `compact-contract.mjs` keeps the generated OpenAPI type artifact below the
  repository line ceiling without changing its TypeScript declarations.
- `check-structure.mjs` audits source line limits, folder documentation,
  complete locale-domain inventory and per-file key/placeholder parity,
  script/template copy, local icon-asset policy, and coverage
  of production-source icons by the Vite client bundle scan or explicit list.
- `locale-domains.mjs` checks that every locale JSON domain is listed once in
  the shared inventory before the structure gate validates each file's messages.

`generate-diy-contracts.mjs` derives Go pointer types, field metadata, API override schemas and bilingual labels from the canonical subscription allowlist. Regenerate these before `generate:api` when changing the allowlist.

Catalog ownership generates separate member `ConnectionOverrides` and
administrator `EndpointOptions` Go/API types. Existing member labels and retired
keys remain confined to member/shared fields; administrator labels belong to the
endpoint editor's localization domain. Inclusive port ranges and field-specific
range bounds are generated for administrator controls without a raw JSON editor.

`workflow-policy.mjs` exports the pure `auditWorkflow(name, parsedYaml)` security check for full upstream SHAs, approved repositories, private checkout credentials and bounded job permissions. `check-workflows.mjs` reads `.github/workflows` through the existing YAML dependency and reports static failures without network access or executing jobs. `workflow-policy.regressions.mjs` supplies hostile-ref, privilege and publication-bypass regressions through Node's built-in runner in hosted Actions only. `audit:structure` includes this static gate; `test:workflows` is the separate hosted regression entry point. Extend the reviewed action/job allowlists deliberately and update `docs/actions-security.md` when changing pins.

`browser-recovery-audit.mjs` is a hosted-only Chromium audit using the pinned
Playwright dependency and a loopback Vite fixture server. It exercises the real
App currency hydration and payment sheet plus browser access recovery at desktop
and mobile widths, including loading, populated, empty and error states. It
distinguishes invalid one-use links from temporary exchange failures and verifies
an explicit retry succeeds without returning the token to the URL. It
blocks unexpected API/external requests, checks console failures and document
overflow, and writes per-state screenshots and `results.json` to
`BROWSER_AUDIT_OUTPUT`. CI uploads these artifacts even after a failure. Run it
only in hosted CI; the repository's local automated-suite restriction applies.

`browser-member-recovery.mjs` adds desktop/mobile checks for intro retry,
agreement revision consent, delayed owner changes, community loading/errors and
retired invite links using the real onboarding and community components.
It also verifies that a different owner retires the prior payment sheet and that
delayed combo/balance replies cannot restore the prior owner's navigation.
The constructed captcha scenario exercises the actual App owner boundary and
rejects prior-widget session replies and callbacks.

`browser-operation-recovery.mjs` checks explicit receipt-outcome selection,
unsupported workflow restrictions and retained drafts after evidence conflicts.
It also verifies that a temporary Mini App session read failure retries the read
without starting another Telegram exchange.

`browser-purchase-recovery.mjs` verifies queued-purchase target replacement,
pending cancellation isolation and visible dialog errors followed by explicit
retry at desktop and mobile widths.
It also checks renewal expiry, health deferral, stale reopen replies and pricing
read recovery while preserving the server's coupon-adjusted net amount.

`browser-activity-recovery.mjs` verifies that refreshed disabled games retire
their selection, reward progress stays inside its accessible range, and an
uncertain bet keeps its idempotency key through a visible error and explicit retry.

`browser-receipt-recovery.mjs` checks desktop/mobile pending, failed, refunded
and paid receipts, including the distinction between order value and credit.

`browser-hourly-recovery.mjs` exercises desktop/mobile reviewed price, gated
offer and paid enrollment replacement; no write occurs until new consent. It
also checks retained purchased traffic with automatic purchases disabled and a
zero starting allowance.
