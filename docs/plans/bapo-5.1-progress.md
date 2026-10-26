# BAPO 5.1 implementation gates

The attached BAPO 5.1 specification governs this work. Pricing is optional;
existing financial transactions and native accounting remain authoritative.

## Ordered delivery

1. Baseline and product activation: inventory existing purchase, PAYG and renewal
   boundaries; add fixed-by-default policies, bounded approved prices and audit.
2. Contracts and renewal integration: explicit owner acceptance, immutable quotes,
   notice, ceiling, eligibility and existing atomic settlement.
3. Operational safety and intelligence: memory-only node health, bounded worker,
   explainable features/models, original forecast vintages and separate actuals.
4. Administration and analysis: versioned interventions, approvals, browser-only
   product/user/operations views and scoped streaming analytical exports.
5. Delivery: retention, hosted financial/statistical/fault regressions, measured
   storage gates, browser verification, reviewed commit and successful CI.

## Baseline inventory

- `database/billing_quote.go` and `billing.go`: stock, coupon, wallet, purchase,
  add-on snapshots, idempotency and entitlement outbox in one transaction.
- `database/xnode_payg_billing.go`: exact 1 GB block charges; existing prepaid
  bytes and charge ledger are independent of future block prices.
- `database/automatic_renewal_plan.go`, `automatic_renewal_commit.go`, and
  `billing_renewal_quote.go`: reward priority, recurring coupons, stock, accounting,
  queued successor and idempotent debit remain owned by existing commerce code.
- `xnode/master_metrics.go`: authenticated live telemetry already resides in
  memory. Pricing must not copy machine metrics or raw connection IPs to SQL.
- PostgreSQL schema 16 / SQLite migration 104: forward migrations must update
  the isolated schema inventory and PostgreSQL contract without rewriting history.
- Browser console has an independent router/session/theme. Pricing administration
  belongs there; renewal policy acceptance belongs to the authenticated owner.

## Evidence

Implementation and validation evidence is recorded here as each gate is met.
Local automated test suites are prohibited. Hosted regressions are required.
Empirical calibration and production rollout require observed completed windows;
prior-only or insufficiently validated models must be identified and gated.

Current delivery and retirement gates are tracked in [compute-pool-progress.md](compute-pool-progress.md). The original draft has been extended with shared numerical execution, coupon cohorts and dual-decision admission.
