# IP Lookup
- `useIPLookup.ts` owns quotes, idempotent submission, account isolation and receipt polling.
- `IPLookupPage.vue` composes member loading, input, errors and report states.
- `IPLookupForm.vue` keeps the address and 44px lookup control on one row, with the current quota or TXB price below.
- `IPLookupReport.vue` presents only the compact frozen verdict, refusal evidence, aggregate facts, field sources, database participation and four MaxMind fields. Absent values are hidden; zero remains visible.
- `reportPresentation.ts` derives localized, nullable aggregate rows without retaining provider payloads.
- `IPLocationMap.vue` and `map.ts` embed the selected coordinate pair through OpenStreetMap's native iframe, preserve its attribution and reuse Telegram-aware external links. Invalid pairs are hidden; map loading never changes lookup billing.
- `presentation.ts` maps safe errors, provider/subdatabase names and source labels to translations.

The feature uses Vue Composition API, existing dark-theme controls and English/Chinese localization. Route views only compose feature components. Async results are scoped to the current member, and scope disposal cancels future polling and quote updates. Browser fixtures exercise these real components through Chrome DevTools MCP. No local automated test suite is required or permitted.

- `useIPLookup.test.ts` covers quote-only reads, duplicate suppression, ambiguous submission replay and polling cleanup for hosted CI.

Quote reads automatically display exact-IP or IPv4 /24 cached reports without creating a paid receipt or consuming quota. A /24 reuse labels both the requested address and the original cached address. Cached addresses offer a signed refresh action. Provider API errors stay in the cached completed report and do not trigger refunds; application/queue failures remain distinct operational errors.

The map uses only a fixed HTTPS OpenStreetMap origin with a bounded nondegenerate extent, a lazy iframe, a localized accessible title and no referrer. The HTTP CSP permits that frame origin without expanding parent script, image or connection origins. Backend coordinates and country/city are selected together; the frontend does not mix database locations.

- `presentation.test.ts` covers coordinate edge cases, hidden nullable rows, zero values and source names for hosted CI.
- `useIPLookup.test.ts` also covers automatic subnet cache display, stale address/account response isolation and receipt rehydration preserving the original fresh status and charge/quota usage when it fills the address input.

## Browser evidence, 2026-10-10

Chrome DevTools MCP 0.26.0 with Chrome 154 reviewed the real components through
the constructed fixture at 320/390px phone widths and 1280px desktop width.
Input/actions stayed inline with 44px controls and viewport-width documents.
Exact/subnet previews issued one quote and zero submissions while retaining the
full allowance. Fresh refusals, partial/all-source outages, empty/loading,
disabled/application errors, insufficient balance, null rows, zero coordinates,
and English/Chinese content were inspected. Fresh receipts retained quota usage
after address hydration. No console warnings/errors or failed requests appeared
in the completed reviews. Provider outcomes were constructed, not live API proof.

The native OpenStreetMap frame loaded with its marker and attribution; final
map sizing was inspected using ordinary viewport resizing because DevTools
emulation temporarily mis-sized the cross-origin canvas. No map lifecycle or
provider-request workaround was added to production code.

## Live enrichment

`useLiveIPDetails.ts` fetches once per requested-address/account/report identity,
uses signed no-store streaming with JSON fallback, cancels obsolete work, and owns explicit refresh. Completed sections remain visible during refresh failures.
`IPLiveDetails.vue` composes independent source sections beside the frozen
reputation report. It emits combined paid refresh intent to the parent; realtime evidence never changes the quality verdict.

- `IPBGPTopology.vue` renders a native SVG with a versioned observed-hop/T1 layout, anchored wheel/pinch zoom, pointer/keyboard pan, full two-dimensional fitting and route highlighting. Only compact route summaries remain public; full AS paths are transient on the backend.
- `bgpGraph.ts` precomputes compact topology indexes and route focus; `useGraphViewport.ts` owns transform interaction without persistent state.
- `IPASNDetails.vue` selects multiple origins, displays nullable registration/routing metrics and their dated provenance.
- `IPTrafficBars.vue` preserves Cloudflare percentages, Other devices, source status, and actual seven-day windows.
- `LiveSourceNote.vue` presents independent source availability and observation times.
- `useLiveIPDetails.test.ts` and `bgpGraph.test.ts` cover requested-IP isolation, cancelled/late responses, refresh, multiple origins and bounded graph visibility in hosted CI.

The backend persists compact details without expiry: exact IP first, then IPv4 /24; IPv6 exact-only. Cache reads are free; all fresh collection uses a quoted operation. Refresh consumes the configured allowance quantity when available, otherwise TXB. ASN names retain their existing cache. Live frontend data is scope-local
and excluded from browser response snapshots by the existing IP Lookup cache
policy. Long refusal reasons use a fixed icon column and wrapping text.

## Browser evidence for live enrichment

