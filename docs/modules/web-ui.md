# Vue web UI module

## Composition and ownership

### Motion architecture

Motion Vue is the semantic animation layer. `motionPresets.ts` owns the shared
timings and critically damped layout spring; `useMotionPreferences.ts` reads
`prefers-reduced-motion` through Motion and returns opacity-only fallbacks. The
shell owns one keyed route presence boundary, navigation owns shared active
markers, and feature components own list/layout/presence feedback. CSS remains
responsible for perpetual loaders, skeletons, radar, and particles. No global
selector animates `.notice`, `.operation-status`, or `.status-badge` on mount.

P1-P3 motion remains component-owned: dashboard value and flip changes,
connection/admin workflows, chart data updates, and completion feedback move
only when their underlying state changes. Route wrappers and admin panel roots
remain static, and every new stateful path uses the central reduced-motion
policy.

The interaction contract is: press feedback is immediate, outgoing content
exits locally, incoming content settles within 180-240ms, and reduced motion
removes translation, scale, rotation, and celebration movement while retaining
a short opacity handoff. Stable domain IDs remain the keys for list presence;
editor-only drafts use a memory-only client key that is never persisted.

The web module is a mobile-first Telegram Mini App built with Vue 3 Composition API and `<script setup lang="ts">`. Route views are composition surfaces; typed feature composables own remote state, polling, and mutations; reusable components own presentation and local interaction state.

The main boundaries are:

- `AppShell`: safe areas, session bootstrap, desktop/mobile navigation, skip link, Telegram BackButton ownership, and guarded focus restoration after navigation. A shared owner stack prevents an overlay close action from competing with route history.
- Member features: `ActivityPage`, `CouponWalletPanel`, `QuestionnairePage`, `EmbyPage`, `CatalogCheckout`, and `BalancePaymentSheet`.
- Shared controls: `TxbAmountField` converts human major-unit input to integer hundredths, Nuxt UI owns accessible switches/forms/overlays/tables, and `MarkdownContent` renders sanitized Markdown with raw HTML disabled.
- `AdminShell`: compact responsive navigation for Commerce, Community, Onboarding, Users, Emby, and System operations. Every registered backend administration domain has a live UI route.
- Workflow surfaces: questionnaire CSV import and database record editing use mobile drawers/dialogs with explicit review states rather than exposing raw SQL or unreviewed mutations.

`src/api/generated.ts` is generated from the split `api/openapi.yaml` contract. `src/api/http.ts` is the only HTTP transport and `src/api/features.ts` is a narrow feature adapter. Once Telegram authentication supplies the companion key, every protected request signs its uppercase method, escaped path and raw query, exact timestamp/nonce, and SHA-256 body hash. JSON, multipart, binary download, and empty-body calls all share that implementation.

Telegram bootstrap waits for a delayed WebApp bridge before binding events. It expands the viewport, calls `ready()` after the application mounts, reflects theme and safe-area changes, configures supported native surface colors, and falls back to browser controls when native controls are unavailable. Visible copy and accessibility labels remain locale-owned.

The mobile audit keeps native form controls at a 16 px minimum for Telegram and iOS, uses Nuxt UI button variants without legacy class collisions, and keeps compact navigation/content above the safe-area-aware bottom bar. Backup and restore panels expose localized operational summaries instead of raw provider diagnostics.

The shared `SwitchField` makes its full row a native label for the Nuxt UI
switch. Its explicit label and help associations preserve the accessible name,
native disabled behavior, and one model update per activation. Global button
resets must preserve Nuxt UI foreground utilities and semantic color tokens.
Inset admin notices use automatic width so their margins remain inside the
panel. Compensation configuration and event grids use shrinkable tracks;
long node and squad names wrap rather than enlarging the phone layout.

Questionnaire distinguishes an unsuccessful initial request from a successful
response with no active questionnaire. The former displays a localized retry;
the latter displays the empty state. A refresh failure retains previously loaded
questionnaire data with an inline notice.

Localization is embedded from matching domain shards under `locales/en/` and `locales/zh-CN/` through `src/i18n/generated.ts`. Vite validates identical nested leaf keys and placeholders before a build; runtime `t()` supports interpolation, persisted switching, and Telegram-language defaulting. API and local failure states resolve to locale keys rather than displaying raw transport text. The language selector is available before authentication and inside the authenticated shell.

