# Route views

- `ActivityView.vue` delegates to games, check-in, draws, and activity history.
- `AdminView.vue` resolves and lazy-loads the selected administrative section, including System compensation review.
- `AdminUserView.vue` is the thin aggregate-profile route entrance and owns native Back to the user list.
- `CatalogView.vue` delegates to plan browsing and checkout.
- `ConnectionsView.vue` delegates to the member connection scan and selected-IP unlink workflow.
- `CommunityView.vue` is the thin Community route view for entitlement-gated Telegram group and channel access.
- `EmbyView.vue` delegates to Emby setup and preference management.
- `EntryView.vue` selects the initial authenticated destination.
- `HomeView.vue` delegates to the member dashboard, including balance funding.
- `OnboardingView.vue` hosts the resumable intro, username, and agreement flow.
- `PaymentResultView.vue` verifies a provider-returned payment order in Telegram or polls the capability-limited status in a regular browser, returns confirmed Telegram payments to Home, and offers failed/expired Telegram orders a localized reissue handoff.
- `AffiliatesView.vue` mounts the member Affiliate Centre page.

`SettingsView.vue` is the thin route entry for user notification, display, subscription and community preferences.

`SubscriptionView.vue` is the thin route composition for Subscription & uptime, delegating fetching and display to the subscription components.
- `IPLookupView.vue` composes the member IP Lookup feature.
- `AdminIPLookupView.vue` composes the standalone administrator IP Lookup settings feature.

`DIYView.vue` is the thin authenticated `/settings/diy` member entry point.

- `AdminPricingView.vue`: Thin browser-only route entry composing the modular pricing workspace.
