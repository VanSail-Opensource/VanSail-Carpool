# Renewal recommendations and immutable settlement windows

`eligibility.go` defines append-only system renewal-basis snapshots. Product-level
activation automatically applies to future renewals, including existing chains.
There is no owner acceptance endpoint, terms checkbox or administrator approval
stage. Snapshot limits come from calculated policy bounds, not user-entered terms.

`price.go` retains the candidate receipt shape and historical analytical fields.
The unused individual-response recommendation function and its input type are
removed. The database adapter selects the public product price within the
purchased contract's ceiling, change limit and paid add-on basis. Member behavior
and predicted renewal probability cannot choose an individual renewal price.

`quote.go` retains an immutable amount, input versions, validity window, model,
health and calculation trace. Notice is queued and recorded independently;
notification delivery does not block settlement. Scheduled preparation starts
within seven days; an explicit early renewal can prepare its quote on demand.

The existing commerce transaction still owns wallet debit, coupons and rewards.
Previously reserved campaign obligations retain their exact settlement: a
multi-term batch consumes reserved budget for every term. Successors recover
the public basis so a historical one-term discount cannot become permanent.

Health failures preserve the amount and permit bounded retries for 72 hours.
Deferral never extends access. Owner information is read-only and isolated from
browser administrator sessions. `quote_test.go` checks immutable windows,
purchase identity, calculated bounds, paid add-on minimums, malformed stored
contracts and stale versions. Existing database retirement regressions cover
public pricing and exact settlement of historical reserved discounts.
