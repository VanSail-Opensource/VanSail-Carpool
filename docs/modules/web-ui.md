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

P1-P3 motion remains component-owned: dashboard value changes,
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
- Workflow surfaces: CSV import and database record editing use mobile drawers/dialogs with explicit review states rather than exposing raw SQL or unreviewed mutations.

`src/api/generated.ts` is generated from the split `api/openapi.yaml` contract. `src/api/http.ts` is the only HTTP transport and `src/api/features.ts` is a narrow feature adapter. Once Telegram authentication supplies the companion key, every protected request signs its uppercase method, escaped path and raw query, exact timestamp/nonce, and SHA-256 body hash. JSON, multipart, binary download, and empty-body calls all share that implementation.

Telegram bootstrap waits for a delayed WebApp bridge before binding events. It expands the viewport, calls `ready()` after the application mounts, reflects theme and safe-area changes, configures supported native surface colors, and falls back to browser controls when native controls are unavailable. Visible copy and accessibility labels remain locale-owned.

The mobile audit keeps native form controls at a 16 px minimum for Telegram and iOS, uses Nuxt UI button variants without legacy class collisions, and keeps compact navigation/content above the safe-area-aware bottom bar. Backup and restore panels expose localized operational summaries instead of raw provider diagnostics.

The shared `SwitchField` makes its full row a native label for the Nuxt UI
switch. Its explicit label and help associations preserve the accessible name,
native disabled behavior, and one model update per activation. Global button
resets must preserve Nuxt UI foreground utilities and semantic color tokens.
Inset admin notices use automatic width so their margins remain inside the
panel. Node and squad names wrap rather than enlarging the phone layout. Outage
Compensation configuration, event grids, and navigation are retired.


Localization is embedded from matching domain shards under `locales/en/` and `locales/zh-CN/` through `src/i18n/generated.ts`. Vite validates identical nested leaf keys and placeholders before a build; runtime `t()` supports interpolation, persisted switching, and Telegram-language defaulting. API and local failure states resolve to locale keys rather than displaying raw transport text. The language selector is available before authentication and inside the authenticated shell.

## Routes and state behavior

Home renders the active purchase directly, with automatic renewal and eligible refund controls. Member traffic predictions and the old prediction flip card are retired. Administrator user-detail traffic forecasts remain available independently of PERSONAL pricing; forecasts do not choose an individual base price. Renewal controls remain available without a balance requirement.

Home gates its member product surfaces on a positive balance or a server-confirmed active combo. A member with neither sees a compact animated, icon-led add-balance instruction that opens the existing funding sheet; Subscription, live usage, Usage Breakdown, entitlements and Around TX are hidden, and the sidebar omits Subscription, Connections and Revoke. Usage Breakdown retains its chart, measure tabs, totals and stale warning without the former coverage, gap, truncation or cost-provenance prose. Node and hourly squad colors stay distinct beyond the curated palette through deterministic derived colors.

The canonical member routes are `/home`, `/catalog`, `/activity`, `/emby`, and `/payment-result`. `/balance` is retained only as a compatibility redirect to Home's funding sheet, and `/games` redirects to `/activity`. Telegram route mode is selected before Vue Router construction from the native `TelegramWebviewProxy` bridge, launch parameters, WebApp data, platform, or Telegram user-agent signals; Telegram uses in-memory route history and strips launch-only parameters from the initial route, while regular browsers use HTML5 history. Failed lazy route imports receive one guarded reload attempt, while the server revalidates `index.html` and gives hashed assets immutable caching. The provider-return route accepts a checkout-issued v2 capability with fixed expiry for its limited browser receipt projection. Legacy or expired links require owner authentication; navigation never creates or renews a capability. Telegram verifies the owner-scoped durable order. Only failed or expired Telegram orders may hand their ID to Home, which refetches it before presenting a replacement payment. The three expanded product areas are real data-backed routes; they no longer share a placeholder view.

Home retains its traffic-limit meter, exact used/remaining bytes, and stale-data disclosure. The old per-node segmented bar and reset timestamp are retired. Traffic detail opens on demand and preserves loading, empty, error, and retry states independently of the cached home summary. Detailed node usage belongs to the usage breakdown chart.

Activity shows daily check-in, group-message reward progress/claimed state, enabled betting games, lucky draws, recent outcomes, exact stakes/fees, and resulting balance. It does not use streak pressure, near-miss animation, celebratory loops, or other manipulative gambling cues. Every financial action is disabled while pending and displays the server result.

Coupon wallet redemption is explicit. Catalog checkout displays at most one selected eligible grant, refreshes the non-mutating server quote whenever that selection changes, and blocks Coupon-step progression until its current quote is usable. It prunes add-on squads newly included by a changed combo; only full paid add-ons are gray, disabled, and labelled `Full`, while bundled squads remain selectable under server authority. Accessible-node cards present the live provider name and favicon with a localized fallback. Combo and squad descriptions pass through `MarkdownContent`; the safe `[text]{color=accent size=lg}` directive maps only allowlisted colors and sizes to CSS classes with raw HTML disabled. Admin editing provides toolbar controls and the same renderer as a live preview.

Questionnaire participation retrieves the same durable validation code on repeat visits. The administrator import flow progresses through upload, header/sample review, validation-column selection, match analysis, explicit settlement, and background-status polling. Closing and destructive deletion are separate actions. Activity result dialogs are focus-trapped, game icons come from the allowlisted external Iconify Phosphor set, and statistics expose accessible table data alongside daily/weekly graphs.

Emby setup collects a write-only password, parental rating, and libraries before debit and shows the exact TXB setup price. Linked accounts expose only approved password and preference controls; raw policy fields are never presented. Failed retryable provisioning shows a bounded retry action.

The payment sheet selects a canonical method ID in two stages: provider account tiles, then rail card controls. Create and cancel use caller-owned UUID idempotency keys, accept `202` operation receipts, and poll those receipts before presenting provider checkout state. Ambiguous transport failures retain the same key; `pending_review` stops automatic polling and shows a localized review state without replaying the mutation. Once creation succeeds, the sheet separately renders provider URL, QR payload, receiving address, actual crypto amount, and currency when supplied. Cancellation stops order polling, while a later authoritative `paid` projection still refreshes the balance exactly once. The browser return page renders the capability-limited receipt projection with amount, payment ID, provider, channel, status, and created/paid timestamps. An expired or denied link clears prior receipt data, stops polling, and offers the configured Telegram entrance for owner authentication.

## Administration

All TXB form fields accept major-unit decimal strings. For example, entering `150` sends `15000` minor units. Rates, percentages, and multipliers retain their documented server units. The Charge by hour policy form additionally accepts human transfer units: `1GB`/`500MiB` (binary, exact bytes) and `100Mbps`/`12.5MB/s` (exact bytes per second), with plain integers still treated as bytes or bytes/second. Values render back in an exact, round-trippable unit form. Its offers list excludes pay-as-you-go (Charge by traffic) squads, which keep their independent per-GB pricing.

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
