# PAYG, DIY, reporting and DDNS delivery

Implement and audit each item in order. Browser checks are required for affected
views; automated regressions run only on hosted CI. A checkbox means implementation
and the applicable validation are complete, not merely that code was written.

## Lifecycle

- [x] 01 Reproduce and repair backup-gated maintenance; verify a successful rerun.
- [x] 02 Retire the continuity job while preserving native paid-successor access.
- [x] 03 Release expired automatic-renewal purchase restrictions atomically.
- [x] 04 Show the repeating Combo prohibition state for members and administrators.

## PAYG

- [x] 05 Add normal, hidden and unavailable administrator squad states; hidden
  removes Explore/subscription entries without cutting existing connections.
- [x] 06 Replace status words with accessible semantic animated dots.
- [x] 07 Remove the PAYG Add balance button.
- [x] 08 Place traffic left and spending right at the card bottom.
- [x] 09 Remove duplicate proxy username from the PM card.
- [x] 10 Add last-72-hour PAYG traffic and spending to PM cards without Combo gates.

## Member experience

- [x] 11 Remove the member TX Statistics shortcut.
- [x] 12 Remove Admin access from member navigation, retaining Settings access.
- [x] 13 Display subscription access without an active Combo.
- [x] 14 Make the DIY save bar nonfloating on phones.
- [x] 15 Refine swipeable DIY categories and retain drafts.
- [x] 16 Repair compatible PAYG Split XHTTP inventory through export.
- [x] 17 Branch multiple routing rules after their shared ingress node.
- [x] 18 Move route-test progress and results into an accessible modal.
- [x] 19 Reduce Activity and sign-in result popup whitespace.

## Reporting and performance

- [x] 20 Isolate bounded report queues while preserving durable traffic order.
- [x] 21 Add negotiated compact telemetry and bounded report compression.
- [x] 22 Load Explore on entry and Combo resources on mode selection.
- [x] 23 Synchronize PAYG entrance animations with fullscreen scene completion.
- [x] 24 Add bounded revision-aware caching and remove redundant reconciliation work.

## DDNS

- [x] 25 Add built-in per-family node-to-host bindings.
- [ ] 26 Add authenticated, acknowledged, replayable address-change events.
- [ ] 27 Rebuild affected routes and stage fleet-wide bypass redistribution.

## Acceptance and delivery

Both repositories must have reviewed commits on main, successful terminal hosted
checks, and verified signed amd64/arm64 agent artifacts. Record browser evidence,
maintenance receipts, protocol compatibility and performance measurements below.

## Evidence

- Item 25 passed terminal panel run `40223378321` on `6d6b998e`, including
  SQLite/PostgreSQL contracts, frontend checks and production image validation.

- Items 26-27 add negotiated authenticated DDNS events, exact replay cursors,
  atomic durable operations and staged protected-bank redistribution. Host
  addresses change only after connected capable nodes acknowledge the union;
  custom routes rebuild from saved endpoint choices through existing barriers
  without a member charge. Previous addresses remain protected through pending
  and offline routes; literal remaining references retain them afterward.
  Startup, dependency changes and reconnect recover the same desired receipts.
  Added hosted regressions for stale/duplicate/oversized events, queue rollback,
  IPv4/IPv6/dual updates, held acknowledgments, credential/ledger preservation,
  bypass replacement and offline convergence. Static review and Go vet passed;
  hosted execution and final signed release verification remain pending.

- Item 25 implements normalized same-node IPv4/IPv6 bindings, atomic node saves,
  generated API types, administrator attribution and negotiated legacy fallback.
  Chrome checked 320px/390px dark reduced motion and desktop light/dark, long host
  names, dual and single family modes, empty host inventory, invalid-source save
  gating/recovery, retained renamed bindings, provider discovery failure/retry
  and retained drafts after save errors. Built-in needs no zone request and
  remains selectable during provider failures. Keyboard Escape closes host and
  provider popups; no overflow or console warnings/errors remained. Typecheck,
  targeted ESLint, Go vet and structure/localization checks passed. Hosted
  SQLite/PostgreSQL acceptance is pending. Address redistribution follows in 26-27.

- Items 21-24 passed combined panel run `40207019355` on `44e52eb9`, including
  SQLite/PostgreSQL regressions, frontend checks and production image builds.
  This terminal result supersedes their intermediate pending notes below.

- Item 20 implementation isolates bounded report lanes in both repositories,
  retains one writer and adds hosted saturation/byte-budget/order regressions.
  Hosted panel run `40199291173` and X-Node run `40199279206` completed successfully.
  No local automated suites ran.
- Item 21 uses connection-header plus hello-capability negotiation for compact
  telemetry, default omission and fresh-snapshot normalization. Both readers cap
  expanded input before JSON parsing, and the agent uses fast report-only
  per-message deflate at 1 KiB with legacy fallback. Hosted fixtures measure
  actual framed socket bytes and enforce at least 50% reduction while retaining
  large integer accounting. No new persistent telemetry or source-IP collection
  was added. Hosted socket measurements: 146,977 legacy bytes versus 2,753
  negotiated compact/compressed bytes (98.13% reduction). Agent regression/schema
  job passed after adding its missing public reference entry. X-Node run
  `40203699740` completed successfully. Panel runs were superseded by later
  reviewed main commits; combined hosted acceptance remains pending.
- Item 22 implementation removes catalog/coupon bootstrap reads and Home's full
  catalog fetch, uses existing usage metadata and optional dashboard squad labels,
  and lazy-imports mode components. Chrome at 320px reduced motion, 390px and
  1280px verified no pre-entry Explore requests, PAYG-only entry, Combo-only
  catalog/coupon reads, slow loading, populated/empty/error states and retries.
  An isolated offline Combo chunk showed contained recovery; online Reload
  restored PAYG with no console errors. No layout overflow remained. Typecheck,
  ESLint, Go vet, structure/i18n checks and production build passed. Hosted
  acceptance remains pending. Removed a stale member-side admin statistics read.
