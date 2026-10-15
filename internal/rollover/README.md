# Purchase rollover
- `period_remote_test.go` verifies purchase-scoped quiescence through the optional `PeriodRemote` provider contract, keeping already provisioned successors active during settlement.
- `service_part2.go` continues the focused implementation from its original package module.

- `service.go` coordinates queued purchase rollover, remote traffic quiescence, strict per-node weighted usage capture, and calculated-but-uncredited local settlement.
- `cadence.go` owns reset-window boundaries, including calendar-month and fixed rolling-month arithmetic.
- `service_test.go` verifies operation ordering and safe finalization when local or remote identities are missing.
- `service_part2.go` contains cadence and aggregate usage calculations. Daily
  usage buckets are clipped to both the term range and each reset period, so a
  non-midnight DAY or WEEK boundary cannot count one date in two periods.
- Every reset period contributes weighted used bytes and its prorated allowance.
  The strict threshold is applied once to the total purchase term; every unused
  byte is eligible when that total passes. Credit rounds to the nearest TXB cent.
  The adapter verifies the provider aggregate only to reject incomplete series;
  aggregate and current-counter fallbacks never decide rollover eligibility. `MONTH_ROLLING`
  advances by a fixed 30-day window, while `MONTH` follows calendar-month boundaries.
- `projection.go` shares the cadence evaluator for live current-term/reset-period projections, fixed-point forecast math, strict maximum thresholds, and net-paid rollover credit.
- `math.go` owns whole-term threshold and cent-rounding rules shared with settlement.
- `math_test.go` covers the 85% threshold example, cent rounding, and strict boundary.
- `weighted_usage.go` converts transient per-node usage series to fixed-point multiplier-weighted daily and total usage without persisting provider series.
- `weighted_usage_test.go` covers fixed-point per-node aggregation.
- `projection_test.go` covers net-paid savings, latest reset selection, strict thresholds, forecast eligibility, and no-cap rollover credit.
- `projection_current_counter_test.go` verifies that current counters do not
  replace strict weighted daily usage and covers cadence boundaries.