Chrome DevTools MCP 1.10.1 with isolated Chrome 154 reviewed the real components
at 320/390px mobile and 1280px desktop widths: initial/loading/populated/empty,
partial/restricted/unconfigured sections, request errors, IPv6 and multiple
origins, zero counts/ratios, long multiple refusals, English/Chinese, graph
expansion/path highlighting/zoom/refresh, and Cloudflare replace/clear controls.
Document widths matched viewports, and final console and network reviews were
clear. Cached/subnet previews performed one live request for the requested IP
and zero paid submissions; manual refresh added one live request. Provider
outcomes in these fixtures were constructed, not paid-provider live proof.

Separate read-only backend probes fetched public IPv4/IPv6 topology, actual RIR
registration, dated CAIDA degrees, and PTR records; source timeouts remained
explicitly unavailable. InternetDB is queried live but its observations are
weekly and do not certify current port reachability. No Telegram messages were
sent by the browser review or the offline PM fixtures.

## Compact progress and graph responsibilities

- `IPLookupProgress.vue` displays completed logical work with accessible Nuxt progress; percentages are not time estimates.
- `IPLookupProgressTasks.vue` renders the per-task checklist under that progress: live sections and reputation providers each show pending, active, success, partial, error, or skipped state with localized labels; legacy progress events without tasks fall back to the bar alone.
- `IPRefusals.vue` and `refusalPresentation.ts` group all distinct refusal types, deduplicate exact evidence and expose every source/value on tap.
- `refusalPresentation.test.ts` verifies evidence preservation in hosted CI.
- `bgpLayout.ts` orders layered nodes by observed-route count and routes shared trunks, separate ports and non-adjacent connections without inventing AS hops.
- `useBGPGraph.ts` owns ASN/route selection state, viewport culling, resize observation and camera preservation during enrichment.
- `IPBGPFocus.vue` paginates compact observed routes to the selected ASN; choosing a route highlights only its links without switching the graph layout.
- `useGraphViewport.test.ts` checks fit bounds, zoom anchors and multi-pointer cleanup in hosted CI.
- `bgpLayout.test.ts` checks that merged visual trunks preserve gaps and separate carriers; no new AS connectivity is inferred.

Tier-1 labels use CAIDA's explicit dated transit-free clique inference. Unknown
classification falls back to direct neighbors; zero upstream degree is never
proof of membership. Names remain rendered at every zoom level; compact observed route summaries and compressed hop counts stay available through ASN selection. Gesture handlers are
scoped to the graph and do not change Telegram navigation.

Membership now comes from `dataset.clique`, not the faulty per-ASN API boolean.
Direct upstreams include only observed, classified transit pairs. An origin
with classified peers/customers and no providers receives an explicit empty
message; unavailable direct classifications receive a separate unknown message.
This distinction keeps transit-free origins useful without inventing upstreams.

## Browser review, 2026-10-11

Chrome DevTools MCP 0.26.0 and 1.10.1 with isolated Chrome 154 reviewed
320/390px mobile and 1280px desktop layouts, English/Chinese, early/loading
sections, empty/error/restricted/unconfigured states, IPv6/multiple origins,
unknown T1 fallback, refusal evidence, wheel zoom, constructed pinch input,
path focus/pagination and refresh failure retention. Document widths matched
viewports; all initial preset nodes fitted both dimensions. Constructed pinch
input disables capture only in the audit script, since synthetic pointers are
not registered with the browser; this is not native-device evidence.

With identical constructed delays, first data appeared after 409ms through
streaming versus 3291ms with all-at-once display; completion remained about
3.3 seconds. Returning to the same IP added no simulated upstream run; Refresh
added one and retained the full quota with zero paid submissions. These timings
and counters describe fixtures, not live paid-provider performance.

The dense fixture retains 300 ASNs, 1,500 connections and 1,000 original paths.
Connections render in grouped SVG paths by relationship/highlight rather than
one DOM path per connection; viewport culling and 50-row pagination bound
interaction work while retaining complete observed evidence.
The SVG itself stays viewport-sized. At distant zoom, unreadable labels are
hidden and coincident straight route trunks are merged without joining gaps;
selected paths retain their precise route geometry.

## Live-source correction review, 2026-10-11

Queued public-source probes reproduced both issues on `47.181.58.83` and
`172.127.58.147`: the per-ASN clique flag marked every observed ASN true, and
50-link queries timed out. Dataset membership and bounded origin connections
now classify Frontier AS5650 as non-T1, retain AT&T AS7018 as T1, and identify
observed Frontier upstreams AS174 and AS701. AT&T's observed direct connections
are peers, so its transit-upstream preset correctly reports an empty result.

Chrome DevTools MCP 1.10.1 reviewed these actual queued projections at 390px
mobile and 1280px desktop widths, including false-positive exclusion, direct
upstreams, valid empty results, constructed unclassified-data errors, initial
fit, English/Chinese, console and network checks. Viewports did not overflow;
final reviews had no console warnings/errors or failed requests. Reputation
and billing were constructed fixtures; no paid checks or Telegram messages ran.

## Persistent cache and combined refresh UI