- Item 23 pauses loaded PAYG card/summary entrances until scene completion,
  replacing the 320 ms guess. Chrome frame sampling at 390px showed card time
  remaining 0 while covered, then advancing to 220 ms at full opacity. Desktop
  slow loading retained skeletons after the cover ended; 320px reduced motion
  showed no cover/entrance and immediate full opacity. Six rapid mode switches
  settled to PAYG with no stuck gate, duplicates, overflow or console errors.
  Typecheck/lint passed; hosted regressions are pending.
- Item 24 adds a revision-keyed, deduplicated 4 MiB topology snapshot and replaces
  per-squad full group queries with one snapshot. Fresh catalog pricing/stock,
  owner eligibility, quotes and operation status are retained. The existing
  dashboard usage cache is bounded to 256 owners/8 MiB; compile scopes batch
  encrypted documents and empty inventories. SQLite/PostgreSQL metadata triggers
  and hosted mutation/cancellation/owner-isolation/budget regressions were added.
  Go vet passed. Hosted CPU/cache job `121205458565` passed: 100 synthetic catalog
  topology reads fetched nodes once and groups once; 1000 unchanged reconciliation
  calls compiled once. The 100-node/1000-user synthetic cached-sync benchmark used
  7.67 ms and 600 allocations per iteration versus 1.76 s/9.75 million allocations
  in the repeated-validation fixture; this is not a production latency claim.
  Hosted frontend job `121205458556` passed. CI found the PostgreSQL version guard
  and generated SQLite schema inventory needed advancing for migration 099;
  both are corrected. Combined hosted acceptance remains pending.

- Starting panel revision: `cdcd729f`; both worktrees were clean.
- Items 01-19 passed implementation/static/browser checks and hosted run
  [40183204793](https://github.com/VanSail-Opensource/VanSail-Carpool/actions/runs/40183204793)
  on c9d5a99c, including SQLite and PostgreSQL regressions. The user confirmed
  successful production maintenance and instructed proceeding to reporting.
  No further SSH checks or maintenance reproduction is required.
  Earlier pending notes below record intermediate states.
- Live revision `cdcd729f` failed manual maintenance operation
  `73225c11-64d9-484a-b685-c5f21247de72` after backup, with SQLSTATE 23503 on
  `xnode_routing_plans_operation_id_fkey`. Fixes `2586aa86` and `d71ce956` protect
  routing/probe receipts and native/receipt-owned members. Hosted rerun pending.
- Lifecycle changes passed Go vet, generated API contracts, Vue type checking,
  ESLint and structure/localization checks. Chrome checked the blocked Combo
  state at 390px and 1280px, 320px reduced motion, PAYG switching and Settings
  access, with no overflow or console warnings/errors. These are fixture checks;
  production maintenance and hosted regression acceptance remain open.
- Authorized reproduction `a22257e7-3971-4d35-bae4-5b9af8eb7286` failed cleanup
  after verified PostgreSQL backup `44a27654-9445-4fa6-be23-6f3a90e23940`.
- PAYG changes passed static/type/lint/structure checks. Chrome exercised 320px,
  390px and desktop cards, reduced motion, seven status reasons, hidden listing,
  loading, empty, error/retry, admin state saving and mobile light appearance.
  No unexpected console messages or layout overflow remained. Hosted acceptance
  and the successful production maintenance rerun are still pending.
- Member navigation no longer exposes TX Statistics or Admin access; Settings
  retains the administrator console entrance. Subscription entry is present on
  Home without a Combo; PAYG links and the no-host state were checked at 320px,
  390px and desktop, including error/retry and native button navigation. Chrome
  reported no warnings/errors or page overflow. Static/type/lint checks passed;
  the missing-native-identity empty response has a hosted regression.
- DIY categories retain one scrollable tab row with 44px targets, directional
  overflow cues and automatic horizontal-only reveal. Chrome checked 320px,
  390px reduced motion, long labels, keyboard Home/End/arrows, retained edits,
  loading and empty/error recovery. Mobile Save is static in document flow;
  desktop Save is sticky. No overflow or console warnings/errors remained.
- Confirmed DIY Split XHTTP uses the same PAYG owner inventory as Routing,
  including queued repair after sub-GB top-ups. Extended hosted coverage through
  preview, save and export, and reject split pairs across billed principals or
  the same published host. Go vet passed; Chrome exercised compatible selectors
  and the ineligible/empty state. The production token lacks member DIY scope,
  so live member-specific inventory was not queried.
- Routing diagrams share the ingress host, then branch by ordered rule identity.
  Chrome checked 320px reduced motion, 390px and desktop, long host labels and
  exact trace highlighting (three shared/selected edges, two other branch edges).
  Separate endpoint identities remain distinct in hosted regression coverage.
- Route-test progress/results now use the existing accessible modal. Chrome
  checked 320px reduced motion, 390px and desktop, long labels, active progress,
  final estimates, status-read failure/retry and uncertain outcomes. Closing and
  reopening retained one probe operation; Escape restores focus to View results
  even while the original Test route action is disabled. Static checks passed.
- Activity removes compounded header/toolbar margins, nested message padding and
  fixed empty-state gaps. On the 390px fixture the page shrank from 1279px to
  991px; check-in modal height fell from 356px to about 261px. Chrome checked
  320px reduced motion, English/Chinese, 390px, desktop columns, initial loading,
  empty games/draws and error recovery without overflow or console errors.
