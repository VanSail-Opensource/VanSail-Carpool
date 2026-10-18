# X-Node administration

`XNodeFirewall.vue` shows shared detection evidence, authoritative deadlines and
remaining node expiry confirmations. `XNodeFirewallRules.vue` edits report-only
rule toggles, thresholds, windows and ban duration. Global policy also sets the
paid exact-IP exemption price and hours; both must be positive to enable purchase.

`XNodeWorkspace.vue` composes the independent browser console's nodes, global
policy, configurations/snippets, access groups/endpoints and live metrics.
`useXNodeWorkspace.ts` owns parallel initial reads, saves and visible-page metric
polling. `XNodeNodes.vue` edits one assigned profile and node utilities and exposes
write-only enrollment tokens. `XNodeOperationalSettings.vue` composes DNS/firewall
with `XNodeDDNSSettings.vue` and `XNodeCertificateSettings.vue`.

`XNodeGlobalSettings.vue` edits fleet policy and shared DNS providers.
`XNodeConfigurations.vue` handles one base per node, ordered shared snippets,
local JSON feedback and debounced pinned-core validation.
JSON inputs reuse `JsonDocumentField.vue` in the common component module.
`XNodeAccessGroups.vue` manages node/inbound access and subscription endpoints.
`XNodeMetrics.vue` renders fleet and detailed collector snapshots.

Typed clients are in `api/xnode.ts`; all copy comes from `xnode.json` locales.
Configuration and enrollment reads bypass response persistence. No component
initializes Telegram SDK controls. Metric rates convert bytes/second to decimal
Mbps; backend byte accounting retains exact integers.

- `XNodeMigration.vue` provides the source preview/import and per-node handover/rollback workflow.
- `useXNodeMigration.ts` loads migration state, protects stale requests and tracks durable operation receipts.
- `XNodeMigrationFinalization.vue` collects the final source backup reference and exposes staged export and verified bridge completion through operation receipts.

`XNodeMigration.vue` localizes shared SS-2022 preview issues and failed handover receipts. The source keeps serving when preflight requires isolated host/DNS bindings.

Node editors reuse the common `JsonDocumentField.vue` field.

`XNodeCommands.vue` uses durable receipts and stable idempotency keys; peer tests select an enabled endpoint on another node and read credential-free results. `XNodeMetricDetails.vue` and `metricRows.ts` render collector fields with byte, rate, duration and percentage units. Node forms bind certificate profiles to configured inbound tags and retain deep reactive feature switches. Save errors remain inside the editor instead of replacing it.

`configurationPresets.ts` creates only new SS-2022 AES-256 listeners with random 32-byte keys; imports preserve their methods and credentials. `XNodeSubscriptionTemplates.vue` owns revision-checked administrator template policy. `XNodeSubscriptionTemplateEditor.vue` validates and previews synthetic output; Retained subscription assignments remain references without editor controls. Format, default and assignment changes persist through the dedicated encrypted settings API.

- `XNodeServiceUsers.vue`: Lists and manages named native identities without Telegram accounts.
- `XNodeServiceEditor.vue`: Edits explicit access groups, expiry, quota and disabled state.
- `XNodeServiceRouting.vue`: Edits owner-scoped native outbounds and routing for authorized nodes.

Named-service browser validation uses the constructed X-Node fixture at 390px/dark and 1440px/light. Creation, access edits, disable, rotation, routing JSON validation, loading, empty, read failures and save failures were exercised through Chrome DevTools MCP. No production agents are contacted by that fixture.

Typed operational editing: `useXNodeGlobalSettings.ts` owns abortable, generation-checked reads and preserves drafts after mutation failures. `XNodeGlobalPolicies.vue` edits opt-in master GeoIP countries, explicit legacy-schedule conversion, and the requested shared-domain JSON list. `XNodeDNSSettings.vue`, `XNodeDNSProviders.vue` and `providerCatalog.ts` provide native resolver controls and write-only provider credential forms verified against the pinned agent registry. Blank credentials retain saved values.

`XNodeBandwidthSettings.vue` applies optional existing per-user speed policies per node. `XNodeFirewallFields.vue` edits typed port/prefix lists and forwarding while `XNodeFirewallRules.vue` retains native detection thresholds. `XNodeAccessBindings.vue` and `nodeFormOptions.ts` derive node/inbound selections from loaded configuration documents. `XNodeEndpointOptions.vue` edits supported client security/transport fields while preserving additional imported settings. Document editors remain for Xray, snippets and administrator subscription documents.


`XNodeRoutingTemplates.vue` edits typed destination matcher presets with guarded
loads, retained failed-save drafts and Go-validated expressions. These presets
control member server routing; subscription client documents stay independent.

`nodeFormOptions.test.ts` verifies explicit zero/max multipliers, six-decimal precision, and rejected omitted/null or over-precise input. `XNodeMetrics.vue` keeps CPU/memory chart buffers only while the metric view is mounted; `XNodeMetricDetails.vue` uses localized metric tables.

`useXNodeWorkspace.test.ts` verifies cancellation and late-response rejection, plus write blocking after failed reads. Workspace refreshes cancel earlier reads and metric polls; disposed observations are cleared.

`XNodeSubscriptionRules.vue` edits ordered exact-template rules and previews selection through the Go endpoint. `subscriptionTemplatePresets.ts` supplies independent Xray, sing-box, Clash and Stash presets and shared format ordering. Assignment controls are removed; retained assignment references remain in policy writes and deletion guards. Reads/previews abort stale requests and disable mutation after failed loads.

`XNodeSubscriptionRules.test.ts` covers aborting and discarding superseded Go previews without changing retained assignments.

Certificate forms show only mode-relevant identity fields; node-file profiles require their two paths. Global policy errors use safe localized asset/selector codes and preserve the draft on failure.

`XNodeFirewallGrant.vue` provides localized, validated IP/CIDR exemptions with custom whole-hour deadlines. It reuses operation receipt polling, retains uncertain request keys, permits a different grant while another is pending, and cancels UI tracking on disposal.

Node command receipts display localized delivery and execution diagnostics without raw agent errors. Recovery controls remain available while the connected runtime is not ready.
