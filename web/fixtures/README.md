# Constructed browser fixtures

`route-recovery-audit.html` mounts the actual browser recovery controller and
startup failure view around a synthetic lazy user route. Open the profile control
for client navigation, add `direct` for an initial destination failure, `permanent`
for repeated chunk failure, or `runtime` for a module evaluation error. A transient
failure retries once with the intended query/hash; persistent failure stops after
one document-replacement attempt and the localized retry recovers on demand.
Inspect `window.__routeRecoveryAudit` for attempts, replacements and captured
errors. Add `zh` and check desktop/mobile layouts. No backend requests are made.

For configuration deletion open `xnode-audit.html?configuration-deletion` and
select Xray & snippets. Shared configuration returns an in-use conflict; the
unused fallback can be deleted. Add `configuration-only-unused` to check the
last-row empty state, `configuration-delete-slow` for pending controls, or
`configuration-delete-error` for a first failure followed by successful retry.
`configurationDeletionFixture.ts` intercepts every configuration delete in this
fixture; no production mutation occurs. Existing `empty`, `error`, `slow`, `zh`
and appearance flags remain available. Inspect `window.__xnodeAudit.writes`.

`hourlyUsageFixture.ts` extends `usage-breakdown-data.ts` with authoritative-shaped
named hourly slices for `fixtures/usage-breakdown-audit.html?hourly`. Use
`hourly-one` for one 100% squad, `hourly-idle` for cost-only paid time,
`hourly-only` for multiple squads, `hourly-small` for named shares below 1% in
both measures, `hourly-ended` for retained ended enrollment facts, `hourly-extra`
for shared Unallocated charges, `hourly-large` for exact values beyond 2^53,
`hourly-unnamed` for the localized historical name and `hourly-omitted` for the
combined 64-node/64-squad limit plus exact omitted totals. The mixed `hourly`
fixture deliberately reuses the same physical node/squad ID. Combine with `zh`,
`light`, `browser`, `slow=1500`, `error`, `empty`, `gaps` or `stale` as relevant.
For error recovery set `window.__usageBreakdownAudit.recover=true`, then Refresh.

`endpointOptionsFixture.ts` adds `endpoint-options` to the X-Node audit. Open the
Groups tab and edit Client endpoint; the default uses XHTTP with TLS and a separate
download connection. Add `endpoint-protocol=hysteria`, `ws`, `grpc`, `httpupgrade`
or `reality` to check relevant controls. Hysteria exposes typed hopping intervals;
XHTTP tabs expose headers, padding/session/sequence, XMUX and download fields.
Use `endpoint-preview-slow`, `endpoint-preview-error` (Retry recovers),
`endpoint-validation-error`, `endpoint-save-error`, and `endpoint-empty` for
states. Existing `zh`, appearance, loading and empty flags still apply. Check
1440px/390px layouts, category scrolling, retained invalid text, disabled Save,
node/inbound changes and `window.__xnodeAudit.writes` for exact typed payloads.
All preview/save operations are constructed; no credentials or external requests
are generated. Client diagnostics demonstrate the GUI; hosted renderer regressions
establish adapter behavior against pinned upstream documentation.

The X-Node audit's `legacy-multiplier` parameter injects an obsolete 1.25 value
directly into its constructed node data. The list and editor must display no
multiplier control or badge, and `window.__xnodeAudit.writes` must omit that field
after saving. Normal fixtures use the compatibility value 1. Existing loading,
read/save failure, empty, Chinese and light/dark flags apply to the same flow.

