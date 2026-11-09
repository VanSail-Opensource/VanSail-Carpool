# Hourly administration

`HourlyAdminPage.vue` loads the encrypted-policy projection, separate offers and
existing X-Node squads. `HourlyPolicyForm.vue` uses Nuxt UI Form with Zod and
accepts human transfer units: sizes such as `1GB`/`500MiB` become exact bytes and
speeds such as `100Mbps`/`12.5MB/s` become exact bytes per second through
`../../../utils/transferUnits.ts`; plain integers remain exact bytes or bytes per
second. Existing values are shown in a round-trippable unit form.
`HourlyOfferForm.vue` configures one squad's independent hourly price and
per-offer required-balance hours; it writes `requiredHours` as an integer from
1 through 2,562,047 and shows the effective hours plus the resulting required
balance. Loaded offers with `requiredHours: 0` display the account activation
threshold, matching the server's effective-hours rule. The offers list excludes
pay-as-you-go squads (`billingMode: "payg"`), because Charge by traffic sells
independently per GB and never participates in hourly access.

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
