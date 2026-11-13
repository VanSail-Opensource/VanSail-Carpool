# X-Node administration

Audit recovery: `XNodeDDNSSettings.test.ts` covers stable editable rows, endpoint
binding isolation after deletion and temporary duplicate job names.
`XNodeCommands.test.ts` rejects results from a command superseded by a later
action. `XNodeNodes.test.ts` additionally covers serialized enrollment creation
and discarded token responses after the selected editor changes.
`useXNodeWorkspace.test.ts` prevents post-disposal mutation refreshes.
Certificate observers suspend on browser expiry and recover durable history only
after verified same-administrator authentication; logout and disposal remain final.
`XNodeFirewallGrant.test.ts` verifies that grant retries use the submitted
normalized IP text as their identity.

`XNodeConfigurations.vue` exposes a resource-named delete action beside each
configuration. It reuses the workspace mutation/reload/error flow, blocks
duplicate actions while pending and retains failed rows for retry. The existing
DELETE API protects every node reference, including historical nodes; shared
snippets remain intact. `configurationDeletionFixture.ts` provides constructed
success, last-item empty, conflict, failure/retry and slow-action browser states.

New configurations/snippets retain one creation ID while their expected revision
stays 0. `catalogCreation.ts` reconciles ambiguous replies and conflicts through
one fresh collection read, matching that ID, exact name/document and ordered
snippet references or snippet kind. A changed saved definition remains a
conflict; existing edits never bypass optimistic revisions. Name uniqueness
already prevents identical-name duplicates; stable IDs also prevent a renamed
uncertain retry from targeting another resource. Hosted helper/editor/browser
cases exercise unavailable recovery reads, changed drafts and matching recovery.
`catalogCreation.test.ts` checks exact-content, ordering, refusal and revision
boundaries; `XNodeConfigurationsCreation.test.ts` verifies retained creation IDs
and distinct subsequent editors through the real component.

`XNodeAccessGroups.vue` retains access-group/resource ownership and composes
`XNodeEndpointForm.vue` for endpoint drafts.
Group and endpoint creation editors retain a private ID across failed retries,
without changing their new-resource display or endpoint preview input. Each new
editor starts another identity; existing edits keep their saved ID. Hosted
coverage models a committed write followed by response loss and a renamed retry.
`XNodeAccessGroupsCreation.test.ts` covers group creation retry identity and
preserved existing IDs; `XNodeEndpointForm.test.ts` covers the endpoint equivalent.
The endpoint form derives metadata and the expanded inbound list through
read-only `/endpoints/preview`, resets the inbound
after a node change, and preserves saved tags through slow/failed discovery.
`useEndpointPreview.ts` debounces and cancels requests, rejects late responses,
and gates Save against the exact current submitted draft. `endpointForm.ts`
validates catalog-shaped values with Zod, preserves opaque legacy overrides and
keeps inactive option drafts locally while omitting them from submitted settings.
Filtering uses the cloned payload's plain values so nested header/range objects
never reintroduce Vue proxies at the API boundary.
Failed saves retain the draft, show endpoint-specific copy, and refresh native
preview/context before another save can pass validation.

`XNodeEndpointOptions.vue` groups server-applicable fields into bounded tabs and
reuses existing DIY controls. `XNodeEndpointPortRanges.vue` owns Hysteria's typed
inclusive intervals and explicit inherit/disable choices. All XHTTP extra, header,
XMUX and download fields use GUI controls. `endpointLabels.ts` localizes catalog
keys/errors; `XNodeEndpointCompatibility.vue` distinguishes excluded endpoints
from optional omitted settings for the five export families. These projections
never expose generated credentials or add an outbound/backend queue request.
The backend repeats validation against current native settings before encrypted
save. Member DIY permissions remain separate from administrator option types.

Hosted regressions in `endpointForm.test.ts`, `useEndpointPreview.test.ts` and
`XNodeEndpointForm.test.ts` cover preserved drafts, context/dependency filtering,
range/header bounds, response races, authoritative inbound selection and failed
saves. `endpointOptionsFixture.ts` in the browser fixtures provides constructed
protocol/error states; frontend checks must not be confused with live exports.

