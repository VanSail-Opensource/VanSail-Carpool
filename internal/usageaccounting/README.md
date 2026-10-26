# Usage accounting

`CalculateUsage` derives a purchase allowance from its reset cadence and exact
native term usage. `CadenceAdvance` is shared by node quota scheduling.
`RefundMinor` rounds an administrator refund suggestion using integer arithmetic.
Native purchase periods remain the authoritative source; incomplete accounting
must remain retryable. X-Node adapters supply snapshots to refunds and statistics.

- `cadence.go`: Period boundaries and cadence calculations.

- `refund.go`: Purchase-wide refund evidence aggregation.

- `types.go`: Shared usage and purchase snapshot contracts.

- `usage.go`: Exact native and reconciled historical usage calculation.