## Routes and state behavior

Catalog checkout prompts for selected activation-required squads one at a time and sends the in-memory code map once with the purchase request. The group-message reward is a progress card showing message count, reward amount, and claimed/available state. Rollover detail renders forecast metrics with localized success, daily-reduction, and no-rollover states, including predicted rollover credit, maximum daily usage, or N/A when current usage already exceeds the allowable maximum.

The canonical member routes are `/home`, `/catalog`, `/activity`, `/questionnaire`, `/emby`, and `/payment-result`. `/balance` is retained only as a compatibility redirect to Home's funding sheet, and `/games` redirects to `/activity`. Telegram route mode is selected before Vue Router construction from the native `TelegramWebviewProxy` bridge, launch parameters, WebApp data, platform, or Telegram user-agent signals; Telegram uses in-memory route history and strips launch-only parameters from the initial route, while regular browsers use HTML5 history. Failed lazy route imports receive one guarded reload attempt, while the server revalidates `index.html` and gives hashed assets immutable caching. The provider-return route is browser-public only for the signed return landing flow: Telegram verifies the owner-scoped durable order, while a regular browser receives only the capability-limited receipt projection. Only failed or expired Telegram orders may hand their ID to Home, which refetches it before presenting a replacement payment. The three expanded product areas are real data-backed routes; they no longer share a placeholder view.

Home's traffic detail opens on demand, defaults to the latest seven inclusive UTC dates, lets the member choose a start and end date within 31 dates, and displays X-Node's top 20 returned nodes with daily byte totals. Beneath the unchanged traffic-limit meter, an unlabeled color-segmented bar previews the current top-node distribution; hover or keyboard focus previews a localized detail panel, while click/tap locks it until the same segment or Escape closes it. The bar joins usage UUIDs to the catalog's read-only enabled-node metadata for multipliers, and shows a neutral remainder when the returned top nodes do not cover all usage. The date-range request is independent of the cached home summary and preserves an explicit upstream-unavailable state.

The active `Your ride` summary is an accessible flip card. Its summary button is independent from renewal and queued-cancellation controls; opening it fetches `/api/v1/purchases/{id}/rollover` fresh, shows loading/retry states, and presents actual usage, projected full-term usage, maximum usable traffic, predicted credit, maximum daily usage, or required total/daily reduction. The interaction uses transform/opacity motion with a reduced-motion state swap, keeps the back face inert while closed, and lets only the visible face determine the card height.

Activity shows daily check-in, group-message reward progress/claimed state, enabled betting games, lucky draws, recent outcomes, exact stakes/fees, and resulting balance. It does not use streak pressure, near-miss animation, celebratory loops, or other manipulative gambling cues. Every financial action is disabled while pending and displays the server result.

Coupon wallet redemption is explicit. Catalog checkout displays at most one selected eligible grant, refreshes the non-mutating server quote whenever that selection changes, and blocks Coupon-step progression until its current quote is usable. It prunes add-on squads newly included by a changed combo; only full paid add-ons are gray, disabled, and labelled `Full`, while bundled squads remain selectable under server authority. Accessible-node cards present the live provider name and favicon with a localized fallback. Combo and squad descriptions pass through `MarkdownContent`; the safe `[text]{color=accent size=lg}` directive maps only allowlisted colors and sizes to CSS classes with raw HTML disabled. Admin editing provides toolbar controls and the same renderer as a live preview.

Questionnaire participation retrieves the same durable validation code on repeat visits. The administrator import flow progresses through upload, header/sample review, validation-column selection, match analysis, explicit settlement, and background-status polling. Closing and destructive deletion are separate actions. Activity result dialogs are focus-trapped, game icons come from the allowlisted external Iconify Phosphor set, and statistics expose accessible table data alongside daily/weekly graphs.

Emby setup collects a write-only password, parental rating, and libraries before debit and shows the exact TXB setup price. Linked accounts expose only approved password and preference controls; raw policy fields are never presented. Failed retryable provisioning shows a bounded retry action.

