# X-Node control plane

## Paid access recovery

`member_access_test.go` covers username-independent admission, pending exports,
idempotent pointer repair and preservation of closed/exhausted purchase evidence.

`member_access.go` provides the common export/admission eligibility fence. Paid
members do not require an assigned username. Missing or mismatched native periods
produce pending provisioning, distinct from inactive access and configured empty
HTTP responses. `access_repair.go` runs through the shared durable outbox;
`EnqueueXNodeAccessRepairs` scans bounded batches at startup and every 30 seconds.
Recovery reuses an existing open period without resetting its usage. Closed or
settling periods for the same purchase require review; they are never replaced
with a fresh allowance. Legacy-owned identities remain the queued bridge's job.

## Telegram identity

Member database keys are decimal Telegram IDs and native member IDs use that
same number. Non-member services allocate IDs from `2^52` up to `2^53-1`.
Private protocol credentials and bearer subscription tokens are independent of
those identifiers. Display names use Telegram username, then full nickname.
The startup database migration reseals ID-bound ciphertext before publishing any
new identity; agent `replaces_id` preserves credential-bound leases and period
counters. Masters require `telegram_identity_rekey_v1` before sending a rekey.
Hosted access regressions use returned native IDs for standalone subscriptions,
traffic and resets, retaining source IDs only for import/resume checks. Saved
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

`rollover_purchase.go` resolves panel ownership and selects periods by explicit purchase ID for projection and settlement. All resets remain in that purchase; quiescence does not implicitly select the current identity period. Existing dated APIs remain compatible; `rollover_usage.go` supplies their common calculation.

`Service` owns authoritative configurations, ordered snippets, node settings,
access groups, endpoints, identities and accounting periods. `Master` accepts
`/v1/agents/connect`, binds enrolled agents, owns per-socket queues, and joins
their lifetimes before SQLite closes. Public wire/renderer packages come from a
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
SS-2022 128/256-bit user keys. SQLite stores encrypted desired configuration and
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
`service_cadence.go` takes over resets only after final source accounting.
`service_eligibility.go` derives access from current enabled groups and quota;
probe bindings retain their separate bounded behavior. `service_subscriptions.go`
uses the existing templates and host ACLs for standalone bearer subscriptions;
probe identities cannot use this path. Hosted `service_access_test.go` and
`service_access_policy_test.go` cover resume, shared exhaustion, late resets,
cadence handoff, host ACLs, native credentials and probe isolation.
`ServiceIdentityByName` resolves saved service names independently of Telegram;
`ServiceProxies` shares the same mixed-provider inventory with monitoring.

`validation_assets.go` substitutes node-local files only in an analysis copy,
then runs native validation; actual activation still verifies node-local assets.
Hosted cases cover port parsing, report freshness and invalid observations.
DNS GeoSite domains, DNS GeoIP expectations and source-IP GeoIP matchers retain
their matcher structure with synthetic analysis values. `validation_assets_test.go`
checks both configuration previews and compiled candidates, preservation of the
administrator document and rejection of unrelated native-core errors.

`ownership.go` stages configured shared geodata in the legacy-owned blank runtime
before candidate validation, with no listeners, users, firewall, DNS, torrent or
update policy. Existing agent asset handling verifies downloads before native
handover. `ownership_assets_test.go` guards this preparation/access boundary.

`types.go` Defines authoritative node, config, group, endpoint and shared policy types.
`accounting_types.go` Defines private identities, credentials, periods and migration state.
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
`master_metrics.go` Projects fresh available fleet and owner-specific snapshots.
`ports_test.go` Pins compact range normalization and invalid inputs for hosted CI.
`master_metrics_test.go` Pins report freshness and invalid/empty count handling for hosted CI.

- `access_window.go`: Entry points: `EffectivePurchase`.

- `account_statistics.go`: Entry points: `MemberOperationState`, `Statistics`, `UsageSnapshotForRollover`, `Usage`.

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

- `identity_import.go`: Entry points: `ImportIdentity`.

- `legacy_sync.go`: Entry points: `SyncLegacyUsage`.

- `master_access.go`: Entry points: `Connected`, `Sync`.

- `master_operations.go`: Entry points: `DeferOperation`.

- `master_socket_test.go`: Hosted regression coverage: `TestEnrollmentReconnectAndReplayUseAuthoritativeRevisions`, `TestNodeSnapshotsReplaceCountsWithoutPersistingTelemetry`.

- `migration_preflight.go`: Entry points: `VerifyClientEndpoints`.

- `ownership.go`: Entry points: `NativeOwner`, `Candidate`, `XNodeMigration`.

- `period_operations.go`: Entry points: `Quiesce`, `CloseForReplacement`, `ResetTraffic`, `RestorePurchase`.

- `periods.go`: Entry points: `ApplyPurchase`.