Chrome DevTools checked endpoints at 1440px/light/English and 390px/dark/Chinese:
derived metadata and protocol-specific TLS/REALITY, WebSocket, gRPC, HTTPUpgrade,
XHTTP/XMUX/download and hopping controls rendered correctly. Invalid headers and
descending ranges blocked Save; valid headers, XHTTP download settings and hopping
1..65535 persisted typed plain payloads with opaque legacy keys. WS-to-gRPC changes
pruned inactive submitted values, node changes cleared stale inbound selections,
and keyboard-cleared ports showed field errors. Empty creation, loading, preview
error/retry, native validation failures and save refusal retained drafts and
refreshed validation. Fresh pointer interactions verified open/change/Save with
clear consoles, contained layouts and no mock-flow fetch/XHR requests. The plain
payload and localized retry fixes passed; earlier HMR/focus warnings were artifacts
of inspection, absent from the final pointer flow.

Node traffic multipliers are retired from node reads, writes and the editor.
`XNodeNodes.vue` constructs only the six editable node fields; it never echoes
server-managed agent bindings, revisions or timestamps. `XNodeNodes.test.ts`
checks this request boundary and immutable save drafts in hosted frontend CI.
Each creation editor retains one generated ID for same-ID PUT upserts after a
lost response. The visible draft stays in creation mode, so pricing/runtime
actions and node-detail reads are unavailable until editing a saved node.
Hosted coverage verifies response-loss retries and a fresh ID for the next editor.
`nodeFormOptions.ts` retains only inbound selection and object coercion helpers;
the retired multiplier validator and its obsolete tests have been removed.
Chrome DevTools checked this flow at 1440px/light/English and 390px/dark/Chinese
with an obsolete 1.25 fixture value before the API field itself was retired. Lists/forms omitted it and successful saves
omitted the property. Failed saves retained drafts and localized errors. Loading,
empty and read-error retry states passed; two/one-column layouts had no body
overflow, console warnings/errors or real mocked-flow network requests.

Certificate administration uses `XNodeCertificateProfile.vue` for mode-specific
fields, configured provider selection and native PEM file imports.
`certificateForm.ts` validates fields and inbound references; the backend checks
the actual PEM key pair and certificate hostname.
File profiles accept relative or absolute paths as the native agent does;
relative paths resolve from its working directory. Validation preserves literal
values, bounds them to 4096 characters and rejects blank paths or NUL/CR/LF.
`certificateForm.ts` derives the native managed mode from a trimmed/case-folded
explicit value, then AutoTLS, a complete PEM pair or a complete file pair. Reads
and unrelated node edits preserve raw configuration; editing a profile field
materializes its inferred mode so clearing one half keeps the relevant controls
visible with an error. Supplied PEM can omit its name; an optional name, including
a literal wildcard, is checked against the real certificate by the backend.
ACME names support IDNA, one trailing dot and single labels without rewriting
stored input. URI separators, encoded names, whitespace and controls are rejected;
optional emails are checked without trimming. Self names require nonblank text.
An unresolved `none` mode remains invalid for a managed certificate profile.
Bound profiles require an
explicit unbind before removal. Existing inline DNS credentials remain opaque.
Inline-only legacy DNS profiles do not depend on global-provider discovery for
unrelated saves; explicit global-provider references still wait for discovery.
Chrome DevTools verified this during a simulated global-provider 503: at
1440px/English, an unrelated node-name edit saved successfully while retaining
the inline provider and opaque credentials. At 390px/Chinese, an explicit global
reference kept its provider error and Save disabled with no writes. Layouts were
contained and fresh consoles/mock request checks were clear.
`XNodeCertificateStatus.vue` and `certificateStatus.ts` project only safe public
metadata, separating failed acquisition/application from retained serving material.
Expiry warnings do not prevent unrelated edits. Missing legacy serving state is
shown as unknown; stale reports cannot authorize lifecycle commands. Failed
activation can still be repaired while runtime readiness is false, using a
connected peer, fresh report, lifecycle capability and the saved revision.

`XNodeCertificateLifecycle.vue` and `useCertificateLifecycle.ts` reuse durable receipt tracking for revision-checked
ACME issue/renew actions. It restores the original operation and preserves a
credential-free original key/action/revision in tab storage after ambiguous
submission. That exact request is reconciled before consulting potentially older
profile history; pending review or partial outcomes block new mutations. Session
end clears attempts and active observers. The backend bounds each operation to
five minutes. New profiles must be saved before lifecycle actions are available.
`certificateAttempts.ts` owns the narrow recovery context; keys and PEM are never
included in status output. `XNodeOperationalSettings.vue` shares the existing
provider discovery read between DDNS and certificates, with cancellation/retry.
The credentials link opens the existing global provider editor in another tab.

