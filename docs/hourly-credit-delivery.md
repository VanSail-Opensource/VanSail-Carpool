# Durable hourly byte-credit delivery

The master can issue only existing free allowance or already funded extra bytes.
An optional `HourlyPolicyRepository` supplies current owner/generation, daily
allowance and per-node paid boundaries. Missing/expired policy creates no credit;
credit issuance itself never invents funding or debits a wallet. Task 6 may attach
an internal `HourlyExtraBuyer`, which buys one decimal GB through the established
wallet transaction only after fresh positive socket demand and complete pool
exhaustion with no held reservations. Activity-only prefetch never buys a block.
Live negotiation also requires the explicit durable-history readiness contract.
Production `Store` opts in through its atomic single/batch hourly history writer;
credit settlement construction alone does not imply that readiness.

`requestHourlyCredit` coalesces at most 8192 fresh owner demand hints. The existing
four credit workers deliver install/seal/ACK commands through each authenticated
peer's established outbound queue. One worker per persistent AgentID preserves
issue order. No global service/master/coordinator lock surrounds an RPC or SQL
wait. Unanswered credit commands cannot occupy command-result or traffic-ACK lanes.
Demand activity, scheduling and retry boundaries share the service clock; measured
RPC round-trip duration uses real monotonic elapsed time.
Hints are filtered through the current cached owner/generation/node policy before
the coordinator lock. Initial reconciliation, open obligations, ACKs/intents and
fresh demand/confirmed activity wake work; zero/stale/unknown-owner hints do not
start an idle 100-ms SQL polling loop. Closed-owner financial evidence is still
accepted independently of that admission filter.

`EnsureHourlyCreditDay` keeps the first UTC-day allowance immutable. SQL reserves
bytes and assigns a persistent AgentID-global sequence before an install can be
sent. A stable in-memory intent retains the exact request across uncertain SQL
replies. Retry resolves the original tuple before sizing a replacement. Retired
or ineligible intents still reconcile: original user/node locks prove absence
only when permanent removal, lifecycle generation/status or UTC expiry forbids
late admission. A missing ordinary read or temporary disable is insufficient.
An exact retry can separately prove "definitely not reserved" after taking the
original locks, observing no row and insufficient pool, and completing rollback.
Only that result permits replacing the matching RAM intent with a smaller fresh
request. Read Committed snapshots ensure a prior ambiguous COMMIT becomes visible
after its lock releases. Cancellation, network/rollback failure and arbitrary
NotFound never authorize that replacement; financial rows remain unchanged.

Before sending a new install, a bounded lower-issue scan catches uncertain older
commits that became visible after the connection's initial snapshot. Exact-session
predecessors already delivered remain active without an RPC. Other predecessors
must install or seal first. This prevents later acknowledged highwaters obscuring
an undiscovered old obligation. Active/recovery evidence is never evicted to pass
this check; exhaustion stops new credit until the dependency settles.

Startup credit targets 64 MiB. Durably confirmed admission rate and measured RPC
latency adapt subsequent targets beyond that size, with a half-target refill
threshold. Fresh demanding nodes share the exact unconsumed day pool. All final
reserves are clamped by locked availability; floating point appears only in the
disposable throughput sizing estimate. Successful passes have no error backoff.
Confirmed admission progress also refreshes activity and permits low-water refill
while no payload is blocked. Zero-pending keepalive alone cannot create activity
or indefinitely retain idle holdings. This is funded/free prefetch, not authority
to buy another GB block when demand is absent.

The producer scans pending obligations in bounded issue-order pages. It seals
fully admitted, old-day/session, ineligible, retired or ten-second idle holdings
using the immutable original install as `Expected`. It applies the matching final
proof durably before sending the digest ACK. Disconnect returns no bytes. Recovery
unknown remains reserved; late authenticated progress can confirm that debt but
cannot fabricate unused credit. Other nodes can reuse only proven normal-unused
bytes. Financial receipts and original scope survive lifecycle closure.

`HourlyCreditAdmissionReady(node)` requires both negotiated agent support and the
full durable repository. `CloseHourlyCreditObligations(owner,generation,node)`
marks existing workers for retirement and returns specific offline, proof-pending
or recovery-unknown dependencies. It accepts generation `-1` only with explicit
owner/node scope, supporting purge after lifecycle generation increments. Callers
fence access first and retain agent control until proofs settle. Node removal also
checks outstanding debt inside its final SQL transaction and retains hourly-only
node history as a tombstone.

Commerce calls `ResumeHourlyCreditAdmission(owner,generation)` only after a new
paid activation is authorized. This clears the transient owner stop for same-day
re-enrollment; demand cannot clear it, permanent node retirement remains fenced,
and no historical credit or daily allowance resets.
Disabling one squad must keep the shared owner pool active while another paid
hourly squad remains. Owner-wide closure applies only after its last hourly
benefit/access ends; node and account retirement retain their broader fences.

Hosted regressions use real PostgreSQL and authenticated TLS WebSockets for
durability-before-install, unanswered install versus native traffic ACK, uncertain
SQL replay, lower-issue commit races, original-session recovery and unknown debt.
Pure tests cover retired intent resolution, exact retry tuples, adaptive sizing,
multi-node share limits and per-Agent worker exclusion. Local suites are not run.
