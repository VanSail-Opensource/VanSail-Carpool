# Hourly member access

`HourlyPage.vue` is the Charge by hour catalog entry. It no longer renders its
own "Charge by hour" heading (that duplicates the catalog mode switch), and its
routine refresh toolbar is removed. Background polling and confirmed-action
refreshes keep the view current; an error still offers a contextual retry.
`useHourly.ts` reads live
owner-scoped state, retains request idempotency keys across uncertain responses,
tracks the existing durable operation receipts, and refreshes confirmed changes.
An authenticated rendering snapshot can populate the page immediately, but
`liveReady` must confirm current terms before any control becomes enabled.
`busy` includes that read barrier; a small loading status explains disabled
cached controls, and an error keeps the contextual retry available. Losing
live confirmation also retires an open consent dialog rather than leaving an
enabled confirmation whose write would be silently ignored.
`hourlyHours.ts` exposes `hourlyRequiredHours(offer, fallback)`: an offer's own
`requiredHours` (`0` inherits the account-wide `activationHours`) uses the
current generated contract directly; the retired compatibility cast is removed. `HourlyPage.vue` passes that effective value to every card, so the
required-balance hint matches the squad's own eligibility hours. The helper lives
in its own module so the hosted `HourlyPage.test.ts` module mock of `useHourly`
does not erase it.

## Allowance and throttling

`HourlySummary.vue` composes the funding-suspended notice and `HourlyAllowanceGraph.vue` without
adding routine reconciliation notices. The graph delegates exact byte arithmetic
to the pure `hourlyAllowance.ts` helper and reuses `HourlyThrottleMeter.vue` for
an exhausted usable budget. These components take readonly usage/policy props;
the existing `extra` event reaches the same consent flow as Settings.

- Capacity remains `freeAllowanceBytes + extraFundedBytes`; confirmed Used remains
  `freeUsedBytes + extraUsedBytes`, clamped separately to each pool's capacity.
  Current-day `accounting.freeRecoveryUnknownBytes` and
  `accounting.extraRecoveryUnknownBytes` are unavailable and are deducted from
  the corresponding remaining pool, never added to Used. Ordinary open credit
  is still allocated for traffic, so neither `freeReservedBytes` nor
  `extraReservedBytes` is deducted wholesale. Historical accounting never
  reduces today's remaining allowance.
- Segment order: free used (`--accent`), purchased extra used (the existing green
  sine-wave decoration), usable purchased extra remaining, unavailable recovery
  credit (a neutral static pattern), and usable free remaining (`--line`).
  BigInt basis-point widths preserve exact proportions, with the trailing
  remaining segment absorbing sub-basis-point rounding. No warning prose or
  accounting-status panel is added to the member summary.
- The compact legend shows Used and one secondary row: "Purchased extra left"
  after usable free allowance is exhausted; otherwise combined Remaining. Both
  subtract only their current recovery holds and retain open allocations and
  purchased credit when automatic purchases are disabled. An older cached
  response with only aggregate recovery still has an exact combined Remaining
  and exhaustion state; the helper does not invent a pool split.
- The reduced-speed meter appears whenever both usable pools are exhausted,
  including a zero allowance before any traffic and an enabled automatic-purchase
  preference with no funded credit. It shows the exact configured rate, not a
  measured live speed. The inline controlled switch keeps its confirmed server
  value and permits opting out while reduced. A zero extra price retains the
  existing unavailable notice.
- Confirming automatic purchases changes only the preference. The meter stays
  reduced until a refreshed accounting snapshot actually restores usable credit.
  A same-day increase in funded extra from an exhausted budget then triggers the
  existing one-off green rocket, retaining the meter for ~1.2s before showing the
  purchased bar. Initial render, preference changes alone and UTC-day resets
  never trigger it. Reduced motion swaps immediately with a static flash and
  freezes packet/wave movement.

All byte math stays in `BigInt`; frontend derivation never authorizes grants,
settles a reservation, debits a wallet or changes saved accounting.

## Accounting status

