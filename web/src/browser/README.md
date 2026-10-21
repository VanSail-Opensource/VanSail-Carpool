# Browser entrance and admin console

Desktop-first, mobile-compatible Vue/Vite experience independent of the member
Mini App. Shares typed clients and business workflows, but does not initialize
the Telegram SDK or load member shell/theme assets.

- `main.ts` initializes browser theme, Vue, Pinia, router, and Nuxt UI.
- `BrowserApp.vue` owns localized providers, renewal, and an expiry overlay that
  preserves mounted editors while login opens in another tab.
- `router.ts` owns browser history, protected routes, aliases, and payment returns.
- `BrowserConsole.vue` composes dashboard navigation and route content.
- `navigation.ts` groups every admin section, including IP Lookup and USDT.
- `BrowserPreferences.vue` provides independent locale and appearance controls.
- `BrowserLanding.vue` explains services, directs administrators to Mini App access links, and forwards members.
- `BrowserAccess.vue` removes the token fragment before one-use exchange.
- `session.ts` restores identity, hydrates the neutral session projection,
  renews cookies, and revokes only the current browser device.
- `session.test.ts` covers bootstrap deduplication, browser/member separation,
  stable renewal, expiry without discarding editor identity, scoped cache
  isolation, logout revocation, and one-use exchange in hosted CI.
- `theme.ts` persists system/light/dark choice and observes system changes.
- `styles.css` owns semantic tokens and workspace rules.
  Text-control sizing excludes hidden fields; button sizing applies to actions
  and comboboxes, leaving switch/checkbox roles and Nuxt UI internals untouched.
  Rules use the experience root so portalled dialogs follow the same policy.
- `landing.css` owns public entrance presentation.

Browser cookies/signatures have their own scope. Snapshots include experience
and identity. No global member/admin preload runs here. Mutations retain audited
reasons, financial precision, validation, idempotency, and durable receipts.

Administrators generate a one-use console link in Mini App Settings > Admin
access. The token is consumed in the browser without sharing Mini App sessions.
Add new routes to both router and navigation, document their module, and validate
every state and both themes using Chrome DevTools MCP.
