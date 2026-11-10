# Utilities

- `usageRational.ts` supplies pure reduced BigInt arithmetic shared by Usage
  Breakdown response validation and chart aggregation. Only bounded drawing
  proportions become numbers; `usageRational.test.ts` covers fractional totals,
  large integers, exact thresholds and invalid input rejection.

- `purchaseValidity.ts` derives display status and current-term eligibility from raw lifecycle status and exclusive validity boundaries. It never mutates purchase or financial records; terminal statuses remain authoritative.
- `purchaseValidity.test.ts` covers exclusive expiry, future terms, terminal states and predecessor/successor isolation in hosted CI.

- `experience.ts` selects browser or Mini App behavior without loading the Telegram SDK.
- `feedback.ts` delegates member haptics installed by Mini App bootstrap and stays inactive in the browser console.

- `browserClipboard.ts` isolates the clipboard API and selection fallback; `browserCompatibility.ts` re-exports `copyText` for existing callers while keeping browser capability checks focused.

- `format.ts` formats authoritative money, dates, bytes, and TXB input; CNY/USD use `￥`/`$` prefixes. `displayCurrency.ts` converts member-facing TXB displays with `BigInt` fixed-point arithmetic only, falls back from a zero USD conversion to CNY and then to TXB, supplies primary/approximate catalog price lines, and exposes the effective input currency (falling back to TXB when the persisted CNY/USD rate is unavailable) for editable amount fields.
- `transferUnits.ts` parses administrator size/speed input such as `1GB`, `500MiB`, `100Mbps`, `12.5MB/s` or exact bytes, and renders editable values that round-trip exactly. `transferUnits.test.ts` covers bit/byte conversion, binary size units and exact display.
- `dom.ts` provides WebView-safe focus restoration.
- `dom.test.ts` covers focus restoration behavior.
- `format.test.ts` covers formatting and money conversions.
- `displayCurrency.test.ts` covers exact CNY/USD conversion, catalog dual-price output, symbol/sign handling, and unavailable-rate or zero-conversion CNY/TXB fallbacks.
- `browserCompatibility.ts` checks baseline browser capabilities, supplies secure UUID entropy, and installs narrow WebView constructor fallbacks before the app bundle mounts.
- `browserCompatibility.test.ts` covers constructor, UUID, and fail-closed entropy behavior.
- `bootstrapFallback.ts` renders the locale-owned recovery action when the app
  cannot load far enough for Vue's render boundary to mount.
- `coupons.ts` centralizes client-side coupon visibility checks for expired or exhausted grants.
- `coupons.test.ts` covers coupon visibility boundaries.
- `telegram.ts` wraps Telegram Mini App integration helpers.
- `telegramFullscreen.ts` owns fullscreen request state, event synchronization, and safe-area reserve behavior.
- `telegramHaptics.ts` maps semantic click intents to Telegram's full impact vocabulary: soft dismiss/navigation/open, light copy/retry/refresh/zoom, medium actions, rigid confirmations, and heavy destructive confirmations. Unmarked buttons and links receive a guarded action/navigation fallback, selection controls remain imperative and reserved for actual value changes, and success, warning, and error remain notification outcomes.
- `telegramLinks.ts` opens Telegram-native links with a browser fallback.
- `telegramContext.ts` classifies the native `TelegramWebviewProxy` bridge, launch markers, SDK-captured `WebView.initParams`, platforms, and user agents before route construction, and skips the delayed-context wait for the public SDK placeholder in regular browsers.
- `telegram.test.ts` covers Telegram capability behavior, semantic impact mapping, non-delegated selection, and listener disposal.
- `telegramHaptics.test.ts` verifies one delegated action impact plus one bet outcome notification, older SDK behavior and safe host-call failure. `notifyBetOutcome` adds no second result-time impact.
- `validation.ts` contains shared input validation helpers.
- `validation.test.ts` covers validation boundaries.
- `latestRequest.ts` supplies generation guards so disposed or superseded async
  work cannot overwrite current Vue state or restart timers.

`turnstile.ts` loads the official explicit-render script once, exposes only documented render/reset/remove contracts, bounds loading time and allows a fresh retry after provider failure.

`adminSettingWrites.ts` saves disable toggles before clearing configuration, then keys/destinations before enabling, shared by Turnstile and PM.

- `pricingDisplay.ts`: Exact minor-unit money display through the existing currency preference and localized pricing states/reasons.
