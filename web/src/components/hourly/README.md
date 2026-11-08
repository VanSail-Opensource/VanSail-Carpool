# Hourly member access

`HourlyPage.vue` is the Charge by hour catalog entry. `useHourly.ts` reads live
owner-scoped state, retains request idempotency keys across uncertain responses,
tracks the existing durable operation receipts, and refreshes confirmed changes.
`HourlySummary.vue` presents the UTC daily allowance, account speed ceiling and
unreserved extra credit. `HourlySquadCard.vue` presents each independent offer,
paid-through time, activation eligibility, funding suspension and cooldown.

Activation never debits the displayed eligibility balance. The server owns exact
proration, stock, activation codes, wallet locking and native application. The
existing catalog `SquadActivationDialog.vue` collects a required squad code.
Voluntary disable and automatic extra purchases require the existing confirmation
dialog; the latter shows the accepted decimal-GB price and midnight expiry.
Pending receipts remain visible and block concurrent mutations. No financial
state is persisted in browser storage and no client-side estimate authorizes use.

`HourlyPage.test.ts` supplies hosted consent, disable and legacy-admission
regressions. Copy belongs to `../../../locales/*/hourly.json`; all API methods and
decimal-string fields are declared in `../../api/hourly.ts`. The browser-admin
editor is independent in `../admin/hourly` and uses no Telegram controls.
