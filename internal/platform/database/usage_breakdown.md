# Owner Usage Breakdown

`Store.XNodeUsageBreakdown` is a read-only repeatable-read projection. It checks
the current active owner and reads all inputs in one PostgreSQL snapshot; no
wallet, journal, native counter, history floor or upstream call changes on GET.
The member service supplies the rolling 72-hour window. Existing administrator
and Charge by traffic history APIs keep their separate contracts.

Accepted raw combo and hourly minute samples combine with raw Charge by traffic
minute samples. Hourly history and receipts match the current account generation.
Legacy raw samples retain their immutable purchase attribution, including unknown
legacy generations. The current account creation time bounds the lifecycle.
Known raw observation bounds preserve the first lifecycle minute. Old PAYG minute
buckets lack a generation/observation split, so the first partial lifecycle bucket
is omitted. The common coverage floor advances to the next minute and remains
incomplete for windows containing that gap; no synthetic gap count is added.

Exact PAYG charges aggregate independently by their retained account/node before
joining traffic, preserving cost-only and retired nodes without receipt fan-out.
Hourly settlement and paid-extra GB receipts have no node attribution and remain
explicitly unallocated. Monetary windows use exact ledger timestamps, independent
of traffic bucket rounding.

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
`nodes + omitted + unallocated = known total` exactly. An unknown legacy amount is
excluded from the known total and explicitly marked as unavailable.

Node aggregation precedes the stable-ID limit of 128. Excess traffic and node
cost remain in `omitted`; positive physical-node counts are computed before that
limit and before frontend Others grouping. Unallocated money is not a node.
Truncation is explicit and prevents a complete-coverage claim. Forward recording
floor and relevant-node loss events remain truthful. Closed combo/PAYG periods
overlapping the current lifecycle/window retain coverage-only loss evidence;
closing a period cannot manufacture complete history. No history is invented.
