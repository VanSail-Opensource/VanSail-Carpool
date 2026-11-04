# WebSocket protocol v1

`telegram_identity_rekey_v1` permits `users[].replaces_id`: the previous numeric
identity for a Telegram-ID replacement. It is unique within a snapshot and must
differ from the new ID. The registry retains matching credential generations,
paid-period counters and existing leases; stale mux admission still checks the
complete credential-bound identity. Masters must accept late journal samples by
their authenticated node and unchanged accounting period, never by a global
numeric alias alone. Cached replacements prevent rollback to an older agent.

`native_auxiliary_inbounds_v1` permits native `dokodemo-door` and `tunnel`
forwarding/utility listeners. These listeners retain native settings, never
receive managed accounts and are excluded from member connection/traffic
accounting. Managed user bindings and account arrays on these tags are rejected.
Masters must negotiate this capability before sending utility listeners.
Unsupported utility protocols remain validation errors. A rollback binary must
support every cached capability. Each newly introduced extension is blocked
during an unconfirmed binary upgrade, including when older extensions already
exist in the cache.

Optional capabilities `traffic_periods_v1`, `measured_user_rates`,
`authorization_expiry`, and `member_outbound_guards` identify the accounting-aware
agent. Masters must verify capabilities before supplying the corresponding
optional configuration fields. Go masters can import `pkg/protocol`.
Hello's `upgrade_pending` marks an unconfirmed binary upgrade. First adoption of
managed extensions is rejected until cached runtime activation confirms that
upgrade, preventing an older watchdog binary from dropping new state on rollback.

Accounting-aware users carry `accounting_period` and an optional RFC3339
`valid_until`. Expiry remains effective while disconnected. Traffic reports keep
their legacy totals and additionally carry `samples` with user ID, accounting
period, UTC `from`/`through`, and exact uploaded/downloaded bytes. For these users,
bill samples rather than adding both representations. Period IDs are master-owned
and must never be reused. Timestamp granularity is one second.

`users.rates[userID]` supplies measured `upload_bytes_per_second`,
`download_bytes_per_second`, and `sample_seconds`. These are payload byte rates;
`users.speed` still describes decimal Mbps policy limits, not measured throughput.

The `traffic.flush` command accepts optional `user_id`, `quiesce` and `accounting_period`.
A period-specific quiescence waits for the old native handlers without closing
renewed streams. It requires `traffic_period_barriers_v1`, a positive user ID,
and an accounting period that no longer admits traffic. The response echoes
`accounting_period`; only seal a period after its sequence is acknowledged.

`config.validate` accepts a complete remote configuration and performs the
same native preparation without listeners. It requires `config_validation_v1`
and validates node-local certificates/assets before a provider handover. Its
results and configuration stay out of durable command history.
With quiescence, block new admission first; the agent closes matching sessions and
waits for their native handlers before capture. The result contains `agent_id`,
the sealed journal `sequence`, `through`, and `quiesced`. The master must commit
reports through that sequence before settling or resetting the old period.
Outstanding batches stay immutable and queued barrier batches survive restart.

`member_outbounds` lists generated member proxy tags. Their endpoints must be
literal public IPs; the master resolves/validates hostnames before compilation
and retains TLS SNI separately. Agent checks reject private, mapped private,
reserved, scoped, and DNS-rebinding endpoints. Member socket overrides and
unvalidated chains are rejected. Managed SS-2022 supports AES-128/256 methods:
server keys stay on inbound settings and native user accounts omit `method`.

Session/IP scans and metrics/history reads are not written to utility history.
Signed installs and rollbacks reject binaries whose version metadata lacks
capabilities required by cached authorization or pending accounting journals.

The agent connects to the configured WSS URL with `Authorization: Bearer <token>` and the `x-node.v1` subprotocol. Authentication, agent enrollment and authorization are the master's responsibility. The agent never places its credential in the URL.

```json
{"v":1,"type":"state.sync","id":"snapshot-1","revision":1,"data":{"xray":{"inbounds":[]},"users":[]}}
```

The agent sends `agent.hello` before other frames on a new socket, then `state.request`. Hello identifies the persisted agent, fresh process boot ID, binary/core versions, applied revision and capabilities. WebSocket Ping/Pong frames keep the connection alive.

## State

