# Automatic firewall leases

Fresh authenticated IP activity creates a temporary exact-address blacklist
exemption. Reports from registered nodes qualify even when their firewall or
proxy runtime is disabled. Source IPs, observations, the expiry heap and delivery
watermarks stay in memory. Only enablement and duration settings are persistent.

The browser console exposes **X-Node → Automatic leases**. Reading its paginated
API (`GET /api/v1/admin/xnode/firewall/leases`) requires `xnode:read`; changing
settings retains the existing global write authorization. The lease form uses
`PUT /api/v1/admin/xnode/global?part=automaticWhitelist` so a stale lease form
cannot replace unrelated credentials, intervals or desired state.

## Duration and delivery

The initial deadline is observation time plus the configured duration, normally
24 hours. Earlier activity updates statistics without another grant. At or after
half the current interval, fresh activity starts a new interval lasting twice
the currently configured duration. Renewal does not compound the old interval.

| Observation | Configured duration | Result |
| --- | --- | --- |
| Hour 0 | 24 hours | Expires at hour 24 |
| Just before hour 12 | 24 hours | Deadline unchanged |
| Hour 12 | 24 hours | Expires at hour 60; next renewal eligible at hour 36 |
| Hour 36 | 6 hours | Expires at hour 48 |

Disabling creation prevents new leases and renewals. Existing leases retain
their deadlines, update activity statistics and synchronize until expiry.
Duration changes affect later creations and qualifying renewals. A shorter
duration can shorten a later renewed deadline.

Delivery uses the existing per-node writer with batches of at most 256 grants
and acknowledged epoch/generation watermarks. Unchanged reports do not duplicate
grants. Joining, reconnecting and firewall enablement synchronize every unexpired
lease. Nodes must connect with a compatible agent and apply their firewall
configuration; offline, outdated or unsynchronized nodes remain pending.

Agents enforce exact timed IPv4/IPv6 entries in a separate volatile bank. Expiry
removes master memory state without sending removal commands. Agent kernel
timeouts expire grants locally. A master restart rebuilds leases from fresh
reports; previously delivered agent deadlines remain effective. An agent process
restart can recover unexpired kernel deadlines. A new master epoch preserves a
longer prior deadline until its original expiry.

## Accounting, bans and privacy

The lease system releases individual exact-IP bans through the durable firewall
transaction/outbox path, retaining their evidence. Broader ranges remain banned;
an exact leased address inside a range does not exempt neighboring addresses.
Manual/paid exemptions and credential/member admission controls remain separate.

Lease commands and IP banks do not enter durable command history or confirmed
firewall snapshots. The admin view and browser polling use private, uncached
responses. Source cardinality is exact up to eight nodes and marked when
truncated. Observation counters count accepted fresh source observations; rejected
capacity counters cover exhausted registry/generation capacity. All statistics
restart with the master process.

The agent's new traffic buffers are also volatile. Report identity, committed
usage, financial effects and gap evidence are handled separately in the existing
accounting transaction. Missing uncommitted traffic can underbill or increase a
refund; settlement uses surviving known usage and never rewrites settled facts.

## Hosted performance evidence

The isolated 10,000-IP gate requires each registry pass below one second,
retained registry memory below 64 MiB, zero duplicate grants, no steady-state
lease database writes and bounded batched synchronization. It runs only in hosted
CI; normal race shards cover correctness without enforcing noisy timing limits.

[Baseline hosted run 41638285479](https://github.com/VanSail-Opensource/VanSail-Carpool/actions/runs/41638285479)
measured the following on panel revision `2cb364ba`:

| Registry phase | Elapsed |
| --- | ---: |
| Create 10,000 leases | 8.855 ms |
| Initial delivery snapshot, 40 batches | 5.613 ms |
| Ten repeated observation passes | 17.348 ms |
| Reconnect synchronization | 1.500 ms |
| Renewal pass | 5.396 ms |
| Renewed delivery snapshot | 4.467 ms |
| Expiry pass | 1.611 ms |

Retained manager memory was 7,165,144 bytes (about 6.8 MiB), with 30,459
allocations and approximately 0.038 runtime user-CPU seconds. There were zero
duplicate grants and zero steady ban-release intents. These registry measurements
exclude network round trips and kernel execution. Hosted artifacts publish CPU
profiles, latency, allocations and memory; subsequent full backend workflows
repeat the same gate.
