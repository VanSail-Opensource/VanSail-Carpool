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
- [ ] 21 Add negotiated compact telemetry and bounded report compression.
- [ ] 22 Load Explore on entry and Combo resources on mode selection.
- [ ] 23 Synchronize PAYG entrance animations with fullscreen scene completion.
- [ ] 24 Add bounded revision-aware caching and remove redundant reconciliation work.

## DDNS

- [ ] 25 Add built-in per-family node-to-host bindings.
- [ ] 26 Add authenticated, acknowledged, replayable address-change events.
- [ ] 27 Rebuild affected routes and stage fleet-wide bypass redistribution.

## Acceptance and delivery

Both repositories must have reviewed commits on main, successful terminal hosted
checks, and verified signed amd64/arm64 agent artifacts. Record browser evidence,
maintenance receipts, protocol compatibility and performance measurements below.

## Evidence

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
  was added. Static checks passed; hosted measurements remain pending.

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