`certificateFixture.ts` augments the X-Node audit with `certificates` for desired
profiles, a failed attempt and a still-serving certificate. Add `certificate-dns`,
`certificate-file`, `certificate-content`, `certificate-expired`,
`certificate-old-agent`, `certificate-pending`, `certificate-recovery-error`,
`certificate-failure`, `certificate-ambiguous`, or `certificate-replay` to inspect
provider selection, PEM controls, status and original receipt recovery. Existing
`node-save-error`, `slow`, `zh` and appearance flags remain available. Constructed
commands and receipts remain inside the fixture; no provider request is made.
Add `certificate-relative-file` with `certificates` to verify that `./tls/cert.pem`
and `../shared/key.pem` stay editable and survive an unrelated node Save unchanged.
`certificate-inferred-file` and `certificate-inferred-content` leave `cert_mode`
empty to check native inference and retained raw values. Clearing one material
field must keep its controls visible/error-marked; refilling it restores validity.
`certificate-content-no-domain`, `certificate-content-wildcard`,
`certificate-inferred-http`, `certificate-normalized-mode`, `certificate-idn`,
`certificate-single-label` and `certificate-self-name` cover remaining native
compatibility cases. PEM in these fixtures is a synthetic envelope for GUI checks;
hosted backend regressions exercise actual key pairs and certificate names.
Use `certificates&certificate-inline&certificate-providers-error` to check a
retained inline DNS profile during global discovery failure: unrelated node
settings can still save with opaque synthetic credentials preserved. Compare
`certificates&certificate-dns&certificate-providers-error`, where the explicit
global-provider reference must keep Save blocked until discovery recovers.

The X-Node audit's `firewall-ranges` parameter loads mixed legacy port lists,
TCP/UDP/SSH intervals, range-to-one and range-to-range forwarding. Combine with
`slow`, `node-load-error`, `node-save-error`, `empty`, and `zh` for state/localization
audits; `window.__xnodeAudit.writes` records saves without contacting a backend.

Add `legacy-dns` to inspect the DNS redirect-only form with a retained legacy
resolver document. Toggling redirect retains its mode/upstream evidence for the
server migration; the fixture's recorded writes expose the original values.

`UsageBreakdownAudit.vue`, `usage-breakdown-audit.ts`, `usage-breakdown-data.ts`
and `usage-breakdown-audit.html` mount the real member panel and Unovis chart.
The shared signed transport intercepts only `/api/v1/usage/breakdown` with
constructed data. `interaction-feedback.ts` provides the same bounded recording
SDK/delegated listener used by other interaction fixtures; browser mode omits it.
Add `zh`, `light`, `browser`, `slow=1500`, `error`, `invalid`, `invalid-total`,
`empty`, `one`, `one-traffic`, `one-cost`, `unallocated`, `hourly-only`,
`unknown-term`, `large`, `tiny`, `grouping`, `omitted`, `truncated`, `incomplete`,
`no-since`, `gaps` or `stale`. `window.__usageBreakdownAudit` exposes synthetic
calls, feedback, parameters and the last snapshot; set `recover=true` before
retrying an injected error. No wallet, configuration or upstream service changes.
Coverage/gap/truncation flags now only exercise response handling because the
member panel intentionally omits that descriptive prose. The
`subscription-audit.html?mode=home&zero-balance` case shows the add-balance guide,
not the history; `mode=home&inactive` with its default positive balance or
`mode=home&zero-balance&combo` still shows the member product sections. Previous
`history-*` fixture controls retain their meaning.

`browser-auth-audit.html?recovery` mounts the actual member session bootstrap and
`AuthGate` against a synthetic `409 ACCOUNT_RECOVERY_PENDING`; add `zh` for
Chinese. Retry repeats only the synthetic exchange (`__browserAuthAudit.exchanges`)
and creates no session, benefit, refund or upstream request.
`recoveryslow=1500` exposes the actual `SessionEntrance` loading state before the
recovery error; retry follows the same session-driven transition as the Mini App.

`HourlyAudit.vue`, `hourly-audit.ts`, `hourly-data.ts` and `hourly-audit.html`
mount the actual hourly member/admin components against synthetic API state.
Open `/fixtures/hourly-audit.html`; add `admin`, `light`, `zh`, `slow=2000`,
`error`, `empty`, `legacy`, `paused`, `active`, `suspended`, `cooldown`,
`activation`, `incompatible`, `extra`, `price-rise`, `no-extra`, `pending`,
`conflict`, `queue`, `failed`, `first` or `catalog` to exercise the corresponding
state. `first` removes enrollment and usage so the never-charged layout shows
squads only. The admin case also lists a pay-as-you-go squad that must not appear
in the hourly offers. `window.__hourlyAudit` exposes only constructed requests, idempotency
keys, haptic observations and fixture state. Set `recover=true` then retry to
recover an injected read failure. Writes and receipt polling never contact
production.

