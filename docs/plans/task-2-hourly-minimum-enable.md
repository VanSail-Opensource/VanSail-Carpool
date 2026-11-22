# Task 2: minimum enable hours

This handoff records the shared backend and UI contract. Delivery validation is
coordinated separately; do not run local suites or builds, push, or access live
data as part of this handoff.

## UI contract

- Keep `offer.requiredHours`, the admin write field `requiredHours`, and the
  inherited `policy.activationHours` wire names. Their existing configured
  values now mean **minimum enable hours**, with `0` still inheriting the policy.
  There is no separate balance-hours setting or data reset.
- `requiredBalance` is the exact nonrefundable amount deducted on activation:
  effective minimum hours multiplied by the accepted hourly price. Preparation
  remains unpaid; native readiness, balance debit and paid activation commit
  atomically. An insufficient balance leaves the activation unpaid.
- `enrollment.paidThrough` initially equals `activatedAt + minimum hours`.
  Disabling stops access immediately and refunds none of that prepayment.
  A later new activation buys a new minimum interval; cooldown rules remain.
- While covered by that interval, hourly workers do not debit it again. At its
  end, the next purchase covers the remainder of the UTC hour, then renewals
  return to whole UTC hours. Accepted minimum price/duration are immutable.
- The rolling 72-hour usage cost projection uses elapsed enabled time within
  recorded paid intervals, clipped to the view window and disable time. It
  excludes unused prepaid time and allocates in whole minor units using the
  difference of cumulative rounded-down costs at the window boundaries. Wallet
  history still shows the actual debit at transaction time.
- The forward migration leaves prior settlements unmarked (`minimum_hours IS
  NULL`), preserving their inclusive ledger-time attribution. New metered
  renewals store `0`; activation prepayments store the positive effective
  minimum. No existing enrollment receives a retroactive minimum debit.
- Existing endpoint/request/response shapes are retained. No additional member
  confirmation or client-side balance arithmetic is required.

## Verification and delivery

The hosted PostgreSQL regression workflow runs the usage breakdown tests for
elapsed minimum coverage, disable clipping, prefetched metered renewals,
and exact legacy ledger bounds. Hosted execution, contract generation, and
final integration remain with the coordinating agent.
