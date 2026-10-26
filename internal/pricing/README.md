# Pricing intelligence

This package calculates recommendations and analytical results. It never debits
wallets, changes purchased traffic or sends an upstream request. Commerce remains
in the existing database transactions; the application owns the single worker.

## Product activation and revised controls

Every product starts fixed. An administrator explicitly saves purchase, renewal
and personal campaign modes independently. The later implementation instructions
replace user acceptance and second-person approval with automatic application
and administrator-only queued notifications. Dynamic renewal policies apply to existing and new
renewal chains automatically; completed charges and paid entitlements stay intact.

`policy.go` defines exact integer money constraints. `limits.go` derives automatic
bounds with a bounded 101-point search over 0–2 times the reference price. Among
operationally feasible prices, candidates retaining at least 95% of the best
expected contribution define the envelope. There is no claim that an observational
model proves a true causal optimum. Without validated response and original
forecast evidence, automatic changes are zero. Explicit administrator rates and
budgeted campaigns take precedence over this inferred envelope and notify admins.

## Statistical and operational modules

- `response.go`: bounded regularized logistic response; chronological holdout,
  sample/variation/bias/Brier gates. Zero-data priors are labeled, never calibrated.
- `optimizer.go`: cost recovery, commitments, capacity and publication gates;
  candidates and assumptions remain in the retained calculation trace.
- `forecast.go`: empirical quantiles, original vintage models and completed-window
  MAE, median error, WAPE, signed bias, interval coverage/width and pinball loss.
- `health.go`: authenticated mean per-core CPU, EWMA, persistence, independent
  outage evidence, freshness and recovery hysteresis. Live samples stay in memory.
- `cache.go`: bounded analytical/exposure caches, shared concurrent work,
  generation invalidation and constant-time FIFO eviction. Cached results never
  authorize financial settlement. Policy edits invalidate affected projections.
- `datasets.go`: explicit secret-free retained source registry, units, dimensions
  and retention. Export reads only these projections.
- `errors.go`: stable degraded-state codes and separate prediction/publication,
  admission, billing and entitlement impacts.

`renewal/` recommends immutable future renewal amounts and snapshots paid add-on
bases. It delegates debit, coupons/rewards, stock, idempotency and queued
successors to the existing authoritative commerce transaction.

## Integration and extensions

Database adapters are `internal/platform/database/pricing_*.go`. Browser-only
administration and owned renewal information are mounted in `internal/httpapi`.
The application worker reuses authenticated X-Node metrics and queued Telegram
notifications. Numerical execution uses the existing outbox compute lane and negotiated X-Node compute_pool_v1 protocol; upstream integrations retain their existing queues.

Extend feature definitions and the export registry together. Forecasts must use
their real creation time and immutable input hash/watermark; evaluations are
separate and may reconcile delayed observations. Numeric forecast identifiers
and nanosecond evaluation timestamps travel as exact decimal strings.

## Retention, validation and rollout

Forecast details retain 90 days; referenced feature lineage follows them. Other
unreferenced features retain 60 days, exposures/candidates 60 days, closed incidents
90 days and compact validation/calibration summaries 12 months. Financial facts
and audit follow existing application retention. Cleanup summarizes before
bounded deletion. The 700 MiB incremental / 3 GB database thresholds suspend
discretionary work; they exclude PostgreSQL WAL and backups.

`policy_test.go`, `forecast_test.go`, `health_test.go`, `cache_test.go` and
`response_test.go` cover exact bounds, quality gates, missing observations,
overload/recovery, concurrent cache work and bounded calculation benchmarks.
Database integration/streaming and representative PostgreSQL size gates run in
hosted CI. Local automated suites are prohibited. Genuine future observations
are required before production model validation; synthetic browser fixtures and
statistical regressions do not establish empirical readiness.

## Compute and decision flow

Deterministic numerical kernels live in the pinned X-Node `pkg/compute` package.
Panel adapters gather immutable snapshots and enqueue `pricing_compute` references
in the existing durable outbox. The dedicated lane calls `Master.Compute` with a
durable acceptance callback. Numerical output is validated before an immutable
receipt is stored; only then is a node acknowledged. Financial publication and
checkout remain separate master transactions with current-version checks.

The coupon-aware response uses activity, prior renewals, gross relative price,
coupon presence and effective discount fraction. Exact coupon scopes, expiry,
caps, limits and grant state version personalized decisions. Candidate economics
use revenue after the applicable coupon, including higher gross prices. Without
validated evidence, the terminal decision is unchanged. Funding failures are
censored explicitly; current balance is neither a feature nor a scheduling gate.
Attached recurring renewal terms preserve their existing contractual availability
rules. One-time and live-addition coupon consumption remain atomic with debit.

Future-dated checkout requires current renewal and selected future Combo decisions.
Fixed renewal decisions use the existing renewal plan directly, without creating
settlement contracts that would freeze legacy catalog behavior. Successful queued
checkout disables current auto-renewal in the same transaction. Failed checkout
changes no preference. Live additions remain available and invalidate the basket.

User forecasts use the last 30 completed UTC accounting days. The total model uses
only days with observations for every node represented in that snapshot; missing
days never become zero. Five complete future days are retained for each user,
and one relationship day supports per-node analysis. Historical actuals, empirical
80 percent prediction intervals, observed coverage and original-vintage calibration
are separate concepts. Calendar-day leads are stored in horizon_seconds in whole
86400-second units. A feature and its vintages publish atomically.

PAYG publication has an independent minute pass and reuses current trained models.
Each changed rate applies only to future prepaid blocks. Exact prior block prices
and credit balances remain immutable. Compute snapshots/receipts retain 24 hours
after their outbox references disappear; pending or failed references are retained.

- `candidates.go`: Pure numerical candidate preparation and selection shared with versioned kernels.

Evaluated personal total vintages retain 60 days; per-user/node one-day vintages retain 30 days. Node/product forecasts retain 90 days. Cleanup requires the daily validation summary before removal; measured calibration aggregates retain one year. This keeps five personal lead bands inside the representative storage budget rather than multiplying all per-node records by five. Financial and purchase evidence is unaffected.