Hosted frontend coverage: `certificateForm.test.ts`, `certificateStatus.test.ts`,
`useCertificateLifecycle.test.ts`, `XNodeCertificateSettings.test.ts`, and
`XNodeCertificateProfile.test.ts` cover native mode validation, safe metadata,
original-key recovery, explicit bindings and bounded PEM import. No local
automated suites are required or run for these changes.

Chrome DevTools checked certificate forms and lifecycle controls using constructed data at 1440 px/light and 390 px/dark, in English and Chinese. Checks covered configured DNS providers, file/content modes, native FileReader imports, bound-profile removal, expiry, capability gating, queued and pending-review states, definitive-failure retry with a new key, original-key recovery after an ambiguous submission, and recovery errors/retry. Dialogs remained contained; console and mock network inspections were clear. File-picker automation could not access workspace files, so browser-generated synthetic PEM envelopes exercised import plumbing; hosted Go coverage validates real key pairs. Live ACME/provider issuance was not performed in these browser checks.

`XNodeDNSSettings.vue` exposes only the native DNS redirect switch and emits a
fresh redirect-only policy. Resolver servers belong to the assigned Xray
configuration. The API rejects retired `mode`, `upstreams` and public
`legacyDnsLoopback` fields. Existing persisted fallback is migrated privately by
the backend, without exposing it to editors. `XNodeDNSSettings.test.ts` covers the
current policy, defaults and immutable edits in hosted CI.

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
Built-in and preserves bindings across job renames. `ddnsForm.ts` validates unique jobs and endpoint ownership.
`ddnsValidation.ts` validates native address families/unicast sources, auto
discovery, HTTPS lookup syntax, interface names, exact DNS zone ownership and
whole-second interval/TTL bounds. DNS owner labels support IDNA, service labels,
leading wildcards and one trailing dot. TTL is bounded at 2147483647 seconds
before native duration conversion. Field errors and the collapsed-section warning
explain why Save is unavailable.
Provider discovery remains cancellable and Built-in is available during failures.
Copy is localized in the node-forms locale files; reduced motion uses native controls.

`XNodeDDNSSettings.vue` edits the pinned agent's DDNS fields, including optional
HTTPS IP lookup URLs and bounded TTL (new jobs default to 300 seconds).
`XNodeDDNSJob.vue` owns one job's localized fields and removal action.
`useDDNSDiscovery.ts` loads saved provider IDs without secrets. `XNodeDDNSZone.vue`
owns cancellable zone loading/retry and searchable name selection. A text field
allows new or retained zones when discovery is unsupported, empty or unavailable.
Results remain memory-only. `ddnsValidation.test.ts` covers the native domain and
source boundaries; the hosted browser audit covers manual zones and TTL errors.

`XNodeFirewall.vue` shows shared detection evidence, authoritative deadlines and
remaining node expiry confirmations. `XNodeFirewallRules.vue` edits report-only
rule toggles, thresholds, windows and ban duration. Global policy also sets the
paid exact-IP exemption price and hours; both must be positive to enable purchase.

`XNodeWorkspace.vue` composes the independent browser console's nodes, global
policy, configurations/snippets, access groups/endpoints and live metrics.
`workspaceTabs.ts` declares the inventory each tab consumes.
`useXNodeWorkspace.ts` loads only that inventory, isolates optional live metrics,
and runs nonoverlapping metric polls only on visible Nodes/Metrics tabs. Success
uses a five-second interval; failures back off to a sixty-second maximum. Tab
changes and session clearing abort/retire obsolete replies. Previous observations
are marked stale until refreshed. Confirmed writes remain successful when only
the follow-up read fails, while required inventory failure blocks new edits.
`XNodeWorkspace.vue` persists its selected tab in `xnodeTab` and keys the loading
boundary per tab so a previous page cannot mark the next page ready. `XNodeNodes.vue` edits one assigned profile and node utilities and exposes
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