| Master frame | Payload | Semantics |
| --- | --- | --- |
| `state.sync` | Complete remote configuration | Authoritative baseline; `users` is required even when empty |
| `config.apply` | Complete remote configuration | Replace configuration with an exact `base_revision` |
| `users.sync` | `{"users": [...]}` | Replace authorization |
| `users.delta` | `{"upsert": [...], "remove": [1]}` | Apply additions/replacements/removals |
| `user.controls` | `{"users":[{"id":1,"block_new":true}]}` | Change admission while retaining established eligible streams |

Revisions increase monotonically. A delta must name the current authorization revision as its base. Gaps trigger `state.request`; stale frames produce an error. Repeated full snapshots at the successfully applied revision are acknowledged without reapplying. The master must advance the revision for corrected configuration.

`state.accepted` acknowledges persisted authorization. `state.applied` acknowledges runtime activation. `state.failed` reports an activation failure; its authorization revocations remain effective. Readiness and synchronization are separate telemetry fields. User-only changes use runtime user managers when available. Listener/global changes may close affected sessions.

Users have a positive ID, credentials, eligible `inbound_tags`, and optional `active_ip_limit`, `connection_limit`, `speed_limit_mbps`, and `block_new`. Zero limits mean no configured user quota. `uuid` is used for protocol defaults; `credentials[tag]` supplies a native client/account object. The agent generates email/level and authenticated SOCKS/HTTP usernames. Transport identities bind credentials and inbound/configuration generations to prevent old mux sessions bypassing revocation.

## Reports

Bandwidth settings are shared per user across upload/download and all inbounds. `speed_limit_mbps` is the normal rate; `burst_speed_limit_mbps` is the hard ceiling, always enforced. Sustained throughput above normal for `speed_limit_after_seconds` (default 10) activates the normal cap. Throughput below 90% of normal for `speed_limit_reset_seconds` (default 1) restores bursts; this small hysteresis prevents oscillation at the enforced rate. Consecutive durations use one-second windows and actual elapsed time, independent of report intervals. A below-normal window resets the high-traffic streak. Omitting the burst ceiling retains the fixed normal rate. `users.speed[userID]` reports `normal_mbps`, `burst_mbps`, `active_mbps`, and `limited` (normal cap active).

```json
{"id":1,"uuid":"550e8400-e29b-41d4-a716-446655440000","inbound_tags":["vless"],"speed_limit_mbps":100,"burst_speed_limit_mbps":500,"speed_limit_after_seconds":10,"speed_limit_reset_seconds":1}
```

`telemetry.report` is an authoritative current snapshot containing agent/boot IDs, timestamp, liveness/readiness, revisions, per-user connection counts and limit status, rejection counters, machine metrics and certificate status. Missing users in its current online set have zero active sessions; the master must replace that set rather than merge indefinitely.

Agents advertising `compact_telemetry_v1` offer `X-XNode-Telemetry: compact-v1`
during the authenticated WebSocket upgrade. A supporting master echoes that
header; missing/unknown responses select legacy reports for that connection.
`telemetry.compact` uses the generated [compact snapshot schema](schemas/telemetry-compact.schema.json):
one row per observed user, short field keys and omitted zero/default values.
`u` must be an explicit array; an empty array clears all earlier user observations.
Default unlimited inactive users need no row. Limits, configured speed caps,
active rates, rejection counters and explicitly opted-in IP sets retain their
meaning. Masters normalize either format to the same fresh `TelemetryReport`.
Negotiation resets on reconnect; queued compact snapshots fall back to legacy
before reaching an older master.

