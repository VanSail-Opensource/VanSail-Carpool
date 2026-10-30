# Session components

- `AuthGate.vue` explains Telegram authentication failures, offers a light-feedback retry, and uses a local Motion status entrance without delaying the session handoff.
- `LoadingScreen.vue` presents outlined-to-solid lettering and a single Chinese-to-English handoff in a reserved word frame. Playback cancels on reduced motion without replay. Decorative glyphs are hidden from assistive technology; the readable identity and one localized live status remain available.
- `SessionEntrance.vue` accepts `status`, `authenticated`, `routeReady`, and a default page slot. It releases ready permitted routes and recovery gates immediately while an inert, noninteractive cover fades for 160ms. Initial route resolution prevents an empty shell or unapproved destination from mounting. Its page wrapper never transforms fixed navigation; AppShell owns the separate 500ms, 8px content entrance.
- `useLoadingSequence.ts` derives cover visibility from real readiness and keeps authenticated content mounted during refresh. There is no minimum introduction duration or authentication/navigation logic.
- `SessionEntrance.test.ts` checks immediate success/error access, draft retention, slow status replacement, and timer disposal.
- `BrowserCapabilityGate.vue` gives unsupported Telegram WebViews a localized recovery screen before any protected action can throw, with an opacity-only entrance.
- `AppErrorBoundary.vue` catches descendant render failures and provides a
  localized full-app reload path with retry feedback and without exposing exception details.

The wordmark and handoff rules live in `../../styles/session-01.css`; the shared indeterminate track lives in `../../styles/loading.css`. Below 768px the prefix sits above the changing word; larger viewports use one baseline. `usePendingStatus` replaces the same status after five seconds without delaying readiness. Brand/status strings live under `auth.loading*` in both core locale files.

`CaptchaGate.vue` renders the first-entry challenge before route content, with loading, expired, provider failure and retry states. Only the server verification response updates the session. `LanguageControl` remains accessible before solving.
