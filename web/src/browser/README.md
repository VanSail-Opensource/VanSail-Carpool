# Browser entrance and admin console

`router.ts` and `navigation.ts` expose `/admin/hourly`, an independent policy and
offer editor implemented by `../components/admin/hourly/HourlyAdminPage.vue`.
It follows console theme/session behavior and uses no Telegram interactions.

Desktop-first, mobile-compatible Vue/Vite experience independent of the member
Mini App. Shares typed clients and business workflows, but does not initialize
the Telegram SDK or load member shell/theme assets.

- `main.ts` initializes browser theme, Vue, Pinia, router, and Nuxt UI.
- `BrowserApp.vue` owns localized providers, renewal, and an expiry overlay that
  preserves mounted editors while the same administrator logs in through another
  tab. A different administrator identity replaces the console subtree so prior
  drafts, one-time secrets and operation context cannot transfer between actors.
- `router.ts` owns browser history, protected routes, aliases, and payment returns.
- `routeRecovery.ts` retries recognized lazy-chunk failures once at the failed
  destination, preserves query/hash, and exposes persistent errors through the
  existing startup retry UI. Runtime errors and unavailable session storage never
  trigger automatic document reloads; `routeRecovery.test.ts` covers these paths.
- `BrowserConsole.vue` composes dashboard navigation and route content.
  Breadcrumbs identify the active page; the centered workspace stays readable
  on wide displays, with wrapping action rows and contained tables on phones.
- `BrowserApp.test.ts` covers draft continuity through expiry and console
  replacement when a different administrator takes over the browser session.
- `navigation.ts` groups every admin section, including IP Lookup and USDT.
- `BrowserPreferences.vue` provides independent locale and appearance controls.
- `BrowserLanding.vue` explains services, directs administrators to Mini App access links, and forwards members.
- `BrowserAccess.vue` removes the token fragment before one-use exchange. Temporary service/network failures retain the token only in component memory for an explicit retry; invalid links, success and disposal erase it. `BrowserAccess.test.ts` verifies retry classification, exclusive submission and disposal without exposing the token in page content or storage.
- `BrowserLoading.vue` and `loading.css` provide the centered identity, one localized pending/slow status, and conditional recovery. Initial bootstrap failures retain the requested URL for a forced bootstrap retry; access-link failures keep their separate entrance action. Readiness immediately releases the destination with a noninteractive 160ms cover fade. Reduced motion is static and readable.
- `session.ts` restores identity, hydrates the neutral session projection,
  renews cookies, and revokes only the current browser device. Successful logout
  and access exchange invalidate earlier identity reads so late bootstrap or
  renewal replies cannot resurrect the old session or overwrite the new one.
  Exchanges also reject superseded identities and aborted entrance scopes before
  dispatch and before applying a reply; `sessionExchange.test.ts` covers logout,
  newer exchanges and disposal while configuration or exchange is pending.
  A verified same-administrator response resumes suspended metric and certificate
  observers; logout and disposal remain final for those mounted observers.
- `session.test.ts` covers bootstrap deduplication, browser/member separation,
  stable renewal, expiry without discarding editor identity, scoped cache
  isolation, logout revocation, and one-use exchange in hosted CI.
- `theme.ts` persists system/light/dark choice and observes system changes.
- `workspaceMotion.ts` observes section headings once and cancels owned motion
  when the console unmounts or reduced motion is enabled. Content remains visible.
- `styles.css` owns semantic tokens and workspace rules.
  Text-control sizing excludes hidden fields; button sizing applies to actions
  and comboboxes, leaving switch/checkbox roles and Nuxt UI internals untouched.
  Rules use the experience root so portalled dialogs follow the same policy.
- `landing.css` owns the responsive public entrance, service overview, and membership presentation. `BrowserLanding.vue` retains the existing session and redirect flow and cleans up its one-time motion observers. The editorial images in `web/public/landing` are generated illustrative imagery, not customer evidence.

Browser cookies/signatures have their own scope. Snapshots include experience
and identity. No global member/admin preload runs here. Mutations retain audited
reasons, financial precision, validation, idempotency, and durable receipts.

Administrators generate a one-use console link in Mini App Settings > Admin
access. The token is consumed in the browser without sharing Mini App sessions.
Add new routes to both router and navigation, document their module, and validate
every state and both themes using Chrome DevTools MCP.