`onboarding-admin-audit.ts` / `.html` mount the actual bilingual admin content
editor with constructed publication receipts. `slow`, `error`, `publish-error`,
and `zh` cover load/publication states; `__agreementAdminAudit.publishes` counts
publications without touching members or sending messages.

`StablecoinAudit.vue`, `stablecoin-audit.ts`, and `stablecoin-audit.html` mount
the built-in token settings without chain calls. `slow`, `error`, `save-error`,
and `zh` exercise localized load/save states; `__stablecoinAudit.writes` records
configuration writes and verifies that no independent rate is submitted.
`payment` displays real built-in payment instructions; `usdc`, `expired`, and
`external` check the fee notice across tokens, expiry, and provider scope.

X-Node `open-metrics` shows an exact large open-flow count; `zero-metrics` shows
zero and the default remains unknown. `node-statistics` mounts the real node
statistics section via `NodeStatisticsAudit.vue`; combine `unknown-metrics`,
`stale-metrics`, `offline`, `slow`, `empty`, and `zero-metrics` for count states.

PAYG `deferred` exposes an Explore entry action; `window.paygAuditCalls` and
resource timings show when the PAYG import and reads start. Combine with
`slow=1800`, `empty` or `error` for loading/recovery states. Use
`hourly-audit.html?catalog` for constructed Charge by hour controls and receipts;
fixed-term checkout is retired.

Routing/DIY `recovered`, `recovered=review`, `recovered=failed` and `poll-error`
exercise receipt resumption on entry, terminal states and status retry.
`shell` includes the real mobile navigation to check sticky Save clearance;
combine with `diy`, `populated`, `split`, `long-names` and `zh` for phone layouts.

`onboarding-audit.html` mounts the real agreement flow with Telegram identity.
`intro`, `legacy`, `slow`, `empty`, `error`, `save-error`, `nickname` and `zh`
exercise username-free onboarding and recovery. `window.__onboardingAudit` records
only constructed transitions; no production authentication or messages run.
`advance-error` refuses the first intro transition, then permits explicit retry.
`defer-advance` holds the transition for owner-clearing checks; the trace exposes
release and owner controls plus a mutable agreement revision and locale switch.

X-Node `controls` mounts Nuxt UI switches, checkboxes, text inputs and a portalled
form under the shared admin sizing classes. Inspect both themes and widths;
hidden successful form controls must retain their native 1x1 geometry.
`AdminControlsAudit.vue` owns that isolated form. X-Node `ddns` supplies saved
providers/zones; `zones-slow`, `zones-empty` and `zones-error` exercise discovery
loading, empty and first-failure retry without external API requests.

`subscription-preview.go` is a manual loopback Go preview bridge for
`xnode-audit.ts`. Run `go run ./web/fixtures/subscription-preview.go` from the
repository root alongside Vite on 127.0.0.1:5173. It invokes production regex
selection and synthetic document previews, accepts only that browser origin,
and bounds HTTP bodies/timeouts. It has no database or upstream calls and is
excluded from production builds.

`BrowserAuthAudit.vue`, `browser-auth-audit.ts`, and `browser-auth-audit.html`
mount the real browser entrance, Mini App link panel, and browser exchange view.
`link`, `exchange`, `slow`, `empty`, `error`, `expired`, `invalid`, and `zh`
exercise link-only administrator access without production requests.
`exchange&unavailable` returns one retryable 503 before successful exchange.
`window.__browserAuthAudit` records mock issuance, exchange, and clipboard actions.

`PMAdminAudit.vue`, `pm-admin-audit.ts` and `pm-admin-audit.html` mount the real
user-profile conversation panel and its reference-only delivery dialogs. Use
`user=ada|mira|review` to select a constructed conversation. `empty`, `error`,
`slow`, `reviewempty`, `reviewerror`, `reviewslow`, `pending`, `moderationerror`,
`pollerror`, `reduced` and `zh` exercise states without sending messages. The
fixture injects a browser-owned operation reader; it never substitutes the
Mini App receipt client. `__pmAudit` and `__pmConversations` expose results.

