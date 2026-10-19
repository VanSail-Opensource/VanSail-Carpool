# Member purchase operations

`combo_control_types.go`, `combo_controls.go` and `combo_control_commands.go`
serve active-term squad access and confirmed early activation. Ownership and
last-enabled validation are transactional; disabled account references carry
between terms. `combo_control_worker.go` bridges receipts to the existing
entitlement identity/reset executor, then invalidates cached dashboard usage.
Early activation forfeits remaining access without creating financial credits,
preserves the successor duration and shifts later terms and continuity jobs.

- `types.go` owns reset/refund contracts and stable reason codes.
- `eligibility.go` applies immutable reset pricing and refund eligibility rules.
- `service.go` creates idempotent durable operations without provider calls in transactions.
- `worker.go` owns shared durable phases and receipt completion.
- `automation.go` owns the account preference service boundary and deterministic purchase-plus-reset-period command key.
- `reset_worker.go` reconciles reset timestamps and requests once-only compensation.
- `refund_worker.go` quiesces, rechecks usage, restores conflicts, and commits refunds.
- `eligibility_test.go` covers cadence rounding and the strict refund boundary.

Provider calls are supplied by the application adapter and always traverse the shared Remnawave queue. Tests are added for CI and are not run locally.

`refund_state.go` selects the optional purchase-scoped provider contract for refund quotes and settlement, preserving current-period reset reconciliation. Native counters combine legacy/native usage without applying node multipliers twice.

`purchase_remote.go` selects an optional owner/purchase-scoped provider without requiring the legacy identity field. Native reset reconciliation uses ResetOperationRemote proof; legacy timestamp-only recovery requires a reset strictly after the attempt and preserves ambiguous outcomes. `worker_recovery.go` closes exhausted admission retries: never-attempted resets are compensated once, while attempted mutations remain pending_review. Refund workers recheck local eligibility before quiescence and keep unsettled usage conservative.
