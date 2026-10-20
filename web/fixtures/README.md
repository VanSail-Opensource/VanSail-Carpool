# Constructed browser fixtures

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
`window.__browserAuthAudit` records mock issuance, exchange, and clipboard actions.

IP Lookup's `subnet-probes` query flag adds per-address TCP/Reality/Cloudflare evidence with yellow, red, gray and legacy-compatible rows. Combine it with `checking` for live subnet progress, `queued` for shared waiting, or `enhanced=categories` for a refused primary IP with visible subnet probes. These fixtures exercise the real Vue components through Chrome DevTools MCP without production requests.

`PMAdminAudit.vue`, `pm-admin-audit.ts` and `pm-admin-audit.html` mount real
administration rows and dialogs with isolated reference-only operations.
`empty`, `error`, `slow`, `reviewempty`, `reviewerror`, `reviewslow`, `pending`,
`moderationerror`, `pollerror` and `reduced` exercise browser states without
sending Telegram messages. `__pmAudit` and `__pmConversations` expose results.

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

- `LuckyAudit.vue` mounts the member Activity page in Nuxt UI without authentication.
- `lucky-audit.ts` supplies a local Activity overview and controllable draw response through `window.__audit`.
- `lucky-audit.html` opens the fixture at `/fixtures/lucky-audit.html` while the Vite development server runs.
- `LuckyAdminAudit.vue`, `lucky-admin-audit.ts`, and `lucky-admin-audit.html` mount the admin activity panel with constructed instant, raffle, catalog, and forecast data. Query parameters `slow`, `empty`, and `error` exercise loading and failure views.

These files are for local visual audits. The production Vite build starts from the root `index.html` and does not include this page.

- `activity-boost-audit.ts` mounts the real activity page with controllable constructed boost, loading, error, reward and check-in states for Chrome DevTools MCP validation. Exposes `window.__boostAudit` and a locale switch for manual inspection.
- `activity-boost-audit.html` opens that fixture directly, including the zero-boost
  card and its spacing against neighboring activity sections.
- The same fixture's `operation` mode exercises simultaneous operation receipts,
  polling errors and retry controls; `reduced` constructs reduced-motion preference.

`SubscriptionAudit.vue`, `subscription-audit.ts` and `subscription-audit.html` render real Home, Subscription & uptime, and connectivity-admin components using constructed owner-specific data. Query flags `mode=home|admin`, `slow`, `empty`, `inactive`, `unknown`, `error` and `noreset` cover loading, gaps, absent ownership, errors and timestamp absence. The audit has no production API traffic; `__subscriptionAudit` exposes read/revoke/save counts and `__subscriptionAuditLocale` switches languages for Chrome DevTools MCP checks.

- `IPLookupAudit.vue` mounts actual member/admin IP Lookup components inside the existing app shell.
- `ip-lookup-audit.ts` supplies constructed allowance, provider, fee, error and admin data for Chrome DevTools MCP review.
- `ip-lookup-audit.html` is the browser fixture entry point; `admin`, `zh`, `no-combo` and `cached` flags select the relevant variants. `mode=cached|subnet` displays reports automatically when an IP is entered, without a submission. `risk`, `scamalytics-risk`, `maxmind-risk`, `partial`, `all-failed`, `nulls`, `zero` and `subsource` exercise compact evidence, cached API errors, source names and coordinate visibility; `loading`, `disabled`, `error`, `balance-error` and `save-error` cover recovery states. `__ipAudit.counters()` records quote/submission counts and the saved provider/geolocation order; `__ipAudit.setCached()` changes cache availability before a new address is entered.
`ip-lookup-live-audit.ts` supplies constructed live topology, origin selection, zero/unknown metrics, traffic windows, errors, slow/cancelled responses and request counters for Chrome DevTools MCP reviews of the real IP Lookup components.

IP Lookup routing audits additionally support `live=transits`, `live=legacy`,
`live=transfer-error`, and `rpki=valid|invalid_asn|invalid_length|not_found|unavailable`.
`window.__ipAudit.seedLive` accepts normalized queued public-source snapshots for
browser inspection; reputation, billing and receipts remain constructed. The
existing empty/slow/error/multiple-origin/large modes cover the new sections too.
`PMPendingContentAudit.vue`, `pm-pending-content-audit.ts` and `pm-pending-content-audit.html` exercise the real delivery dialog with expired/invalid pending-content receipts in English and Chinese without Telegram mutations.

- `ip-lookup-progress-audit.ts` supplies progressive, interrupted and 300-node/1,500-edge/1,000-path browser fixtures. The live fixture counts simulated upstream runs and implements a 60-second reuse window with forced refresh.

`ip-lookup-enhanced-audit.ts` provides constructed named-source, subnet, full-probe progress and return-to-IP states. Use `enhanced=clean|risk|partial|skipped` and `checking` or `progress-error`; `__ipEnhancedAudit.start()` / `.finish()` control shared work without charges.

`ip-lookup-rating-audit.ts` constructs all six report tiers (`rating=perfect|excellent|good|average|poor|incomplete`). The enhanced fixture supports `source-warming`, `source-partial`, `source-error` and `source-status-error` for shared admin polling/refresh; `many-transfers` and `long-labels` exercise pagination and graph clipping. These are constructed browser evidence, not upstream collection or billing proof.

- `XNodeAudit.vue`, `xnode-audit.ts` and `xnode-audit.html` provide constructed X-Node workspace and migration browser states. Query flags cover empty, slow, error, source preview, unsupported methods, pending receipts, recovery and Chinese localization. They never connect to production agents.
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

`stablecoin-audit.ts` and `stablecoin-audit.html` mount actual token settings, review dismissal and selected-payment credit dialogs with constructed data. Flags `credit`, `zh`, `dark`, `slow`, `empty`, `error`, `save-error` and `action-error` cover both outcomes, failed loads/saves and retry. `window.__stablecoinAudit` records requests; it never contacts a chain.

`NativeAdminAudit.vue`, `native-admin-audit.ts` and `native-admin-audit.html` mount actual profile metrics/scans/TXB controls, machine tables/charts and browser statistics with constructed signed-request data. Flags `view=machine`, `view=statistics`, `empty`, `slow`, `error`, `save-error`, `dark` and `zh` exercise states; `window.__nativeAdminTarget` steers profile races and `window.__nativeAdminAudit` records scoped calls.

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

Native admin `view=scan` isolates connection scanning; `scan-failed` projects a durable safe error code, while `error` rejects the request and `empty` returns a successful empty scan.

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
Native admin `view=connections` displays only nodes with observed active user IPs.