`CaptchaAudit.vue`, `captcha-audit.ts` and `captcha-audit.html` mount the actual
entry gate with Cloudflare's public always-pass test site key and a mocked signed
verification response. `mock`, `rejected`, `unavailable`, `missing`, `loaderror`
and `delay` provide repeatable failure, expiry, loading and success states;
`__captchaAudit`, `__captchaExpire` and `__captchaProviderError` expose state.
`admin` mounts the real settings panel with write-only secret data; its recorded
`__captchaSettingsAudit.writes` verifies key saves precede enabling.

`SettingsAudit.vue`, `settings-audit.ts` and `settings-audit.html` mount the actual
settings page with isolated squad/activation receipts and existing preferences.
`single`, `inactive`, `noqueued`, `slow`, `error`, and `pollerror` exercise eligible,
loading, empty and recovery states. `window.__controlsAudit` records mutations;
`window.__controlsSnapshot()` exposes resulting term dates for inspection.
The Settings fixture also supplies two optional squads with one node each and
different popularity shares for Add squads geometry, sorting and checkout audits.
`addonempty`, `addonslow` and `addonerror` exercise popup state handling.
`refund` mounts retained purchase/refund actions and verifies the manual reset
control and dialog are absent while server-quoted refunds remain available.
`CommunityAudit.vue`, `community-audit.ts` and `community-audit.html` exercise the
full-width community layout with `locked`, `joined`, `slow` and `error` states.
`defer-membership` and `defer-invite` hold owner-bound replies; volatile
`window.__communityAudit` controls clear the owner and release the reply, and
record attempted external opens without navigating to Telegram.

- `LuckyAudit.vue` mounts the member Activity page in Nuxt UI without authentication.
- `lucky-audit.ts` supplies a local Activity overview and controllable draw response through `window.__audit`.
- `lucky-audit.html` opens the fixture at `/fixtures/lucky-audit.html` while the Vite development server runs.
- `interaction-feedback.ts` installs the real delegated click listener against a constructed Telegram SDK, records haptic/closing/swipe calls and disposes the listener on page exit. It stays inactive for browser administration. The lucky fixture exposes `__audit.feedback`; add `bet`, `loss` or `zh` for the actual bet flow and bilingual controls. `slow`, `empty` and `error` cover initial states; set `__audit.loadFailure=false` before retrying a read failure. `__audit.failNext` injects a result failure. One bet tap records `impact:rigid`, then one `notification:success|warning`; it must not record another result-time impact.
- `LuckyAdminAudit.vue`, `lucky-admin-audit.ts`, and `lucky-admin-audit.html` mount the browser admin activity panel with a constructed game, instant draw, raffle, catalog, and forecast data. Query parameters `slow`, `empty`, `error`, `zh` and `dark` exercise loading, failure, localization and appearance. Game/draw removal is mocked; `delete-error` preserves the target on failure, and setting `window.__audit.recover=true` permits retry. `window.__audit.deletes` records attempts without contacting a backend. The retained-game dialog uses the real shared confirmation component.

These files are for local visual audits. The production Vite build starts from
the root `index.html`, `admin.html` and `subscription.html` and excludes fixtures.

- `activity-boost-audit.ts` mounts the real activity page with controllable constructed boost, loading, error, reward and check-in states for Chrome DevTools MCP validation. Exposes `window.__boostAudit` and a locale switch for manual inspection.
- `activity-boost-audit.html` opens that fixture directly, including the zero-boost
  card and its spacing against neighboring activity sections.
- The same fixture's `operation` mode exercises simultaneous operation receipts,
  polling errors and retry controls; `reduced` constructs reduced-motion preference.

`SubscriptionAudit.vue`, `subscription-audit.ts` and `subscription-audit.html` render constructed member Home and Subscription states for Chrome DevTools MCP.
Use `mode=home&history-shares` to inspect traffic/cost aggregation, including a node at exactly 1%, on the real PAYG history chart.
The balance sheet uses constructed balance/limits and empty payment methods;
`balance-error` exercises its warning notice without a backend request.

