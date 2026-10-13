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
- `BrowserLanding.vue` explains services, starts OAuth, and forwards members.
- `BrowserAccess.vue` removes the token fragment before one-use exchange.
- `session.ts` restores identity, hydrates the neutral session projection,
  renews cookies, and revokes only the current browser device.
- `session.test.ts` covers bootstrap deduplication, browser/member separation,
  stable renewal, expiry without discarding editor identity, scoped cache
  isolation, logout revocation, and one-use exchange in hosted CI.
- `theme.ts` persists system/light/dark choice and observes system changes.
- `styles.css` owns semantic tokens and workspace rules.
- `landing.css` owns public entrance presentation.

Browser cookies/signatures have their own scope. Snapshots include experience
and identity. No global member/admin preload runs here. Mutations retain audited
reasons, financial precision, validation, idempotency, and durable receipts.

Register the displayed callback and site origin in BotFather; save client ID and
write-only secret in the access panel. Blank secrets preserve existing values.
OAuth stays unavailable until configured. Mini App handoffs work independently
for initial setup. Add new routes to both router and navigation, document their
module, and validate every state and both themes using Chrome DevTools MCP.
