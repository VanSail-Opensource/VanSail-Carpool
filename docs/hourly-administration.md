# Hourly administration and configuration

`internal/admin/hourly_policy.go` reads complete encrypted administrator terms
and supplies `HourlyPolicy(context.Context)` to the native service composition.
Decryption and `model.ValidateHourlyPolicy` run outside the database. Missing
configuration returns `model.DefaultHourlyPolicy`: sales off, no invented traffic
or extra prices, a 168-hour eligibility threshold, and a 24-hour cooldown.
An existing unreadable, incomplete, unknown-field, or invalid document fails
closed; it does not receive the absent-document defaults.

`SaveHourlyPolicy` accepts the current opaque version and a complete policy.
`internal/platform/database/hourly_settings.go` fingerprints the exact encrypted
document using SHA-256. `hourly_settings_save.go` commits the replacement and a
redacted audit in one SQL-only retry transaction. First-time setup uses an
advisory transaction lock, and existing rows use `FOR UPDATE` to reject stale
drafts and serialize with wallet operations. Generic settings writes explicitly
reject `xnode.hourly_policy`.

Commerce receives a validated `model.HourlyPolicySnapshot` from the service.
Before a debit, `lockHourlyPolicyVersionTx` reads the stored ciphertext's version
with `FOR SHARE`; the caller must compare it with that snapshot while retaining
the lock. Store never decrypts or accepts a client-provided policy as authority.
An absent row has no version and cannot authorize commerce. This reuses the
existing settings relation rather than persisting a second policy copy.

The `SalesEnabled` switch controls new activation. It does not hide the terms
needed to enforce paid access or renew existing hourly enrollments. Daily
allowance and reduced rate use bytes and bytes per second; extra pricing uses
minor TXB per decimal GB (1,000,000,000 bytes). Zero extra price means extras are
unavailable. The activation threshold is checked with
`model.HourlyActivationMinimum`; it is never a debit or a reservation.

`internal/admin/hourly_offers.go` lists and validates separate squad offers.
`internal/platform/database/hourly_offers.go` reads only explicit configured
rows; `hourly_offers_save.go` compares revisions and atomically commits the
offer's audit. Revision zero creates a new row, successful edits increment its
positive revision, and stale edits fail. New table timestamps are PostgreSQL
`TIMESTAMPTZ`, scanned as UTC `time.Time`. A native group is locked before its
offer, and removing that group cascades only its current offer.

Enabled offers require a positive hourly price. Disabled offers may retain a
zero price, and administrators can configure offers before enabling new sales.
Native group availability, participating-agent readiness and stock remain
activation checks; an enabled offer does not assert those conditions. Existing
fixed-term products, purchases, ordinary coupons and renewal prices are neither
seeded nor rewritten by this module.

Hosted regression sources are `internal/admin/hourly_policy_test.go`,
`internal/admin/hourly_offers_test.go`,
`internal/platform/database/hourly_settings_test.go`,
`internal/platform/database/hourly_offers_test.go` and
`internal/model/hourly_validation_test.go`. They cover encrypted CAS, competing
first writes and offer edits, wallet/configuration lock compatibility, audit
atomicity, unavailable terms, strict decoding, overflow bounds, exact JSON
integers and preservation of legacy merchandising. Local automated suites
remain prohibited; delivery requires the hosted workflow.