Both endpoints negotiate per-message deflate using the existing
[Gorilla WebSocket extension](https://pkg.go.dev/github.com/gorilla/websocket#hdr-Compression_EXPERIMENTAL).
The agent's single writer uses fast compression only for report bodies at least
1 KiB. Uncompressed peers remain supported. Readers bound expanded input to
16 MiB before JSON decoding; compressed frame size alone is not sufficient.
Traffic journal bodies and acknowledgement ordering remain unchanged.

User source IPs are omitted by default. Request a one-time report with `users.active_ips`, or opt into periodic IP sets with `telemetry.include_active_ips: true`. Each requested record carries `user_id`, canonical `source_ip`, and its current connection count.

Per-user `users.status[userID]` reports `limited: true/false`, `reason`, and `scope`. A full active-IP quota has scope `new_ip`: existing IPs can still create streams when the connection quota allows it. `block_new` and connection quotas have scope `new_connection`. Each rejected admission also emits `admission.rejected` with `user_id`, `limited: true`, `reason` and `scope`; `source_ip` appears only when periodic IP reporting is enabled. Reasons include `ineligible`, `block_new`, `torrent_ban`, `ip_temp_block`, `blocked_port`, `connection_limit`, `active_ip_limit`, and `storage_backpressure`.

`traffic.report` contains `{id, traffic}`, with `traffic[userID] = [upload_bytes, download_bytes]`. The batch ID is stable across retries and process restarts. Deduplicate it before billing, then send:

```json
{"v":1,"type":"traffic.ack","data":{"id":"agent-id:sequence"}}
```

Only the matching pending batch is retired. Status and logs are disposable snapshots; traffic is acknowledged durable data. `logs.report` supplies timestamped entries and a dropped-entry count. Secrets are redacted before console, file forwarding and remote report serialization.

## Commands

```json
{"v":1,"type":"command","id":"operation-1","data":{"name":"connections.list","args":{"limit":100}}}
```

Commands emit `command.accepted`, optional `command.progress`, and `command.result`. Results contain `id`, `command`, `status`, optional `data/error`, and completion time. Two utility workers run concurrently. The last 128 completed results are retained across restart for request-ID deduplication; IDs must remain unique beyond this window.

| Command | Arguments |
| --- | --- |
| `connections.list` | `after`, `limit` (default 100, maximum 1000); cursor is the last returned session ID |
| `connections.close` | Session `id`, or `user_id` plus optional `inbound_tag` |
| `users.active_ips` | Optional `user_id`; omission requests current active source addresses for all users |
| `ips.control` | `source_ip`, optional `user_id`, and `drop`, `block_seconds` or `unblock` |
| `metrics.get`, `certificates.get`, `jobs.list`, `firewall.status`, `update.check` | Empty object |
| `runtime.reload` | `{force?:boolean}`; recheck material and apply incrementally; force requests recovery |
| `ddns.run` | `job_id` from the accepted configuration |
| `geodata.refresh` | Empty object; refresh then incrementally apply the verified generation |
| `speedtest` | `peer_outbound`, HTTPS `download_url`/`upload_url`, optional `max_bytes`/`timeout_seconds` |
| `firewall.apply` | `policy`, optional `dns_redirect` |
| `firewall.confirm` | `transaction_id` from `firewall.pending` or the apply result |
| `update.install` | Optional stable `version` |
| `update.rollback` | Empty object |

Cancellation uses `command.cancel` with `{id: operation-id}`. Completed operations remain queryable; cancellation cannot undo a mutation already accepted by an external provider.

IP controls target exact client source addresses, normalized so IPv4-mapped IPv6 equals IPv4. An omitted/zero `user_id` targets every user at that address. `drop: true` closes current matching logical sessions. `block_seconds: 1..604800` rejects new matching sessions until a persisted deadline; existing sessions remain unless drop is also requested. `unblock: true` clears that scoped ban. Drop-only does not create a ban.

```json
{"v":1,"type":"command","id":"ip-action-1","data":{"name":"ips.control","args":{"source_ip":"192.0.2.10","user_id":42,"drop":true,"block_seconds":600}}}
```

Firewall transactions default to 45 seconds. The master must confirm using the existing or re-established socket after testing connectivity. An independent systemd watchdog restores both live rules and persisted policy if confirmation is absent.

The native Go engine commits ordered rule/set changes as one atomic Netlink
transaction while preserving existing bans, meters, and counters. Repeating the
current transaction ID is idempotent; changing its inputs is rejected. Pending
transactions are recovered after restart. The `native_firewall_v1` capability
identifies binaries that support native policy restoration; firewall command and
policy wire formats remain unchanged.

`host_firewall_ownership_v1` additionally identifies nftables/UFW service takeover
and durable service-state rollback. A confirmed policy enables nftables, disables
and masks competing UFW enforcement, and restores original service/config state
on rollback or uninstall. A stored takeover journal prevents unsafe binary
downgrades. The privileged CI fixture covers enabled/disabled nftables and Ubuntu
24.04 UFW in marked disposable VMs.

`xtables_compat_snapshot_v1` identifies binaries that can snapshot and replay the
tables iptables-nft owns. nft cannot express their `xt match`/`xt target` rules,
so they are captured per table with the iptables front end and restored after the
native snapshot. A stored original snapshot prevents downgrades that would flush
host rules they cannot recreate.

Speedtests use a separate outbound-only Xray client and transfer HTTPS payloads through the peer. They default to three request-latency samples, at most 32 MiB download, 8 MiB upload and 60 seconds total. The transfer bound can reach 64 MiB and timeout 120 seconds. Test credentials and HTTP endpoints are supplied and managed by the master.

`torrent.detected` reports `user_id` and ban deadline. `torrent.expired` reports expiry. `block_new` controls and per-machine connection reports implement approximate global totals; no per-admission master reservation is performed.

## Operational settings

The configuration schema describes provider credentials, certificate profiles, inbound certificate references, DDNS jobs, resolver policy, firewall policy, outbound port ranges, asset hashes, telemetry, torrent policy and update policy. Operational defaults are one-second sampling, two-second reporting, five-minute DDNS refresh, ten-minute torrent bans and disabled automatic binary installation.

DDNS provider `builtin:panel` discovers enabled `auto` IPv4/IPv6 families without
publishing DNS records or requiring provider credentials, zone or record name.
The master retains host bindings and supplies opaque per-job `generation` values;
host IDs never enter agent configuration. `builtin_ddns_v1` gates this job type.
Legacy named providers keep their existing record behavior. Failed discovery
does not replace a previously successful address.

For `ddns_changes_v1`, upgrade negotiation uses
`X-XNode-DDNS-Changes: acknowledged-v1`. The master echoes that value only when it
supports durable changes and verifies the hello capability. Successful generated
jobs produce ordered `ddns.changes` batches (at most 16 events), retaining IDs,
configuration generation and addresses until `ddns.changes.ack`. A new generation
produces an initial observation; unchanged addresses in that generation do not.
The master validates authenticated node/job ownership, exact current generation
and event order, persists new operations before acknowledging, and discards
duplicate or obsolete effects. The private bounded `ddns.json` journal remains
separate from accounting and survives old-agent rollback. Legacy masters receive
no new extension frames; new masters strip generation fields for older agents.

The agent owns verified geosite.dat domain downloads and suppresses the core's duplicate downloader. GeoIP data and source-prefix resolution belong to the master. Native domain assets stay inside the managed directory; completion-based hour intervals, retained legacy cron and download outbound selection are honored. SHA-256 hashes are supplied through `asset_hashes[file]`.

Signed handover checks compare authenticated hello `binary_sha256` and `platform`
with the official version-bound minisign manifest. This measurement identifies
a trusted enrolled agent build; it is not hardware attestation. Masters can use
`pkg/releases` to apply the exact update trust policy.

Connection-derived `block_new` controls remain in memory. The agent advertises `volatile_connection_controls_v1`; cached authorization keeps credentials, expiry and accounting periods, but never resurrects a historical global connection decision after restart.

## Firewall control v1

Agents advertise `firewall_control_v1`. A supporting master sets
`firewall.master_controlled`; older masters receive no new event frames.
`firewall.events` carries a stable `instance_id`, ordered event IDs, timestamps,
IP/rule/count evidence and live kernel status. Masters acknowledge committed IDs
with `firewall.events.ack`. Each report contains at most 256 durable events;
agents retain and resend the remainder until acknowledged. Detection never
inserts a blacklist entry.

Commands `firewall.bans.sync`, `firewall.bans.update` and `firewall.bans.get` use
the existing command executor. Every mutation requires a positive master
`version` and immutable `snapshot_id`. Snapshot chunks are ordered by `index`
and remain inactive until `final`. `sources_complete` controls authoritative
source replacement; `remove_sources` explicitly retires sources during partial
coverage. A source `reference` requires the existing matching generation.

Bans include prefix, generation, absolute `until` and refresh seconds; new commands
can reset shorter lifetimes. Retries do not extend the original deadline. IPv4
uses /32 and IPv6 /64; exemptions use exact addresses. Only a newer master
authorization can extend a ban. Expiry events retain their
generation so obsolete events cannot clear newer bans. A source-expired event
means its authorized deadline has expired.

Source inventories use `sources_only` snapshots and a separate source version.
Dynamic bans and exemptions continue to accept newer deltas while an older source
generation stages; source commits never overwrite a newer paid exemption.

## Sampled activity and master policies

`sampled_activity_v1` reports only live flows with increasing payload counters in the latest interval. Idle/open/native completion resources and traffic accounting are independent. `open_connections_v1` adds `users.total_connections` (compact user field `o`), counting all logically open client flows including idle TCP/UDP, but excluding internal transit hops and logically closed native handlers. Missing user entries mean zero on capable agents; older agents do not provide total counts. `users.connections` (compact `c`) continues to govern connection limits. `transit_credentials_v1` adds bounded peer credentials to each owner, scoped by source node, inbound and generation; peer hops remain metered but are absent from client connection/IP counts.

`source_geoip_firewall_v1` accepts only master-resolved prefix generations and non-REALITY listener bindings. Agents reject GeoIP downloads and routing/DNS selectors. `shared_domain_blocks_v1` adds a front-of-routing domain block policy. `geodata_interval_v1` permits completion-based `geodata.interval_hours` (0 disables, 1..8760 enables); legacy cron is retained when interval is absent.

Commands have a separate 30-second acceptance deadline. Execution is bounded to 30 seconds by default, two minutes for native validation/reload/update checks and traffic barriers, 90 seconds for speedtests, five minutes for geodata refresh/firewall preparation and ten minutes for installation or bulk range preparation. `runtime.reload` and `geodata.refresh` on agents advertising `activation_commands_v1` complete only after activation, retaining the previous runtime until native preparation succeeds. `update.status` returns the durable transaction (ID, version, previous version, status and deadline); it and `jobs.list` reply without a utility slot. `update_transaction_v1` enables reconciliation after reconnect. Confirm an upgrade only with a matching confirmed transaction plus fresh ready/synchronized telemetry at the current configuration revision. A version string alone is insufficient.

## Whitelists and fleet detection

`member_source_whitelist_v1` adds `whitelist_enabled` and exact `whitelisted_ips`
to durable user authorization. An enabled empty list denies member ingress.
Changes close affected existing ingress sessions before acknowledgment. This
check uses Xray's authenticated source address and trusted-proxy configuration,
not an untrusted request header. Independently authenticated transit credentials
bypass member IP checks on subsequent routing hops; expiry, accounting and
routing revocation still apply. `whitelist_transit_test.go` pins this separation
and delayed transit classification after a Telegram identity rekey in hosted CI.

`firewall_master_observation_v1` reports bounded IP/rule count deltas without a
node-local threshold. Event IDs remain durable and retryable until acknowledged;
masters aggregate across nodes and deduplicate retries. With probe protection
on, detections pass and only master-issued bans drop traffic. With protection
off, matching invalid/probe/closed-port traffic drops silently. Normal TCP
handshakes on open ports remain available. Source inventories follow the same
mode and never cause autonomous bans in observation mode.

`firewall.protected.sync` replaces exact volatile managed-node addresses through
the existing executor. They are excluded from detection and all blacklist banks,
including source inventories. Active user whitelist IPs join the kernel bypass
until their authorization expiry; removing one owner's exemption preserves any
other owner's active exemption. Non-Reality geolocation filtering precedes every
bypass. Protected-address commands and live IP projections are private and do
not enter durable command history. Whitelist/firewall changes update registry
and nftables state without reconstructing native Xray.

Sampling and reporting use independent master-configured tickers. Activity IPs, connection counts and payload rates follow the sample interval; periodic frames follow the report interval. A bounded coalescing worker encodes reports independently of command receipt and sampling. The deprecated include_active_ips switch is ignored; sampled IPs are always reported. Matching traffic acknowledgements immediately release the next sealed journal batch; new batches and lost-ACK retransmissions follow the configured reporting interval. Socket/process/GPU scans refresh at most once per ten seconds.

## Negotiated compressed reports

`report_frame.go` exposes `report_zstd_l6_v1` and `X-XNode-Reports: zstd-l6-v1`.
A master echoes this offer before an updated agent uses binary report frames.
The first four bytes are 58 4e 5a 01 (XNZ and binary version 1), followed by a
four-byte big-endian decoded JSON length, then independent Zstandard data with
checksum. The JSON envelope and payload schemas remain unchanged. Controls,
commands, acknowledgements and agent.hello remain text frames. Never combine
binary report framing with WebSocket permessage-deflate. Legacy peers retain
bounded text/deflate decoding.

Level 6 uses EncoderLevelFromZstd(6), one reused encoder worker, a 1 MiB window
and low-memory operation. Decoders use one worker, enforce both the declared
length and the 16 MiB decoded cap, and bound compressed input separately. Idle
scratch buffers retain at most 512 KiB; larger buffers are disposable. These
codecs never touch the filesystem. `report_frame_test.go` covers malformed sizes,
checksums, expansion bounds and retained memory; its hosted benchmark measures
round trips over 10,000 synthetic observations.

Local `xnodectl status` queries the permission-restricted Unix socket `/var/lib/x-node/status.sock`; no periodic status file is written. Expiry, retry and watchdog maintenance retain their one-second timer independently of report/sample settings.

## Volatile boot traffic streams

Updated peers negotiate `X-XNode-Traffic: stream-v1` and `traffic_stream_v1`.
The stream ID is the authenticated 32-character lowercase hex boot ID. New report
IDs are `agent_id:boot_id:sequence`, starting at one per process. The canonical
report payload is immutable for its ID. Legacy persisted `agent_id:sequence`
reports drain first without changing identities. A legacy master receives only
those historical reports; new stream usage is never relabeled as legacy usage.

New raw totals, period samples and sealed reports remain bounded memory: at most
8192 accumulation entries, 64 sealed reports and a conservative 32 MiB queued
allocation allowance. Compatible period samples coalesce only within matching
capture minute boundaries. A rate-change flush seals the prior batch. Overflow
may lose uncommitted samples; `dropped_samples` records loss observations without
inventing a lost byte quantity. Queue pressure makes flush retryable and never
pauses admission or changes the durability of credentials, commands or bans.
Configuration writes cannot serialize volatile buffers. New traffic captures,
reports, retries and acknowledgments never spill to disk on disconnect/failure.

The master durably closes replaced boot streams and maintains independent
sequence/digest cursors and rate boundaries. Duplicate/reordered reports never
charge twice. Sequence gaps and explicit buffer loss retain known usage and
record evidence; conflicts never rewrite settled usage. Usage, cursors and all
financial effects commit before ACK. A closed stream cannot reopen. Settlements
use surviving committed usage after restart, accepting underbilling or larger
refunds. No cursor is advanced for a report that was never committed.

Flush barriers name their stream plus any pending legacy sequence. Financial
operations wait for both real cursors; ordinary command/telemetry receipt stays
independent of that wait. The report and barrier schemas are published under
`docs/schemas`, with negotiation/framing metadata in OpenAPI and AsyncAPI.

## Sampled IP observations

Peers independently negotiate `X-XNode-Activity: observations-v1` and hello
capability `activity_observations_v1`. Full `users.sampled_at` is the actual
sampling boundary; `users.active_ip_observations` contains `user_id`, canonical
`ip`, `last_active` and a boot-scoped decimal-string `observations` count.
`users.observation_drops` is a decimal-string saturating loss-observation counter.
Compact top-level fields `q`, `h`, `j` carry these same values. The serialized
writer removes all three unless the current master accepted the extension,
including reports queued before a reconnect. Compression negotiation alone does
not authorize these fields.

History retains at most 16,384 user/IP keys for two hours, covering the maximum
one-hour sample and report intervals. Brief payload flows that close between
samples enter a separately bounded pending map and publish at the next sample.
Their observation time is close/completion time; live payload activity is dated
at its sampling boundary. Idle samples and repeated reports preserve old times
and counters. Internal transit never produces client-IP evidence. Overflow may
discard old volatile observations; it never changes accounting or admission.

An observation must have a positive user/count, a valid address and a nonzero
time no later than its sample. Duplicate user/IP rows and oversized histories
are rejected. A master checks authenticated agent/boot identity, current peer,
timestamp freshness and monotonic sampling independently of report publication.
The full and compact schemas live in `docs/schemas`; all history remains in
memory and never enters configuration, command receipts or local status files.

## Volatile automatic firewall lease commands

An agent advertising `volatile_firewall_leases_v1` accepts the master-only command
`firewall.leases.update`. Arguments contain a process-unique master `epoch` and
at most 256 `leases`: canonical public exact `ip`, decimal-string `generation`
and absolute `until`. The completion result is `{epoch,generation}`, with the
largest processed batch generation. Masters advance one generation watermark
per node only after this result, retry uncertain delivery through the existing
per-node command queue, and fully resynchronize after reconnect/firewall enable.

Agent storage is a separate bounded memory bank and two exact timed nftables
sets. Duplicate generations do not add entries; conflicting deadlines for one
epoch/generation reject atomically. A newer generation may shorten the current
epoch's deadline. Changing master epoch preserves the maximum prior grant until
its original deadline. Process restart hydrates unexpired kernel grants as prior
deadlines. Expired in-flight entries are acknowledged without resurrecting them.
Local metadata expiry and kernel timeouts need no master removal command.

Automatic exemptions do not remove or replace durable manual/paid grants, alter
member source whitelists, admit credentials, open closed ports or bypass country
admission checks. Exact IPv6 exemptions never widen to the containing /64 ban.
The command, IP bank and replay receipts never enter durable command history;
confirmed/rollback snapshots omit `leased_v4` and `leased_v6` elements. All other
command/configuration/credential/ban durability remains unchanged. Public schema:
`docs/schemas/automatic-leases.schema.json`.
