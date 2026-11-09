# X-Node administration

Chrome DevTools checked the retired DNS controls at 1440 px and 390 px, in
English/light and Chinese/dark. Only interception and native-configuration
guidance render; toggling interception retains legacy opaque fields for server
migration. Save refusal keeps the editor/draft; console inspection was clear.

`XNodeDNSSettings.vue` exposes the native DNS redirect switch only. Resolver
servers belong to the assigned Xray configuration; the separate mode/upstream
editors are retired. Redirect updates preserve the original settings document,
including legacy fields needed by the backend's configuration-aware migration.
`XNodeDNSSettings.test.ts` covers this preservation, default behavior and the
absence of retired controls in hosted frontend CI. The X-Node browser fixture's
`legacy-dns` parameter supplies a retained SmartDNS document for visual review.

Firewall port editing uses `firewallPorts.ts` to read legacy singleton lists and
native intervals, validate 1..65535 bounds, and merge overlaps/adjacency without
expanding the range. Commas, newlines and existing whitespace separators work;
spaces around a range hyphen are accepted. Singletons remain in the legacy lists,
while contiguous intervals use the additive TCP/UDP/SSH range fields.
`firewallForwarding.ts` validates one source interval and either one destination
port or an equal-length destination interval. It canonicalizes IPs, deduplicates
identical rules, rejects conflicting listeners/families and caps translated maps
at 262,140 entries. `XNodeFirewallFields.vue` retains incomplete text locally,
publishes valid typed policies, and propagates validity to the node save control.
It uses existing Nuxt UI fields and the node-forms locales, without upstream calls.
`firewallPorts.test.ts`, `firewallForwarding.test.ts`, and
`XNodeFirewallFields.test.ts` cover interval normalization, forwarding conflicts,
map bounds and retained invalid form drafts in hosted frontend CI only.
Chrome DevTools checked the real node editor with constructed firewall data in
English and Chinese at 1440 px desktop and 390 px mobile widths. Both mappings,
invalid ACL/unequal-length errors, normalized save payloads, retained save-error
drafts, initial/loading/empty states and read-error retry were exercised. Forms
used two columns on desktop and one on mobile without horizontal overflow.
Console inspection found no warnings/errors; mocked saves made no upstream calls.

The Metrics connection summary displays total open flows, including idle flows.
Unknown or incomplete negotiated coverage displays unavailable; active counts
remain available in the API for admission limits.

Built-in DDNS uses `XNodeDDNSHosts.vue` for separate IPv4/IPv6 multi-host references,
limited to the edited node's existing endpoints. Each endpoint can be assigned to
only one family/job. External-provider bindings remain single-host. The job editor
keeps these references outside discovery settings, hides public-DNS fields for
Built-in and preserves bindings across job renames. `ddnsForm.ts` reuses Zod and
native URL validation to gate saves on unique hosts and enabled discovery sources.
Provider discovery remains cancellable and Built-in is available during failures.
Copy is localized in the node-forms locale files; reduced motion uses native controls.

`XNodeDDNSSettings.vue` edits the pinned agent's DDNS fields, including optional
HTTPS IP lookup URLs and nonnegative TTL (new jobs default to 300 seconds).
`XNodeDDNSJob.vue` owns one job's localized fields and removal action.
`useDDNSDiscovery.ts` loads saved provider IDs without secrets. `XNodeDDNSZone.vue`
owns cancellable zone loading/retry and searchable name selection; saved values
survive unsupported providers and failed discovery. Results remain memory-only.

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
`XNodePricingCostSettings.vue` loads the edited node's existing cost and capacity
metadata, and saves versioned changes and optional payments separately with an
audit reason. Failed metadata reads block saving until retried. These settings are
panel-owned and are not part of the agent configuration.

`XNodeGlobalSettings.vue` edits fleet policy and shared DNS providers.
`XNodeConfigurations.vue` handles one base per node, ordered shared snippets,
local JSON feedback and debounced pinned-core validation.
JSON inputs reuse `JsonDocumentField.vue` in the common component module.
`XNodeAccessGroups.vue` manages node/inbound access and subscription endpoints.
`XNodeMetrics.vue` renders fleet and detailed collector snapshots. `telemetryAvailable` gates fresh machine/certificate details and CPU/memory history; `available` separately gates sampled-activity rates and connection metrics. Its badge distinguishes offline, stale, unsynchronized, metrics-unavailable and fully available states.

Typed clients are in `api/xnode.ts`; all copy comes from `xnode.json` locales.
Configuration and enrollment reads bypass response persistence. No component
initializes Telegram SDK controls. Metric rates convert bytes/second to decimal
Mbps; backend byte accounting retains exact integers.



Node editors reuse the common `JsonDocumentField.vue` field.