`useIPLookup.ts` obtains a signed refresh quote and owns idempotent submission,
receipt polling, owner isolation and current-price display. `IPLiveDetails.vue`
receives the completed `liveDetails` snapshot and emits refresh intent;
`useLiveIPDetails.ts` performs free cancellable reads and retains current data
during refresh. A neighboring-IP cache displays its source IP and original fetch
time so PTR/InternetDB observations are not presented as evidence for the
requested host. Blue upstreams, green peers, muted customer/unknown links and
dashed compressed paths have a localized legend inside Details. Selection,
keyboard operation, anchored zoom, pan and fit stay in the existing SVG/view
composables. Earlier browser reviews below predate persistent cache and billing;
new validation must exercise these revised contracts independently.

## Routing and registry presentation, version 2

`IPBGPTopology.vue` always renders ASN/name labels, with readable scale-aware type
and full accessible titles. Phones use a 440–600px stable-viewport height. All five
relationship explanations live in the native Details disclosure. `bgpGraph.ts`
indexes compact observed route summaries, normalizes edge direction and highlights
only recorded segments. `bgpLayout.ts` sorts each column by observed-route count
descending, then ASN; crossing reduction cannot override this ordering.
`IPBGPFocus.vue` presents connection direction, count, hop length and source date.

T1-origin snapshots show observed T1 context only. Other origins retain observed
intermediates toward T1 and nearby inferred upstreams; irrelevant non-T1 collector
branches are removed by the backend. Missing inference stays explicitly unknown.
Older cached projections remain readable and require paid refresh for extra data.

`IPRPKIStatus.vue` receives the optional frozen `report.rpki` projection and shows
each origin's validity, source status and available ROA details beside quality.
`IPTransferHistory.vue` receives the optional snapshot history and loading state;
it displays newest-first holder/registry/resource rows with 25-row pagination,
separate empty/error states and a legacy refresh hint. `IPASNDetails.vue` extends
the existing origin selector with safe company links and abuse email contacts.
`IPLiveDetails.vue` keeps PTR values flush with their heading and distinguishes
suppressed legacy blocks. All new copy is localized in English and Chinese.

The API schemas are additive; the existing details/check endpoints and signed
refresh flow remain in use. New transfer events and RPKI tasks use actual progress
totals. `useLiveIPDetails.ts` continues to cancel obsolete work and retain cached
source identity; neighboring-IP evidence never becomes an exact-IP observation.

Chrome DevTools MCP reviewed the revised components at 320/390px mobile and
1280px desktop widths: initial/loading/populated/empty/error and interrupted
streams, English/Chinese, all RPKI states, multiple origins, a 300-node graph,
labels after zoom, route focus, keyboard pan/fit, Details and legacy paid refresh.
PTR indentation measured zero and document widths matched viewport widths;
final console and network checks were clear. Constructed refresh consumed no
quota, and subnet cache reuse made no additional upstream run or paid submission.

Queued public-source snapshots were also rendered for `172.127.58.147` (15 T1
nodes only), `23.151.104.0` (22 nodes with transit/peer intermediates retained),
and `118.127.59.0` (19 nodes, retaining AS9280/AS38195/AS58511/AS7473 and excluding
unrelated collectors). Every rendered column sorted by observation count.
Reputation and billing in these browser fixtures were constructed; the separate
queued RPKI probe validated the real covering prefixes for all three addresses.

## Background combined report presentation

`useSharedIPProgress.ts` retrieves identity-free exact-IP work on entry, polls every five visible seconds and resumes immediately after returning. `IPPortProbes.vue` shows per-port coverage, suspected/confirmed evidence and long observation milestones; `IPProxyEvidence.vue` displays administrator source names and compact coverage rows; `IPSubnetGrid.vue` renders 255 neighboring blocks with touch/keyboard details and an independent requested-IP indicator. Completed reports provide all three projections without extra fresh requests. Latitude/longitude remain in map inputs and API data but no longer appear as text facts.

## Shodan findings

`IPShodanDetails.vue` presents the saved or streamed Shodan section through a typed
`result` prop. It shows every CPE, tag and vulnerability alongside hostnames and
ports, wrapping long identifiers on phones and using two columns on desktop.
Tags and vulnerabilities use the error color; CPEs are informational.
`IPRefusals.vue` opens Shodan refusal groups by default so detected items appear
beside the unsuitable verdict. `presentation.ts` resolves their source name.
The backend owns the verdict; this component performs no extra requests or
billing and accepts legacy snapshots with absent arrays. Labels and policy copy
are localized in English and Chinese. Browser fixtures accept `shodan=tags`,
`vulns`, `both`, `cpes`, `many-cpes`, `partial` or `legacy`.

Chrome DevTools MCP reviewed initial/loading/populated/empty/error and legacy
states at 320/390px mobile and 1280px desktop, English/Chinese copy, native
disclosures and streamed completion. Tag-only and vulnerability-only reports
showed Refuse; CPE-only reports showed Pass. A 300-CPE fixture rendered through
the final item, long identifiers wrapped without horizontal overflow, and console
and network checks were clear. These browser reports use constructed evidence;
backend parser/verdict/persistence regressions run in hosted CI.
