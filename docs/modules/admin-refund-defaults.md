# Administrator refund defaults

Administrator entitlement refunds now expose a live GET quote on the existing
refund path. The quote includes net paid, allocated and observed used traffic,
the suggested refund, calculation time and nullable unavailable reason.
The POST command and member refund rules retain their existing contracts.

The default is `roundHalfUp(netPaidMinor * max(allocated-used,0) / allocated)`.
It reuses rollover's whole-term cadence allowance and nearest-cent helper, with
the purchase's effective traffic and reset overrides. Observed multiplier-weighted
traffic comes from native X-Node accounting; current-reset counters, forecasts
and rollover thresholds do not set the amount. Started terms read by panel owner
and exact purchase ID.
This includes reset periods and reconciled imported history without relying on
the user's legacy proxy-ID field or selecting an ambiguous date-range match.
The read does not quiesce access or make an upstream call.

Queued terms use zero observed usage and the full net debit. Started terms require
a native identity and verified purchase accounting; missing data or accounting
errors produce no suggestion. The amount stays editable within the authoritative
net debit cap. Zero defaults cannot be submitted without a positive manual amount.
The existing required reason, idempotency, ledger credit and queued sync remain
the settlement boundary; a quote does not automatically issue a refund.

`useAdminRefundQuote` guards requests by user, entitlement, open state and request
version. Manual edits survive a late response or retry. Changing the entitlement
clears the amount and reason; closing or disposing the dialog rejects old results.

## Browser evidence, 2026-10-07

Chrome DevTools MCP loaded the real refund dialog with constructed API data at
390 × 844 and 1280 × 900. A 100 TXB net debit with 25 GB used from a 100 GB term
prefilled 75.00 TXB. Editing to 60.00 submitted `amountTxbMinor: "6000"`.
Zero suggestions remained disabled after entering a reason. An unavailable quote
left the amount empty with a retry notice. A manual 55.00 edit survived a delayed
75.00 quote; changing the target loaded its separate 40.00 default.
Both viewports had no horizontal overflow and no console warnings/errors.
These observations use mocks and do not establish live provider availability.

Hosted regressions cover discounts, custom limits, multiple DAY/WEEK periods,
half-cent rounding, exhausted/overused allowance, free/queued terms, incomplete
usage, outages, ownership and stale UI responses. Existing database refund-bound
and replay coverage continues to guard manual overrides and duplicate submission.
No local automated test suite was run.
