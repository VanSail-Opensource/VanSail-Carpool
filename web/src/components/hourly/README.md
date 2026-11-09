# Hourly member access

`HourlyPage.vue` is the Charge by hour catalog entry. `useHourly.ts` reads live
owner-scoped state, retains request idempotency keys across uncertain responses,
tracks the existing durable operation receipts, and refreshes confirmed changes.
`HourlySummary.vue` presents the balance, UTC daily allowance, account speed
ceiling and unreserved extra credit. `HourlyAllowanceGraph.vue` renders the free
allowance as a progress bar with exact used/remaining byte values.
`HourlySquadCard.vue` reuses the catalog `SquadPricingCard` presentation, so
Charge by hour squads share the Charge by traffic card geometry, profile facts,
price treatment and status slot while adding paid-through time, activation
eligibility, funding suspension and cooldown details. Enable/disable is a compact
36px icon control sitting at the right of the paid-through/eligibility line
rather than a footer row; enable is green, disable is red, the active `Enabled`
badge is omitted, and a blocked enable (cooldown, insufficient balance or other
ineligibility) is grey with a prohibit icon.

A member without an enrollment or any recorded hourly usage has not charged yet;
`HourlyPage.vue` then shows only the squad cards, omitting the account summary
until the first hourly charge exists. The page is data-first: it renders labels,
values and actions without the former intro, billing prose, allowance hint or
extra-credit expiry copy.

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
editor is independent in `../admin/hourly` and uses no Telegram controls. The
dev-only `hourly-audit.html?first` fixture exercises the never-charged layout.