- `release_check.go`: Entry points: `AgentBuild`, `VerifyRelease`, `WaitApplied`.

- `service_import.go`: Entry points: `ImportServiceIdentity`.

- `service_users.go`: Entry points: `AuthorizeService`.

- `subscriptions.go`: Entry points: `BearerMetadata`, `OwnedSubscription`, `BearerSubscription`, `RoutingTemplates`.

- `validation_base.go`: Entry points: `ValidateConfigurationDocument`.

- `validation_complete.go`: Entry points: `ValidateCompiled`.

- `operations.go` queues durable node commands and dispatches waits through bounded controller-owned tasks, retaining and renewing outbox leases so purchases continue.

- `member_metrics.go` returns scoped live member rates, connection counts and reasons.
- `metric_scope.go` retains node scope when quota, expiry or restrictions deny admission.

- `tag_ownership.go` reserves stored member tags, including inactive users, against administrator/shared outbounds and other owners.
- `tag_ownership_test.go` covers decimal owner namespaces, including inactive and expired members whose historical display names overlap.

- `master_firewall.go` requires liveness and a successful transaction confirmation before reporting applied operational state.
- `capabilities.go` checks the exact capabilities required by compiled state, including native utility listeners with no paid users. `capabilities_test.go` covers utility and streamed-country negotiation boundaries.
- `binding_config.go` accepts group and subscription bindings only for managed proxy inbounds. Native utility listeners cannot become member endpoints.
- `rollover_usage.go` selects purchase-owned accounting periods and combines native and imported source history. `rollover_usage_test.go` covers successor isolation, pending journals, exclusive date boundaries and overflow rejection.

`endpoint_group_policy.go` intersects paid/enabled groups with administrator endpoint filters and preserved legacy host ACLs, including previously imported endpoints. `endpoint_group_policy_test.go` checks owner subscription and routing-template visibility for allowed, excluded, empty and unknown policies.

`member_operation_state.go` separates current-reset reconciliation from purchase-wide refund checks. Refund counters include all same-purchase resets and already-reconciled legacy usage; unfinished journals cannot qualify. `commerce_test.go` covers this boundary and in-place cadence edits.

`configuration_preview.go` expands saved snippets for read-only pinned-core validation. Base validation reports bounded technical core diagnostics to the administrator editor.

`metric_numbers.go` preserves large unsigned telemetry integers as decimal strings at the browser boundary; `metric_numbers_test.go` guards values beyond 2^53. `subscription_templates.go` persists encrypted template documents plus native user references, validates revision/default/assignment constraints and uses synthetic previews. Xray preview validation omits the client display-only `remarks` field from the server-core validator while retaining it in rendered output. `subscription_render.go` selects subscription-specific, format-default or built-in rendering and respects enabled formats. `template_repair.go` adds missing converted per-host client templates before cutover without replacing administrator overrides.

`subscription_templates_test.go` covers preview validation, native subscription assignments, stale revisions and dangling default rejection.

`BrowserMetricNumbers` also protects historical command result counters. Machine JSON normalization runs after releasing the master snapshot lock, and client-template writes use their own revision mutex so previews do not hold the accounting lock.

- `compile_cache.go`: Caches validated per-peer state against transactional desired-state revisions and access deadlines; controls remain memory-only.
- `compile_reads.go`: Reuses groups, identities, users, periods and purchase reads within one compilation.
- `compile_cache_test.go`: Hosted regression and before/after CPU profile with 1, 10 and 100 nodes and 1000 synthetic users.
- `customization_owner.go`: Resolves member and admin-only native-identity routing ownership without introducing Telegram principals.
- `named_services.go`: Admin-managed standalone access terms; quota edits preserve exact usage and probe identities remain separate.
- `named_services_test.go`: Hosted standalone quota, routing scope and disable regressions.

Firewall control: `firewall_types.go` defines typed operational interfaces and public-IP validation. `firewall_service.go` gates capabilities, handles durable reports and quote-bound paid grants. Firewall event batches share the 256-event protocol limit; safe rejection codes distinguish malformed batches, storage failures and ACK failures. `firewall_events_test.go` covers the oversized-frame boundary and secret-safe codes. `firewall_reconcile.go` preserves migration ownership, publishes successful source generations and rebuilds learned evidence only when its version changes. `firewall_delivery.go` defers existing outbox leases and sends dynamic deltas or bounded snapshots through the per-node queue. `firewall_source_delivery.go` streams independent source snapshots, retains warm-up positives and explicitly removes disabled feeds. Agent capability firewall_control_v1 is required; older agents receive protection disabled rather than autonomous banning. Source transfer slots are bounded separately, preserving capacity for immediate exemptions and traffic acknowledgments.

Task 2 pins protocol/native validation at signed X-Node v2.0.5, source commit `5352b67e882b`. Hosted Quality `39029889093` and signed release `39030494260` succeeded. The version-bound manifest and all amd64/arm64 agent/CLI hashes were independently verified with the production trust root.