`XNodeCommands.vue` uses durable receipts and stable idempotency keys; peer tests select an enabled endpoint on another node and read credential-free results. `XNodeMetricDetails.vue` and `metricRows.ts` render collector fields with byte, rate, duration and percentage units. Node forms bind certificate profiles to configured inbound tags and retain deep reactive feature switches. Save errors remain inside the editor instead of replacing it.

`configurationPresets.ts` creates only new SS-2022 AES-256 listeners with random 32-byte keys; imports preserve their methods and credentials. `XNodeSubscriptionTemplates.vue` owns revision-checked administrator template policy. `XNodeSubscriptionTemplateEditor.vue` validates and previews synthetic output; Retained subscription assignments remain references without editor controls. Format, default and assignment changes persist through the dedicated encrypted settings API.

- `XNodeServiceUsers.vue`: Lists and manages named native identities without Telegram accounts.
- `XNodeServiceEditor.vue`: Edits explicit access groups, expiry, quota and disabled state.
- `XNodeServiceRouting.vue`: Edits owner-scoped native outbounds and routing for authorized nodes.

Named-service browser validation uses the constructed X-Node fixture at 390px/dark and 1440px/light. Creation, access edits, disable, rotation, routing JSON validation, loading, empty, read failures and save failures were exercised through Chrome DevTools MCP. No production agents are contacted by that fixture.

Typed operational editing: `useXNodeGlobalSettings.ts` owns abortable, generation-checked reads and preserves drafts after mutation failures. `XNodeGlobalPolicies.vue` edits opt-in master GeoIP countries, explicit legacy-schedule conversion, and the requested shared-domain JSON list. `XNodeDNSSettings.vue`, `XNodeDNSProviders.vue` and `providerCatalog.ts` provide native resolver controls and write-only provider credential forms verified against the pinned agent registry. Blank credentials retain saved values.

Per-node bandwidth controls are retired; legacy settings decode without restoring those overrides. Native per-user speed policy remains available for commerce. `XNodeFirewallFields.vue` edits typed port/prefix lists and forwarding while `XNodeFirewallRules.vue` retains native detection thresholds. `XNodeAccessBindings.vue` and `nodeFormOptions.ts` derive node/inbound selections from loaded configuration documents. `XNodeEndpointOptions.vue` edits supported client security/transport fields while preserving additional imported settings. Document editors remain for Xray, snippets and administrator subscription documents.


`XNodeRoutingTemplates.vue` edits typed destination matcher presets with guarded
loads, retained failed-save drafts and Go-validated expressions. These presets
control member server routing; subscription client documents stay independent.

`nodeFormOptions.test.ts` verifies explicit zero/max multipliers, six-decimal precision, and rejected omitted/null or over-precise input. `XNodeMetrics.vue` keeps CPU/memory chart buffers only while the metric view is mounted; `XNodeMetricDetails.vue` uses localized metric tables. Status labels are localized in `locales/en/xnode.json` and `locales/zh-CN/xnode.json`.

`useXNodeWorkspace.test.ts` verifies cancellation and late-response rejection, plus write blocking after failed reads. Workspace refreshes cancel earlier reads and metric polls; disposed observations are cleared.

`XNodeSubscriptionRules.vue` edits ordered exact-template rules and previews selection through the Go endpoint. `subscriptionTemplatePresets.ts` supplies independent Xray, sing-box, Clash and Stash presets and shared format ordering. Assignment controls are removed; retained assignment references remain in policy writes and deletion guards. Reads/previews abort stale requests and disable mutation after failed loads.

`XNodeSubscriptionRules.test.ts` covers aborting and discarding superseded Go previews without changing retained assignments.

Certificate forms show only mode-relevant identity fields; node-file profiles require their two paths. Global policy errors use safe localized asset/selector codes and preserve the draft on failure.

`XNodeFirewallGrant.vue` provides localized, validated IP/CIDR exemptions with custom whole-hour deadlines. It reuses operation receipt polling, retains uncertain request keys, permits a different grant while another is pending, and cancels UI tracking on disposal.

Node command receipts display localized delivery and execution diagnostics without raw agent errors. Recovery controls remain available while the connected runtime is not ready.

Subscription endpoint rows expose owner-named Delete actions through the existing
DELETE endpoint and workspace mutation guard. Successful deletion reloads the
authoritative list; failures retain rows and use the shared localized action
error. Buttons are disabled while a mutation runs; empty inventories are explicit.

`XNodeMetricRow.vue` mounts charts and detail sections only while expanded.
`XNodeMetricSection.vue` defers expensive table flattening/mounting until that
individual section opens. The parent maps node names once and keeps bounded
sample histories. Workspace polling requests `summary=true`, avoids overlapping
slow requests, and omits the fleet log payload without changing legacy API reads.
`XNodeLogs.vue` opens selected-node live logs with pause, refresh, stale and empty
states. `useXNodeLogs.ts` polls only while open/visible, fences stale replies,
aborts on close/node changes, clears on session expiry, and holds at most the
server's 500 entries. `useXNodeLogs.test.ts` covers isolation and lifecycle cleanup.

