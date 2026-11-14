# Route views

- `ActivityView.vue` delegates to games, check-in, draws, and activity history.
- `AdminView.vue` resolves and lazy-loads the selected administrative section.
- `AdminUserView.vue` is the thin browser aggregate-profile route entrance and returns to the user list.
- `CatalogView.vue` delegates to Charge by traffic and Charge by hour. Existing fixed-term renewal remains in Home benefits.
- `ConnectionsView.vue` delegates to the member connection scan and selected-IP unlink workflow.
- `CommunityView.vue` is the thin Community route view for entitlement-gated Telegram group and channel access.
- `EmbyView.vue` delegates to Emby setup and preference management.
- `EntryView.vue` is the empty initial view whose destination is selected by the member route guard.
- `HomeView.vue` delegates to the member dashboard, including balance funding.
- `OnboardingView.vue` hosts the resumable intro and agreement flow.
- `PaymentResultView.vue` verifies a provider-returned payment order in Telegram or polls the capability-limited status in a regular browser, returns confirmed Telegram payments to Home, and offers failed/expired Telegram orders a localized reissue handoff.
- `AffiliatesView.vue` mounts the member Affiliate Centre page.

`SettingsView.vue` is the thin route entry for user notification, display, subscription and community preferences.

`SubscriptionView.vue` is the thin route composition for Subscription, delegating fetching and display to the subscription components.
- `IPLookupView.vue` composes the member IP Lookup feature.
- `AdminIPLookupView.vue` composes the standalone administrator IP Lookup settings feature.

`DIYView.vue` is the thin authenticated `/settings/diy` member entry point.

- `AdminPricingView.vue`: Thin browser-only route entry composing the modular pricing workspace.

`AdminUserView.test.ts` verifies that changing the member route replaces the profile editor and its dialogs, keeping pending drafts scoped to one owner.

## Member visual consistency

Every member route uses the shared graphite/cyan tokens and member font stack. Home retains its balance-first hierarchy; Explore owns its billing-mode selector and tutorials; Subscription, Connections, Activity, Community, Emby, Affiliate Centre, IP Lookup, Settings and DIY share restrained heading geometry. Onboarding, session entry and payment returns inherit the same base controls and status colours. Long data values wrap within shrinkable tracks. Community and IP Lookup leave the main landmark to AppShell; Settings and the one-use admin-access entrance reserve the shell navigation clearance on phones. Hosted PR-specific `member-design` scenarios capture real page components with synthetic data at desktop/mobile sizes.
