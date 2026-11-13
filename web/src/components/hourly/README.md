# Hourly member access

`HourlyPage.vue` is the Charge by hour catalog entry. It no longer renders its
own "Charge by hour" heading (that duplicates the catalog mode switch); only a
compact right-aligned refresh toolbar remains. `useHourly.ts` reads live
owner-scoped state, retains request idempotency keys across uncertain responses,
tracks the existing durable operation receipts, and refreshes confirmed changes.
`hourlyHours.ts` exposes `hourlyRequiredHours(offer, fallback)`: an offer's own
`requiredHours` (`0` inherits the account-wide `activationHours`) uses the
current generated contract directly; the retired compatibility cast is removed. `HourlyPage.vue` passes that effective value to every card, so the
required-balance hint matches the squad's own eligibility hours. The helper lives
in its own module so the hosted `HourlyPage.test.ts` module mock of `useHourly`
does not erase it.

## Allowance and throttling

`HourlySummary.vue` no longer renders the former "Combined speed after
allowance" and "Unreserved extra credit" metric rows. `HourlyAllowanceGraph.vue`
owns one segmented purchased-usage bar; every segment width is computed with
`BigInt` basis points and the trailing free-remaining segment absorbs the
sub-basis-point remainder:

- Segment order: free used (`--accent`), purchased extra used (same `--accent`
  base with 2–3 thin, smooth standout-green sine wave lines drawn as inline SVG
  paths and clipped inside it; the line group translates left→right on a
  seamless 1.8s loop, and reduced motion leaves the lines static), purchased extra remaining (the accent base at reduced
  opacity with a faint green inset wash), free remaining (`--line`). The wave
  lines and the "Purchased extra left" legend swatch use a local
  `--hourly-extra` derived from the theme-aware `--success` mixed with a vivid
  green, so it stays distinct from `--warning` in both themes.
- Existing credit is retained whether automatic purchases are enabled or disabled:
  capacity is `freeAllowanceBytes + extraFundedBytes`
  and usage is `freeUsedBytes + extraUsedBytes` (clamped to capacity). The legend
  always shows used traffic and exactly one secondary row: "Purchased extra
  left" (`extraFunded - extraUsed - extraReserved`, never reserved bytes as used)
  once the free allowance is exhausted, otherwise the remaining capacity. The
  two rows are mutually exclusive. Caption and accessible `aria-valuetext` use
  `hourly.fullSpeedAllowance` and `hourly.allowanceUsedOf`.
- Turning automatic extra purchases on (false → true) while the previous state
  had exhausted both free and purchased budgets keeps the reduced-speed card mounted and flies a one-off
  decorative green rocket through it for ~1.2s (`data-hourly-boost` is on this
  card-context rocket); the graph swaps to the purchased bar only when the
  rocket finishes. It never plays on initial load or within allowance. Reduced
  motion swaps immediately and shows a static green flash instead of travel.
- Extra purchases disabled and both observed budgets exhausted
  (`freeUsedBytes >= freeAllowanceBytes` and `extraUsedBytes >= extraFundedBytes`,
  including a zero budget with zero use): the caption is omitted and `HourlyThrottleMeter.vue` replaces the whole
  bar area with a depressed cool-gray card: a sagging/flattening waveform, a slow
  dim breathing pulse and a slow dashed drift, the exact rate, and one short
  advice line (`hourly.throttledAdvice`; a purchase-unavailable
  variant when the extra price is 0). The label identifies the configured reduced
  rate and observed budget, rather than claiming a live measured connection speed. Reduced motion freezes the sad waveform
  while keeping the same information.
- Otherwise the ordinary free-allowance bar renders.

All byte math stays in `BigInt`.

## Fixed-term blocking (outcome 8)

When the server reports `data.legacyBlocked` (active, activating or paid queued
fixed-term purchase), `HourlyPage.vue` renders `HourlyLegacyBlock.vue` instead of
the summary, notices and squad cards. The block is a full-bleed animated
lock/portal that fills the component (`min-height: 12rem`): layered gradient
surfaces, a slowly rotating conic sweep and pulsing concentric rings behind
centred content that stays legible in both themes. Copy is intentionally limited
to `hourly.legacyTitle` plus one short `hourly.legacyBlocked` line; there is no
separate "why" copy and no in-block refresh button, so the compact page toolbar
refresh remains the single refresh action. Reduced motion keeps the gradient and
rings static without pulsing or rotation. Card and extra actions are not rendered,
and the page guards activation, disable and extra mutations while the flag is set,
so no hourly call can fire.

## Cards, stock and funding

`HourlySquadCard.vue` reuses the catalog `SquadPricingCard` presentation, so
Charge by hour squads share the Charge by traffic card geometry, profile facts,
price treatment and status slot while adding paid-through time, activation
eligibility, funding suspension and cooldown details. Enable/disable is a compact
36px icon control sitting at the right of the paid-through/eligibility line
rather than a footer row; enable is green, disable is red, the active `Enabled`
badge is omitted, and a blocked enable (cooldown, insufficient balance or other
ineligibility) is grey with a prohibit icon.

When `reasonCode === "capacity"` (shared hourly/fixed-term seat stock is full)
the card shows the localized refusal reason. If the member already has an
active or preparing enrollment for that squad, `capacityContinuity` makes clear
that the next paid hour continues and only new activations are refused. A
`funding_suspended` enrollment keeps its localized status badge and adds the
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

`HourlyPage.test.ts` supplies hosted consent, disable and legacy-admission
regressions. Approved extra rates and consent copy retain the exact native TXB
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

Use `extra` together with `out-of-allowance` to inspect the purchased-usage bar
with `extraEnabled` true, and `zh`/`light` for locale and theme parity. The
`out-of-allowance` fixture keeps existing purchased credit visible while the
preference is off. `credit-exhausted` shows the reduced-rate meter; enabling
automatic purchases there (after explicit confirmation) triggers the decorative
rocket and returns to the budget bar without inventing a funded block. Add `reduced-motion` to
see the immediate swap and static flash/lines instead.

Open activation-code, extra-price and disable dialogs retain the reviewed policy
version, offer revision or enrollment identity. A conflicting refresh closes the
dialog and asks the member to review current terms. `HourlyPage.test.ts` covers
those replacement races; server revision and policy checks still authorize writes.
`HourlyAllowanceGraph.test.ts` covers usable purchased/reserved credit with
automatic purchases disabled and zero free budget before any observed traffic.