Metric detail tables with more than 100 rows use Nuxt UI native virtualization
in a bounded scroll viewport, keeping expanded CPU inventory tables responsive.

Subscription rules select either a template (including Empty HTTP response) or
Browser page. Previews use the production Go matcher through the existing API
and show response kind/status. Empty templates hide the document editor and only
accept final status 200-599 (403 initially); client-format defaults exclude them.

`useFirewallSearch.ts` owns debounced server search, cancellation/stale-reply
fencing, cursor history and exact-generation unban receipts. `XNodeFirewall.vue`
keeps pending delivery visible and reloads the current search after acknowledged
success; retrying a read does not create another release. Existing positive-hour
administrator grants remain a separate form with unchanged behavior.
`FirewallFilterGroup.vue` builds bounded nested AND/OR expressions and
`FirewallFilterCondition.vue` edits localized IP, rule, status, time and hit
conditions. Report-only records remain in memory for their detection window.

`XNodeGlobalDetection.vue` adapts fleet rules to the existing rules table.
`XNodeFirewallRules.vue` keeps node enable/probe switches separate from the global
threshold/window/ban controls; the global form displays migration conflicts.
The editor omits the retired inventory `source_match` rule, including on old
saves. Its localized name remains available for historical ban evidence; the
backend always disables the rule in effective node and detection policies.

`XNodeCommands.vue` defaults to normal material-rechecking reload and exposes
an explicit forced recovery checkbox. Both use the existing durable command
receipt and capability validation; no additional confirmation flow is added.

Provider cost settings support fixed cycles or hourly plus per-GiB rates, and finite or unlimited traffic. Rate inputs use six decimal places; recorded payments remain separate from estimated rates.

Access groups show a localized delete action only with known zero members and server-confirmed eligibility. Mutations disable controls, reload on success, and refresh eligibility after conflicts. Group deletion reuses the workspace error state.

Global telemetry has no active-IP opt-out. The sampling interval describes the
mandatory IP/activity/rate sample; the independent report interval remains
editable. `useXNodeGlobalSettings.ts` normalizes older loaded opt-outs before
submission, while the backend normalizes both legacy documents and writes.
Loading and failed reads cannot enable saving; failed writes preserve the draft.

`XNodeAutomaticLeases.vue` composes the private, paginated automatic lease view.
`useAutomaticLeases.ts` owns debounced search and visible-page polling, aborts
replaced requests, fences stale responses and clears volatile rows on session
end. `XNodeAutomaticLeaseTable.vue` preserves exact counters and exposes activity,
renewal boundaries, source saturation, expiry and delivery/ban-release state.
`XNodeAutomaticLeaseSettings.vue` reuses the global settings lifecycle and write
authorization, validating the 1..8760 hour duration before saving. IP rows never
enter browser storage; failed refreshes retain a clearly marked last snapshot.

`useAutomaticLeases.test.ts` covers aborted and stale search replies, failed
refresh retention, hidden-page polling, session teardown and shrinking pages.
It runs in hosted frontend CI only.

The lease form uses the existing global write endpoint with
`part=automaticWhitelist`, merging only that field on the server and preserving
newer credentials or sampling settings. The general global form omits the
unedited lease property, preserving lease changes from another administrator.

Node removal uses one confirmation and the existing receipt tracker with explicit blockers. Every squad binding hides removal, including disabled squads. Automatic lease paging retains metadata while loading, offers adjacent/edge controls, and clamps expired final pages. Activity charts use adaptive byte/rate units and raw bytes as secondary detail.

`XNodeRemovalDialog.vue` confirms queued removal and displays explicit blockers.

Node removal remains available for bound nodes. `XNodeNodes.vue` shows durable pending removal and prevents configuration/enrollment changes; `XNodeRemovalDialog.vue` explains the cascade before admission stops and displays exact resumable dependencies. Historical accounting is retained.

`XNodeServiceEditor.test.ts` checks that new terms omit cadence and legacy edits
retain their saved strategy without exposing a reset selector. The editor still
clones existing terms, so quota edits do not mutate the loaded service object.
It copies known access fields and the group-ID array explicitly, supporting
reactive props without passing Vue proxies to `structuredClone`. Unit fixtures
stub Nuxt UI's resolved `Modal`/field names and require the body inputs to render.
Chrome DevTools checked constructed loading, populated, empty, read-error and
save-error states at 390 px and 1440 px in Chinese and English. Keyboard creation
omitted cadence and defaulted to `NO_RESET`; an unchanged legacy edit retained
`DAY`. Dialogs stayed contained, drafts survived refusal, and console/network
checks found no errors or backend requests. Hosted Actions run the regressions.
