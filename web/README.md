# VanSail Carpool web client

VanSail Carpool is a bilingual, mobile-first Telegram Mini App built with Vue 3.5, Vite 8, and Nuxt UI v4. The flat graphite-and-mint interface supports phone and tablet layouts, Telegram theme/safe-area variables, keyboard navigation, reduced motion, and short English or Simplified Chinese copy.

`vite.config.ts` prebundles the Nuxt editor's TipTap and ProseMirror imports together so development mode uses a single selection/plugin registry.

Nuxt UI uses stable 4.11.3 rather than a PR preview. Vitest and its V8 coverage
adapter are pinned together at 5.0.3; `clearMocks: false` preserves the previous
mock lifecycle. TypeScript stays at 5.9 because `openapi-typescript` requires
TypeScript 5, although newer TypeScript majors are available. Other dependencies
are upgraded within their supported peer and runtime constraints.

## Commands

```sh
npm ci
npm run generate:api
npm run audit:structure
npm run lint
npm run typecheck
npm run build
npm run dev
```

`npm run check` and all Vitest suites are hosted-CI-only under `AGENTS.md`.
Local validation uses the separate static commands above and Chrome DevTools
MCP browser inspection. The build output goes to `../internal/webui/dist` for Go embedding.

## Application boundaries

- `src/App.vue` provides Nuxt UI's `UApp` context and locale binding.
- `src/main.ts` bootstraps compatibility before loading the app bundle, then either mounts a localized capability gate or initializes Telegram, Pinia, Vue Router, Nuxt UI, Motion for Vue, and global styles.
- `src/router/` lazy-loads member and administrator views and enforces onboarding/role access.
- `src/stores/` holds only session-wide authenticated identity state.
- `src/api/` contains generated OpenAPI types, focused contracts, and one same-origin transport. Authenticated calls use the companion request key to sign the exact method, path/query, timestamp, nonce, and body bytes, including a secure HMAC fallback when Web Crypto is unavailable.
- `src/i18n/` merges parity-checked locale shards from `locales/en/` and `locales/zh-CN/`; user-facing copy belongs there rather than in components or composables.
- `src/composables/` owns remote state, polling, cancellation, idempotency, and mutations.
- `src/components/` composes Nuxt UI controls into domain features; Iconify supplies all Phosphor and country icons, so no local icon assets are shipped. `vite.config.ts` keeps the Nuxt UI icon registry and dynamic feature registries in the client bundle explicitly; source scanning remains enabled for static icon references.
- `src/styles/` defines the flat premium-dark tokens, safe-area layout, restrained motion, and tablet breakpoints without gradients or decorative shadows.
- `scripts/` generates compact contract output and enforces repository structure in CI.

Each source folder has its own `README.md` file map. Start with [the source map](src/README.md), [component architecture](src/components/ARCHITECTURE.md), and [locale catalog](locales/README.md).

## Member features

- Resumable intro, reserved username, and agreement onboarding; a confirmed
  X-Node username collision after a paid purchase shows the full-refund
  notice and returns the member to username entry. Decisive onboarding actions
  mirror to Telegram's native MainButton when available.
- Community group and channel access stays outside onboarding, shows canonical Telegram membership, and requires a currently active combo only when joining.
- Dashboard balance, entitlement, X-Node traffic, node summaries, and subscription-link rotation.
- A six-step catalog journey for core combos, optional squads, accessible nodes, coupon redemption, review, and idempotent purchase confirmation.
- Provider/rail selection, exact payment instructions, QR/address rendering, cancellation, and authoritative settlement polling.
- Provider redirects land on `/payment-result`, which verifies durable payment state: paid orders show the green confirmation, while failed or expired orders offer an owner-verified replacement handoff to Home.
- Daily check-in, group-message progress, betting games, lucky draws, and bounded outcome history.
- Coupon wallet/redeem flow, active questionnaire participation, and durable validation codes.
- Emby account setup, retry, password change, parental rating, and library preferences.

## Administration

The live admin routes cover settings, catalog/squads, activity, coupons, questionnaires/CSV settlement, onboarding content, users, Emby accounts, entitlements, payments/refunds, backups/restore, the reviewed database editor, and audit history. Nuxt UI provides forms, selects, tables, modals, drawers, alerts, badges, and loading states; Motion owns local list presence, layout, selection, and workflow feedback.

Financial and destructive operations retain text labels, busy states, explicit reasons, confirmations, and localized failures. The database editor never sends raw SQL: it uses typed filters, cursors, optimistic hashes, server diff review, rescue backup, and typed confirmation.

## Root files

- `.gitignore` excludes local frontend artifacts.
- `auto-imports.d.ts` records generated Nuxt UI composable declarations.
- `components.d.ts` records generated global component declarations.
- `eslint.config.js` defines Vue and TypeScript lint policy.
- `go.mod` keeps the frontend outside the root Go module walk.
- `index.html` is the dark, isolated SPA document.
- `package.json` and `package-lock.json` pin commands and dependencies.
- `tsconfig.json` defines strict TypeScript/Vue compilation.
- `vite.config.ts` configures Nuxt UI, locale parity, tests, proxying, and embedded output.
- The draw artwork uses Lottie's local light SVG player through the Vite alias; these assets need only vector shapes.
- `fixtures/README.md` describes the constructed lucky-draw browser fixture used for visual audits.

`subscription.html` is the isolated public bearer entry. It starts no browser-admin or Mini App session and carries no-referrer/no-store policy.
