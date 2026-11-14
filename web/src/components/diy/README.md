# Member DIY Center

Administrator endpoint forms reuse the typed `DIYFieldControl.vue` widgets with
an explicit administrator label and optional inherit/reset action. Selection
feedback passes through the existing experience gate, so browser use does not
invoke Telegram. `DIYFieldControl.test.ts` covers that separation and pinned
fixed-off controls. `diyStructuredValidation.ts` exports structural `clientHeaders`
validation while retaining the stricter member protected-header policy in
`diyHeaders`; this does not expand the member catalog or accepted member writes.
Fields without predefined string values use the documented Nuxt SelectMenu
custom-item UI for typed arrays such as ALPN. Curated member enums stay restricted.

`useDIY.ts` refreshes every 1.5 seconds while `inventoryPending` reports queued
PAYG provisioning, preserving option, split and routing drafts. Polling pauses
for submission and is disposed with the view. `useDIY.test.ts` covers completion
and teardown. `DIYCenter.vue` keeps the loading indicator visible until that
inventory is ready. Positive sub-GB balances can expose hosts in both selectors;
backend queue/account ownership and protocol compatibility remain authoritative.

This module composes client-only subscription tuning and existing managed
routing under one page Save. Drafts stay in memory; failed validation, requests
and revision conflicts retain both sections. Reset restores subscription
inheritance without touching routing. The backend owns the allowlist,
compatibility and atomic encrypted persistence; generated clients type all data.

Categories use the actual Nuxt UI `UTabs` component with icon-bearing pill
triggers in one scrollable row. `useScrollableTabs.ts`
keeps the selected tab visible using horizontal-only scrolling, exposes overflow
cues, observes label/viewport changes and disposes its listeners with the view.
Hidden category panels stay mounted, preserving drafts. The Save bar follows
document flow below 900px and is sticky only on desktop; reduced motion disables
smooth category scrolling and panel animations.

The page uses member theme assets and no administrator resources. Client
refresh, not route deployment, activates connection tuning. Routing remains on
the existing durable fleet deployment queue and keeps its own pending status.

`DIYCenter.vue` composes the two sections and one Save. `useDIY.ts` owns drafts,
both revision fences, idempotent retry keys and durable deployment receipts.
`useDIYPreview.ts` debounces cancellable server validation with stale-response
fencing. `DIYSettingsGroup.vue` groups server-described typed fields;
`DIYFieldControl.vue` chooses native/Nuxt controls, with `DIYRangeField.vue`,
`DIYHeadersField.vue` for structured values.
`diyValidation.ts` and `diyStructuredValidation.ts` provide frontend Zod checks;
the backend repeats validation and checks generated Xray with the pinned parser.
`DIYCompatibility.vue` and `TuningCompatibilityList.vue` display shared server
applicability, host counts and documented client/platform restrictions.

`useDIY.test.ts` covers both draft sections through validation, same-key lost replies, revision conflicts and inheritance reset in hosted CI.



`DIYTuningPanel.vue` renders one tab's active catalog groups. `diyApplicability.ts`
uses the server's shared dependency rules for visibility and Save validation;
hidden values stay in the draft but leave the submitted overrides. The Nuxt tabs
retain mounted routing and input state. `DIYSplitProfiles.vue` edits only host
references and a split-compatible mode from the server catalog. Pair-specific
download modes are authoritative; the legacy per-upload union is only a fallback
for rolling compatibility.

DIY saves are free and do not consume an IP Lookup allowance. `useDIY.ts` retains
the exact request and key after uncertain replies; a retry reads the committed
receipt before validating current access. Backend transactions own revision fencing.

`diyRange.ts` parses exact values or ascending hyphen ranges into the existing min/max wire object. `DIYRangeField.vue` retains incomplete text in the shared draft, shows examples when empty, and blocks invalid Save/automatic preview without discarding input.

The four tabs are Connection, XHTTP, Routing and Compatibility. Applicable fields
stay expanded without subgroup headings. Compact field labels pair a group icon
with inline units. Compatibility rows show an option icon and status; accessible
popovers retain host counts and detailed restrictions without repeating prose in
every row. One Reset and Save bar owns submission. Native tab scrolling retains
keyboard/swipe access and overflow cues; mounted panels retain drafts.

The page grid and tab root constrain intrinsic width so long labels scroll
inside navigation instead of widening the phone viewport. The Save bar uses
equal-width touch actions on phones and inherits the shell's mobile
navigation clearance; desktop keeps one row. Routing inherits the page surface
without an extra padded card. Split pairs stack both hosts above mode/delete
on phones, then use bounded columns on desktop. Compatibility names wrap beside
their status without squeezing either into a narrow column.

Mini App tabs and selection controls use selection haptics. Buttons use existing intent feedback; Save reports success/error through the existing Telegram notification helper. Scoped CSS clarifies presses and panel/status changes only when reduced motion is not requested. Browser admin roots do not import this page or initialize these controls.

`RoutingDemoButton.vue` lazily loads `RoutingDemo.vue` on demand.
`RoutingDemoScene.vue` keeps a simulated editor and route graph on screen while
the pointer chooses entry/template/exit fields, validation checks complete,
Save enters a queue, and the graph becomes active only after acknowledgement.
The final chapter animates both matching and default paths without replacing the
scene. The real scenario tabs let viewers inspect either branch and pause the
film. These chapters and selectors are local presentation state; the demo does
not call APIs or modify the draft. Shared `useIllustrationPlayback.ts` handles a
continuous seekable clock, exact pause/resume, replay, manual chapter snapshots,
hidden-page suspension and reduced motion. `DemoTransport` and `DemoPointer`
share playback and cursor/touch behavior with Explore. Full explanations remain
available to assistive technology while the visible scene favors icons.

Routing no longer repeats the select-node instructions above the graph or the
save/trace prerequisites beneath speed testing. Existing accessible graph naming,
control labels, prerequisite gating and the metered-test notice remain intact.

`useDIY.ts` can restore a typed, account-scoped in-memory rendering snapshot up to
60 seconds old before its live read. It merges untouched sections from the first
fresh response while preserving edits made during that read. Save and automatic
previews require live confirmation; failed reads keep the draft visible but do
not authorize a save or start inventory polling from cached state. The routing
editor accepts `previewPaused` from this freshness boundary. Hosted
`useDIYCache.test.ts` covers draft preservation, fresh replacement and failed-read
blocking. No settings, credentials or previews are persisted in browser storage.
