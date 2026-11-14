# Frontend maintenance scripts

`browser-pm-recovery.mjs` verifies ambiguous moderation retains its error and
key, accepted flags refresh without losing receipt polling, a failed quiet read
keeps the PM owner mounted, and terminal recovery releases the controls. Both
desktop and mobile scenarios use the constructed PM fixture only.

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

`browser-recovery-audit.mjs` is a pull-request-only hosted Chromium audit using
the pinned Playwright dependency and a loopback Vite fixture server. It exercises the real
App currency hydration and payment sheet plus browser access recovery at desktop
and mobile widths, including loading, populated, empty and error states. It
distinguishes invalid one-use links from temporary exchange failures and verifies
an explicit retry succeeds without returning the token to the URL. It
blocks unexpected API/external requests, checks console failures and document
overflow, and writes per-state screenshots and `results.json` to
`BROWSER_AUDIT_OUTPUT`. CI uploads these artifacts even after a failure, named
`frontend-browser-recovery-pr-<number>`. `scope.json` records the PR, commit and
selected suites; every result records its scope. Run it only in hosted PR CI;
the repository's local automated-suite restriction applies.

`plan-browser-recovery.mjs` reads the current GitHub PR event and compares exact
base/head commits using a NUL-delimited Git diff, including both sides of moves.
It requires no API token and passes filenames as data, never shell code.
`browser-recovery-selection.mjs` maps the changed files to related feature
suites. The job summary and planner log show the selection for that PR.
Documentation, unit-test-only and backend-only edits skip this browser job.
Member-only themes select member suites; administrator themes select admin
suites; shared fonts, runtime, dependencies, API contracts and unmapped frontend
files retain full coverage. An unavailable diff also selects all suites.

`browser-recovery-scopes.mjs` is the registry shared by selection and execution.
The runner rejects missing, empty or unknown `BROWSER_AUDIT_SCOPES`, imports only
selected suites, and refuses non-PR events. There is no universal default run.
Add a scope/ownership rule with a new browser scenario; keep the conservative
fallback for runtime paths without a rule. `browser-recovery-scope.regressions.mjs`
is included in the hosted `test:workflows` gate and covers isolation, shared
changes, moved/deleted paths, unknown files and invalid selections.

`browser-core-recovery.mjs` preserves the currency, checkout and browser access
scenarios formerly embedded in the runner. They are separate registry scopes,
so a focused feature PR does not always rerun unrelated core flows.

`browser-xnode-availability.mjs` verifies optional telemetry failure, tab-scoped
inventory loading, stale status, long node identities, loading/empty state and
manual DDNS zone/TTL validation at desktop and mobile widths.
`browser-xnode-creation.mjs` checks committed configuration/snippet creation with
a lost response, failed recovery read, retained changed-draft conflict and exact
saved-content recovery before creating a new resource with a distinct identity.
The same cases verify pointer and Enter submission, explicit error announcements
and visible footer feedback without programmatically scrolling to the error.
IP read-failure coverage checks both the disabled fieldset property and effective
disabled states of its descendant controls.

`browser-member-recovery.mjs` adds desktop/mobile checks for intro retry,
agreement revision consent, delayed owner changes, community loading/errors and
retired invite links using the real onboarding and community components.
It also verifies that a different owner retires the prior payment sheet and that
delayed combo/balance replies cannot restore the prior owner's navigation.
The constructed captcha scenario exercises the actual App owner boundary and
rejects prior-widget session replies and callbacks.
The native member fixture also validates the current endpoint/template routing
contract and durable save receipt, including the exact revisioned payload.

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
Its `browser-hourly-preferences.mjs` scenarios cover inline auto-buy consent and
the confirmed boost transition, switching off from Settings while exhausted,
failed saves retaining the confirmed value, and zero/unavailable allowances.
Failed-save screenshots wait for the current consent title's finite presence
animation to settle, preserving the visible error and retained switch evidence.

`browser-demonstrations.mjs` validates the continuous traffic/hourly/routing
walkthroughs using the real fixture components, desktop cursor/mobile touch
cues, pause/seek/replay, reduced motion, exact illustrative wallet states,
routing match/default branches, restored focus and the absence of real writes.
It opts individual scenarios into normal motion; other recovery suites keep
their reduced-motion preference. Paused captures finish finite CSS transitions
while asserting that the illustrative clock and pointer pose stay fixed.

