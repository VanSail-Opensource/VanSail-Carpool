# Member IP lookup

`IPLookupPage.vue` composes owner-scoped quote, receipt and exact-address report state. `useIPLookup.ts` fences stale responses by input and authenticated owner. Paid submission uses an expiring signed quote and an idempotency key; cached previews create no receipt.

`IPLiveDetails.vue` composes BGP, ASN/RPKI, provider scores, PTR, InternetDB and transfer history. `useLiveIPDetails.ts` accepts only a matching address and cancels obsolete streams. The UI has no inventory/probe controls, neighbor grid or subnet-cache fallback. Historical report verdicts remain readable without exposing scan procedures.

Backend contracts live in the generated IP lookup API. Shared progress contains exact-address stages only. Localization owns all labels, notices and evidence descriptions. Existing charts, forms and notices handle loading, populated, empty, error and retry states.

Network context and streaming: `IPASNDetails.vue`, `IPBGPFocus.vue`, `IPBGPTopology.vue`, `IPLiveASNSection.vue`, `IPLiveDetails.vue`, `LiveSourceNote.vue`, `bgpGraph.ts`, `bgpLayout.ts`, `useLiveIPDetails.ts`.

Feature implementation and contracts: `IPLocationMap.vue`, `IPLookupForm.vue`, `IPLookupPage.vue`, `IPLookupProgress.vue`, `IPLookupProgressTasks.vue`, `IPLookupReport.vue`, `IPRPKIStatus.vue`, `IPRefusals.vue`, `IPShodanDetails.vue`, `IPTrafficBars.vue`, `IPTransferHistory.vue`, `map.ts`, `presentation.ts`, `refusalPresentation.ts`, `reportPresentation.ts`, `useBGPGraph.ts`, `useGraphViewport.ts`, `useIPLookup.ts`, `useSharedIPProgress.ts`.

Hosted regression coverage: `bgpGraph.test.ts`, `bgpLayout.test.ts`, `presentation.test.ts`, `refusalPresentation.test.ts`, `useGraphViewport.test.ts`, `useIPLookup.test.ts`, `useLiveIPDetails.test.ts`.
