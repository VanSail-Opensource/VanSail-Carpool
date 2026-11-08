# Hourly administration

`HourlyAdminPage.vue` loads the encrypted-policy projection, separate offers and
existing X-Node squads. `HourlyPolicyForm.vue` uses Nuxt UI Form with Zod for
explicit byte, byte/second, second, hour and TXB units. `HourlyOfferForm.vue`
configures one squad's independent hourly price without changing legacy modes.

Policy writes carry the exact loaded ciphertext-fingerprint version; offer
writes carry the exact revision, with zero creating an unconfigured offer.
Conflicts retain drafts and require a deliberate refresh/review. The API server
remains authoritative for money bounds, activation eligibility and concurrency.
New-sales switches do not cancel paid access or stop existing hourly renewals.

The browser router exposes `/admin/hourly`; navigation and theme follow the
standalone console. The responsive forms share native Nuxt UI input and keyboard
behavior, with no Telegram SDK, host buttons or haptic calls. The dev-only
`../../../../fixtures/hourly-audit.html?admin` fixture exercises both themes,
locales, loading, empty, error and configuration-conflict states without spending.