The member summary does not repeat the account wallet balance or render
reserved-credit, unconfirmed-usage or approved price-ceiling prose.
All underlying accounting fields and balances remain intact;
only the member presentation is quieter. Historical-only evidence still cannot
make the summary appear. `HourlyAccountingStatus.vue` remains the browser-admin
presentation, where current/historical holds are available for inspection.
Its category, large-integer, locale and legacy fallback regressions stay intact.
Hosted hourly browser recovery checks that the notices are absent while the
constructed accounting evidence is unchanged.

## Automatic extra-traffic preference

`SettingsHourlyControls.vue` in `../settings` is the permanent home of the
preference, including when allowance is exhausted, so members can always turn it
off or review a changed rate. `HourlySummary.vue` forwards the allowance graph's
`extra` event; it has no standalone preference row. `HourlyThrottleMeter.vue`
emits the same event from its inline switch whenever both usable free and
purchased capacity are exhausted, with the confirmed preference shown on or off. Retained
purchased credit stays usable and prevents a premature reduced-speed state.

`useHourlyExtra.ts` shares consent behavior between Settings and Explore. It
uses the existing `useHourly` live-data barrier and mutation API, retains the
reviewed policy version, closes obsolete consent after a changed policy or
failed live confirmation, and preserves the displayed server preference after
a failed save. Opening or cancelling the dialog never writes. A member may
turn off existing automatic purchases even when the current extra price is zero.
Price rises expose the existing review action in Settings, without a price-ceiling
tag. Financial authorization and midnight expiry still appear in the existing
confirmation dialog; there are no new API fields or persistence.

## Fixed-term blocking (outcome 8)

When the server reports `data.legacyBlocked` (active, activating or paid queued
fixed-term purchase), `HourlyPage.vue` renders `HourlyLegacyBlock.vue` instead of
the summary, notices and squad cards. The block is a full-bleed animated
lock/portal that fills the component (`min-height: 12rem`): layered gradient
surfaces, a slowly rotating conic sweep and pulsing concentric rings behind
centred content that stays legible in both themes. Copy is intentionally limited
to `hourly.legacyTitle` plus one short `hourly.legacyBlocked` line; there is no
separate "why" copy or routine refresh button. Reduced motion keeps the gradient and
rings static without pulsing or rotation. Card and extra actions are not rendered,
and the page guards activation, disable and extra mutations while the flag is set,
so no hourly call can fire.

## Cards, stock and funding

`HourlySquadCard.vue` reuses the catalog `SquadPricingCard` presentation, so
Charge by hour squads share the Charge by traffic card geometry, profile facts,
price treatment and status slot while adding paid-through time, activation
eligibility, funding suspension and cooldown details. Enable/disable is a compact
44px icon control sitting at the right of the paid-through/eligibility line
rather than a footer row; enable is green, disable is red, the active `Enabled`
and other enrollment badges are omitted, and a blocked enable (cooldown, insufficient balance or other
ineligibility) is grey with a prohibit icon.

When `reasonCode === "capacity"` (shared hourly/fixed-term seat stock is full)
the card shows the localized refusal reason. If the member already has an
active or preparing enrollment for that squad, `capacityContinuity` makes clear
that the next paid hour continues and only new activations are refused. A
`funding_suspended` enrollment keeps the
`fundingSuspendedTitle`/`fundingSuspendedBody` notice in `HourlySummary.vue`,
telling the member to top up and re-enable the squad.

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

Explore lays out hourly squads in the same aligned desktop grid as traffic
squads, with independent title/price columns and a bottom activity/action area.
Its mode-level demonstration button opens the shared read-only billing modal
documented in `../catalog/README.md`; it does not bypass activation eligibility,
policy-version checks, required confirmations or durable operation receipts.

`HourlyPage.test.ts` supplies hosted consent, disable and legacy-admission
regressions. Extra rates and consent copy retain the exact native TXB
amount even when the account prefers another display currency. `useHourly.test.ts`
covers a late activation response after disposal: it must not restart receipt
tracking or data refresh. Copy belongs to `../../../locales/*/hourly.json`; all API methods and
decimal-string fields are declared in `../../api/hourly.ts`. The browser-admin
editor is independent in `../admin/hourly` and uses no Telegram controls.

