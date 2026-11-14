# Layout components

Language and currency controls use the flat settings-row variant when labelled.
Members without combo access and without a positive balance see the compact
unlabelled strip on Home (`HomePreferences.vue`) instead of a Settings page.
Add squads is available in Settings; the desktop Home submenu omits its former
direct action. Existing Home query links redirect to the Settings action.

- `AppShell.vue` is the single route-motion entrance. It uses keyed `AnimatePresence` with 14px local offsets, no initial hydration animation, and a reduced-motion opacity-only path.
- `MobileNavigation.vue` owns shared `layoutId` active markers and short selection feedback. The independent browser console owns administrator navigation.
- `usePageTransition.ts` computes forward/backward direction from navigation order and tracks the wide viewport axis without blocking router resolution on animation callbacks.

- `AppShell.vue` owns the fullscreen safe-area shell, route-change scroll reset, content focus, Telegram back navigation, and the resizable Nuxt UI `UDashboardSidebar` desktop shell. It derives the shared new-user state from `useCommunityAccess` (no active fixed-term combo, no positive balance) and treats administrators as having access so their Settings console entrance stays reachable. The native Telegram Settings button follows the same `!isNewUser` visibility getter, so it is hidden for new users and restored after access arrives. Phones keep their centered fixed bottom navigation, while desktop exposes the complete member and administrator route hierarchy without a fold control or product wordmark; its Home parent is a real route link while its quick actions remain expanded children.
- Route focus restoration and Telegram BackButton callbacks are guarded against WebView teardown and rejected promises. The native BackButton uses a shared owner stack so an open payment sheet closes before route history changes.
- `LanguageControl.vue` provides the compact `ULocaleSelect` language control. `CurrencyControl.vue` owns the member-only TXB, CNY, and USD popover. Its fixed, viewport-bounded menu aligns to the trigger's end edge so asynchronous error text wraps without changing the overlay width. Both selectors appear as labelled Settings rows for members with access and as the compact Home strip for new users.
- `MobileNavigation.vue` maps the exact primary routes to a floating Nuxt UI tab strip with localized labels, native keyboard navigation and safe-area spacing. It receives the new-user flag from the shell, so the tab strip drops Settings until access exists. Administrators use the same member navigation; their one-use console entrance remains in Settings. Secondary member routes intentionally leave every primary tab unselected rather than falsely selecting Home.
- Desktop omits member identity and usernames. `AppShell.vue` instead places compact DIY Center and administrator-only Admin access shortcuts in the sidebar footer, reusing the Settings navigation row. New members without access do not receive these shortcuts. Mobile retains them in Settings; the resizable sidebar starts at a 14rem minimum so the English Knowledgebase label fits and can expand to 20rem.
- `navigation.ts` owns the localized member mobile and hierarchical desktop sidebar item definitions. The Home submenu's Subscription, Connections and Revoke entries render only when the account has a positive balance or a server-confirmed active combo; Home itself remains available. New users (no combo access, no positive balance, not an administrator) keep Home and Explore as their mobile primary tabs. Their desktop sidebar also includes Around TX with Knowledgebase, which stays available to every member. Settings, Activity and optional Around TX tools retain their access gates. Browser administrator navigation is independent.
- `AppShell.test.ts` verifies focus restoration, Telegram BackButton behavior and the shared member navigation for administrators.
- `navigation.test.ts` checks the new-user gating directly: Home and Explore remain the primary tabs, desktop Knowledgebase stays available, and members with access keep Settings.
- `useNavigationExpansion.ts` supplies controlled expansion for the member's
  vertical Nuxt menu. Every item uses a stable value rather than its array index.
  Around TX starts open because Knowledgebase is always available. Home quick
  actions open when access arrives; subsequent refreshes, localization and menu
  insertions preserve explicit user collapses. Removed groups lose their open
  state. `useNavigationExpansion.test.ts` covers asynchronous arrival, retained
  manual choices and eligibility removal/re-entry in hosted CI.

Route and native Back actions use soft navigation feedback; locale feedback is emitted only after the locale actually changes.

Settings holds the labelled language/currency controls for members with access
and stays in their desktop and mobile navigation; new users get the compact Home
strip instead. Activity and optional Around TX tools require opted-in preferences
plus current combo access. Knowledgebase is always available from Around TX and
has a separate Home introduction for new users.

For members with access, enabled IP Lookup appears in Around TX regardless of combo access or the optional entrance preference. Other optional Around TX tools retain their existing gates. Availability is account-scoped and refreshed after navigation/admin saves.

AppShell mounts its first permitted content with a 500ms, 8px entrance; navigation remains outside the transformed group. Reduced motion uses immediate final states. Ordinary route transitions retain their existing lifecycle.