- `IPLookupAudit.vue` mounts actual member/admin IP Lookup components inside the existing app shell.
- `ip-lookup-audit.ts` supplies constructed allowance, provider, fee, error and admin data for Chrome DevTools MCP review.
- `ip-lookup-audit.html` is the browser fixture entry point; `admin`, `zh`, `no-combo` and `cached` flags select the relevant variants. `mode=cached` displays reports automatically when an IP is entered, without a submission. `risk`, `scamalytics-risk`, `maxmind-risk`, `partial`, `all-failed`, `nulls`, `zero` and `subsource` exercise compact evidence, cached API errors, source names and coordinate visibility; `loading`, `disabled`, `error`, `balance-error` and `save-error` cover recovery states. `__ipAudit.counters()` records quote/submission counts and the saved provider/geolocation order; `__ipAudit.setCached()` changes cache availability before a new address is entered.
`ip-lookup-live-audit.ts` supplies constructed live topology, origin selection, zero/unknown metrics, traffic windows, errors, slow/cancelled responses and request counters for Chrome DevTools MCP reviews of the real IP Lookup components.
Its constructed cache reuses exact addresses only; neighboring IPv4 addresses
require independent simulated reads. Network prefixes remain routing context.
The same fixture exposes `__ipAudit.feedback`. Use `live=large` (optionally `zh`),
`live=empty`, `live=slow`, `live=error` or `live=stream-error` to inspect the graph
inside the actual scroll shell. Default wheel/touch scroll belongs to the page;
Interact enables graph gestures, and Done/Escape restores native scrolling.
Browser-emulated events establish DOM behavior and SDK call counts only; real
Telegram iOS/Android/desktop gestures and physical haptics remain device checks.

IP Lookup routing audits additionally support `live=transits`, `live=legacy`,
`live=transfer-error`, and `rpki=valid|invalid_asn|invalid_length|not_found|unavailable`.
`window.__ipAudit.seedLive` accepts normalized queued public-source snapshots for
browser inspection; reputation, billing and receipts remain constructed. The
existing empty/slow/error/multiple-origin/large modes cover the new sections too.
`PMPendingContentAudit.vue`, `pm-pending-content-audit.ts` and `pm-pending-content-audit.html` exercise the real delivery dialog with expired/invalid pending-content receipts in English and Chinese without Telegram mutations.

- `ip-lookup-progress-audit.ts` supplies progressive, interrupted and 300-node/1,500-edge/1,000-path browser fixtures. The live fixture counts simulated upstream runs and implements a 60-second reuse window with forced refresh.

`ip-lookup-rating-audit.ts` constructs all six report tiers (`rating=perfect|excellent|good|average|poor|incomplete`). `many-transfers` and `long-labels` exercise pagination and graph clipping. These are constructed browser evidence, not upstream collection or billing proof.

- `XNodeAudit.vue`, `xnode-audit.ts` and `xnode-audit.html` provide constructed X-Node workspace and migration browser states. Query flags cover empty, slow, error, source preview, unsupported methods, pending receipts, recovery and Chinese localization. They never connect to production agents.
  `legacy-source-rule` supplies an old enabled inventory-match rule to verify
  that the global firewall editor still exposes only supported detection rules.
  `not-ready` and `stale-metrics` exercise connected-but-unavailable node badges;
  the default ready fixture deliberately represents an older activity capability set.
  `final` selects all-native checkpoints; `final-error` rejects completion to
  exercise export review and retry without changing provider ownership.

- `public-subscription-audit.ts` and `public-subscription-audit.html` supply populated, loading, empty, partial and invalid/revoked-link browser cases. Metadata requests record credential omission; copy and QR remain local.

`APITokensAudit.vue`, `api-tokens-audit.ts` and `api-tokens-audit.html` provide constructed API credential creation, copy and revocation flows. Query flags: empty, slow, error, create-error, revoke-error, copy-error and zh. No real token is used.

- `DetectorRetirementAudit.vue`, `detector-retirement-audit.ts`, and `detector-retirement-audit.html` exercise the retained browser navigation and affiliate profile at desktop/mobile sizes. Query flags `empty`, `slow`, `error`, `member`, and `zh` cover empty/loading/retry/localized states without detector resources.

X-Node audit flags `source&shared` and `shared-error` exercise the localized shared SS-2022 preview warning and a refused handover that retains source ownership.