## Audit fixture

Open `/fixtures/hourly-audit.html` with the dev server. Existing switches keep
their meaning (`admin`, `light`, `zh`, `slow`, `error`, `empty`, `paused`,
`active`, `suspended`, `cooldown`, `activation`, `incompatible`, `extra`,
`price-rise`, `no-extra`, `pending`, `conflict`, `queue`, `failed`, `first`,
`catalog`, `legacy`). New switches:

- `settings`: show the permanent hourly preference control in isolation.
- `out-of-allowance`: free allowance exhausted, with existing purchased credit.
- `allowance-zero`: zero free allowance with recorded use and retained extra.
- `credit-exhausted`: both free and purchased budgets exhausted → reduced-rate meter.
- `allowance-zero-unused`: zero free allowance, no purchased credit and no recorded
  use → reduced-rate meter immediately.
- `extra-off`: combine with `extra` to keep automatic purchases off while the
  synthetic account still carries existing purchased credit.
- `offer-hours`: per-offer required hours (24, 72, inherit 168) with matching
  required balances.
- `capacity`: all shared seats used and every card refused for `capacity`.
- `capacity-preparing`: same stock state with a preparing enrollment on the first
  squad to exercise next-hour continuity copy.
- `reduced-motion`: overrides `matchMedia` before mount so Motion Vue reports the
  reduced-motion branch.
- `current-unknown` (also `pending`): current-day recovery-unknown held credit.
- `recovery-exhausted`: a 2 GiB allowance with 115,152,132 bytes held after
  recovery and no usable credit, reproducing the former frozen 109.81 MiB label.
- `open-credit`: the same allowance and admitted traffic with that residual in
  an ordinary open allocation, which remains usable and displays 109.81 MiB.
- `historical-unknown`: retained unknown credit from other UTC days, omitted from
  member notices.
- `historical-unsealed`: retained other-day unsealed proofs and held bytes.
  Combine with `first` to verify that historical-only evidence does not make the
  member summary appear.

Use `extra` together with `out-of-allowance` to inspect the purchased-usage bar
with `extraEnabled` true, and `zh`/`light` for locale and theme parity. The
`out-of-allowance` fixture keeps existing purchased credit visible while the
preference is off. `credit-exhausted` shows the reduced-rate meter; enabling
automatic purchases there (after explicit confirmation) keeps the reduced meter
until the fixture receives an actual funded-block refresh. Only that refresh
triggers the decorative rocket and returns to the budget bar. Add `reduced-motion`
to see the immediate funded swap and static flash/lines instead.

Open activation-code, extra-price and disable dialogs retain the reviewed policy
version, offer revision or enrollment identity. A conflicting refresh closes the
dialog and asks the member to review current terms. `HourlyPage.test.ts` covers
those replacement races; server revision and policy checks still authorize writes.
`hourlyAllowance.test.ts` covers the exact recovery residual, mixed free/extra
holds, ordinary open credit, historical isolation, large integers and older
aggregate-only snapshots. `HourlyAllowanceGraph.test.ts` covers those presentation
boundaries, consent without funding, retained paid credit after opt-out, UTC-day
reset and zero budget before observed traffic.

`SettingsHourlyControls.test.ts` covers persistent discoverability, failed writes,
changed-policy consent, unavailable purchases and legacy blocking. The browser
preference scenarios live in `browser-hourly-preferences.mjs` and are included by
`browser-hourly-recovery.mjs`; they cover Settings and inline controls at the
selected desktop/mobile sizes without mutating a production account. Hosted
recovery also contrasts recovery exhaustion with an equal ordinary open credit.
The helper and graph select hourly, settings and member-design browser scopes.

`HourlyAccountingStatus.test.ts` preserves the administrator accounting-status projection and tests unknown/stale reservation detail independently of the concise member summary.
