# Activity module

## Ownership and interfaces

Activity owns administrator-defined betting games, one daily check-in per configured local day, subscription-gated group-message rewards, weighted lucky draws, immutable outcomes, and pending subscription-extension credits. Member operations are `GET /api/v1/activity` plus the idempotent `POST /api/v1/activity/check-ins`, `/bets`, and `/draws` actions. Administrator operations manage Activity settings, games, and complete lucky-draw prize lists.

Member draw previews expose only IDs and names of prizes with available stock. Draw receipts and recent results include the selected draw ID, prize ID, and prize name; presentation never selects the prize.

The HTTP layer accepts TXB as decimal-string minor units and basis-point integers. The domain and database use `int64`; no Activity calculation uses binary floating point. The `Idempotency-Key` header is required for a bet or draw and is scoped to the member action.

## Financial and randomness invariants

- A game snapshots its name, icon, win chance, stake bounds, and total-return multiplier into every immutable result.
- The stake is debited before the injected cryptographic random source selects an integer in `[0,10000)`. A win credits `floor(stakeMinor * returnMultiplierBps / 10000)` as total return.
- A check-in key is the member plus local `YYYY-MM-DD` in the administrator-configured IANA timezone. The first claim draws once from the cryptographic source inside the inclusive configured minimum/maximum and persists that amount; replays return it without another random draw or ledger credit. Reward amounts are edited only through Settings.
- A lucky draw validates its total positive weight and available stock in the same write transaction that charges the fee and applies its reward.
- Before drawing, available balance must cover the fee plus the largest possible negative TXB prize. This prevents the random result from producing unapproved debt.
- Prize effects are a closed `Reward` union: `none`, signed `txb_delta`, `coupon_grant`, or positive `subscription_extension`. The selected definition is snapshotted in the result.
- A subscription extension moves an active term's expiry and every queued term by the same number of days, and queues one X-Node desired-state synchronization in the same transaction. When no term exists, a durable extension credit is consumed by the next purchase without an upstream call until activation.

## Group-message rewards

`activity.group_message_threshold` and `activity.group_message_reward_txb` are human-configured settings; either zero disables the feature. Telegram processing accepts only human messages in the configured group. The local user must have an active or activating purchase whose validity contains the message time. The message's Telegram chat/message pair is the deduplication key, and bounded event records are pruned after 31 days.

The counter is stored per user and local calendar date in `activity.timezone`. At the threshold, the counter update, one positive `activity_group_message_reward` ledger entry, and balance update commit in one transaction. Replayed or concurrent Telegram deliveries cannot increment twice or issue a second reward. `GET /api/v1/activity` returns enabled state, local date, count, threshold, reward amount, and claimed state.

Every debit and credit is committed with its ledger entry and a semantic reference derived from the immutable result. Failed validation, insufficient funds, exhausted stock, or reward application failure rolls back the result, stock, balance, and ledger together.

## Product and safety behavior

The member interface presents probability and cost plainly, renders only whitelisted Phosphor game icon keys, shows each result in a focus-trapped dialog, and never uses streak pressure, near-miss, loss-chasing copy, or persistent celebratory loops. A successful bet may show one short fireworks burst; losses, check-ins, and draws do not. All result states respect reduced motion. Disabled games and draws cannot be invoked through a stale API request.

Administrators may hard-delete games and draws when no protected processing work is running. Definitions, participation/results, prizes/snapshots, feature-linked ledger rows, and non-processing jobs are removed transactionally; already-settled balances, coupons, and extensions are not reversed. Statistics expose date-filtered daily/weekly participation, stake/fee, payout/reward, house-net, and win/loss or prize distributions.

## Verification

- Table-driven tests inject deterministic randomness for loss, win, weight boundaries, and invalid upper bounds.
- SQLite tests cover insufficient funds, fee-plus-deduction coverage, stock races, idempotent replays, ledger atomicity, and extension application.
- Calendar tests cover the configured timezone at UTC/local day boundaries.
- Vue tests cover human-major TXB conversion, loading/error/empty states, visible cost and odds, bet feedback boundaries, reduced motion, keyboard use, and narrow layouts.

- Daily check-in requires queued live verification of active personal boosts to the configured supergroup. Group message progress additionally requires positive authenticated `sender_boost_count` metadata and the existing active subscription. Rewards scale the sampled base by max(1, boost count / 2) for positive counts, half up to a TXB hundredth; ledger results and duplicate protection remain authoritative. Removal pauses progress without invalidating previously eligible messages. The activity page replaces both reward blocks with one boost-information panel when ineligible or unavailable; other activities retain their behavior.

## Boost reward validation (2026-10-07)

- Chrome DevTools MCP 1.10.1 exercised the real activity components using `web/fixtures/activity-boost-audit.ts`, constructed API responses, and an isolated headless Chrome context. Mobile used a 390 × 844 touch viewport; desktop used 1440 × 1000. Both layouts had document width equal to viewport width, with a two-column activity grid on desktop and 44 px action buttons on mobile.
- Browser inspection covered initial loading, empty games/draws, zero boosts, unavailable verification, counts 1–4, active message progress, disabled and already-paid message rewards, check-in busy/claimed states, boost-link navigation, refresh disabling/recovery, refusal after boost removal, and English/Chinese copy. A three-boost constructed receipt displayed 1.88 TXB from a 1.25 TXB base; refusal retained the balance and hid both reward controls.
- Final console inspection found no warnings or errors. Network inspection covered 890 requests with no failed or HTTP 4xx/5xx responses. Screenshots are local evidence under `D:/Remna-User-Panel/.tmp/group-boost-browser/`; the MCP transcript is under the isolated task worktree's `.tmp/` directory.
- Local validation uses structure/i18n review, ESLint, Vue type-checking, API generation, Go vet and formatting inspection. Automated regression suites run only in hosted CI. These browser checks use constructed data and do not establish live Telegram permissions or credit production balances.