- `RetiredControlsAudit.vue` exercises real catalog-node, statistics, administrator account-action and squad-editor components through the legacy mode of `detector-retirement-audit.ts`, including constructed loading, empty and error states.

`MemberNodesAudit.vue` exercises member metrics, formats and revision-aware routing in native mode of the retirement fixture.

The X-Node fixture also supplies named service users, access edits, isolated routing and subscription rotation. Flags `services-empty`, `service-error` and `service-save-error` exercise its browser states without contacting agents.

`RoutingAudit.vue`, `routing-audit.ts` and `routing-audit.html` mount actual member
routing and browser preset forms with isolated mock APIs. Flags `admin`, `zh`,
`slow`, `empty`, `no-routes`, `no-templates`, `load-error`, `save-error`,
`preview-error`, `stale-preview`, `poll-error` and `disabled` cover UI states.
`window.__routingAudit` records writes, previews and member operation polling.
`load-timeout`, `preview-timeout` and `save-timeout` retain the real API deadline
wrapper for that call. A local fetch interceptor stalls those routing requests
until their cancellation signal fires, so even the save case cannot reach a
configured backend. This exercises timeout errors and retained drafts without
requiring manual network interception. Initial-load timeout retries use the
normal fixture data.

`stablecoin-audit.ts` and `stablecoin-audit.html` mount actual token settings, review dismissal and selected-payment credit dialogs with constructed data. Flags `credit`, `zh`, `dark`, `slow`, `empty`, `error`, `save-error` and `action-error` cover both outcomes, failed loads/saves and retry. `window.__stablecoinAudit` records requests; it never contacts a chain.

`NativeAdminAudit.vue`, `native-admin-audit.ts` and `native-admin-audit.html` mount actual profile metrics/TXB controls, machine tables/charts and browser statistics with constructed signed-request data. Flags `view=machine`, `view=statistics`, `empty`, `slow`, `error`, `save-error`, `dark` and `zh` exercise states; `window.__nativeAdminTarget` steers profile races and `window.__nativeAdminAudit` records scoped calls.

The X-Node fixture includes administrator CIDR grants; `grant-error`, `firewall-empty`, `slow` and `zh` exercise retained drafts, empty/loading states and Chinese labels.

X-Node `policy-error=XNODE_DOMAIN_SELECTOR_INVALID|XNODE_GEOSITE_REQUIRED|XNODE_GEOSITE_UNAVAILABLE|XNODE_GEOIP_INVALID|XNODE_GEOIP_UNAVAILABLE`
returns safe policy-save errors for localized diagnostics and draft retention.
The metrics view uses `slow`, `empty`, `error`, `not-ready`, `unsynchronized`,
`stale-metrics`, and `metrics-unavailable` to exercise loading, empty, failed,
offline, degraded, stale, and limited-report badges with constructed node data.
`node-file` starts certificate editing with paths and no Domain/email fields;
`no-certificates` starts with no profiles. Both combine with node load/save errors.

Public subscription `diagnostics` shows compatible formats alongside explicit
endpoint omissions and a failed Stash template; it uses synthetic credentials.

The X-Node `commands` view isolates native utilities. `command-outcome=UNDELIVERED`, `REJECTED`, `FAILED` or `OUTCOME_UNKNOWN` projects each durable outcome; `slow` shows submission and polling.

Subscription home flags `offline`, `unknown`, `metrics-error`, `slow` and `expire` exercise native activity independent of legacy statistics timestamps. The IP Lookup entrance is disabled inside this isolated fixture.

Subscription `monitor-off` hides home network charts and squad timelines while retaining subscription navigation and host copy/QR controls. Native routing and subscription metadata are mocked in this fixture to isolate monitoring checks.

Settings flags `blocked&noqueued` simulate eligibility becoming available after a background mutation; `addonlost&noqueued` simulates a charged add-on whose first reply is lost. `__controlsAudit` records eligibility reads, request keys and actual additions.

Settings `addonoverride&noqueued` rejects quotes with the localized override 409;
`addoncommitoverride&noqueued` rejects commits after a successful quote. The
constructed addition counter stays unchanged in both cases.

Routing `geosite&geosite-missing` and `geosite&geosite-unavailable` return specific
selected-template preview errors. The tree combines user/node/host/template paths,
shows administrator names and status, and opens original selectors in a modal.

