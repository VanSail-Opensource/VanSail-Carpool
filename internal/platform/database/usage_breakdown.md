# Owner Usage Breakdown

`Store.XNodeUsageBreakdown` is a read-only repeatable-read projection. It checks
the current active owner and reads all inputs in one PostgreSQL snapshot; no
wallet, journal, native counter, history floor or upstream call changes on GET.
The member service supplies the rolling 72-hour window. Existing administrator
and Charge by traffic history APIs keep their separate contracts.

Accepted raw combo and Charge by traffic minute samples form physical-node
entries. Hourly samples form separate named `hourlySquads`, grouped across all
their retained enrollments and physical nodes. Every hourly row matches its
enrollment reference, owner, generation, principal and period before grouping.
Hourly history and receipts match the current account generation.
Legacy raw samples retain their immutable purchase attribution, including unknown
legacy generations. The current account creation time bounds the lifecycle.
Known raw observation bounds preserve the first lifecycle minute. Old PAYG minute
buckets lack a generation/observation split, so the first partial lifecycle bucket
is omitted. The common coverage floor advances to the next minute and remains
incomplete for windows containing that gap; no synthetic gap count is added.

Exact PAYG charges aggregate independently by their retained account/node before
joining traffic, preserving cost-only and retired nodes without receipt fan-out.
Hourly settlement receipts aggregate independently by enrollment's squad before
joining traffic. Idle paid squads, closed enrollments and deleted/disabled native
groups remain visible; absent names fall back to the public group ID. Settlement
costs never depend on current offers, rates or traffic allocation. Shared paid
extra-GB receipts remain explicitly unallocated. Monetary windows use exact
ledger timestamps, independent of traffic bucket rounding.
Retained canonical UTC TEXT timestamps are normalized for comparison to nine
fractional digits with C collation; PostgreSQL timestamp casts never round their
nanoseconds. Typed native observations use a microsecond ceiling for inclusive
lower bounds and a microsecond floor for inclusive upper bounds. These rules
also apply to lifecycle admission and closed-period coverage overlap.

The current legacy purchase estimate is its cumulative `charged_txb_minor`
multiplied by 259,200 seconds and divided by its immutable original
`purchased_term_seconds`. Cumulative charges already include paid addon changes;
addon rows are not summed again. Catalog edits, free extensions and current price
never change the denominator. Historical NULL duration stays explicitly unknown,
including on zero-price old purchases; no read backfills it.

Only that current purchase's raw 72-hour node traffic allocates its estimate.
With no matching traffic, the entire estimate remains unallocated. Other-purchase
and hourly traffic still contribute to traffic totals but not this allocation.
All sums use PostgreSQL numeric and Go big integers. Reduced nonnegative rational
minor amounts have decimal-string numerators/denominators; zero is `0/1`.
`nodes + hourlySquads + omitted + unallocated = known total` exactly. An unknown legacy amount is
excluded from the known total and explicitly marked as unavailable.

All aggregation precedes the shared limit of 128 entries. Physical nodes retain
their stable C-sorted ID order and consume slots first, followed by hourly squads
in C-sorted group-ID order. Excess traffic and exact costs of either kind remain
in `omitted`. Existing `positiveNodes` remains physical-only for compatibility;
`positiveEntries` counts both positive entry kinds before the shared limit and
before frontend Others grouping. Unallocated money is not a named entry.
Truncation is explicit and prevents a complete-coverage claim. Forward recording
floor and relevant-node loss events remain truthful. Closed combo/PAYG periods
overlapping the current lifecycle/window retain coverage-only loss evidence;
closing a period cannot manufacture complete history. Hourly participants use
the same period-overlap bounds as Combo/PAYG, and hourly raw coverage references
match the same immutable enrollment tuple as display aggregation. Loss-only
closed hourly participants remain relevant; old unrelated periods and malformed
foreign references cannot import shared-node gaps. No history is invented.

`usage_breakdown_hourly.go` owns independent hourly traffic/cost SQL and bounded
response selection; `usage_breakdown_hourly_totals.go` merges exact integer costs
with existing rational estimates/omissions. It does not change the shared-extra
or legacy-unallocated amount. The existing repeatable-read transaction encloses
every query. Group/enrollment/ledger keys are unique, and no minute row is joined
to settlement rows before each bank has aggregated, preventing fan-out.

Hosted regressions cover multi-node/multi-enrollment aggregation, idle and ended
squads, catalog-price independence, lifecycle/owner/principal/period isolation,
values beyond JavaScript's integer precision, shared caps, mixed fractional
omissions, source immutability and retained coverage-only loss. No new table,
column, migration, upstream call or persistent presentation cache is introduced.
`usage_breakdown_time_bounds_test.go` covers nanosecond receipt/expiry boundaries,
typed observation bounds and lifecycle isolation without rewriting stored facts.
