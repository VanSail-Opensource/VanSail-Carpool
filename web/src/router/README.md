# Router

The member guard resolves session, onboarding and administrator-access routing.
Catalog and billing endpoints enforce purchase eligibility; route or cached state
never grants purchase permission.

- `index.ts` declares routes and installs the session guard.
- `memberViews.ts` shares the member lazy loaders with bounded, idle code preloading after authenticated onboarding. Data Saver and slow connections skip optional warming; failed chunks retain normal foreground recovery. Session changes cancel queued work, and administrator routes are excluded.
- `history.ts` selects WebView-safe in-memory history or browser history.
- `recovery.ts` performs one-shot recovery for stale lazy route chunks.
- `guards.ts` resolves onboarding, administrator, and member access.
- `index.test.ts` confirms the protected Community route is registered without changing primary navigation state.
- `guards.test.ts` covers protected-route decisions.
- `history.test.ts` covers Telegram and browser history selection.
- `recovery.test.ts` covers one-shot lazy-route recovery.
  Recovery destinations must be local absolute paths without network-path,
  backslash or control-character redirects, including retained storage records.

Telegram sessions are detected before router construction from the native
`TelegramWebviewProxy` bridge, launch markers, WebApp data, platform, or
Telegram user-agent signals. They use Vue Router's
in-memory history so internal links do not reload the Mini App at a new WebView
URL and lose Telegram's launch context; Telegram-only launch parameters are
removed from the initial in-app route. Regular browser sessions use HTML5
history and keep shareable route URLs.
If a cached browser bundle cannot load a lazy route chunk, the router retries
once after reloading and preserves the intended route in session storage. In a
Telegram session it retries through the in-memory router instead, so a chunk
failure never changes or reloads the Telegram launch URL.

`/payment-result` is an immersive provider-return landing route. Telegram opens it with owner-scoped durable order polling; a provider redirect opened in a regular browser uses the short-lived capability-return status endpoint and never boots the protected app shell. Failed or expired Telegram orders hand their ID to Home for an owner-scoped reissue fetch.

`/balance` remains a compatibility redirect to `/home?topUp=1`; Home owns the real funding sheet.

Statistics belongs to the separate browser-administrator `/admin/statistics`
route. The member router does not expose a statistics view.

`/community` is a protected thin member route reached from Around TX. It keeps
group and channel joining outside onboarding and uses the shared shell back behavior.

`/connections` is a protected thin route. The application shell owns route-level native Back, while the selected-IP confirmation temporarily takes overlay ownership and restores the route owner when it closes.
