# Member DIY Center

This module composes client-only subscription tuning and existing managed
routing under one page Save. Drafts stay in memory; failed validation, requests
and revision conflicts retain both sections. Reset restores subscription
inheritance without touching routing. The backend owns the allowlist,
compatibility and atomic encrypted persistence; generated clients type all data.

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

`DIYWhitelist.vue` edits exact IPs in the shared draft and displays the active
combo's capacity and deployment phase. Empty enabled lists explicitly deny all
connections. The single Save persists desired policy; node acknowledgment controls
activation status. Browser admin combo capacity uses the existing catalog editor.

`DIYTuningPanel.vue` renders one tab's active catalog groups. `diyApplicability.ts`
uses the server's shared dependency rules for visibility and Save validation;
hidden values stay in the draft but leave the submitted overrides. The Nuxt tabs
retain mounted routing and input state. `DIYSplitProfiles.vue` edits only host
references and a split-compatible mode from the server catalog. Pair-specific
download modes are authoritative; the legacy per-upload union is only a fallback
for rolling compatibility.

Quotes beside Save use the existing IP-check allowance, then the fresh-check TXB
price. `useDIY.ts` retains the exact request, quote and key after uncertain replies;
a retry reads the committed receipt before validating current access or prices.
A changed quote requires another Save with the updated displayed price. No extra
confirmation is introduced. Backend transactions own fees and revision fencing.

`diyRange.ts` parses exact values or ascending hyphen ranges into the existing min/max wire object. `DIYRangeField.vue` retains incomplete text in the shared draft, shows examples when empty, and blocks invalid Save/automatic preview without discarding input.

The five tabs are Connection, XHTTP, Routing, Whitelist and Compatibility. Applicable fields and compatibility rows stay expanded without subgroup headings. One Reset and Save bar owns submission. Native tab scrolling hides its scrollbar while retaining keyboard/swipe access; mounted panels retain drafts.

Mini App tabs and selection controls use selection haptics. Buttons use existing intent feedback; Save reports success/error through the existing Telegram notification helper. Scoped CSS clarifies presses and panel/status changes only when reduced motion is not requested. Browser admin roots do not import this page or initialize these controls.