`browser-knowledgebase.mjs` exercises all nine cards through the shared player
in desktop/mobile English and Chinese. It checks every visible chapter and its
three icon-led details, shorter durations, direct source links, reduced motion,
keyboard seek/pause/replay, dialog focus return, and the explicit new-member
tutorial entrance. The six subscription steps each retain a preview image.
`browser-knowledgebase-helpers.mjs` shares the fixture settlement, transport,
visible-content, source-link and renderer assertions.
`browser-knowledgebase-rendering.mjs` verifies real Three.js canvas rendering,
pixel-stable paused poses, seek-driven redraw, silent unsupported-WebGL fallback,
actual context loss/recovery, closing during a deferred renderer import, and
every chapter at 320px in both shipped languages.
The constructed full-shell fixture records
all mocked API calls and refuses unexpected network access, proving lesson
playback has no real or mocked member actions. Assertions wait for the initial
preferences, access, balance, currency and availability reads to finish before
recording the baseline. Each locale and viewport retains a top-of-page overview
and clean wallet/gas and exchange-fee payment frames with details already visible.
Chapter/seek captures wait two animation frames, then compare the unique visual
and accessible chapter selections plus the rendered slider range and contained-thumb geometry.
The `knowledgebase` scope follows
the feature, its locales, shared player/clock/3D changes and member-shell changes.

`browser-member-design.mjs` captures every member route and both Explore modes,
all DIY categories, public subscription and payment results in desktop/mobile
Chromium. Narrow 320px Chinese captures check the densest member layouts and
assert the document locale; Home and both Explore modes also retain narrow
English captures. The catalog demonstration keeps a 44px target on the mode
selector row. Normal-motion hover/focus captures settle entry transitions, hold
only the decorative card sweep midway for visual review, then resume it;
existing page captures use reduced motion.
It checks loaded content, connection filtering, loading/empty/error states,
document overflow, enabled DIY Save contrast, unobstructed desktop Add route,
and the shared console/network guards. Missing destination
fixtures use `member-utility-audit.html` with only constructed data. This scope
is selected by member theme/layout changes, including the member-only font;
it does not become a mandatory universal suite for unrelated PRs.

`browser-member-design-checks.mjs` supports those existing captures with rendered
text checks: the bet confirmation label fits without clipping, long host names
retain room for a 12-character sample in their actual font, and allowance legend
labels remain separated from values. It also settles finite card-entry CSS before
capturing hover/focus decoration. Changes to the three guarded components select
the design scope alongside their functional recovery suites.

`browser-ip-recovery.mjs` checks partial BGP geometry, same-prefix route-page and
selection recovery, and retirement of paid live snapshots on account changes.
Desktop/mobile checks retain screenshots and never contact IP data providers.

`browser-settings-recovery.mjs` verifies owner-bound administrator links, expired
link creation failures, queued activation reconsent and retained tag drafts after
save failures. Only constructed accounts, links and preference writes are used.

`browser-chart-recovery.mjs` checks exact decimal traffic readouts, unavailable
buckets and safe numeric metric detail in desktop/mobile Chromium, including
keyboard navigation. Its existing chart fixture issues no API requests.

`browser-admin-settings-recovery.mjs` holds cached administrator settings reads
through reviewed profile, amount-limit and reward saves, then resolves or rejects
the obsolete reads. It also verifies deleted providers stay absent. All six
mobile/desktop scenarios use constructed data in the existing Emby settings fixture.

`browser-backup-recovery.mjs` checks locked pending backup/job deletion, visible
refusal and explicit retry, refreshed empty inventories, and retirement of an
in-flight archive download when the owning workspace unmounts.

`browser-admin-activity-recovery.mjs` checks game write locking, explicit retry,
a confirmed save followed by read failure, retirement of cached draw inventories,
and forecast recovery in desktop/mobile browser editors.

`browser-admin-coupon-recovery.mjs` checks loading, locked pending edits,
refusal/retry, retained save/deactivation effects after read failures, option
retry and retired write completions in desktop/mobile Chromium. Exact recorded
payloads verify that unrelated expiry and financial terms are omitted.

`browser-admin-database-recovery.mjs` checks exact large numeric values, locked
pending review/apply, visible failure and reviewed retry, retired metadata,
nullable booleans and disposed Apply completions on desktop and mobile. It uses
constructed rows and records both review and apply payloads without a database.

`browser-admin-settings-recovery.mjs` also verifies the stablecoin fixture server probe and failed-then-recovered transfer dismissal. Both actions are constructed, and the browser harness rejects every backend/provider request.
