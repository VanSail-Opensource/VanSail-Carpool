# Native accounting aggregation and retention

`database.XNodeFleetUsage` streams indexed date ranges in node/date order and
combines native reset-period rows with reconciled legacy rows using checked
integer addition. Browser-facing counters remain decimal strings in their
existing projections. Overflow fails explicitly rather than rounding totals.

Migration 087 adds query/expiry indexes without copying accounting data.
`retention_native.go` runs inside the existing backup-gated maintenance transaction:
each pass removes at most 1,000 expired service bindings and 1,000 superseded
multiplier boundaries below acknowledged report cursors. The current boundary,
future boundaries, report cursors, period participation and exact usage remain.

Purchase compaction excludes every native period reference and any unsettled
purchase lifecycle. This preserves refund evidence, unfinished journals and settled native
purchase identity while allowing existing legacy financial rollups to continue.
No live observations or duplicate upstream dataset are persisted. Hosted cases
in `retention_native_test.go` cover accounting preservation and precise aggregation.
