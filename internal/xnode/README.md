# X-Node control plane

`credentials.go` supports administrator-authored VLESS reverse portals without a
separate panel surface: an inbound document may carry `"reverseTag": "<tag>"`.
The native validator requires that marker on a `vless` inbound, with a valid
unused tag that does not shadow a static outbound. `inboundCredential` then adds
`"reverse": {"tag": "<tag>"}` to the generated per-user account, so the bridge
identity bound to that portal passes the pinned VLESS reverse authorization
check while every other identity keeps an ordinary credential. The exit node
authors the matching VLESS outbound (`settings.reverse.tag`), and its freedom
outbound for that session needs an explicit `finalRules` allow plus a
`dokodemo-door`/`tunnel` inbound that names the tag, because the agent admits
reverse-mux egress only through an auxiliary listener tag. Hysteria2 listeners
remain ordinary inbounds with `inboundCertificates` profiles. `reverse_portal_test.go`
covers credential generation and document validation.

`endpoint_inbound.go` derives administrator endpoint protocol/transport/security from the node's resolved inbound, certificate bindings and the same saved snippet expansion used by compilation. It keeps source configuration reads unchanged. `endpoint_preview.go` returns applicable typed fields, sanitized field errors and independent client compatibility for incomplete drafts using fixed synthetic credentials. Preview performs only local reads and native client analysis; it never queues external work or persists a draft. Save reuses this validation against current desired state before encrypting `overrides.clientOptions`. `endpoint_validation.go` decodes that separate administrator type and validates the client document without server runtime side effects. `endpoint_options.go` merges nested inherited XHTTP headers, XMUX and download settings before applying typed overrides. `endpoint_preview_test.go` covers read-only preview, encrypted round trips and protocol changes between preview/save; `endpoint_options_test.go` protects nested inheritance. `endpoint_inventory_compatibility_test.go` checks that inherited mode changes preserve compatible peers across all six export formats; runtime projection failures are isolated to the affected endpoint while store/credential failures remain fatal. These regressions run in hosted CI.

Node traffic multipliers are retired. `node_decode.go` ignores the deprecated input and serializes it as constant `1`; `SaveNode` sets the fixed-point default without an agent traffic flush. Catalog/statistics adapters retain compatibility properties at `1`. Host name `{price}` expands only the existing PAYG price, with no Combo multiplier label. Recorded weighted usage and financial evidence remain authoritative. The PostgreSQL cutover establishes acceptance-sequence boundaries before new ingestion; see the database module for deployment and preservation constraints. Hosted tests in `node_multiplier_retired_test.go` and `native_policy_test.go` cover obsolete inputs and offline node edits.

`certificate_commands.go` queues revision-fenced ACME lifecycle operations through
the existing durable command executor and recovers the latest profile receipt.
Its result allowlist retains safe intent, active fingerprint and activation facts
even after failed/uncertain completion. `certificate_validation.go` checks mode
fields, provider references and bounded matching PEM pairs locally; it never
reads node paths or contacts a provider. Effective mode follows X-Node: normalized
explicit mode, automatic HTTPS, a complete PEM pair, then a complete file pair.
Partial stale PEM fields do not hide a valid file pair; inferred or normalized
domainless file profiles require `node_file_certificates_v1`. Raw saved settings
remain intact, and managed profiles still reject an effective `none` mode.
`certificate_mode_test.go` protects inference, validation and older-agent gating.
`certificate_progress.go` keeps one
typed advisory snapshot per active node in memory, outside terminal result slots.
Certificate execution has a five-minute deadline, capability negotiation and
the existing per-node queue. Cross-device uncertainty is fenced by the store's
existing operation records, without another queue or table.
`certificate_commands_test.go`, `certificate_progress_test.go` and
`certificate_validation_test.go` cover command isolation, safe failure evidence,
nonblocking progress, PEM identity and mode validation in hosted CI.
`certificate_delivery.go` binds encrypted queued intents to the original agent;
replay keeps the original revision and command ID, while an uncertain operation
cannot issue through a replacement agent. `certificate_delivery_test.go` covers
this fence and encrypted private metadata without widening browser inputs.

`node_dns.go` retires resolver selection at settings boundaries. Effective DNS
always uses Xray. Its encrypted `legacyDnsLoopback` compatibility state preserves
the old SmartDNS loopback fallback only for scoped interception; explicit native
DNS servers take priority. `compiler.go` applies the fallback to a node-local
compiled copy, never to the shared configuration. Older clients cannot erase
the marker during unrelated saves. The deprecated upstream list remains unused.
`node_dns_test.go` covers compatibility, malformed/native DNS preservation,
non-mutating compilation and encrypted round trips in hosted CI.

Node firewall port ranges use the pinned public protocol's compact interval
normalizer before encrypted node settings are saved. Legacy singleton ports
remain readable. Forwarding supports many-to-one and equal-length offset
mapping; invalid/conflicting mappings fail before persistence. Compilation and
peer synchronization require `native_firewall_port_ranges_v1` whenever range
fields are present, preserving the last applied state on older agents. This
extends existing configuration delivery; it adds no upstream request path,
queue or database table. The browser contract is documented in `api/README.md`.
`capabilities_firewall_ranges_test.go` covers range capability negotiation and
legacy singleton compatibility in hosted CI.

`usage_breakdown.go` exposes the member-owned consistent 72-hour projection.
It forwards request cancellation, fixes the exact UTC window and preserves
exact rational minor-unit costs from the store. Physical Combo/PAYG nodes and
hourly squads are distinct projection entries; shared hourly extra purchases
remain unallocated. It sends no external requests
and cannot substitute current prices or invented empty history for unavailable
records. `usage_breakdown_test.go` covers those public interface guarantees.

`hourly_coordinator.go` runs the optional two-second account rate controller;
`hourly_policy.go` defines the future commerce policy repository and a cloned,
four-second eligibility cache. Missing commerce support grants no hourly access.
`hourly_connection.go` installs authority only after the supported session offer
and hello capability, and preserves allocation on disconnect. `hourly_receive.go`
validates live demand/individual or batched ACKs while forwarding financial
progress after enrollment closure. `hourly_lane.go` keeps bounded hourly database
work separate from command results and traffic acknowledgments.
`hourly_grant_sender.go` coalesces bounded per-peer intents; `hourly_grants.go`
sends each frame through the existing per-node queue outside all control locks.
The independent memory-only allocator lives in `internal/hourly`; public readiness
uses `Master.HourlyCapable` for every source/transit participant. Full behavior and
the capability/commerce separation are in `docs/hourly-master-enforcement.md`.
`hourly_history.go` separately gates both live negotiation and historical receipt
advertisement on an implemented atomic history writer. Production Store now
implements it with the single/batch journal transaction; mixed reports cannot be
ACKed by a repository without it.
`hourly_history_test.go` covers false readiness and ordinary-traffic continuity.

Hourly native entry points are `PrepareHourlyPrincipal`,
`HourlySquadReadiness`, `PreflightHourlyEnrollment`, `SyncHourlyEnrollment`,
`CloseHourlyOwnerIfIdle`, `CloseHourlyOwnerForPurge` and `ResumeHourlyOwner`.
`hourly_principals.go` defines immutable enrollment projections and dormant
credential preparation. `hourly_compile.go` applies exact paid boundaries;
`hourly_compile_reads.go` retains owner/node-scoped reads only within one compile.
`hourly_inventory.go` keeps distinct billing/endpoint identities for subscription
and routing; `hourly_subscription.go` represents unmetered traffic without a hard
quota and exposes the exact paid expiry. `hourly_readiness.go`,
`hourly_preflight.go` and `hourly_deployment.go` require compatible source/transit
participants, validate an unpaid local preview, and verify exact applied scope.
`hourly_lifecycle.go` orders idle/purge retirement and resume on the shared owner
row; actor work never re-marks newer same-generation benefits after that lock.
`hourly_policy_provider.go` composes encrypted settings outside Store;
`hourly_extra.go` supplies the internal fresh-demand buyer extension through the
existing credit worker. Providers are attached before starting Master workers.
`usage_history.go` exposes bounded owner-scoped observations and explicit
coverage. `hourly_native_fixture_test.go` and `hourly_native_test.go` cover real TLS
native publication, dormant preparation, exact renewal and prepaid exports.
`hourly_capabilities_test.go` covers independent live/history native capabilities;
`hourly_extra_test.go` checks original positive-demand freshness and prevents
activity-only, held-credit or disconnected purchases.
`hourly_preparation_test.go` confirms never-issued cancelled preparations without
waiting for an offline agent or creating credentials.
`hourly_native_agent_test.go` owns the native TLS fixture's single-writer applied
state/telemetry loop. `hourly_demand_policy_test.go` fences hints delayed across
policy removal. Readiness also checks the allocator's current-paid/retained-owner
capacity union and cold-start quiet; wallet admission independently fences the
global distinct-active-owner ceiling before charging.
`hourly_entitlements.go` adds current paid hourly benefits to existing manual
connection controls, owner metrics and customization tags; automatic observation
and community membership policies remain independent and unchanged.
See `docs/hourly-native-history.md` for lifecycle, timestamp and data-flow limits.

Hosted `hourly_fixture_test.go`, `hourly_socket_test.go`, `hourly_lane_test.go`,
`hourly_policy_test.go`, `hourly_receive_test.go`, `hourly_grants_test.go` and
`hourly_delivery_test.go` cover TLS negotiation, replacement sessions, blocked
credit isolation, strict policy/ACK scope and exact serialized grant delivery.
`../httpapi/hourly_command_boundary_test.go` covers the HTTP and service boundary:
credit installation, sealing and proof ACKs remain private coordinator operations,
and rejection precedes receipt replay, persistence and node reads.

`firewall_records.go` defines the read-only active-record search contract.
`firewall_filter.go` compiles bounded nested AND/OR predicates; its hosted
`firewall_filter_test.go` covers matching and expression limits.

Admin fleet metrics sum negotiated open connections with overflow-safe decimal
arithmetic. Disabled nodes are excluded; an enabled unavailable or legacy node
makes the fleet total unknown. These memory-only counters never affect limits.

`member_metrics.go` projects owner/principal-scoped sampled and total client
connections from fresh memory-only telemetry. `master_http.go` negotiates
`open_connections_v1` through the upgrade header plus hello capability. The
additive `totalConnections` API field is a nullable decimal string: all scoped
nodes must provide fresh negotiated totals. Any observed open flow means online;
offline requires complete fresh totals of zero. Legacy positive traffic remains
online evidence, but legacy idle reports cannot establish offline. Expiry/error
handling still retires stale status. Compatible agents enforce hard admission
through live non-transit reservations; sampled `connections` remain observation
inputs for the master. Native-handler and accounting completion retain their
separate lifetimes. `member_activity_test.go` covers these
status and compatibility boundaries in hosted CI.
`member_connections_test.go` checks idle presence, exact totals, legacy/stale
unknowns, residual-rate behavior and invalid counters. The socket fixtures in
`master_report_frames_test.go` check negotiated totals and snapshot replacement.

PAYG GiB purchases retain the same principal, credentials and accounting period.
`master_sync.go` renews node authorization before connection observation work;
an observation error cannot skip that pass. Journal settlement remains atomic
and independent. `payg_leases.go` applies the bounded authenticated clock offset
to both renewable leases and prepaid expiry caps. No upstream protocol changes
or persisted connection state are required. Hosted continuity regressions cover
the byte boundary, prepaid clock cap and observation failure ordering.
`payg_continuity_test.go` covers funded authorization across GiB credit boundaries,
the bounded agent-clock prepaid cap and synchronization before observation errors.

