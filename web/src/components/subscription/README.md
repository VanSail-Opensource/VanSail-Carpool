# Subscription

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
Multiple rules share their ingress node/host before branching by ordered rule
identity. Separate entry hosts stay separate, and only the traced branch plus
its shared ingress is highlighted.

`RouteSpeedtestPanel.vue` opens progress, measurements, estimates and failures in
the existing Nuxt modal. Closing the dialog does not reset the durable command
or repeat billed probes; View results reopens the same receipt. Retry refreshes
its status without issuing another probe. Draft changes remain independent.
`MemberRoutingTemplateDetails.vue` shows read-only original match settings and
exclusions when a template leaf is selected. Leaf status represents applied,
draft, checking, unavailable or disabled routing; it never implies measured
destination reachability. `routingPresentation.ts` simplifies only proven literal
selector inclusion and preserves other intersections/exclusions without guessing.

`HomeSubscriptionEntrance.vue` links every onboarded member to Subscription using
Vue Router so Telegram keeps the launch URL. `HomeSubscriptionEntrance.test.ts`
covers navigation without a document link. `SubscriptionPage.vue` composes the
owner-only link, format diagnostics and accessible host links. At wide desktop
widths, the subscription link and host inventory share the main column while
client compatibility uses the adjacent column; phones retain their vertical order.
`HostLinkRow.vue`
provides name editing, clipboard feedback and on-demand QR codes.
At narrow panel widths, copy and QR controls move below the host identity so
long names retain useful reading width; panels at least 28rem wide keep a
single row. Names wrap at words before breaking long tokens, while actions
retain 44px targets and the same DOM/focus order.
Custom-route and split-XHTTP indicators are inline immediately after country
flags, with localized keyboard-accessible tooltips. Desktop subscription actions
reuse the sidebar entrances; mobile retains Connections and Revoke in the link
panel, and the sidebar revoke query still opens the same confirmation dialog.
`useSubscriptionData.ts` polls only while visible and synchronously clears private
data and live readiness on disposal, plus errors on an account/role/onboarding
identity change. Identity changes abort requests and cancel polling before Vue
updates the session view. A surviving authenticated view then makes a fresh read
after that update, without restoring the previous snapshot; logout stays inactive.
Its hosted `useSubscriptionData.test.ts` covers those lifecycle boundaries,
same-user identity recovery and obsolete success/failure responses.
`useSubscriptionDataCache.test.ts` covers immediate optional snapshot rendering,
live confirmation before actions, snapshot expiry, failed refresh recovery,
synchronous identity retirement and ignored replies/polling after account replacement
in hosted CI.
The typed `subscription.ts` client uses no-store responses; `subscription.json`
provides English and Chinese copy.

`MemberRoutingEditor.vue` edits ordered endpoint/template choices; `useMemberRouting.ts` owns versioned loads and previews, with revision-checked saves or the DIY page draft owner. The backend enforces ownership and validates references.


`MemberRoutingGraph.vue` reuses BGP placement/edge routing and automatic SVG fitting
for selectable nodes, keyboard activation and exact route-instance highlighting.
`MemberRoutingEditor.vue` now edits ordered purchased-node/preset/node choices.
`useMemberRouting.ts` aborts superseded reads/previews, guards generations and saves
through durable member receipts; failed loads cannot save defaults. Graph predicate
analysis and traces come from the same backend matcher as native compilation.

`SubscriptionFormats.vue` retains availability diagnostics alongside the automatic subscription link. Empty diagnostics leave no card or reserved desktop column; loading and recoverable errors retain their visible feedback.


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
Compatibility omissions are grouped by their actual output client/format, each
with a separately expandable section. Public-page diagnostics remain scoped to
the selected format and open that section by default; no client support is
inferred beyond the server-provided diagnostic records.

The DIY Center entrance lives on mobile Settings and the desktop sidebar. This page retains subscription links,
host inventory and availability diagnostics.

In the external DIY workspace, MemberRoutingEditor omits its repeated heading/refresh description and delegates Save to the shared bar; rule editing and trace controls keep their existing behavior.

Recovered routing receipts use the shared OperationStatusNotice, including
terminal review/failure and polling errors. Applying is shown only for active
work; a status failure retains its refresh action and never unlocks an
unfinished deployment. Standalone refresh remains available while deployment runs.
`useMemberRouting.test.ts` checks receipt resumption, terminal refresh, preserved
drafts and review locks in hosted CI.

Member responses include ready/firstSyncedAt. A pending first sync shows a
localized state and refreshes at two-second intervals; confirmed links remain
visible during node outages because proof is durable on the owner identity.
Routing selectors retain stable IDs and refreshed unavailable labels. Refresh
never substitutes endpoints using names, addresses or current list ordering.

Managed routing endpoint selectors display localized billing mode and address/port so identities on the same listener remain distinguishable. A dependency review reason marks the graph as needing review even when its older durable phase remains active; route testing stays disabled. The routing audit fixture supports `billing` and `stale-plan`.

Host-name writes capture the host ID and submitted draft. Replacing the row or
disposing it invalidates previous name/copy feedback and closes obsolete dialogs.
`HostLinkRow.test.ts` covers a late success or failure while a replacement host's
draft is open. Busy name edits cannot be dismissed or resubmitted.