The payment sheet selects a canonical method ID in two stages: provider account tiles, then rail card controls. Create and cancel use caller-owned UUID idempotency keys, accept `202` operation receipts, and poll those receipts before presenting provider checkout state. Ambiguous transport failures retain the same key; `pending_review` stops automatic polling and shows a localized review state without replaying the mutation. Once creation succeeds, the sheet separately renders provider URL, QR payload, receiving address, actual crypto amount, and currency when supplied. Cancellation stops order polling, while a later authoritative `paid` projection still refreshes the balance exactly once. The browser return page renders a signed public receipt projection with amount, payment ID, provider, channel, status, and created/paid timestamps, without a Mini App navigation action.

Catalog review is the final purchase step and includes authoritative validity,
traffic/reset and rollover details, accessible-node count, add-ons, coupon
effect, and total. A successful purchase replaces the flow with a localized
confirmation summary containing the charged amount, discount, validity window,
status, traffic limit, and reset cadence; its Home action is the only navigation
out of that transient state. Combo/add-on/coupon selections and the current step
are stored in user-scoped `sessionStorage` and cleared after successful purchase;
progress is blocked when the quote has no accessible nodes. Home's ride summary
uses a focused `UModal`/`USwitch` automatic-renewal control: it displays the
authoritative gross/discount/net amount, charge date, next-cycle end, stable
eligibility reason, and a centered green-or-red action. A member with automatic
renewal enabled is redirected away from Catalog, with the server enforcing the
same restriction. Usage also includes a responsive native SVG graph with a
textual fallback, while Activity preserves a selected game's entered stake across
refreshes and has no refresh button.

## Administration

All TXB form fields accept major-unit decimal strings. For example, entering `150` sends `15000` minor units. Rates, percentages, and multipliers retain their documented server units.

The database editor lists allowlisted application tables, debounces broad search, supports up to five typed filters, cursor-paginates fingerprinted queries, and renders typed null, boolean, numeric, text, and blob controls. A write first requests a server diff/review hash, then requires the optimistic record hash, reason, and typed `EDIT <table>` confirmation. The UI warns that direct edits bypass domain synchronization hooks.

Backup downloads use an authenticated binary response. Restore requires `RESTORE <filename>`, describes the automatic rescue backup, submits a staged restore, polls its operation, and enters reconnect/reauthentication state after the server begins its graceful restart.

Encrypted settings render as masked values and only permit write-only replacement. Financial and destructive actions retain visible text labels and Nuxt UI controls. Payment administration renders one visual card per configured provider account, supports adding multiple EZPay/BEPusdt accounts, uses Nuxt UI checkbox groups for independent channel selection, and preserves enabled state, acknowledgement, custom name, and masked credentials. Catalog review restores its quote when revisited, and purchase controls remain in normal mobile flow.

## Visual and accessibility contract

The visual system preserves the existing premium-dark graphite/mint identity: graphite canvas, layered charcoal surfaces, off-white text, one muted mint accent, 16 px panels, and 12 px controls. Member density remains lower than administrator density, copy is task-oriented, and motion is restrained.

The Home ride summary exposes cancellation only for the authenticated user's queued entitlement. Cancellation and automatic-renewal toggle changes refresh the dashboard after the server atomically changes the ride and records the corresponding immutable ledger entry.

Interactive targets are at least 44 px. Keyboard focus is visible and restored to the main landmark after route changes. The skip link, semantic fieldsets, status live regions, Nuxt UI overlay focus management, Telegram BackButton/MainButton integration, safe-area variables, and 320 px layouts are part of the acceptance contract. Faint text meets usable contrast, and `prefers-reduced-motion` disables nonessential transitions.

## Build, failure behavior, and verification

`npm run generate:api` regenerates the OpenAPI types. `npm run audit:structure` enforces line limits, README inventories, locale parity, the no-local-icon/native-control policy, and Vite client-bundle coverage for production-source icons. `npm run check` then lints, type-checks, runs Vitest, and builds. Production assets are written to `internal/webui/dist` for Go embedding.

Feature requests abort or stop polling on unmount, route change, sheet close, or cancellation. Standard loading, empty, stale, validation, upstream-failure, retry, cancelled, and late-settlement states are rendered rather than represented by blank panels. Telegram bootstrap initializes before router construction, waits for delayed WebApp initialization for authentication, and accepts the standard WebApp data source; an already-detected WebApp context reports a loading state rather than the misleading outside-Telegram message. Internal controls use Vue Router actions rather than native anchor navigation. A 401 clears session state and restarts Telegram authentication; no plaintext password, callback capability, subscription URL, or encrypted setting is logged by the client.
