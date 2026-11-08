# Hourly native access and accepted history

Each explicit enrollment owns one `member-hourly-<enrollmentID>` service-range
principal and one stable `hourly-<enrollmentID>` accounting period. The parent
member identity owns the public subscription token. Preparation creates encrypted
credentials and a NO_RESET period while no paid user is authorized. A local
BlockNew preview validates credentials/configuration against current source and
transit capabilities; this preview never enters an outbound queue. Commerce
rechecks wallet, terms and compile revision before committing its paid boundary.

Renewal changes only the exact `PaidThrough` authorization boundary. It does not
rotate credentials, reset usage or inherit legacy continuity/PAYG lease padding.
An absent/expired committed boundary grants no native access. Native publication
uses existing per-node queues and confirms the expected owner generation,
enrollment, principal, period and exact expiry in current applied state. Routing
participants additionally retain the existing generation/source/transit proof.
The public operation must remain pending while any participant lacks that proof.

Billing mode is explicit on internal subscription proxies. A secondary hourly
principal is not a PAYG principal; zero wallet balance cannot hide an already paid
hour. Routing choices use enrollment-specific endpoint IDs. Stock admission
counts distinct active owners across grandfathered and hourly seats; unpaid
preparation does not reserve a seat. Mutating admissions lock sorted groups and
stock rows before checking capacity; pure quotes remain advisory.

Global encrypted hourly settings are decrypted by an attached typed service
provider, outside database Store. Master combines those terms with current paid
enrollments for its four-second memory-only policy cache. Pausing sales preserves
existing paid access. An optional buyer callback runs in the existing credit
worker and receives only server-observed pending bytes/time and owner/node/day.
Its original socket receipt survives coalescing and database waits. Zero hints,
confirmed-activity prefetch and idle/disconnected reservations cannot buy extras.
Wallet code separately rechecks opt-in price consent, actual exhaustion, held
bytes and funds; issuance can consume only durable already-funded credit.

Idle last-benefit closure names the terminal enrollment and captures only that
principal/period. Under the users-row lifecycle lock, a newer paid/preparing
benefit supersedes an owner-wide RAM stop, including within the same generation.
Purge uses a separate exact current purging generation and historical upper bound;
admins and superseded generations cannot pass. Resume shares that lock. Actor
work after release closes only captured periods and inspects credit evidence;
it never re-stops a newly resumed owner from each historical row.

Accepted combo and hourly raw observations enter the same savepoint, transaction
and replay cursor as native traffic. Batched ingestion preloads scope and period
references, stages each report separately and accepts its buffer only after all
validation succeeds. A history/storage failure prevents commit and ACK. Hourly
native samples remain on the existing accounting path but are not added a second
time to presentation traffic; only `HourlySamples` represent admitted hourly
ingress. Successful Begin may precede a failed native write, so those raw counts
must not be compared with native-writer-success counts or charged again.

Hourly samples require the original owner/generation/enrollment/principal/period,
a retained paid receipt and recorded node participant. Historical closed scopes
remain ingestible without current wallet, credentials or generation inference.
The interval must be ordered, agree with its declared UTC day and be no more than
five minutes in the future. These are conservative presentation sanity checks,
not a second authorization clock: ordinary agent clock offset must not block a
mixed journal. Native Begin, session grants and exact ValidUntil enforce paid
access independently. Existing combo periods have no recorded generation, so
their history retains NULL rather than copying a newer users-row generation.

Each sample's exact directional bytes are grouped by its actual Through minute,
with original interval bounds retained. No uniform split across unknown minutes
or pre-feature daily-history backfill is invented. Combo rows retain the original
purchase ID for current-purchase-only cost allocation. History reads return a
persisted forward coverage floor, recent node gaps and truncation explicitly.
Dropped-hourly-only reports commit their loss count and real cursor without
fabricating traffic. Presentation projections expire after 72 hours; financial
receipts, byte-credit obligations, journal cursors and audit evidence do not.