X-Node `large-metrics` supplies a large CPU tree to inspect lazy mounting.
`log-error`, `log-empty`, `log-dropped`, `stale-logs` exercise selected-node log
errors/retry, empty data, exact large counters and late responses. Log updates
are synthetic and traced as logs:<node ID>; close/pause must stop polling.
`delete-error` retains endpoint rows and reports a localized action failure.

Routing flags `speedtest-start-error`, `speedtest-poll-error` and
`speedtest-uncertain` exercise same-key lost replies, status-only retry, progressive
pairs and retained partial results. The fixture never sends agent commands.

`diy-audit.ts` extends the routing fixture with DIY revision/receipt scenarios.
`RoutingAudit.vue` mounts DIY Center with `?diy`; add `populated`, `slow`,
`load-error`, `save-error`, `lost-reply` or `deployment-error`. Run the manual
`subscription-preview.go` bridge for actual Go allowlist, compatibility and
pinned-parser validation. It uses synthetic identities and no upstream traffic.

X-Node `combo-quota` mounts the real combo editor and records its capacity payload.
`migration-notice` exercises shared policy migration warnings; `firewall-member`
with `whitelist` verifies paid-unban controls stay hidden. DIY `whitelist` and
`small-quota` exercise saved lists, quota bounds and queued deployment.

DIY flags `split` and `inherited-stream-one` exercise current-reference profiles
and inherited field dependencies. The split fixture changes the download endpoint and verifies that the
mode resets from that exact pair. Command fixture writes record normal versus
explicit forced reload payloads.

`SettingsAudit.vue` and `settings-audit.ts` reuse the DIY fixture when the real
Settings entrance is opened. This checks the actual DIY route at mobile/desktop
widths without production traffic; returning restores the settings receipt loader.

`payg-audit.html`, `payg-audit.ts` and `PAYGAudit.vue` mount the actual Explore entrance and administrator squad editor with synthetic signed-transport-compatible data. Query flags select loading, empty, failure, locale, appearance and admin states; no real member data or upstream calls are used.
`validity-audit.html` mounts the real member dashboard summary and admin entitlement tables with constructed raw lifecycle states. `boundary` expires an active term after 15 seconds; `inventory`, `zh`, `light` and `error` cover filters, themes and offline loading. `payg-audit.html?cards` mounts shared squad cards; PAYG fixtures include all profile types, missing profiles and long carriers, with an internal node-name sentinel that must never appear.

PAYG fixtures record `paygAuditHaptics` through the real delegated feedback. `slow`, `empty` and `error` exercise the member traffic view without upstream traffic. Settings fixtures expose PAYG rows with `payg-empty` and `save-error` scenarios.

`pricing-audit.html`, `pricing-audit.ts` and `PricingAudit.vue` mount browser pricing policy, workspace, catalog, predictions and chart components with constructed data. Query flags select `view=policy|predictions|charts|catalog|overview|product|user|operations|exports` (default: policy), `empty`, `error`, `slow`, `save-error` and `zh`. The retired checkout fixture is removed. No production credentials, member records or external services are used.

`PricingAudit.vue`, `pricing-audit.ts` and `pricing-audit.html` mount pricing components with deterministic API fixtures. `pricing-data.ts` supplies administrator-scoped user forecasts and paginated prices, including evaluated-but-unvalidated coverage. `ChartsAudit.vue` exercises line, interval, stacked and donut composition in both appearance modes. These fixtures support Chrome DevTools inspection and never write to production.

The X-Node audit fixture accepts `cost-hourly`, `cost-unlimited`, `cost-slow`, `cost-error`, and `cost-save-error` for provider metadata states.

The X-Node fixture accepts group-deletion, group-delete-error, empty, slow and error to inspect occupied/free/unknown/service groups, stale conflicts and disabled mutation controls. stablecoin-audit accepts purged-review to inspect the localized account review reason.

`xnode-audit.html?leases` mounts the actual automatic lease composition.
`automaticLeaseFixture.ts` supplies exact large counters, IPv4/IPv6, truncated
source metadata, pagination and pending delivery. `lease-state=loading|empty|error`
and the volatile `__leaseFixture` controls exercise recovery and stale refreshes.
No lease fixture writes production data or stores source IPs in browser storage.

