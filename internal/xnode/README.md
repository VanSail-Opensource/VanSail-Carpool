# X-Node control plane

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

`credentials.go` preserves imported secrets and creates independent random
SS-2022 128/256-bit user keys. SQLite stores encrypted desired configuration and
credentials plus exact accounting; no live count, source-IP set, rate, log or
connection-derived block decision enters persistence. `ports.go` keeps compact
intervals. `public_transport.go` prevents private destinations and DNS rebinding
when validating external asset links through the existing request queue.

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
- `tag_ownership_test.go` covers ambiguous hyphen namespaces across active and expired members.

- `master_firewall.go` requires liveness and a successful transaction confirmation before reporting applied operational state.
- `capabilities.go` checks the exact capabilities required by compiled state, including native utility listeners with no paid users. `capabilities_test.go` covers that negotiation boundary.
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

Firewall control: `firewall_types.go` defines typed operational interfaces and public-IP validation. `firewall_service.go` gates capabilities, handles durable reports and quote-bound paid grants. `firewall_reconcile.go` preserves migration ownership, publishes successful source generations and rebuilds learned evidence only when its version changes. `firewall_delivery.go` defers existing outbox leases and sends dynamic deltas or bounded snapshots through the per-node queue. `firewall_source_delivery.go` streams independent source snapshots, retains warm-up positives and explicitly removes disabled feeds. Agent capability firewall_control_v1 is required; older agents receive protection disabled rather than autonomous banning. Source transfer slots are bounded separately, preserving capacity for immediate exemptions and traffic acknowledgments.

Task 2 pins protocol/native validation at signed X-Node v2.0.5, source commit `5352b67e882b`. Hosted Quality `39029889093` and signed release `39030494260` succeeded. The version-bound manifest and all amd64/arm64 agent/CLI hashes were independently verified with the production trust root.

Master policy extensions: `native_policy_types.go` owns country, completion-schedule and per-user node-speed contracts. `geoip_asset.go` streams verified public GeoIP data through the existing validation queue and temporary files; `geoip_decode.go` decodes bounded country messages with the pinned Xray protobuf and compacts selected prefixes without expanding addresses. `policy_assets.go` shares last-good generations, completion deadlines and bounded retries across compiler views. `geoip_decode_test.go` and `native_policy_test.go` provide hosted precision, schedule, prefix-bound and activity-capability regressions.

`live_availability.go` excludes rolled-back, stale and non-sampled agents from live activity decisions. Service identities are excluded from global connection controls; open-handler limits and exact accounting remain agent-owned. `node_decode.go` requires explicit nonnegative multipliers with at most six fractional digits. Global/node/compiler changes negotiate typed native capabilities against signed X-Node v2.0.6; byte counters remain decimal strings for browser transport.

`client_document_validation.go` validates administrator client subscription documents independently of the agent IP-geodata prohibition. Client-local assets remain in the actual output; analysis-only replacements perform no downloads. Shared geosite block selectors also defer file contents to agent preflight.

`geoip_wire.go` walks the pinned protobuf fields with bounded CIDR records, avoiding repeated-message allocation amplification and retaining cancellation checks during country resolution.

State requests coalesce into the existing reconciler. The socket reader never waits on native compilation or GeoIP refreshes, so queued utility results and traffic acknowledgments keep progressing during external asset work.

`zero_multiplier_test.go` exercises explicit zero and six-decimal settings through the real hosted store fixture.
