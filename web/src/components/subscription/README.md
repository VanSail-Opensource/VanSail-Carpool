# Subscription and uptime

`SubscriptionFormatDiagnostics.vue` shares localized format omission/failure
messages between the member center and public page. It renders only bounded safe
codes and endpoint counts from metadata; disabled format actions remain gated.

Member routing selects published endpoints using host names alone. Long names
wrap in selected values and dropdown items. Node
correspondence and ownership are rechecked on preview, save and compilation;
the editor retains invalid drafts and resolves old node-only selections on save.
Graph selection supplies Add Route's inbound node and remembered entry host.
Only configured route nodes appear in the graph; all authorized endpoints remain
available in selectors, including hosts on nodes not yet used by a route.
`routingTree.ts` combines You, shared ordered node/host hops and administrator-
named template leaves into the interactive graph. `routingTree.test.ts` covers
endpoint/rule-sequence isolation, complete-prefix trace highlighting and readable
template attribution. Graph fitting runs on layout/data/viewport changes without
capturing wheel/touch scrolling or exposing pan/zoom controls. Only configured
inbound nodes/hosts can start a trace; nodes without rules retain graph selection
and filtering without becoming trace entries.
`MemberRoutingTemplateDetails.vue` shows read-only original match settings and
exclusions when a template leaf is selected. Leaf status represents applied,
draft, checking, unavailable or disabled routing; it never implies measured
destination reachability. `routingPresentation.ts` simplifies only proven literal
selector inclusion and preserves other intersections/exclusions without guessing.

`HomeUptimePanel.vue` replaces the home subscription block only for active combos.
Its subscription entrance stays centered on mobile and desktop; a failed read
places Retry on a separate row so it does not shift the entrance.
The entrance is a native button calling the named Vue Router route, following
the existing Connections action. It has no anchor URL for mobile Telegram to
open as a new Mini App document; in-memory routing preserves the launch context
and the shell retains native Back ownership.
`SubscriptionPage.vue` composes the full subscription URL, existing revoke and
connections actions, native host links, and squad history. `HostLinkRow.vue`
owns clipboard feedback and an on-demand QR popup using the installed `qrcode`
library and Nuxt UI modal. `SquadUptime.vue` keeps host history in native,
initially closed disclosures. `UptimeBar.vue` renders time-proportional intervals
with pointer and arrow-key inspection and localized, accessible status labels.

`useSubscriptionData.ts` owns live fetching, 60-second visible-page polling,
superseded request cancellation and lifecycle cleanup. It never restores or
persists bearer links. Successful revocation clears the previous snapshot before
loading current native links; unmounting drops all member data.

The typed client `web/src/api/subscription.ts` calls authenticated member routes.
The server authorizes active local ownership and preferences, joins upstream
host/inbound/node metadata, and derives histories from the existing monitor.
Unknown measurements are neutral gaps. Hosts are green/red; squads and the home
summary add yellow for partial outages, only with complete measurement coverage.
No probe credentials, latency, or numeric HTTP results enter these interfaces.

English and Chinese copy inherits the host-connectivity locale domain. Views
reuse the existing premium-dark layout and do not add dependencies.
`useSubscriptionData.test.ts` covers revocation clearing, obsolete response rejection, visible-page polling and disposal of private state in hosted CI. `UptimeBar.test.ts` covers keyboard inspection and neutral current status after refresh data expires.
`HomeUptimePanel.test.ts` covers entry navigation during loading, the absence of
a document link, and preservation of the Telegram launch URL in hosted CI.

`MemberRoutingEditor.vue` edits ordered endpoint/template choices; `useMemberRouting.ts` owns versioned loads and previews, with revision-checked saves or the DIY page draft owner. The backend enforces ownership and validates references.


`MemberRoutingGraph.vue` reuses BGP placement/edge routing and automatic SVG fitting
for selectable nodes, keyboard activation and exact route-instance highlighting.
`MemberRoutingEditor.vue` now edits ordered purchased-node/preset/node choices.
`useMemberRouting.ts` aborts superseded reads/previews, guards generations and saves
through durable member receipts; failed loads cannot save defaults. Graph predicate
analysis and traces come from the same backend matcher as native compilation.

`SubscriptionFormats.vue` retains availability diagnostics alongside the automatic subscription link.

Member network timelines require summary.monitoringEnabled. The home subscription entrance and published host links remain available while monitoring is disabled. `HomeUptimePanel.vue` also hides retained timelines after a failed refresh.

If preview is unavailable, the tree keeps individual configured edges and template
leaves inspectable with error/draft status; it never infers a validated full path.

`RouteSpeedtestPanel.vue` presents metered pair progress, retained results and a
clearly labeled estimate. `useRouteSpeedtest.ts` reuses durable command keys and
receipt polling with the owner result endpoint. Retry after a status-read error
only refreshes the result; uncertain/partial operations never auto-repeat traffic.
Draft changes and untraced/inactive routes cannot start a test.

`SubscriptionFormats.vue` now retains availability/incomplete/error diagnostics
only. Mini App subscription actions use the existing automatic link; public-page
format selection and explicit-format URLs remain available.

`SubscriptionTuningDiagnostics.vue` shows omitted client options in Mini App availability and the public page. Custom routing is composed by DIY Center; `useMemberRouting.ts` accepts an external draft/save owner while preserving the existing routing API.

The DIY Center entrance lives on Settings. This page retains subscription links,
host inventory and availability diagnostics.

In the external DIY workspace, MemberRoutingEditor omits its repeated heading/refresh description and delegates Save to the shared bar; rule editing and trace controls keep their existing behavior.

Recovered routing receipts use the shared OperationStatusNotice, including
terminal review/failure and polling errors. Applying is shown only for active
work; a status failure retains its refresh action and never unlocks an
unfinished deployment. Standalone refresh remains available while deployment runs.
`useMemberRouting.test.ts` checks receipt resumption, terminal refresh, preserved
drafts and review locks in hosted CI.
