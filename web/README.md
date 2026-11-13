# VanSail Carpool web client

VanSail Carpool is a bilingual, mobile-first Telegram Mini App built with Vue 3.5, Vite 8, and Nuxt UI v4. The flat graphite-and-mint interface supports phone and tablet layouts, Telegram theme/safe-area variables, keyboard navigation, reduced motion, and short English or Simplified Chinese copy.

`vite.config.ts` prebundles the Nuxt editor's TipTap and ProseMirror imports together so development mode uses a single selection/plugin registry.

Nuxt UI uses stable 4.12.0. Vitest and its V8 coverage adapter are pinned together
at 5.1.0, and both Unovis packages are pinned at 1.8.0 to match their exact peer
requirements. Tailwind 4.3.5 and its Vite, PostCSS and native engine packages
are updated together. `clearMocks: false` preserves the existing mock lifecycle.
TypeScript stays at 5.9.3 because `openapi-typescript` requires TypeScript 5;
`typescript-eslint` also excludes TypeScript 6.1 and later. Use Node 26 or newer
with the committed npm lockfile. Vue 3.5, Vite 8 and the remaining dependency
upgrades stay within their supported peer and runtime constraints.
The hosted Vitest runner inlines both Unovis packages through Vite because the
Vue distribution imports directories that native Node ESM cannot resolve. This
uses the real chart modules rather than replacing them with test stubs; see
[Vitest's dependency configuration](https://vitest.dev/config/server.html#server-deps-inline).

Security overrides scope Minimatch 11.0.5 to ESLint, `@eslint/config-array`,
`@typescript-eslint/typescript-estree` and EditorConfig, whose matching APIs
remain available. JS Beautify uses Glob 14.0.4 through its retained `globSync`
and `hasMagic` APIs. These resolve the
[glob amplification](https://github.com/advisories/GHSA-g3hv-75wg-m249) and
[cancellation](https://github.com/advisories/GHSA-3g3m-2fjr-6pw4) advisories
without replacing Redocly's API-compatible Minimatch 5.1.9. Keep the
existing Redocly `js-yaml` and esbuild overrides when updating this graph.

The November 2026 refresh was checked with Chrome DevTools on constructed member
and administrator fixtures at 390 px and 1440 px: chart mode/keyboard selection,
DDNS host selection, payment receipt states and paired integration-secret edits.
Loading, empty and error states stayed contained without console errors or
backend requests. These browser checks do not establish Telegram host gestures
on physical devices. Hosted Actions remain the automated regression gate.

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

- Resumable intro and current-revision agreement onboarding use the authenticated
  Telegram identity. Agreement completion records local acceptance and durable
  provisioning work; subscription readiness follows native deployment evidence.
- Community group and channel access stays outside onboarding, shows canonical Telegram membership, and requires a currently active combo only when joining.
- Dashboard balance, entitlement, X-Node traffic, node summaries, and subscription-link rotation.
- Explore selects Charge by traffic or Charge by hour. Existing fixed-term
  owners retain renewal and refund controls, plus optional-squad selection,
  coupon redemption and idempotent add-squad confirmation from Settings.
- Provider/rail selection, exact payment instructions, QR/address rendering, cancellation, and authoritative settlement polling.
- Provider redirects land on `/payment-result`, which verifies durable payment state: paid orders show the green confirmation, while failed or expired orders offer an owner-verified replacement handoff to Home.
- Daily check-in, group-message progress, betting games, lucky draws, and bounded outcome history.
- Emby account setup, retry, password change, parental rating, and library preferences.

## Administration

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
- `fixtures/README.md` describes isolated constructed browser fixtures for member and administrator workflows.

`subscription.html` is the isolated public bearer entry. It starts no browser-admin or Mini App session and carries no-referrer/no-store policy.
