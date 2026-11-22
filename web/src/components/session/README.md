# Session components

- `AuthGate.vue` explains Telegram authentication failures, offers a light-feedback retry, and uses a local Motion status entrance without delaying the session handoff.
- `AuthGate.test.ts` verifies that verified partial-cleanup recovery uses a
  localized review heading and preserves retry without claiming failed identity.
- `LoadingScreen.vue` uses the static route artwork from the pre-mount screen, shows the product name, and keeps one localized live status. The illustration is decorative and removed in forced colors.
- `SessionEntrance.vue` accepts `status`, `authenticated`, `routeReady`, and a default page slot. It releases ready permitted routes and recovery gates immediately. Only a successful handoff can fade out the inert cover for 160ms; reduced motion skips that fade. Initial route resolution prevents an empty shell or unapproved destination from mounting. Its page wrapper never transforms fixed navigation; AppShell owns the separate content entrance.
- `useLoadingSequence.ts` derives cover visibility from real readiness and keeps authenticated content mounted during refresh. There is no minimum display duration or authentication/navigation logic.
- `SessionEntrance.test.ts` checks immediate success/error access, retry, draft retention, slow status replacement, and timer disposal.
- `BrowserCapabilityGate.vue` gives unsupported Telegram WebViews a localized recovery screen before any protected action can throw, with an opacity-only entrance.
- `AppErrorBoundary.vue` catches descendant render failures and provides a
  localized full-app reload path with retry feedback and without exposing exception details.

`member-startup.css` is shared by the entry module and `LoadingScreen.vue`; `session-01.css` owns the Vue cover handoff. `usePendingStatus` replaces the same status after five seconds without delaying readiness. The product name comes from `app.name`, and the account messages come from `auth.loading*` in both core locales.

`CaptchaGate.vue` renders the first-entry challenge before route content, with loading, expired, provider failure and retry states. Only the server verification response updates the session. `LanguageControl` remains accessible before solving.
The App keys this gate by authenticated owner. Challenge replacement and unmount
retire pending verification and provider callbacks; `CaptchaGate.test.ts` covers
both boundaries without contacting a verification service.
