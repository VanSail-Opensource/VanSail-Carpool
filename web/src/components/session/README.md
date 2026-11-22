# Session components

- `AuthGate.vue` explains Telegram authentication failures, offers a light-feedback retry, and uses a local Motion status entrance without delaying the session handoff.
- `AuthGate.test.ts` verifies that verified partial-cleanup recovery uses a
  localized review heading and preserves retry without claiming failed identity.
- `MemberSessionStatus.vue` renders the shell's in-flow `role="status"` while verification or the first permitted route is pending. It has no artwork, timeouts, or transition.
- `BrowserCapabilityGate.vue` gives unsupported Telegram WebViews a localized recovery screen before any protected action can throw, with an opacity-only entrance.
- `AppErrorBoundary.vue` catches descendant render failures and provides a
  localized full-app reload path with retry feedback and without exposing exception details.

The pending status uses `auth.loadingAccount` and `auth.loadingPage` from both core locales. SDK and entry failures remain recoverable through the native reload action.

`CaptchaGate.vue` renders the first-entry challenge before route content, with loading, expired, provider failure and retry states. Only the server verification response updates the session. `LanguageControl` remains accessible before solving.
The App keys this gate by authenticated owner. Challenge replacement and unmount
retire pending verification and provider callbacks; `CaptchaGate.test.ts` covers
both boundaries without contacting a verification service.
