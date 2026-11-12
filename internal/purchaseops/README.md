# Member purchase operations

`combo_control_types.go`, `combo_controls.go` and `combo_control_commands.go`
serve active-term squad access and confirmed early activation. Ownership and
last-enabled validation are transactional; disabled account references carry
between terms. `combo_control_worker.go` bridges receipts to the existing
entitlement identity/reset executor, then invalidates cached dashboard usage.
Early activation forfeits remaining access without creating financial credits,
preserves the successor duration and shifts later terms and continuity jobs.

- `types.go` owns refund contracts and historical reset identifiers and stable reason codes.
- `eligibility.go` applies refund eligibility rules.
- `service.go` creates idempotent durable operations without provider calls in transactions.
- `worker.go` owns shared durable phases and receipt completion.
- `legacy_reset.go` recognizes historical scanner keys for settlement only.
- `reset_worker.go` reconciles reset timestamps and requests once-only compensation.
- `refund_worker.go` quiesces, rechecks usage, restores conflicts, and commits refunds.
- `eligibility_test.go` covers the strict refund boundary.

Provider calls are supplied by the application adapter and always traverse the shared X-Node queue. Tests are added for CI and are not run locally.

`refund_state.go` selects the optional purchase-scoped provider contract for refund quotes and settlement, preserving current-period reset reconciliation. Native counters combine legacy/native usage without applying node multipliers twice.

`purchase_remote.go` selects an optional owner/purchase-scoped provider without requiring the legacy identity field. Native reset reconciliation uses ResetOperationRemote proof; legacy timestamp-only recovery requires a reset strictly after the attempt and preserves ambiguous outcomes. `worker_recovery.go` closes exhausted admission retries: never-attempted resets are compensated once, while attempted mutations remain pending_review. Refund workers recheck local eligibility before quiescence and keep unsettled usage conservative.
# Immediate squad preferences

`SwitchSquad` commits the saved choice and a completed idempotency receipt in one
transaction. It creates no provider-operation outbox job and waits for no node.
Ownership, current validity, conflicts and the last enabled squad remain checked
under the owner balance lock. Existing compile revision triggers wake normal
native reconciliation; outbound commands retain the per-node execution queue.
Legacy queued switch receipts remain supported by `ComboControlWorker`.

Manual paid resets and automatic reset admission are retired. Only the existing
worker may finish a reset with an owner-matching, immutable historical debit.
Unpaid queued reset receipts fail without native mutation; uncertain attempted
commands remain under review. Refund quotes, purchased scheduled quota cadence,
initial period setup and renewal accounting remain separate. No new preference,
quote or reset-debit producer remains.

Terminal combo-control items recover their recorded status, provider reference,
error and result before consulting mutable purchases or invoking synchronization
again. A successful quiesce request authorizes no refund until the final
purchase-scoped state confirms closure; unconfirmed closure remains under review.

- `combo_control_recovery_test.go` covers all terminal outcomes and recovery after a receipt-write failure without another synchronization.
- `refund_quiescence_test.go` covers confirmed closure, ambiguous admission, usage restoration and unknown settlement outcomes.
- `refund_quiescence_test_helpers_test.go` supplies focused durable-receipt and provider-state fixtures for those regressions.