Typed operational editing: `useXNodeGlobalSettings.ts` owns abortable, generation-checked reads and preserves drafts after mutation failures. `XNodeGlobalPolicies.vue` edits opt-in master GeoIP countries, explicit legacy-schedule conversion, and the requested shared-domain JSON list. `XNodeDNSSettings.vue`, `XNodeDNSProviders.vue` and `providerCatalog.ts` provide native DNS redirect and write-only provider credential forms verified against the pinned agent registry. Blank credentials retain saved values.

Per-node bandwidth controls are retired; legacy settings decode without restoring those overrides. Native per-user speed policy remains available for commerce. `XNodeFirewallFields.vue` edits typed port/prefix lists and forwarding while `XNodeFirewallRules.vue` retains native detection thresholds. `XNodeAccessBindings.vue` and `nodeFormOptions.ts` derive node/inbound selections from loaded configuration documents. `XNodeEndpointOptions.vue` edits supported client security/transport fields while preserving additional imported settings. Document editors remain for Xray, snippets and administrator subscription documents.


`XNodeRoutingTemplates.vue` edits typed destination matcher presets with guarded
loads, retained failed-save drafts and Go-validated expressions. These presets
control member server routing; subscription client documents stay independent.

`XNodeMetrics.vue` keeps CPU/memory chart buffers only while the metric view is mounted; `XNodeMetricDetails.vue` uses localized metric tables. Status labels are localized in `locales/en/xnode.json` and `locales/zh-CN/xnode.json`.

`useXNodeWorkspace.test.ts` covers dependency scoping, telemetry failure/latency
isolation, backoff, visibility, session cleanup, cancellation, stale-response
rejection and write blocking after failed required reads. `nodeStatus.ts` shares
missing, stale, offline, not-ready, unsynchronized and online distinctions across
list/metrics rows. Node rows index metrics once and wrap long identities; mobile
actions occupy their own row. `nodeStatus.test.ts` covers those evidence states.

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
editable. The ignored `include_active_ips` field is removed from public types,
new writes and editor normalization. The backend privately reads historical
encrypted settings without restoring a writable opt-out.
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

Node removal uses one confirmation and the existing receipt tracker with explicit resumable dependencies. Automatic lease paging retains metadata while loading, offers adjacent/edge controls, and clamps expired final pages. Activity charts use adaptive byte/rate units and raw bytes as secondary detail.

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

The final certificate compatibility pass used Chrome at 1440px/light/English and
390px/dark/Chinese. Unrelated node saves preserved inferred file/content/HTTP
modes and literal relative paths, empty content domains and IDN/trailing-dot
names. Clearing an inferred file path kept both file controls visible, marked
the field invalid and blocked Save; refilling restored validity and materialized
the displayed mode only after that explicit profile edit. Normalized explicit
modes, single-label hosts, private self-signed names and optional wildcard PEM
names rendered correctly. The mobile optional-domain/PEM layout was inspected
without horizontal overflow. Console checks were clear and mock fetch/XHR made
no actual failing requests. PEM envelopes were synthetic; real key/name matching
is covered by hosted backend regressions. No live issuance was performed.

Service routing reads and saves capture both native owner and node identity.
Changing either invalidates older requests; saving outbounds preserves unsaved
rules and saving rules preserves unsaved outbounds. Named-service list refreshes
cannot overwrite newer mutation reloads, returned subscription links identify
their owner, and disposed views cannot restart loading.

Configuration, snippet, group and endpoint writes capture their payload before
the shared mutation runner executes. Editors clone reactive API data through the
existing compatibility helper and retain failed drafts. Busy dialogs and fields
stay locked until the result is known. Group writes include only editable
identity/name/enabled/binding fields; server-derived membership remains read-only.

`XNodeServiceRouting.test.ts` covers owner replacement, late saves, reactive
snapshots and independent routing drafts. `XNodeServiceUsers.test.ts` covers
stale refreshes, owner-labelled links and disposal during a save.
`XNodeEditorSnapshots.test.ts` covers deferred configuration/group writers and
busy controls. `XNodeServiceEditor.test.ts` also guards busy keyboard submission
and modal dismissal. These additions require hosted CI; no new browser execution
is claimed for the audit reconstruction.

`XNodeNodes.test.ts` also covers immutable node-save payloads and local pending
locks when a workspace mutation runner defers dispatch. Node fields and native
modal closing stay disabled until that reviewed save settles.