`xnode-audit.html?profile-metrics` mounts the real user metrics composition with
`profileMetricsFixture.ts`. `profile-state=loading|empty|error`, `profile-truncated`
and volatile `__profileFixture` controls exercise precision, historical nodes,
fresh empty IPs, unavailable/stale activity and recovery without production data.

`member-connections-audit.html` exercises the native member connections UI using
constructed owner observations and manual grants. Query switches include `zh`,
`slow`, `empty`, `error`, `stale`, `incomplete`, `disabled` and `grant-error`.
It uses the member root assets; browser administrator fixtures remain independent.

Subscription fixtures add first-sync pending state and Home PAYG usage with
`mode=home&inactive&payg`; `history-empty`, `history-error`, `history-slow`,
`history-stale` and `history-truncated` exercise retained-history states. The
constructed data includes exact byte totals beyond Number integer precision.

`browser-auth-audit.html?console` renders the actual browser console navigation with an authenticated synthetic administrator, including the absence of retired product entries.

`detector-retirement-audit.html?legacy&usage` mounts the legacy quota meter without the retired node breakdown or reset display. The existing slow, empty, error and stale switches exercise section states.

`reset-reward-audit.html` mounts the actual browser draw editor. `legacy` checks
that historical reset rewards cannot be saved, `empty` opens a new definition,
`slow`/`error` exercise forecast state, and `busy`/`save-error` exercise save
feedback. `zh` switches localization. Ordinary reward choices remain selectable.
`ResetRewardAudit.vue` owns constructed editor inputs; `reset-reward-audit.ts`
provides browser-only initialization and the forecast adapter.

`EmbyConnectionAudit.vue`, `emby-connection-audit.ts` and `emby-connection-audit.html` mount the actual browser settings panel with synthetic configuration. `slow`, `empty`, `error`, `save-error`, `conflict` and `zh` exercise localized states. `__embyConnectionAudit` records only constructed writes for manual Chrome DevTools inspection; no provider is contacted.

Stablecoin `stored` provides a masked Ethereum RPC/WS credential to exercise fresh-key validation. `secret-rejected` returns a constructed configuration refusal after local validation; drafts remain available. All values are synthetic and no chain calls run.

X-Node `commands&update-review` renders retained update uncertainty; add `not-ready` for confirmed-transaction readiness waits and `zh` for Chinese. The actual command receipt component uses constructed outcomes, with no native dispatch or production calls.

`xnode-audit.html?service-legacy` supplies an existing daily-cadence service term.
The editor has no cadence selector; `window.__xnodeAudit.writes` records successful
`namedService` request payloads plus the mock's `appliedStrategy`. New writes omit cadence and the mock defaults to
`NO_RESET`; existing writes retain the saved value. Combine with `services-empty`,
`service-error`, `service-save-error`, `slow`, and `zh` for the existing states.
Add `service-reactive` to exercise reactive API results and confirm editor copies
work without cloning a Vue proxy or mutating the loaded term.

Public subscription `malformed` returns a constructed invalid metadata shape;
the actual transport must show read recovery without exposing a usable link or
crashing the page. Ready fixtures include current first-deployment fields.

`RecoveryAudit.vue`, `recovery-audit.ts` and `recovery-audit.html` mount the actual
App session watcher, currency control and payment sheet with constructed requests.
`currency-slow` holds preference reads for explicit release, `currency-error`
refuses saves until recovery, and `empty` removes payment methods. Cancellation
receipts default to failed; `cancel-succeeded` returns success while the fetched
order deliberately stays pending. `window.__recoveryAudit` exposes only synthetic
read/write counts, deferred releases, owner replacement and payment completion.
The hosted `browser-recovery-audit.mjs` checks desktop/mobile state, recovery,
logout isolation, and the existing browser-auth fixture, retaining screenshots
and console/network diagnostics. No production payment or authentication occurs.
It verifies actual currency-menu bounds after errors and closes the explained
empty-payment state. `browser-member-recovery.mjs` adds onboarding and community
lifecycle checks to the same browser run.