Master policy extensions: `native_policy_types.go` owns country, completion-schedule and per-user node-speed contracts. `geoip_asset.go` streams verified public GeoIP data through the existing validation queue and temporary files; `geoip_decode.go` decodes bounded country messages with the pinned Xray protobuf and compacts selected prefixes without expanding addresses. `policy_assets.go` caches parsed generations across global-save validation and compiler views, with completion deadlines and bounded retries, so one save does not download the same full asset repeatedly. `geoip_delivery.go` queues country preparation durably, streams 256-prefix chunks through the per-node command queue for `source_geoip_stream_v1`, then activates a generation reference; older agents keep inline data when it fits the frame limit. `routing_preflight.go` uses that same reference when validating member route changes. `geoip_delivery_test.go`, `geoip_decode_test.go` and `native_policy_test.go` cover chunk bounds, compatibility, asset precision and selector validation.

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

`zero_multiplier_test.go` exercises explicit zero and six-decimal settings through the real hosted store fixture.


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

`routing_transit.go` derives private owner/source/destination/inbound/period-scoped
credentials from encrypted unpredictable seeds. `routing_compile.go` adds those
accounts only in enabling/active phases and generates private native outbounds.
Member subscription and graph APIs never return peer secrets. Client entry counts
remain chargeable activity; authenticated transit retains accounting and bandwidth.

`routing_preflight.go` uses capability negotiation and queued native validation.
`routing_operations.go` resumes durable revoke/drain, apply and enable barriers;
all affected nodes must acknowledge each phase before advancing. Interrupted work
uses existing operation leases and per-node execution queues. Reconnect can wake
exhausted routing jobs in that same queue. Financial periods are never reset.
`routing_legacy.go` converts only exact endpoint/preset matches from pre-start saved
routes; incompatible documents are disabled with encrypted evidence and audit.

`routing_transit_test.go` covers private generation scope and template validation;
`routing_geosite_test.go` covers streamed selector kinds, attributes, truncation and
unknown server contexts. These regressions run in hosted CI.

`routing_tags.go` reserves fixed-width owner tags across inactive plans and native/shared configuration edits, without retaining private peer documents in public APIs.

`routing_deployment_test.go` exercises real authenticated master sockets and a deliberately held node acknowledgement, proving transit accounts stay disabled until every node applies and accounting periods remain intact.

`compilation_errors.go` keeps private native validation details out of default errors/logs while allowing explicit validation diagnostics, including synthetic hosted fixtures.

`routing_peer_guards.go` keeps generated private authentication and endpoint destinations intact through imported client mappers and refuses unmodelled dialer chains. `routing_peer_guards_test.go` guards these bindings and per-hop sniffing uncertainty. Conditional-cycle matcher proofs require fixed destination metadata: active destination overrides and protocol predicates with variable sniffing are refused for those graphs. Structurally acyclic managed graphs cannot revisit a listener, so normal server sniffing, DNS resolution and administrator proxy fallbacks remain usable. Torrent detector changes invalidate the saved graph dependency digest. Existing imported documents remain encrypted and unchanged.

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

`member_metrics.go` derives online, offline or unavailable from fresh native activity within the authorized node scope. `member_activity_test.go` covers stale/unsupported/rolled-back observations, idle nodes and transfer-only activity. The expiry is based on master receive time, stays memory-only and is transported in both member and profile projections.

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
pruning. Remaining choices are disabled for review because removing a rule can
expose a formerly shadowed cycle. Previous operational choices and superseded
legacy customization rows are erased; immutable encrypted conversion evidence
and financial records remain intact. Existing compile-version triggers and
serialized master synchronization remove routes and private peer credentials
from connected nodes and on reconnect, without a second queue or ACK blocking.
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

`metered_probe_lifecycle_test.go` exercises compiled admission, quota exhaustion, expired grants, real traffic reset and refund quiescing with duplicate late probe samples. Unfinished barriers retain origin evidence and leave successor periods uncharged.

## Firewall and member whitelist

`firewall_policy.go` owns global rules and preserves common legacy fields during
migration; `firewall_policy_test.go` checks conflict defaults.
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
settings, whitelist and routing for free unchanged saves. `diy_billing.go` seals
owner-bound five-minute quotes; the existing database transaction rechecks price,
allowance, balance and revisions before any committed change.

`diy_split_test.go` covers current-reference exports, disconnected nodes, immediate
host-access exclusion, pinned-parser validation and free reference reconciliation.
`diy_split_catalog_test.go` covers per-pair mode metadata and deduplicated legacy
compatibility modes in hosted CI.

`command_arguments.go` requires `incremental_reload_v1` for runtime reload,
rechecks capability at execution and forwards typed optional forced recovery.
`operations.go` retains durable command replay and per-node execution ordering.
