# Member IP lookup

`IPLookupPage.vue` composes owner-scoped quote, receipt and exact-address report state. `useIPLookup.ts` fences stale responses by input and authenticated owner. Paid submission uses an expiring signed quote and an idempotency key; cached previews create no receipt.

`IPLiveDetails.vue` composes BGP, ASN/RPKI, provider scores, PTR, InternetDB and transfer history. `useLiveIPDetails.ts` accepts only a matching address and cancels obsolete streams. The UI has no inventory/probe controls, neighbor grid or subnet-cache fallback. Historical report verdicts remain readable without exposing scan procedures.

Backend contracts live in the generated IP lookup API. Shared progress contains exact-address stages only. Localization owns all labels, notices and evidence descriptions. Existing charts, forms and notices handle loading, populated, empty, error and retry states.

Network context and streaming: `IPASNDetails.vue`, `IPBGPFocus.vue`, `IPBGPTopology.vue`, `IPLiveASNSection.vue`, `IPLiveDetails.vue`, `LiveSourceNote.vue`, `bgpGraph.ts`, `bgpLayout.ts`, `useLiveIPDetails.ts`.

Feature implementation and contracts: `IPLocationMap.vue`, `IPLookupForm.vue`, `IPLookupPage.vue`, `IPLookupProgress.vue`, `IPLookupProgressTasks.vue`, `IPLookupReport.vue`, `IPRPKIStatus.vue`, `IPRefusals.vue`, `IPShodanDetails.vue`, `IPTrafficBars.vue`, `IPTransferHistory.vue`, `map.ts`, `presentation.ts`, `refusalPresentation.ts`, `reportPresentation.ts`, `useBGPGraph.ts`, `useGraphViewport.ts`, `useIPLookup.ts`, `useSharedIPProgress.ts`.

Hosted regression coverage: `bgpGraph.test.ts`, `bgpLayout.test.ts`, `presentation.test.ts`, `refusalPresentation.test.ts`, `useGraphViewport.test.ts`, `IPBGPTopology.test.ts`, `useIPLookup.test.ts`, `useLiveIPDetails.test.ts`.

`useGraphViewport.ts` leaves wheel scrolling, browser pinch zoom and touch page
scrolling native by default. Mouse drag, centered toolbar zoom and focused
keyboard controls remain available. `IPBGPTopology.vue` exposes a localized,
pressed-state interaction toggle for graph touch/pinch and wheel zoom; Done,
Escape, a new topology and disposal release pointer captures. The interaction
class changes `touch-action` before a new gesture begins, never mid-gesture.
`useBGPGraph.ts` owns fitting and viewport size; graph input has no global touch
or wheel listener and does not change Telegram's swipe policy.

Chrome DevTools inspection on 2026-11-09 covered mobile/desktop, EN/Chinese,
loading/populated/empty/provider-error states, native mouse drag, toolbar/keyboard
controls and Escape. Default wheel/touch events were not canceled; PageDown
scrolled the sole page container from the focused graph. Explicit interaction
contained graph wheel input while Ctrl-wheel retained browser zoom. Browser
emulation does not verify Telegram iOS/Android/desktop host gestures; those remain
physical-device acceptance checks. No swipe suppression was enabled speculatively.

Paid lookup attempts keep their signed quote and idempotency key when the member
edits away from an IP and returns after an ambiguous failure. Accepted operation
IDs remain associated with their exact input; late acceptance and polling errors
cannot replace a newer address. Receipt reads resume after a visibility pause and
are fenced by owner, input generation and read generation. `useIPLookup.test.ts`
covers these transitions without issuing provider traffic.
