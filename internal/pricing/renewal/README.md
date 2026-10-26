# Renewal recommendations and immutable settlement windows

`eligibility.go` defines append-only system renewal-basis snapshots. Product-level
activation automatically applies to future renewals, including existing chains.
There is no owner acceptance endpoint, terms checkbox or administrator approval
stage. Snapshot limits come from calculated policy bounds, not user-entered terms.

`price.go` considers current/public/limit/eligible campaign candidates, forecast
renewal probability, contribution, relationship value and volatility. Insufficient
model evidence keeps the current basis. Low propensity alone grants no discount.
Campaign eligibility and spend are independently rechecked in the database.

`quote.go` retains an immutable amount, input versions, validity window, model,
health and calculation trace. Notice is queued and recorded independently;
notification delivery does not block settlement. Scheduled preparation starts
within seven days; an explicit early renewal can prepare its quote on demand.

The existing commerce transaction still owns wallet debit, coupons and reward
A multi-term batch consumes campaign budget for every term. Successors recover
the public basis so a one-term discount cannot become a permanent discount.

Health failures preserve the amount and permit bounded retries for 72 hours.
Deferral never extends access. Owner information is read-only and isolated from
browser administrator sessions. `quote_test.go` checks immutable windows,
calculated bounds, stale versions and the separation of propensity and discounts.