`subscription_usage.go` projects existing owner accounting into subscription
metadata and the HTTP `Subscription-Userinfo` header. An effective combo retains
its expiry and current matching period's usage/quota even when PAYG hosts are
also exported. A prepared successor never inherits its predecessor's usage.
Without a combo, PAYG info reports exact cumulative period usage and a total of
that usage plus currently unexpired prepaid bytes across the owner's accounts;
it has no subscription expiry. This is a prepaid snapshot, not a wallet-derived
quota or an admission decision. Expired credit and old combo usage are excluded.
No state is added or changed. A positive PAYG snapshot also preserves FlClash's
info display, which requires `total > 0` in
[its profile view](https://github.com/chen08209/FlClash/blob/main/lib/views/profiles/profiles.dart).
`subscription_usage_test.go` covers combo, mixed and PAYG-only projections.
`endpoint_clients.go` retains published names without appending endpoint IDs; native tags remain derived from stable IDs. Split-profile label collisions likewise use numeric suffixes in profile-ID
order; exported host names never include generated profile IDs or hashes.

`runtime_reads.go` selects optional bounded repository reads with compatibility
fallbacks. PAYG connection observation reads only positive live connection IDs;
lease overlays reread current funding candidates and remove missing PAYG periods
from authorization. Subscription inventory still contains unfunded accounts.
`cadence.go` loads only resettable owners; `cadence_recovery.go` explicitly excludes
PAYG from scheduled resets while retaining combo/service recovery and settlement.
`runtime_reads_test.go` checks idle observation, PAYG reset exclusion and removal
of unfunded PAYG authorization while preserving combo users.
`payg_funding.go` defines the private-control-plane `PAYGFunding` projection.
Authorization uses its fresh balance, prospective price, credit and disablement
inputs without loading historical usage. Its predicates delegate to `PAYGAccount`
so prepaid expiry and debt rules have one implementation. The optional repository
read and legacy fallbacks remain in `runtime_reads.go`; read errors propagate
without reusing an older snapshot. Compilation, accounting, owner data and billing
continue to use full accounts. Hosted runtime-read and continuity regressions
cover projection selection, current funding, expiry caps and recovery ordering.

`routing_reconcile.go` retains a committed `routing_access_removed` plan when
it is disabled and every operational choice has already been pruned. It keeps
the revocation reason and avoids generating another empty routing operation.
Partial prunes, dependency failures and in-flight recovery retain the existing
reconciliation and durable delivery barriers. `routing_access_test.go` checks
this boundary repeatedly across authoritative revision changes; the hosted
deployment regression still verifies native peer removal and accounting safety.

`ddns_changes.go` accepts bounded `ddns.changes` batches only after independent
upgrade-header and hello-capability negotiation. It validates authenticated agent
ownership, canonical addresses, enabled families and the current configuration
generation. Obsolete jobs/generations are acknowledged without mutation; durable
cursor/operation commit precedes every successful acknowledgment. Delivery uses
the existing control lane and single WebSocket writer, independently of traffic.

`ddns_operations.go` stages the existing protected firewall bank, atomically
updates bound hosts and resumes automatic free routing in `ddns_routes.go`.
Saved endpoint choices, credentials and accounting remain authoritative. Pending
routes keep old bypass addresses through their existing apply barriers. Offline
nodes retain desired state; startup, dependency changes and reconnect resume the
same receipts through the existing outbox. `ddns_protection.go` caches the derived
bank by durable-state digest plus compile revision. `ddns_references.go` retains
literal endpoint/custom-outbound references and trusts only authenticated or
administrator-owned address evidence, never arbitrary DNS/CDN answers.

`ddns_changes_test.go` covers ownership, generation, malformed/oversized input
and legacy negotiation. `ddns_integration_test.go` exercises real sockets and
durable handlers, held firewall acknowledgments, free graph regeneration,
credential/ledger preservation, bypass replacement and offline convergence.
`ddns_fixture_test.go` supplies the paid encrypted-route scenario and real job
drain; `ddns_agent_fixture_test.go` supplies one serialized synthetic agent writer
and controllable protected-bank acknowledgments. These run only in hosted CI.
`ddns_protection_test.go` checks reconnect activation for both outbound target
addresses and inbound transit source nodes, plus legacy/unrelated compatibility.
`ddns_origins.go` projects successful external-provider attestations onto matching
same-node endpoint hostnames after bypass staging. It preserves TLS/SNI names,
invalidates stale DNS/compile inputs and participates in routing dependency
digests. `ddns_origins_test.go` covers scope, generation, staging and unchanged
cache reuse without external DNS requests.

`ddns_bindings.go` and `ddns_configuration.go` separate administrator-owned host
references from encrypted agent discovery settings. Built-in DDNS can bind each
enabled address family to multiple existing hosts on the same node, with each host
assigned once across jobs and families; it creates no public DNS records. External
providers may use optional single-host bindings. The authenticated configuring
administrator supplies audit ownership. Generations are derived from
discovery/provider configuration and binding references; neither host IDs nor
audit ownership enter the agent configuration. Omitted bindings preserve older
editor writes. Unsupported peers retain ordinary authorization updates and
external DNS jobs while built-in jobs await negotiated support.
`ddns_bindings_test.go` checks actor ownership, omission/rename compatibility,
generation changes, separate references and legacy configuration projection.

`catalog_topology.go` retains one credential-free node/group snapshot (4 MiB
encoded limit), keyed by the transactional compile revision. Each read checks
the authoritative revision; a loader verifies it again before publication.
Overlapping loads share a flight, canceled waiters leave immediately, and callers
receive isolated copies. It caches no member access, health, balance or price.
`catalog_topology_test.go` covers query counts, mutation/copy isolation, byte
bounds, cross-revision retries and canceled waiters. Node metadata changes join
the existing revision triggers; live agent bookkeeping does not invalidate it.

`master_report_frames.go` bounds expanded WebSocket input before parsing and
normalizes negotiated compact reports into existing live snapshot types. The
upgrade echoes `X-XNode-Telemetry: compact-v1` only when offered; compact frames
also require `compact_telemetry_v1` in the authenticated agent hello. Legacy
reports remain accepted. Per-message deflate is negotiated by Gorilla; master
commands and acknowledgements retain their uncompressed single-writer path.
`master_report_frames_test.go` covers mixed agents, negotiation and clearing an
earlier online user snapshot. Accounting reports retain their original body.

`master_report_lanes.go` separates incoming command results, ordered traffic,
latest-snapshot telemetry and disposable logs. `master_report_lanes_test.go`
covers saturation and report freshness. Each lane has its own bounded
consumer; queued command/traffic payloads have a 32 MiB byte budget, logs have
eight frames capped at 256 KiB each, and telemetry retains one current frame.
The existing per-node outbound queue remains the only WebSocket writer. Durable
overflow disconnects without acknowledgement, allowing journal replay; telemetry
coalesces and discarded logs increment the memory-only dropped counter. Replaced
peers cannot overwrite current telemetry/log snapshots. Shutdown cancels workers
and waits for their active context-aware work. Hosted saturation regressions
hold accounting blocked while requiring fresh telemetry and command delivery.

`payg_squad_state_test.go` verifies hidden squads retain credentials and access,
unavailable squads lose PAYG authorization, and normal restores the same period.
Member PAYG status uses the configured telemetry reporting window, matching the
master's admission check, and still requires the committed recovery barrier.
`payg_export.go` removes hidden PAYG entries from refreshed subscriptions and
host links before split-profile rendering. Server admission, routing references
and active connections retain the same principal and credentials.

PAYG billing ignores node multipliers and uses a 100 MiB rolling 72-hour raw
traffic threshold. Prepaid remainder expires 72 hours after its charge; funds,
credit expiry and readiness jointly govern volatile admission, with leases
capped at credit expiry when replacement funds are unavailable. Exact period
totals and financial history remain intact. `payg_repairs.go` shares the existing
missing-principal outbox repair between PAYG and DIY reads. Positive balances
request DIY repair even below a GB price, and `inventoryPending` lets the client
refresh both routing and compatible XHTTP choices without changing drafts.
`diy_payg_test.go` covers this queued path with a one-minor-unit balance and no
combo. Existing stock, activation and wallet admission gates remain independent.

`profile_payg_costs.go` adds exact immutable owner-scoped charges to the existing
72-hour PAYG history when supported by the repository. Cost-only nodes remain
visible with zero traffic; historical charges are never reconstructed from live
prices. `profile_payg_costs_test.go` covers exact large minor-unit values,
pre-purchased credit separation and repositories without charge history.

New members receive PAYG provisioning from the onboarding outbox without buying
a combo. Completed onboarding exposes owner-scoped PAYG listings and stable
subscription links even with an empty wallet; actual connection leases require
funds for a GB block (or unused paid bytes), a compatible ready agent and no owner
restriction. `onboarding_payg_access_test.go` covers unfunded onboarding through
the queue, incomplete-onboarding rejection, no purchase/debit and stable links.
`onboarding_payg_sync_test.go` covers funded admission through the agent barrier.

Native validation applies the shared `platform/xraydiagnostics` raw-log policy
before building configurations, including startup validation. Repeated upstream
protocol advisories do not flood logs; validation failures still return through
`ValidationResult`, with the same pinned validator and agent-side checks.

`master_traffic.go` handles durable report ingestion for the socket reader.
An unchanged rejected head backs off from one to 30 seconds using one in-memory
fingerprint per peer; telemetry and command results continue immediately.
Suppressed reports receive no ACK. Changed frames are checked immediately, and
successful commit/ACK clears the delay. Pending PAYG recovery remains coalesced.
`traffic_errors.go` supplies fixed diagnostics for accounting conflicts. The
store audits and discards unmatched reports and missing ranges by explicit
operator policy, then commits the actual received cursor before ACK. A valid
report after a gap still counts; no missing bytes are invented. Invalid frames,
agent mismatches and storage failures remain unacknowledged. `master_traffic_test.go`
covers retry bounds, command delivery and safe diagnostics in hosted CI.
`master_traffic_discard_socket_test.go` checks that a discarded missing-period
report and its replay receive matching acknowledgments before the next sequence.

Firewall reconciliation retains per-node execution and durable receipts. Its
database queue projects only lagging banks, avoiding fleet delivery work for
unchanged observation/event replay and preserving pending retry deadlines.

`master_observability.go` keeps aggregate commit/ACK counts and timing in memory.
`master_receive.go` records ACK latency only after durable report commit; failed
sends leave reports replayable. No journal protocol fields or live user metrics
are persisted for monitoring.

Replacement/reset revocation never waits for connected agents' replay backlogs.
It wakes the existing state reconciler and retains the old period as `closing`;
the accounting worker completes its barriers under its existing short deadline.
`master_commands.go` also bounds the full traffic barrier, including cursor ACK
waiting, to two minutes even for application-lifetime callers. The socket reader
continues committing/acknowledging reports independently.
`period_successor_test.go` covers an online agent withholding its traffic barrier
while a successor becomes available with predecessor usage retained.

Scheduled resets retry an interrupted successor publication from a current
closing/closed period only after its cadence boundary and recorded revocation.
`cadence_recovery.go` checks current purchase ownership, restrictions, existing
open allowances and the existing member mutation fence before recovery. Earlier
closures and review periods remain unchanged. Old usage remains in its original
period until every journal barrier settles; a paid successor can admit traffic
while that settlement waits. `cadence_recovery_test.go` covers recovery, replay
and financial/review fences without changing immutable usage.

Routing/DIY saves bound preparation and agent validation to 45 seconds. Their
read-only preparation runs outside the global mutation mutex; PostgreSQL remains
authoritative for atomic owner, revision and idempotency fences.
Agent validation and deployment still use existing per-node queues and receipts.
`routing_agent_test.go` supplies the hosted two-node agent fixture and checks
that pending native validation does not block unrelated member mutations.

Firewall event ingestion joins eligible native nodes before persisting their
journal, independently of synchronized telemetry. Retired/disabled nodes can
drain durable events; they cannot authorize new bans. ACKs follow committed
cursors, and this socket path never waits for a command result.
`firewall_journal_socket_test.go` covers event ACKs before synchronized telemetry.
`frame_diagnostics.go` supplies allowlisted rejection/reconciliation causes;
raw database errors, source addresses and frame contents never enter logs.
`frame_diagnostics_test.go` checks wrapped errors and private-detail redaction.

## Paid access recovery

`member_access_test.go` covers username-independent admission, pending exports,
idempotent pointer repair and preservation of closed/exhausted purchase evidence.

`member_access.go` provides the common export/admission eligibility fence. Paid
members do not require an assigned username. Missing or mismatched native periods
produce pending provisioning, distinct from inactive access and configured empty
HTTP responses. `access_repair.go` runs through the shared durable outbox;
`EnqueueXNodeAccessRepairs` scans bounded batches at startup and every 30 seconds.
All onboarded members receive stable subscription credentials through this queue,
including members without a combo or wallet funds. The same scan backfills older
members without identities; credential creation alone grants no node admission.
`onboarding_provisioning_test.go` covers queued signup, idempotent links and the
absence of unpaid combo quota or admission in hosted CI.
Recovery reuses an existing open period without resetting its usage. Closed or
settling periods for the same purchase require review; they are never replaced
with a fresh allowance. Frozen imported usage remains settlement evidence.

Repair searches the whole purchase history for one open reset period before
considering settled predecessors. Multiple open periods require review. Repairs
use the master's bounded deferred operation leases, so traffic barriers cannot
occupy the shared outbox dispatcher or wait without an operation deadline.
`access_repair_period_test.go` covers recovery after a settled reset while
preserving the open period's counters and subscription access.

## Telegram identity

Member database keys are decimal Telegram IDs and native member IDs use that
same number. Non-member services allocate IDs from `2^52` up to `2^53-1`.
Private protocol credentials and bearer subscription tokens are independent of
those identifiers. Display names use Telegram username, then full nickname.
The startup database migration reseals ID-bound ciphertext before publishing any
new identity; agent `replaces_id` preserves credential-bound leases and period
counters. Masters require `telegram_identity_rekey_v1` before sending a rekey.
Hosted access regressions use returned native IDs for standalone subscriptions,
traffic and resets, retaining old IDs only as historical evidence. Saved
member tags stay reserved while inactive; decimal owner suffixes isolate members
even when their historical display names contain overlapping hyphens.

`provider_zones.go` discovers Cloudflare zones from saved provider credentials
through the existing external validation queue, one page at a time. The admin
transport returns only ID/name pairs and a supported flag, never credentials or
provider error bodies. Deadlines, pagination/body bounds and cancellation apply
before returning a complete, request-local inventory. `provider_zones_test.go`
covers pagination, malformed/oversized/error responses and cancellation in CI.
The agent still receives the DNS zone name; its protocol and release are unchanged.
Reference: https://developers.cloudflare.com/api/resources/zones/methods/list/.

`configuration_metadata_test.go` covers pinned parser extraction and endpoint
metadata. `validation_cache_test.go` covers incremental validation-cache reuse.

`routing_member_preview.go` accepts trace entries only for configured inbound
rules and their exact source endpoints. Accessible outbound-only/unused nodes or
another host on the same node cannot start a trace. Existing preview/save APIs
remain compatible, including deterministic entry selection when omitted.

## Incremental native configuration

`validation_cache.go` keeps up to 128 owner/node/document contexts, each with one
successful proof and the pinned agent's immutable section validator. Config
contract checks always run; identical effective inputs skip native compilation.
The key retains original local-asset declarations, so synthetic analysis cannot
hide `NeedsAgentValidation`. Real node-file/asset validation remains queued on
the agent. `validation_certificate_cache.go` stabilizes analysis-only certificates
for 30 minutes; their private keys never become serving credentials.

`compile_cache.go` retains transactional revision and time-boundary fences. After
an unrelated coarse epoch, effective per-node fingerprints preserve the existing
prepared wire object. Authorization, routing/peer resolution and dependency
reads still execute when invalidated; expensive core validation reuses native
sections. Control overlays are excluded from those fingerprints and synchronized
separately. No new database records or generated credentials are persisted.

`configuration_metadata.go` preserves encrypted documents during label-only
edits and retains revision checks and ordered snippets. `global_changes.go`
separates admission/commerce/probe settings from serving native inputs, preventing
unrelated saves from recompiling every node or probing unchanged asset links.
Hosted cache/metadata regressions complement agent TCP/UDP/mux continuity and
work-count regressions. See the agent's `docs/hot-reload.md` for action consumers,
forced-reload semantics and conservative fallback boundaries.


`Service` owns authoritative configurations, ordered snippets, node settings,
access groups, endpoints, identities and accounting periods. `Master` accepts
`/v1/agents/connect`, binds enrolled agents, owns per-socket queues, and joins
their lifetimes before PostgreSQL closes. Public wire/renderer packages come from a
pinned X-Node revision; the same native core validates configuration.

`compiler*.go` combines one base per node, global policy, node utilities, eligible
paid users and owner-scoped customizations. `UserOutbound`/`UserRouting` are empty
object markers in outbound/routing arrays; compiled native JSON removes them.
The default outbound must precede the outbound marker. Shared snippets appear
around member insertions: outbound snippets precede the member marker and shared routing fallback rules follow it. Native rendering and certificates contain credentials.

`master*.go` handles revisions, memory-only reports, durable traffic ACKs,
bidirectional firewall confirmation, command results and snapshot replacement.
Command waiting never occupies the frame-send queue. Connection limits use fresh
available node counts and block only new admission when the global threshold
is reached. Missing nodes mark totals incomplete and do not force blocking.
Node liveness uses fresh native telemetry independently of activity capabilities
and migration ownership; older ready agents and agents awaiting cutover remain
online while their unsampled counts stay excluded from admission decisions.
`master_metrics_test.go` covers that rolling-upgrade boundary.
`compile_cache.go` reuses unchanged candidates for baseline resends and backs off
failed compilation in memory, capped at one minute. Authoritative revisions,
asset generations and access deadlines bypass obsolete failures immediately.
Timeout/cancellation failures are recorded at compile completion, so a slow or
timed-out attempt cannot immediately retry on the next one-second controller tick.
`master_wire.go` prepares capability projection and the normalized digest once
per immutable compiled candidate/peer. Only scalar admission controls are copied
for a resend. It shares read-only documents/CIDRs/credential maps, and caches
capability rejections without reusing an older successful state. Hosted
`master_wire_test.go` covers the immutable and unsupported-policy boundaries,
including oversized inline GeoIP compatibility errors.
`master_retry_test.go` covers command errors and obsolete failures without full
resync, paced activation retries, and recovery through the original applied ACK.
The send digest is published before writing the frame so an immediate failure
cannot be overwritten by the sender's return path.

`credentials.go` preserves imported secrets and creates independent random
SS-2022 128/256-bit user keys. PostgreSQL stores encrypted desired configuration and
credentials plus exact accounting; no live count, source-IP set, rate, log or
connection-derived block decision enters persistence. `ports.go` keeps compact
intervals. `public_transport.go` prevents private destinations and DNS rebinding
when validating external asset links through the existing request queue.
`asset_http.go` applies the same bounded HTTPS redirect policy to GeoIP and
GeoSite downloads and keeps rejected redirects distinct from network failures.
`asset_http_test.go` covers destination filtering and safe failure classes.

`service_access.go` stores explicitly imported standalone terms inside encrypted
credentials, without creating Telegram principals or purchases. Existing quota
periods retain source and native usage exactly once, including reset tails.
`service_cadence.go` resets eligible native service periods under settlement fences.
`service_eligibility.go` derives access from current enabled groups and quota;
probe bindings retain their separate bounded behavior. `service_subscriptions.go`
uses the existing templates and host ACLs for standalone bearer subscriptions;
probe identities cannot use this path. Hosted `service_access_test.go` and
`service_access_policy_test.go` cover resume, shared exhaustion, late resets,
cadence handoff, host ACLs, native credentials and probe isolation.
`ServiceIdentityByName` resolves saved service names independently of Telegram;
`ServiceProxies` shares the same native inventory with monitoring.

`validation_assets.go` substitutes node-local files only in an analysis copy,
then runs native validation; actual activation still verifies node-local assets.
Hosted cases cover port parsing, report freshness and invalid observations.
DNS GeoSite domains, DNS GeoIP expectations and source-IP GeoIP matchers retain
their matcher structure with synthetic analysis values. `validation_assets_test.go`
checks both configuration previews and compiled candidates, preservation of the
administrator document and rejection of unrelated native-core errors.

`ownership.go` checks membership in the authoritative native node catalog.

`types.go` Defines authoritative node, config, group, endpoint and shared policy types.
`accounting_types.go` defines private identities, credentials and periods.
`metrics_types.go` Defines transient collector, node and member metric projections.
`repository.go` Defines storage/settings dependencies.
`service.go` Owns injected dependencies and secret-safe readers.
`global.go` Normalizes and validates fleet policy.
`global_save.go` Merges write-only credentials and queues link validation.
`service_nodes.go` Validates node utilities, enrollment and access/endpoint changes.
`service_configuration.go` Validates and stores base configs and snippets.
`compiler.go` Expands shared/member slots into one native config per node.
`compiler_users.go` Projects current paid ownership, preferences, quotas and restrictions.
`master.go` Owns socket/queue lifetimes and serialized outbound execution.
`master_http.go` Authenticates WSS enrollment and agent/boot identity binding.
`master_receive.go` Replaces reports, commits traffic before ACK and confirms firewall liveness.
`master_sync.go` Serializes revisions and computes memory-only global connection controls.
`master_commands.go` Waits for commands outside frame queues and verifies accounting barriers.
`master_metrics.go` Projects fresh machine telemetry separately from sampled activity availability. Activity-dependent rates and connection counts remain gated by advancing samples; receive-time freshness avoids false stale status from agent clock drift.
`ports_test.go` Pins compact range normalization and invalid inputs for hosted CI.
`master_metrics_test.go` Pins report freshness and invalid/empty count handling for hosted CI.

- `access_window.go`: Entry points: `EffectivePurchase`.


- `cadence.go`: Entry points: `ResetCadences`, `ResumeAccounting`, `RunAccounting`.

- `command_arguments.go`: Entry points: `commandArguments`.

- `compiler_snippets.go`: Entry points: `expandMarker`, `appendSharedSnippet`.

- `controlplane_fixture_test.go`: Hosted regression coverage: `Optional`, `PutXNodeGlobal`.

- `controlplane_test.go`: Hosted regression coverage: `TestRegistrationDoesNotAuthorizeProxyAccess`, `TestPaidAccountsOwnRoutingTagsAndMatchers`, `TestSubscriptionRevocationPreservesProxyCredentials`.

- `custom_destinations.go`: Entry points: `resolvePublicProxy`, `preserveProxyHost`, `validateMemberRule`.

- `custom_routing.go`: Entry points: `SaveOwnedOutbounds`, `SaveOwnedRouting`.

- `custom_stream.go`: Entry points: `validateMemberStream`.

- `custom_validation.go`: Entry points: `allowedFields`, `validateMemberOutbound`.

- `customization_read.go`: Entry points: `OwnedCustomization`.

- `endpoint_client_options.go`: Entry points: `optionObject`, `applyEndpointClient`.

- `endpoint_clients.go`: Entry points: `ServiceProxy`.

- `endpoint_inventory.go`: Entry points: `MemberProxies`.

- `endpoint_options.go`: Entry points: `applyEndpointOptions`, `stringField`.

- `endpoint_transport.go`: Entry points: `clientStream`.

- `identities.go`: Entry points: `Identity`, `IdentityForUser`, `IdentityUser`, `UserByUsername`.

- `master_access.go`: Entry points: `Connected`, `Sync`.

- `master_operations.go`: Entry points: `DeferOperation`.

- `master_socket_test.go`: Hosted regression coverage: `TestEnrollmentReconnectAndReplayUseAuthoritativeRevisions`, `TestNodeSnapshotsReplaceCountsWithoutPersistingTelemetry`.


- `ownership.go`: Entry points: `NativeOwner`.

- `period_operations.go`: Entry points: `Quiesce`, `CloseForReplacement`, `ResetTraffic`, `RestorePurchase`.

- `periods.go`: Entry points: `ApplyPurchase`.

- `release_check.go`: Entry points: `AgentBuild`, `VerifyRelease`, `WaitApplied`.

- `service_users.go`: Entry points: `AuthorizeService`.

- `subscriptions.go`: Entry points: `BearerMetadata`, `OwnedSubscription`, `BearerSubscription`, `RoutingTemplates`.

- `validation_base.go`: Entry points: `ValidateConfigurationDocument`.

- `validation_complete.go`: Entry points: `ValidateCompiled`.

- `operations.go` queues durable node commands and dispatches waits through bounded controller-owned tasks, retaining and renewing outbox leases so purchases continue.

- `member_metrics.go` returns scoped live member rates, connection counts and reasons.
- `metric_scope.go` retains node scope when quota, expiry or restrictions deny admission.

- `tag_ownership.go` reserves stored member tags, including inactive users, against administrator/shared outbounds and other owners.
- `tag_ownership_test.go` covers decimal owner namespaces, including inactive and expired members whose historical display names overlap.

- `master_firewall.go` routes liveness pongs by firewall transaction and retries confirmation for a bounded interval. If a confirmation response is lost, authenticated status for the same confirmed transaction resolves it. Pending, rolled-back, unknown, or mismatched status never clears the guard. `master_firewall_test.go` covers pong isolation and confirmation fencing.
- `capabilities.go` checks the exact capabilities required by compiled state, including native utility listeners with no paid users. `capabilities_test.go` covers utility and streamed-country negotiation boundaries.
- `binding_config.go` accepts group and subscription bindings only for managed proxy inbounds. Native utility listeners cannot become member endpoints.

`endpoint_group_policy.go` intersects paid/enabled groups with administrator endpoint filters and preserved legacy host ACLs, including previously imported endpoints. `endpoint_group_policy_test.go` checks owner subscription and routing-template visibility for allowed, excluded, empty and unknown policies.

`member_operation_state.go` separates current-reset reconciliation from purchase-wide refund checks. Refund counters include all same-purchase resets and already-reconciled legacy usage; unfinished journals cannot qualify. `commerce_test.go` covers this boundary and in-place cadence edits.

`configuration_preview.go` expands saved snippets for read-only pinned-core validation. Base validation reports bounded technical core diagnostics to the administrator editor.

`metric_numbers.go` preserves large unsigned telemetry integers as decimal strings at the browser boundary; `metric_numbers_test.go` guards values beyond 2^53. `subscription_templates.go` persists encrypted template documents plus native user references, validates revision/default/assignment constraints and uses synthetic previews. Xray preview validation omits the client display-only `remarks` field from the server-core validator while retaining it in rendered output. `subscription_render.go` selects subscription-specific, format-default or built-in rendering and respects enabled formats. `template_repair.go` adds missing converted per-host client templates before cutover without replacing administrator overrides.

`subscription_templates_test.go` covers preview validation, native subscription assignments, stale revisions and dangling default rejection.

`BrowserMetricNumbers` also protects historical command result counters. Machine JSON normalization runs after releasing the master snapshot lock, and client-template writes use their own revision mutex so previews do not hold the accounting lock.

- `compile_cache.go`: Caches validated per-peer state against transactional desired-state revisions and access deadlines; controls remain memory-only.
- `compile_reads.go`: Reuses groups, identities, users, periods and purchase reads within one compilation.
- `compile_read_inventory.go`: Reuses nodes, endpoints, disabled squads and restriction reads within one compilation.
- `compile_read_documents.go`: Batches encrypted configuration/snippet reads across branches and preflight nodes within the existing compile scope; errors are not retained. Empty group/identity inventories are cached explicitly, including nil results.
- `compile_reads_test.go`: Hosted regression proving repeated shared inventory reads hit the repository once per compile.
- `compile_cache_test.go`: Hosted regression for revision/boundary invalidation, timeout backoff and the before/after CPU profile with 1, 10 and 100 nodes and 1000 synthetic users.
- `routing_compile_scope_test.go`: Hosted regression proving unrelated routing plans do not enter member inventory while plans touching the node still do.
- `customization_owner.go`: Resolves member and admin-only native-identity routing ownership without introducing Telegram principals.
- `named_services.go`: Admin-managed standalone access terms; quota edits preserve exact usage and probe identities remain separate.
- `named_services_test.go`: Hosted standalone quota, routing scope and disable regressions.

Firewall control: `firewall_types.go` defines typed operational interfaces and public-IP validation. `firewall_service.go` gates capabilities, handles durable reports and quote-bound paid grants. Firewall event batches share the 256-event protocol limit; safe rejection codes distinguish malformed batches, storage failures and ACK failures. `firewall_events_test.go` covers the oversized-frame boundary and secret-safe codes. `firewall_reconcile.go` retires old inventory sources and rebuilds learned detection evidence only when its version changes. `firewall_delivery.go` defers existing outbox leases and sends dynamic deltas or bounded snapshots through the per-node queue. `firewall_source_delivery.go` only sends final, acknowledged removals of retired source sets; no proxy inventory crosses this boundary. Agent capability firewall_control_v1 is required; older agents receive protection disabled rather than autonomous banning. Source transfer slots are bounded separately, preserving capacity for immediate exemptions and traffic acknowledgments.

Task 2 pins protocol/native validation at signed X-Node v2.0.5, source commit `5352b67e882b`. Hosted Quality `39029889093` and signed release `39030494260` succeeded. The version-bound manifest and all amd64/arm64 agent/CLI hashes were independently verified with the production trust root.

Master policy extensions: `native_policy_types.go` owns country, completion-schedule and per-user node-speed contracts. `geoip_asset.go` streams verified public GeoIP data through the existing validation queue and temporary files; `geoip_decode.go` decodes bounded country messages with the pinned Xray protobuf and compacts selected prefixes without expanding addresses. `policy_assets.go` caches parsed generations across global-save validation and compiler views, with completion deadlines and bounded retries, so one save does not download the same full asset repeatedly. `geoip_delivery.go` queues country preparation durably, streams 256-prefix chunks through the per-node command queue for `source_geoip_stream_v1`, then activates a generation reference; older agents keep inline data when it fits the frame limit. `routing_preflight.go` uses that same reference when validating member route changes. `geoip_delivery_test.go`, `geoip_decode_test.go` and `native_policy_test.go` cover chunk bounds, compatibility, asset precision and selector validation. `source_listeners.go` derives country source-filter listeners from the same document with Hysteria inbounds removed, so direct Hysteria clients rely on the node's probe protection instead of the country rule; `source_listeners_test.go` covers that exclusion and preserves unrelated native sections.

`live_availability.go` excludes rolled-back, stale and non-sampled agents from live activity decisions. Service identities are excluded from global connection controls; open-handler limits and exact accounting remain agent-owned. `node_decode.go` requires explicit nonnegative multipliers with at most six fractional digits. Global/node/compiler changes negotiate typed native capabilities against signed X-Node v2.0.6; byte counters remain decimal strings for browser transport.

`client_document_validation.go` validates administrator client subscription documents independently of the agent IP-geodata prohibition. It removes only the client display label `remarks` from the analysis copy for both DIY exports and template previews; server configurations retain their strict root schema. Client-local assets remain in the actual output; analysis-only replacements perform no downloads. Shared geosite block selectors also defer file contents to agent preflight. `client_document_validation_test.go` covers the label-only relaxation, client-local asset preflight and strict server-root rejection.

Shared policy saves additionally resolve actual geosite categories/attributes
through the validation queue. The bounded lookup preserves native asset key
case rules; `routing_domain_validation.go` compiles expanded values with the
pinned native parser/matcher, rejecting invalid expressions inside an asset.
`routing_domain_validation_test.go` covers accepted native semantics and typed
country diagnostics.

Subscription metadata, template previews and downloads share client capability
filtering. `subscription_diagnostics.go` reports omitted endpoint counts and
template failures explicitly; compatible endpoints still render when another
endpoint uses an unsupported client transport. No generated credentials persist.
`subscription_diagnostics_test.go` covers shared diagnostics and per-endpoint
omission behavior.
Selection previews also invoke the selected live renderer with synthetic proxies,
so valid regex matching cannot conceal an invalid selected client document.

Managed routing choices carry published inbound/outbound endpoint IDs.
`routing_endpoints.go` projects only owner-authorized native endpoints and resolves
legacy node-only defaults in their original ID order. IDs persist on the next
member save; unchanged legacy documents retain their digest and runtime behavior.
`routing_endpoint_context.go` binds native credentials and source rules to selected
listeners. Endpoint graphs prove paths across matching listeners, while public
paths expose node IDs and safe host details rather than internal vertex keys.
`routing_member_preview.go` assembles owner-scoped previews through the same
selection and compilation boundaries. `routing_endpoint_selection_test.go`
covers authorized endpoint IDs, legacy order and listener-aware proofs.

`geoip_wire.go` walks the pinned protobuf fields with bounded CIDR records, avoiding repeated-message allocation amplification and retaining cancellation checks during country resolution.

State requests coalesce into the existing reconciler. The socket reader never waits on native compilation or GeoIP refreshes, so queued utility results and traffic acknowledgments keep progressing during external asset work.

`node_multiplier_retired_test.go` verifies that obsolete multiplier inputs cannot affect an offline node edit or its 1x billing default.


## Managed member routing

`routing_types.go`, `routing_templates.go` and `routing_service.go` define encrypted
owner plans, admin matcher presets, revision checks, scoped projections and replay.
`routing_graph.go` validates purchased native endpoint access and shares
`internal/routingmatch` for ordered compilation, tracing and conditional paths.
`routing_context.go` rejects untracked server hops and inconsistent destination
detection when loop safety depends on a conditional-cycle proof. One-way graphs
use the matcher structural proof and support existing sniffing, DNS strategies and
administrator fallback policies. `routing_projection.go` keeps expanded selector sets out of browser
responses; graph labels resolve returned rule IDs to preset names.

`routing_geosite.go` streams the pinned protobuf format with bounded records and
AND attribute filtering. `routing_geosite_download.go` fetches only through
`upstreamqueue`; `routing_assets.go` retains the last valid memory-only generation
with completion scheduling and bounded failure retries. `routing_peers.go` bounds
public endpoint DNS caching and gates maintenance by the existing compile epoch.
`routing_reconcile.go` revalidates dependency changes outside the socket reader.
Dependency drift is re-prepared as a new durable generation instead of disabling
the plan; only durable access loss and node removal are terminal. One member's
transient queue or preflight failure does not block the remaining members, and
unprovable dependencies are retried after transient gaps. See
[docs/routing-resilience.md](../../docs/routing-resilience.md).
`routing_generation.go` distinguishes a dependency-only refresh from a graph edit
and seals reported counters before refreshed credentials install. `routing_continuity.go`
keeps the routing plan while a paid successor is queued/activating or an enabled
automatic renewal may still settle. `routing_resilience_test.go` covers
renewal-boundary continuity, refresh detection, reconcile decisions and
per-member failure isolation.

`routing_transit.go` derives private owner/source/destination/inbound/period-scoped
credentials from encrypted unpredictable seeds. `routing_compile.go` adds those
accounts only in enabling/active phases and generates private native outbounds.
Plans that do not reference the compiling node are skipped before member
inventory, credentials or native validation are resolved; their ownership still
suppresses the member's legacy routing document.
Member subscription and graph APIs never return peer secrets. Client entry counts
remain chargeable activity; authenticated transit retains accounting and bandwidth.

`routing_preflight.go` uses capability negotiation and queued native validation.
`routing_operations.go` resumes durable revoke/capture, apply and enable barriers;
all affected nodes must acknowledge each phase before advancing. Interrupted work
uses existing operation leases and per-node execution queues. Reconnect can wake
exhausted or delayed routing jobs in that same queue and restores a missing job
for an unfinished routing/DIY receipt. Financial periods are never reset.
Revocation acknowledgments fence old route/credential generations before capture.
`master_commands.go` seals the immutable agent journal separately from waiting
for its committed cursor: routing captures counters without waiting for unrelated
historical replay; financial flush/period settlement still requires that cursor.
`routing_legacy.go` converts only exact endpoint/preset matches from pre-start saved
routes; incompatible documents are disabled with encrypted evidence and audit.

`routing_transit_test.go` covers private generation scope and template validation;
`routing_geosite_test.go` covers streamed selector kinds, attributes, truncation and
unknown server contexts. These regressions run in hosted CI.

`routing_tags.go` reserves fixed-width owner tags across inactive plans and native/shared configuration edits, without retaining private peer documents in public APIs.

`routing_deployment_test.go` exercises real authenticated master sockets and a deliberately held node acknowledgement, proving transit accounts stay disabled until every node applies and accounting periods remain intact.
`routing_agent_test.go` returns an unacknowledged journal barrier during that
deployment, proving replay backlog cannot block route activation.

`compilation_errors.go` keeps private native validation details out of default errors/logs while allowing explicit validation diagnostics, including synthetic hosted fixtures.

`routing_peer_guards.go` keeps generated private authentication and endpoint destinations intact through imported client mappers and refuses unmodelled dialer chains. `routing_peer_guards_test.go` guards these bindings and per-hop sniffing uncertainty. Conditional-cycle matcher proofs require fixed destination metadata: active destination overrides and protocol predicates with variable sniffing are refused for those graphs. Structurally acyclic managed graphs cannot revisit a listener, so normal server sniffing, DNS resolution and administrator proxy fallbacks remain usable. Torrent detector changes invalidate the saved graph dependency digest. Existing imported documents remain encrypted and unchanged.

Managed peers omit only the neutral client-export `tcpMptcp:false` flag, using
the native default without changing source settings or subscription policy.
Every other socket override remains visible to the native member guard.
`routing_endpoint_policy_test.go` covers typed endpoint projection, unchanged
client/raw documents and rejection of real socket overrides and dialer chains.

`routing_context_integration_test.go` exercises owner-scoped previews using
valid REALITY endpoints, production-style sniffing, IPOnDemand, proxy defaults and
administrator routing. It verifies eligible endpoint inventory before the routing
proof and checks the specific rejection cause, so a fixture transport failure
cannot satisfy a cycle-rejection case.
It preserves fixed-metadata disjoint cycles and refuses uncertain or overlapping
cycles. `routing_deployment_test.go` carries these production policies through
queued native validation and every acknowledged deployment barrier. Neither
change adds persistence, alters financial periods nor bypasses peer guards.

`profile_metrics.go` projects owner-scoped native node names, sampled activity, rates and availability without changing authenticated agent transport liveness. `profile_accounting.go` builds exactly seven UTC days from native counters plus reconciled source history, using arbitrary-precision aggregation and decimal strings. Historical source bytes retain their unavailable directional split.

`profile_metrics_test.go` covers rolled-back/disconnected/stale activity, independent agent liveness and exact seven-day native/legacy accounting above browser integer precision.

`subscription_selection.go` validates and previews ordered Go regex rules against exact template references. `subscription_formats.go` preserves explicit native links and encodes Base64 output; Stash is a separate client format. `subscription_selection_test.go` covers precedence, Go syntax, retained assignments, legacy policy, exact template selection and REALITY hybrid output. Existing encrypted policy storage and optimistic revisions are retained. Missing legacy Base64 policy inherits native enablement.

Global saves validate shared selectors against native parsing and the configured geosite categories/attributes before publishing. Country decoding uses bounded field reads over the staged asset, including large unselected records. Policy failures expose safe localized codes rather than asset URLs.

`firewall_admin.go` validates canonical IP/CIDR grants and dispatches local delivery observation. `geoip_delivery.go` queues country preparation through the existing durable outbox, uploads bounded chunks, and wakes the state reconciler to publish a generation reference for capable agents. Older agents retain inline data when it fits the frame limit. `geoip_stream.go` bounds protobuf country decoding without loading full records. `policy_validation.go` and `policy_validation_test.go` validate native shared selectors and preserve safe GeoIP failure codes through route preflight. Bulk delivery and CIDR exemptions are capability-gated; dynamic enforcement has independent staging and reserved admission capacity.

`command_outcomes.go` bounds acceptance separately from protocol execution and reports undelivered, rejected, failed and uncertain results. `command_updates.go` reconciles durable upgrades after reconnect, requiring fresh readiness at the current configuration revision. `command_outcomes_test.go`, `command_socket_test.go` and `command_update_reconnect_test.go` cover typed results, concurrent accounting acknowledgments and lost upgrade replies.

`member_metrics.go` derives online, offline or unavailable from fresh native activity within the authorized node scope. Activity sample freshness advances only when the authenticated agent sample timestamp advances, but the expiry is based on its master receive time so bounded agent clock drift cannot make current samples appear stale. This state stays memory-only and is transported in both member and profile projections.

`squad_normalization_test.go` covers normalization of the final enabled squad during native entitlement activation without creating a legacy link. `periods.go` normalizes only mutation paths, preserving current counters and paid-successor accounting.

`purchase_operations.go` guards reset/refund actions against the selected owner and effective purchase before touching the current period. `reset_operations.go` uses the durable reset operation UUID as the new accounting-period ID, proving that specific reset independently of timestamps or natural cadence. `purchase_operations_test.go` covers successor protection, boundary rechecks and proof replay. Keep periods referenced by unresolved reset operations until reconciliation; settled financial facts are never rewritten.

Asset scratch files use process DATA_DIR/tmp. `asset_storage.go` classifies local
create/write/seek failures as unavailable and exposes only fixed safe causes.
`asset_storage_test.go` covers unwritable spool storage without leaking paths.
Policy warnings include safe diagnostics for invalid input and unavailable assets.

Routing asset expansion resolves only template IDs used by current choices.
An unrelated GeoSite template cannot trigger a download or prevent preview.
Missing selected GeoSite URL returns `XNODE_GEOSITE_REQUIRED` (422); unavailable
request/HTTP/storage returns `XNODE_GEOSITE_UNAVAILABLE` (503); invalid asset,
hash or selector returns `XNODE_DOMAIN_SELECTOR_INVALID` (422). Refresh failures
keep verified stale data for the same dependency key. `routing_assets_test.go`
covers policy codes/stale data; the managed deployment regression includes an
unused GeoSite template with no URL. Existing plans whose digest included unused
templates can encounter one `routing_dependencies_changed` transition when the
new selected-only digest is first reconciled, followed by normal revalidation.

`routing_access.go` uses authoritative endpoint/entitlement inventory to prune
inaccessible choices during the existing master reconciliation, including pending
or disabled plans. Read failures and transient node telemetry cannot cause
pruning. A paid renewal boundary is not access loss: while a successor is queued
or activating, or an enabled automatic renewal may still settle, combo and
legacy choices are retained while PAYG/hourly references remain governed by
their own accounts and enrollments. Retained rows emit no rules without an
effective entitlement. Remaining choices are disabled for review because
removing a rule can expose a formerly shadowed cycle. Previous operational
choices and superseded legacy customization rows are erased; immutable encrypted
conversion evidence and financial records remain intact. Existing
compile-version triggers and serialized master synchronization remove routes and
private peer credentials from connected nodes and on reconnect, without a second
queue or ACK blocking.
`routing_access_test.go` and managed deployment coverage verify selection and
native revocation; database tests verify ownership/revision fencing and invalidation.

## Signed firewall release and panel pin (2026-10-20)

Pinned public protocol/native packages to
`v1.0.2-0.20261020144243-153cf0d10bbc`, signed agent `v2.0.11`, source
`153cf0d10bbc895f8a05cf4aa7b2fb8b02be2007`.
Hosted [Quality 39441871106](https://github.com/VanSail-Opensource/X-Node/actions/runs/39441871106)
and [signed release 39444134348](https://github.com/VanSail-Opensource/X-Node/actions/runs/39444134348)
passed. The production-trusted Minisign manifest/version and downloaded agent/CLI
hashes were independently checked through the agent's release verifier.

| Artifact | SHA-256 |
| --- | --- |
| x-node-linux-amd64 | 6f61ef51d350c722a3fb37aac7472af675b60f9035f043cbe015b154e71ded62 |
| x-node-linux-arm64 | 0835c52811ee0d2a7bca36dda3596beab06f02ba327f48b8ce4023354f6838dd |
| xnodectl-linux-amd64 | aa51791ffb33f700c30b76014e9d0c9b5e8cdf7cc3e2d96f8f15f348cab5a413 |
| xnodectl-linux-arm64 | 342c30d2a6f05ac095cd7d8bbfc5cadc5ba9fac29d0b56236cd6efce9b3b9e49 |

`master_logs.go` exposes only one selected node's 500-entry volatile log buffer;
agent `logs.report` remains the delivery source, and dropped counters are decimal
strings. `Metrics(ctx,true)` omits log copies while default metrics remain
compatible. No log/source-IP database persistence or extra agent command is added.
`master_logs_test.go` covers bounded buffers, safe large counters, stale/offline
snapshots, unknown nodes and the lightweight metrics projection.

Access pruning also fences the authoritative compile revision and defers across
a purchase boundary. A concurrent entitlement/endpoint change rejects stale
cleanup instead of removing routes under newly granted access.

## Metered route speedtests

`route_speedtest_types.go` defines owner-scoped start/result contracts.
`route_speedtest.go` authorizes a saved active revision and successful trace,
checks synchronized native nodes/capabilities, and reuses durable receipts.
`route_speedtest_worker.go` visits adjacent pairs in order; intent/results live
in existing operation items. `route_speedtest_pair.go` uses the per-node command
queue with administrator URLs, 32 MiB download, 8 MiB upload and 60 seconds per
pair. Waiting stays outside the send queue, leaving traffic ACKs independent.
`route_speedtest_results.go` exposes public IDs/measurements and estimates
latency by summing medians, with each bandwidth direction taking its minimum.
Command-result byte counts never bill. `route_speedtest_test.go` covers estimates,
sanitization, owner isolation, partial recovery and uncertain-command non-replay.

`metered_probe_types.go` defines the minimal durable probe-to-origin reference.
`metered_probes.go` grants one unique short-lived identity per operation pair,
revokes it, and records acknowledged barriers. `metered_probe_access.go` limits
admission to the exact target inbound, current origin period, entitlement and
quota; expiry is bounded by both probe and paid access. Probes have isolated
credentials and no member routes/customizations. Normal period-tagged journal
samples retain deduplication and multiplier rounding while charging the origin.
Late tails after expiry/revoke still count. Reset/renewal/refund revoke probes
and flush their journals before sealing the period; offline journals hold
settlement without touching a successor. References and completed pair results
survive restarts. Recovered processing intent becomes uncertain, completed pairs
never repeat, and later pairs stop after uncertainty.

Subscription response precedence: explicit client format bypasses ordered UA
rules; otherwise the first rule chooses its exact template or Browser page.
Legacy rules omit action and retain template selection. Unmatched clients retain
built-in detection/Base64 and browser Accept negotiation. Empty HTTP templates
are UA-only (no member/client defaults), document-free, and default to 403.
`subscription_responses_test.go` covers shared preview/live selection, explicit
precedence, legacy rules, empty status validation and access-before-selection.

`firewall_search.go` parses address/CIDR queries with normalized network boundaries
and returns bounded pages. Zero-hour administrator requests require the exact
active generation and release only that ban; positive-hour grants keep existing
exemption behavior. Release receipts use the same firewall version/delivery lane.

`diy_types.go` defines typed owner DIY contracts and minimal persistence interfaces.
`diy_read.go` decrypts one document and annotates exports only; members without
hosts keep options for future hosts. `diy_preview.go` combines existing routing
proofs with shared tuning compatibility and pinned client-parser validation.
`diy_save.go` validates both sections, fences settings/routing revisions and
atomically commits a receipt, encrypted settings and changed routing desired
state. `routing_prepare.go` shares the existing deployment preparation with
legacy routing APIs. `diy_test.go` covers concurrent same-key retries, encryption,
inheritance, export isolation and pinned-parser output. Saved settings are local;
combined receipts continue through the existing routing deployment guarantees.
`diy_split.go` derives split names from current host labels and reserves ordinary
names first. Collisions, including reversed IPv4/IPv6 pairs, receive a numeric
suffix only in the export; saved endpoint/profile references stay unchanged.
`diy_split_names_test.go` covers reversed pairs, ordinary-host collisions and
successful Xray/Mihomo/Stash template rendering with unique names in hosted CI.

`payg_subscription_test.go` covers two PAYG squads sharing a published endpoint
and PAYG overlapping a paid combo. Metadata and bearer downloads use the shared
subscription name allocator after client mapping, keeping every distinct billing
credential while avoiding duplicate sing-box tags and Mihomo/Stash names.

`metered_probe_lifecycle_test.go` exercises compiled admission, quota exhaustion, expired grants, real traffic reset and refund quiescing with duplicate late probe samples. Unfinished barriers retain origin evidence and leave successor periods uncharged.

## Firewall and member whitelist

`firewall_policy.go` owns global rules and preserves common legacy fields during
migration; `firewall_policy_test.go` checks conflict defaults.
Inventory `source_match` is explicitly disabled in defaults and every effective
policy, including saved enabled values. The pinned protocol enables omitted
rules, so an explicit disabled entry preserves agent compatibility without
distributing inventories. Migration ignores retired-rule conflicts; historical
ban evidence remains intact.
`firewall_protected.go` derives managed addresses from authenticated telemetry and
unproxied authenticated peers, synchronizing them through existing per-node jobs.
Those observations remain memory-only; shared CDN ranges are never inferred as
node addresses. Existing false-positive detections are released, not erased.

`firewall_hosts.go` refreshes hostname A/AAAA through the existing validation
queue. Its bounded volatile cache accepts only exact answers corroborated by the
associated authenticated node or explicit DDNS addresses, so shared CDN edges
cannot become exemptions through endpoint DNS. `firewall_hosts_test.go` checks
mixed-family answers, mapped addresses, CDN answers and removal of origin proof.

`diy_document.go` reads versioned encrypted desired state and legacy bare client
overrides. `whitelist_policy.go` validates exact public IPs, preserves insertion
order and derives capacity. `whitelist_deployment.go` requires applied, fresh node
state before reporting protection active. `diy_deployment.go` shares the existing
operation dispatcher with routing and waits outside the socket reader.
`master_access.go` shares the publish-all-then-wait barrier between whitelist and
routing deployment. Every affected node receives its configuration before the
first acknowledgement wait. Failures retain the stage and node ID, and whitelist
confirmation timeouts list pending node IDs without exposing member IPs.
`deployment_nodes_test.go` holds one acknowledgement to verify publication to
other nodes, mandatory barriers and contextual cancellation in hosted CI.
`diy_reconcile.go` trims excess newest IPs for free; disconnected nodes do not
change saved eligibility. Empty enabled lists deny all member credentials.

Detection-enabled nodes report deltas below the threshold and enforce only
master-authorized bans. Counters are per IP/rule across nodes, bounded to 65536
active keys and cleared on master restart or policy changes. Durable event cursors
prevent replay charges/counts; no new raw observation table is introduced.
Non-Reality country filtering precedes whitelist/node exemptions. Paid unban is
hidden and new purchases rejected in whitelist mode; existing retries replay.

Member source-IP checks apply to ordinary subscription credentials on ingress.
Routing uses independently derived transit credentials on later hops; those
remain authorized and metered without comparing the first node's address with
the member whitelist. Changing a whitelist revokes matching ingress sessions
only; revoking routing credentials still closes transit sessions.

`diy_split.go` projects accessible XHTTP host pairs and appends current-reference
split exports. Both legs require the same native node, inbound and session path;
returned host metadata carries the exact mode list for each download pair while
the compatibility union remains available to older clients. No generated
credentials enter the encrypted settings document. Exports omit an
ineligible pair immediately; `diy_reconcile.go` later removes its references for
free. Authoritative inventory errors and pending provisioning defer cleanup;
node connectivity never decides ownership. `diy_changes.go` compares normalized
settings, whitelist and routing. All DIY saves are free and consume no IP Lookup
allowance; the existing transaction still fences revisions and idempotent replay.

`diy_split_test.go` covers current-reference exports, disconnected nodes, immediate
host-access exclusion, pinned-parser validation and free reference reconciliation.
`diy_split_catalog_test.go` covers per-pair mode metadata and deduplicated legacy
compatibility modes in hosted CI.

`command_arguments.go` requires `incremental_reload_v1` for runtime reload,
rechecks capability at execution and forwards typed optional forced recovery.
`operations.go` retains durable command replay and per-node execution ordering.

`shadowsocks_methods.go` validates managed native listener methods. Node ownership is native-only. Imported usage/history are frozen accounting evidence; there is no source integration, importer, synchronization loop, or migration gate in runtime compilation.

`diy_retirement.go` upgrades encrypted version 0/1 documents to version 2 before HTTP starts. It removes retired tuning, retains whitelist and split references, and reseals with a fenced revision increment without charging or modifying receipts. Unknown versions or malformed retained values refuse startup rather than erase data.

`diy_retirement_test.go` checks encrypted cleanup, retained whitelist/split references, fenced revisions, unchanged ledger and idempotent upgrades.

Member deployment latency: `operation_capacity.go` reserves four bounded member
slots alongside the three ordinary and one urgent firewall slots in the existing
durable executor. `master_operations.go` retains heartbeat/generation fencing,
joins cancellation and converts deferred panics to sanitized failures.
Transactional outbox resource keys still serialize work for the same member;
independent members can advance while another waits for an agent.
`operation_capacity_test.go` covers capacity, panic/cancellation and receipt scope.
Routing/DIY attempts have a five-minute total deadline; each phase is bounded to
one minute and sends all phase configurations before awaiting acknowledgements.
Revoke/apply/enable barriers remain mandatory. A disconnect ends an observed
applied-state wait promptly. `diy_deployment.go` reads the existing receipt's
`whitelistChanged` evidence: routing-only changes do not redeploy whitelist state
to unrelated paid nodes. Legacy receipts retain the conservative barrier.
The existing outbox retry policy caps only desired-state routing/DIY delays at
30 seconds, preserving external-payment uncertainty rules and the ten-attempt
limit. Startup and reconnect recovery resume the same receipt and phase,
including routing jobs left in the former multi-hour retry delay.

## Automatic PAYG

`payg_types.go` defines optional repository contracts and owner-only transport models. `payg_provision.go` provisions encrypted internal member/squad/node identities through the shared outbox; they have no independent bearer subscription. Active and paid queued combo squads suppress PAYG admission without deleting prepaid balances.

Agreement completion immediately queues provisioning for configured PAYG squads,
including wallets funded before onboarding. Existing 30-second bounded scans and
wallet-change jobs repair missing tuples after catalog or node changes.
`onboarding_payg_sync_test.go` follows funded signup through the real durable
dispatcher, agent synchronization, committed recovery barrier and expiring PAYG
authorization; idle provisioning must preserve the wallet and prepaid counters.

`payg_compile.go` compiles owner restrictions and exact inbound bindings. `payg_leases.go` overlays 60-second authorization expiry in 20-second intervals without native recompilation. It adjusts the lease timestamp by fresh authenticated agent clock offset, capped at two minutes, so bounded node clock drift does not expire every lease immediately. Reconnection first withdraws PAYG and commits a traffic-flush barrier through a deferred outbox operation. Missing capabilities, stale telemetry and journal failures stop new leases. `payg_observe.go` consumes memory-only connection observations for settlement of pending traffic only when at least 100 MiB (104,857,600 bytes) was metered in the rolling 72-hour window on that member/squad/node. Initial funds, idle connections and sub-threshold probes never prepay; exact raw totals survive reconnects; only unbilled bytes inside the rolling window count toward a newly purchased GB. The unchanged agent can overrun between reports; authoritative delayed usage may debit the wallet below zero.

`payg_inventory.go` separates owner inventory from financial admission and maps internal principals to their member for controls, metrics and routing. `payg_export.go` excludes PAYG entries from newly generated subscriptions when the current wallet balance is nonpositive; Combo entries remain available even on shared endpoints. This export filter does not revoke existing credentials or change saved preferences or prepaid credit. `payg_view.go` projects squad status, exact raw bytes and wallet spend over the last 72 completed hours, with a minute-aligned asOf. Unused prepaid bytes remain scoped to their original node and survive reconnects until 72 hours after their original charge.

`payg_owners.go` shares member ownership across activity counters without reading credentials each reconciliation. `payg_leases_test.go` verifies lease-only refresh, stale/unsupported/debt admission and whitelist gating; `payg_access_test.go` covers paid combo precedence, retained unfunded inventory and parent disablement. `payg_subscription_test.go` covers balance-gated exports and shared Combo/PAYG endpoints. These regressions run only in hosted CI.

`master_peer.go` contains per-socket transport, synchronization and ephemeral PAYG lease state; the master retains ownership of peer lifetimes and the existing execution queues. Existing oversized application/catalog composition files retain their established boundaries; added PAYG behavior lives in focused modules.

PAYG member opt-outs apply to provisioning, inventory/DIY, native compilation, lease overlay and observed-connection purchases. Dormant account identities and prepaid credit remain unchanged; native journal ingestion always settles delivered traffic. PAYG reconcile jobs wake the existing node reconciler, with no new queue or protocol commands.

`payg_preferences_test.go` covers member opt-outs across subscription inventory and compilation, re-enabling and paid Combo isolation.

`payg_account_provision.go` owns idempotent internal principal/account creation; reconciliation and coverage remain in `payg_provision.go`.
# PAYG catalog reads

PAYG compilation returns before reading products/topology when a node has no
accounts. PAYG provisioning, inventory, export and display use stockless local
overrides; derived catalog capacity never enters the reconciliation deadline.

## Negotiated compute pool

`compute_pool.go` exposes `Master.Compute(ctx, task, accept)`; `compute_batch.go`
coalesces compatible versioned inputs for 100 ms, at most 64 tasks / 256 KiB.
The ordinary command writer and durable accounting acknowledgements retain their
existing paths. `compute_award.go` uses bounded concurrent announcements, atomic
first-claim awards and one reservation per connection through durable acceptance.
`compute_collect.go` assembles bounded low-priority zstd chunks outside socket
readers. Attempt hashes, connection identity, deadlines and renewable leases fence
stale results; disconnects and failed attempts reannounce work. Accepted numerical
receipts are idempotent and disposable transport buffers remain memory-only.

CPU pressure selects remote execution after two five-second samples at 90 percent;
30 seconds below 75 percent restores one bounded local worker. Old agents remain
usable and are excluded from compute awards. Nodes receive numerical inputs and
opaque hashes only. Financial policy, coupon authorization and SQL access stay on
the master. Protocol details are documented in the pinned agent compute reference.

- `accounting_purchase.go`: Closes native purchase periods without revoking a paid successor.

- `accounting_usage.go`: Projects native/historical usage into generic accounting.

- `compute_fixture_test.go`: Hosted negotiated transport fixture.

- `compute_pool_test.go`: Hosted batching, reservation and disconnect reassignment regressions.

- `compute_receive.go`: Bounded compressed result assembly outside socket readers.

- `compute_remote.go`: Attempts, leases, announcements and connection fencing.

- `host_metadata.go`: Current per-principal rate/multiplier and explicit applied routing metadata.

- `host_names.go`: Owner-scoped names and {price} interpolation at the export boundary.

- `host_names_test.go`: Hosted owner isolation and export consistency regressions.

- `subscription_repair_test.go`: Hosted repair regression preserving valid identity and accounting.

Native usage and immutable refund evidence; historical regression name retained: `account_statistics.go`, `rollover_usage_test.go`.

- `compute_validation_test.go`: Hosted concurrent claim, stale connection/attempt, malformed chunk and decompression-limit regressions.

`account_purge.go` closes every owner paid/PAYG period through the existing synchronization/flush barriers. Offline or review periods remain pending. `account_rejoin.go` rotates purged PAYG credentials and starts an empty period while keeping charge numbering and historical periods immutable. Closing PAYG periods accept final acknowledged journal tails; settled periods reject further traffic.

`group_deletion.go` enriches admin group reads and protects unexpired encrypted service assignments. `group_deletion_endpoints.go` removes group references from encrypted endpoint options while preserving empty allow lists; its ciphertext updates commit with deletion. `group_deletion_test.go` and `group_deletion_endpoints_test.go` cover services and deny-all behavior. Named services exclude member PAYG identities, including purged tombstones.

Access repair uses `access_repair.go` and `access_recovery_test.go` to reload current entitlement state under the service mutex, retry stale state three times, and defer operation ownership without exhausting retries. `ensureIdentity` shares that lock; new allowances use the same transactional entitlement fence. `RepairXNodeAccessPeriod` atomically restores a unique open allowance or resumes the current closing allowance; settled/review records and usage counters remain unchanged.

`compile_members.go` preloads disposable member inputs for one compilation. `compile_members_integration_test.go` checks equivalence and fresh usage. `master_sync_workers.go` rotates two bounded workers, skips occupied peer locks and joins before the next pass; its hosted test verifies capacity and cancellation. `command_notifications.go` coalesces acceptance frames and retains terminal results, covered by `command_notifications_test.go`. Definite utility-busy errors defer the receipt without consuming an execution attempt. Saved telemetry intervals remain unchanged; new defaults report every two seconds.

`master_send.go` retains the single outbound queue and safe queue/write timing. `master_commands.go` records command class, delivery outcome and elapsed time at debug level without exposing arguments or results.

`master_sync_workers_test.go` verifies bounded fleet execution. `master_controls.go` retains monotonic volatile control updates. `command_operation.go` owns receipt attempts, definite busy deferrals and terminal outcome persistence.

`access_repair_fences_test.go` checks refund ownership, scheduler exclusion and stale-pointer rejection against the real store.

`reporting_defaults_test.go` protects explicit saved intervals and historical omitted/zero intervals while new configurations target two seconds.

`master_firewall_socket_test.go` exercises dropped liveness pongs and lost confirmation replies over an authenticated socket, preserving the transaction guard until confirmed.

`master_registration.go` binds authenticated peer replacement and durable boot
stream closure under the existing enrollment lock. `traffic_streams.go` exposes
optional native repository contracts and dual legacy/boot flush barriers.
Settlements wait on real committed cursors; a closed boot settles surviving
known usage without synthetic cursor advancement. Normal control/traffic receipt
remains independent of long financial barrier waits. Negotiated binary reports
use the public bounded codec in `master_report_frames.go`.

The coordinated protocol dependency currently tracks the boot-stream implementation commit for hosted integration. Final delivery requires replacing this intermediate pin with the source of the verified signed amd64/arm64 release.

`traffic_batch_types.go` defines the typed repository batch contract.
`master_traffic_batch.go` joins the existing inbound traffic lanes into bounded
32 MiB collection, flushing after 100 ms, 256 reports or the payload allowance.
It never acknowledges a report before the shared SQL commit. Fleet raw traffic
input is independently bounded to 64 MiB and released on receiver shutdown.
Large pre-upgrade legacy journals retain isolated inbound execution so they do
not consume the modern fleet's complete batch allowance. Commands/telemetry keep
their independent lanes and the existing per-node outbound execution queue.
`master_traffic_batch_test.go` verifies the deadline, immediate full batches,
commit barrier, memory budget and joined shutdown using hosted fake-time tests.

`global_cache.go` privately retains the validated fleet settings snapshot. Only
`SaveGlobal` writes this document after startup migrations, and publishes a deep
copy after the durable settings/audit write succeeds. Readers receive independent
provider credentials, rule maps, selectors and interval pointers, including when
HTTP responses redact secrets. Failed reads are never cached.
`global_cache_test.go` covers concurrent cold reads, failed and committed writes,
and caller mutation of secrets and policies. Adding a reference-valued global
field also requires extending `cloneGlobal`.

Active-IP sampling is mandatory. `DefaultGlobal`, `DecodeGlobal` and `SaveGlobal`
normalize the retained legacy telemetry opt-out to enabled; older deployed agents
therefore receive true without changing their saved sample/report intervals.
`reporting_defaults_test.go` also protects this upgrade behavior.

`master_activity.go` accepts only authenticated current-peer snapshots and
independently negotiated observation history. Actual sample timestamps must be
monotonic and no later than report publication (with bounded future clock skew).
`live_availability.go` bounds activity by both report receipt and actual sample
age, respecting separate configured intervals; report replay cannot refresh
old activity. A negotiated agent with no first sample remains unavailable.
Legacy measured snapshots retain decoding compatibility. `activity_observations_test.go`
covers old/replaced/unnegotiated frames, missing samples and slow sample cadence.
Managed-address protection also fences replaced peers before publishing IPs.

## Automatic lease registry

`automatic_lease_types.go` bounds the memory registry to 100,000 canonical public
IPs and eight source-node watermarks per IP. Saturation is explicit in read views.
`firewall_addresses.go` shares pre-parsed public-address exclusions with existing
firewall administration. `automatic_lease_observe.go` creates observation-time
leases, ignores duplicate/stale reports and renews only after half the current
interval, using twice the current configured duration. Configuration changes
apply to future grants/renewals; older global writers preserve existing settings.

`automatic_lease_heap.go` indexes expiry without stale heap entries. Expiry clears
IP and cached snapshot state without removal commands. `automatic_lease_delivery.go`
builds shared generation-sorted snapshots only after grants, renewals or expiry,
returns private batches of 256 and tracks bounded durable-ban-release intents.
Nodes need one acknowledged generation watermark rather than per-IP ACK maps.

`automatic_lease_activity.go` consumes authenticated current-peer observation
timestamps, including sources with disabled firewalls or unsynchronized runtimes.
It does not query SQL on the report path. Old snapshots without negotiated actual
sample timestamps remain decodable but cannot manufacture fresh lease evidence.
`automatic_lease_service.go`, `automatic_lease_page.go` and
`automatic_lease_views.go` provide private paginated read projections and pending
node counts from durable node configuration plus volatile delivery watermarks.
Settings alone are durable; all IPs, source metadata and delivery state are lost
on master restart and rebuilt from fresh authenticated activity.

`automatic_lease_test.go` and `automatic_lease_activity_test.go` cover exact half
boundaries, configurable non-compounding renewal, expiry, IPv4/IPv6 normalization,
source saturation, replay, disabled-firewall sources and replaced peers.
`automatic_lease_performance_test.go` runs its isolated 10,000-IP timing/memory
gate only in hosted CI. It measures creation, repeated observations, renewals,
expiry and initial/reconnect synchronization, rejecting duplicate grants and
steady release requests. Each registry pass must finish within one second and
retained manager memory must remain below 64 MiB. Hosted artifacts include CPU
profiles, runtime user-CPU estimates, latency, allocations and retention.

`automatic_lease_runtime.go` owns joined one-second expiry maintenance, so offline
nodes do not retain expired IPs indefinitely. Observation/configuration changes
coalesce wakeups independently of sampling and command receipt.

`automatic_lease_send.go` delivers private 256-grant commands through the existing
per-node writer, advancing a single generation watermark only after a matching
epoch/generation completion. `automatic_lease_runtime.go` rotates two joined
senders with bounded execution and retry backoff; ordinary telemetry and traffic
ACKs remain independent. New connections start at zero, and firewall enable
resets its watermark while fencing older in-flight completions.

`automatic_lease_bans.go` batches exact-ban release intents without steady report
writes. Event exclusion reads only addresses in the current event frame; a
post-commit recheck closes the observation/event/release race. Existing broad
ranges remain in the durable ban bank and agent exact-address exemptions protect
only leased addresses. Replayed firewall journals do not rewrite unchanged
cursors. `automatic_lease_bans_test.go` covers zero steady release calls and late
ban commits; `automatic_lease_socket_test.go` exercises authenticated batched
synchronization, unchanged grants and firewall-enable acknowledgement fences.

Enrollment revocation removes peer authority before closing the socket; queued
activity cannot create leases after token rotation. `master_revocation_test.go`
covers revoked reporters and drained deletion. Registration and deletion share
the same master lock, preventing a handshake from crossing the drain check.

`profile_activity.go` provides bounded owner-scoped sampled IP observations with
actual last-activity times, exact counts, truncation and distinct available,
stale and unavailable states. Old agents without capture-time negotiation do
not claim fresh IP evidence. `profile_payg_history.go` composes recorded 72-hour
per-node PAYG totals and hourly history, including removed/unavailable nodes.
`profile_activity_test.go` covers scope, freshness, precision, bounds and native
graph counters. Native graph counters exclude imported evidence; canonical
period/lifetime and historical accounting fields retain their original meaning.

`automatic_lease_settings.go` merges lease settings under the existing global
writer lock. The scoped global update preserves every unrelated committed field;
legacy full-document writes that omit automaticWhitelist preserve newer leases.

Disabling automatic creation stops new leases and renewals. Existing leases keep
their deadlines, update activity statistics and synchronize to joining nodes
until expiry. Canonical address searches accept expanded IPv6 notation.

Profile IP output has a 256-address node limit and 4096-address response budget.
Selection copies truncated rows so excess history does not remain retained by a
clipped slice. Capturing immutable report headers under the master lock keeps
sorting and projection outside report ingestion critical sections. A truncated
empty list never claims no activity; it remains visibly incomplete.

Owner activity includes valid private/LAN addresses as reported; only the separate
automatic firewall lease classifier requires public routable addresses.

Member connections use `member_connections.go` and `member_connection_leases.go`.
The existing sampled native reports populate bounded principal/IP associations in
memory, sharing the lease lifetime and aggregate capacity. GET resolves owner
principals outside socket readers. Manual grants require normal account status,
completed onboarding, active combo or positive balance, and enabled automation;
canonical public IPv4/IPv6 use the existing volatile fleet delivery and exact-ban
release mechanism. Duplicate grants preserve deadlines and manual grants have no
observed time until telemetry. Initial lease time is immutable through renewal.
DIY document version 3 retires only whitelist data through encrypted resealing;
tuning, split and routing choices survive. Paid exemption creation is retired.

Member subscription readiness is centralized in `subscription_readiness.go`.
Only exact authenticated state.applied membership can stage proof. Joined master
reconciliation persists first_synced_at once on the owner identity; no traffic
acknowledgment waits for it. Old node-wide applied revisions cannot backfill user
membership, so pending owners request a fresh per-node synchronization. Metadata,
Home URLs, host links and bearer exports withhold credentials until proof; normal
export/access rules continue afterward. Service subscriptions remain independent.

`routing_entitlements.go` prunes authoritative endpoint/entitlement loss,
including administrator-disabled product groups. Disabled nodes/endpoints and
provisioning or quota pauses suspend the graph while preserving IDs.
`routing_inventory_view.go` keeps refreshed labels for unavailable choices.
Dependency changes compile via the existing revoke/apply/enable queue;
legacy node-only graphs never rebind automatically after dependency changes.
Owner PAYG history uses the existing exact recorded 72-hour per-node repository.

Node removal runs node.remove as an existing durable operation, never a wire command. It fences encrypted routing reads with the compile revision, prunes squad/service bindings and routing choices, fences new admission, waits for apply and committed quiescent traffic barriers, seals per-period evidence, then revokes the exact drained peer. Offline participants and accounting gaps remain explicit resumable dependencies. Historical nodes retain evidence behind retired_at; never-enrolled unreferenced nodes are deleted.

`member_connection_types.go` defines private transport; `member_connection_leases_test.go` checks grants and isolation. `node_removal.go` runs queued removal. `subscription_readiness_test.go` checks exact-peer applied proof.

`member_connections_eligibility_test.go` checks funding, onboarding, account eligibility, automation gates, duplicate expiry and unchanged balance.

`service_enrollment.go` rotates enrollment through the master lock and rejects retired nodes.

Period settlement and routing deployment proof: `period_closure.go` fences and seals only recorded participants; purge and background settlement never hold the shared service mutation mutex over network waits. `routing_config_proof.go` and `routing_deployment_proof.go` validate the sent native rules, selected principals/listeners, transit generations and applied revisions. See [job and routing reliability](../../docs/job-routing-reliability.md) for constraints and incident evidence.

`native_job_dispatch.go` dispatches queued native work, completes superseded disabled-node intents, and preserves offline/capability waits without exhausting delivery attempts. Access repair retains settled/ambiguous accounting for review instead of granting another allowance. `native_job_dispatch_test.go` covers the firewall delivery wait classification: undelivered/uncertain commands stay retryable, while rejected or agent-failed commands do not.

Node-level bandwidth overrides are retired. Stored node settings are historical evidence: `node_settings.go` decodes them through a compatibility reader that ignores unknown or retired keys (including the former top-level `bandwidth` field) while rejecting malformed JSON, documents that are not a JSON object (`null`, arrays, scalars) and wrong-typed known fields. `Service.Node`, DNS preservation during saves and automatic firewall lease reads share that boundary. Reads never rewrite stored state, and subsequent saves re-encode only current typed settings, so retired keys are dropped. The public SaveNode request type and its validation remain strict, so writes cannot persist unsupported fields. Native per-user speed capabilities remain available for commerce policies. `node_settings_test.go` covers encrypted reads, rejected malformed/foreign-node state, save round trips and populated lease status; `node_settings_decode_test.go` covers tolerated retired keys and rejected non-object documents directly, and `node_dns_test.go` retains legacy resolver compatibility when old bandwidth data exists.

`node_removal_routes.go` prunes typed node choices, freezes member routing saves, retains every affected revocation participant and synchronizes applied credential revocation before final retirement. `node_removal_routes_test.go` covers saved/previous choices, unrelated plans, replay and concurrent removals. Empty plans stay disabled; surviving choices reconcile after removal.

`update_receipt.go` defines bounded native transaction evidence and immutable pre-dispatch intent. `update_prepare.go` resolves a stable target once inside the existing operation worker and persists it before mutation; offline/status/release dependencies use existing outbox waits. `update_wait.go` polls documented `update.status`, matches transaction ID/version/previous release and requires fresh ready/synchronized telemetry at the current revision on that same peer. `update_result.go` retains intent and sanitized native evidence on success, failure or review; utility paths never enter receipts. `command_updates.go` never repeats interrupted mutations. Only an actual completed, receipt-bound install no-op can finish without a new native transaction. `update_receipt_test.go`, `update_readiness_test.go` and `command_update_reconnect_test.go` cover mismatched IDs/versions, pending status, legacy uncertainty, stale/unready/replaced peers and reconnect confirmation. See `docs/update-reconciliation.md` for extension constraints.

Named-service quota writes default to `NO_RESET` for new terms and preserve the
authoritative stored cadence when omitted on edits. The editor exposes no reset
control. `ServiceAccess.NamedWrite` and the database writer reject cadence changes;
the transaction locks the current period and updates only its quota, preserving
usage and reset evidence. Grandfathered/imported cadence readers and purchased
legacy schedules remain compatible. A recurring term without current-period
proof stays untouched and rejects edits; no data conversion is performed.
Hosted named-service cadence regressions cover new/changed writes, omitted values,
legacy edits, periodless uncertainty and accounting preservation.

`named_service_cadence_test.go` verifies omitted/new cadence, unchanged legacy
edits and rejected changes without rewriting credentials or accounting.

Strict period settlement: `period_strict_closure.go` captures the exact native
owner, purchase, period and durable closure generation while fencing admission
under the mutation mutex, then drops that mutex before network waits. Refund
reconciliation and generic quiescence recheck the captured target before success.
`period_closure.go` uses mandatory generation-bound repository methods for both
participant sealing and finalization, including checks of closed snapshots.
`period_strict_closure_test.go` holds native barriers across unrelated mutations,
cancellation, reopen and concurrent successor creation. The compatibility
fixtures and stale-closed admission regression live in
`period_closure_fixtures_test.go`. See [period closure](../../docs/period-closure.md)
for native guard behavior, persistent fencing and extension constraints.

`member_refund_snapshot_test.go` checks real stored reopen races after closed
capture and during refund history aggregation. Refund snapshots retain the
original closure generation, recheck it after aggregation and reject any
unfinished same-purchase period before returning a quiesced total. Node removal
uses the mandatory removal-fenced sealer; it never invokes an unqualified
legacy or stream participant write.

`hourly_credit_*.go` extends the established per-node queue with funded-only,
durable shared byte reservations, adaptive delivery and exact seal/ACK recovery.
Retirement keeps offline/unknown obligations pending and preserves agent control.
See [hourly credit delivery](../../docs/hourly-credit-delivery.md) for repository
interfaces, worker bounds, retry ordering and lifecycle extension points.

Credit delivery responsibilities:

- `hourly_credit_repository.go` defines the durable production interface;
  `hourly_credit_receive.go` authenticates progress/proofs and commits before ACK.
- `hourly_credit_queue.go` bounds coalesced state; `hourly_credit_worker.go`
  schedules fair work with the service clock and bounded per-agent concurrency.
- `hourly_credit_demand.go` filters admission hints; `hourly_credit_sizing.go`
  derives disposable throughput estimates from confirmed usage and RPC latency.
- `hourly_credit_producer.go` reconciles retained obligations before allocation;
  `hourly_credit_replenish.go` reserves only free or already-funded bytes.
- `hourly_credit_intent.go` resolves uncertain SQL outcomes;
  `hourly_credit_order.go` discovers lower issues before installing later credit.
- `hourly_credit_delivery.go` sends exact install/seal/ACK arguments through the
  existing node queue; `hourly_credit_retirement.go` exposes lifecycle fences.
- Hosted `hourly_credit_integration_fixture_test.go` and
  `hourly_credit_integration_test.go` exercise PostgreSQL and real TLS sockets.
- `hourly_credit_intent_test.go`, `hourly_credit_order_test.go` and
  `hourly_credit_uncertain_test.go` cover ambiguity and predecessor ordering.
- `hourly_credit_idle_test.go`, `hourly_credit_prefetch_test.go`,
  `hourly_credit_residual_test.go` and `hourly_credit_sizing_test.go` cover bounded
  activity, replenishment and exact residual availability.
- `hourly_credit_clock_test.go` checks injected-clock scheduling and retry edges.
- `hourly_credit_demand_test.go` checks cold zero/unknown-hint suppression after
  completed scans while preserving positive-demand and financial recovery wakes.
- `hourly_traffic_recovery_test.go` keeps complete mixed journals admissible after
  hourly storage/admission failure without enabling live grants or commands.

Hourly inbound capacity is implemented by `hourly_lane.go` (validated routing),
`hourly_lane_control.go` and `hourly_lane_live.go` (bounded owner coalescing and
fair live-control batches), plus `hourly_lane_financial.go` and
`hourly_lane_financial_worker.go` (cumulative/proof retention, bounded SQL turns
and fair retry). `hourly_lane_control_test.go` checks full 8192-owner captures,
receipt stability and native-drain ACK ordering. `hourly_lane_financial_test.go`
checks capacity, cancellation, conflicting scope and exact replay;
`hourly_lane_retry_test.go` checks poisoned-proof/progress isolation.
`hourly_lane_capacity_test.go` sends a full owner/ACK burst through TLS while
SQL blocks and verifies native journal/command continuity. Bounds and the agent
replay obligations are documented in `docs/hourly-master-enforcement.md`.

`hourly_receive_batch_test.go` checks batched allocator admission/ACK adapters,
original socket receipts and owner/node policy boundaries.

Account cleanup enters `CloseAccountPeriods(ctx, model.AccountPurge, jobID)` in
`account_purge.go`. The exact purging generation and retained-generation bound
are rechecked under the database lifecycle/evidence lock before cursor updates,
period admission closure, barrier sealing and finalization. The existing outbox
payload retains `nativePeriodCursor` and `nativeIPCursor`; each advances before
its individual RPC attempt, so offline/slow periods do not monopolize retries.
`account_purge_period.go` waits outside all account/service locks and retains
specific offline, journal and review dependencies. `hourly_lifecycle.go` routes
purges through this path; bare hourly purge intents cannot bypass grace evidence.

`account_purge_ip.go` uses the existing per-node executor and stable command IDs
for positive, currently applied owned subjects only. Completed zero-deadline
unblock replies become generation-bound metadata receipts; plaintext addresses
stay out of receipts/logs. No absent subject is recreated and no global unblock
is sent. Opaque legacy block rows lack an authenticated original agent/deadline
receipt, so their local scheduled expiry cannot prove remote absence: they stay
`ACCOUNT_PURGE_IP_EVIDENCE_PENDING` until confirmed cleanup/operator evidence.

`account_purge_activity.go` and `account_purge_activity_metrics.go` detach only
captured owner associations and scrub memory presentation banks. Their bounded
100000-scope, two-hour-plus-30-second fences admit rejoining activity only after
a different paid native period is applied on the current connection. They use
connection/revision authority, without a new-activity timestamp cutoff; shared
IP lease banks and other owners remain intact. `master_activity.go` filters late
old-scope reports before lease observation, and `whitelist_deployment.go` records
new native authority. Native IP history lacks observation generations, so exact
immediate erasure of all node-local two-hour presentation history is not proved;
historical entries alone still cannot mint a lease under existing freshness rules.

Hosted `account_purge_ip_test.go`, `account_purge_activity_test.go` and
`account_purge_activity_rejoin_test.go` cover positive scope, unapplied state,
shared-IP preservation, late reports, reconnects and skewed new activity.

`payg_account_prepare.go` adds the optional typed `PAYGPreparation` store path;
production creates principal, empty PAYG period, account and participant together
under the exact current owner-generation lock. `payg_account_provision.go` keeps
existing accounts on their prior read-only fast path and legacy test repositories
compatible; compile-read wrappers resolve the authoritative store implementation.
Encryption and entropy precede the transaction. Only a provably unpublished
older orphan can rotate into a new preparation; uncertain exposure/accounting is
retained. `account_rejoin.go` uses the same authoritative repository for exact
generation-fenced renewal of purged traffic accounts.
`payg_account_prepare_test.go` verifies compile-wrapper forwarding, encrypted
scope and the existing-account fast path.

Account purge also retires owned durable firewall exemptions through
`ReleaseAccountPurgeExemptions` and the existing fleet-delivery outbox. It waits
for every active member's release version and cleared full-sync requirement;
offline members remain dependencies. Financial grant facts stay available for
sanitized archival before operational deletion. Host-disabled members can retire,
and same-connection re-enablement forces a fresh-epoch full replacement so an
old persisted allowance cannot reappear after its removal tombstone is scrubbed.

`account_purge_ip_secret.go` opens exact owned block targets through the shared,
bounded historical-owner AAD resolver. `account_purge_ip_secret_test.go` checks
that wrong purge owners and altered node/digest scopes fail before native
control. Ciphertext is never resealed; current applied subject/agent proof,
cleanup admission and acknowledged scoped removal are still required.
