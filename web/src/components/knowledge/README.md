# Member Knowledgebase

Nine short visual lessons are available to every onboarded member, including
accounts without a purchase. `/knowledgebase` is a thin route; Around TX links
to it on both screen sizes and new members receive a separate Home invitation.

## Boundaries

- `KnowledgePage.vue` composes cards and lazily opens the selected guide. Only
  one modal mounts at a time, retaining the initiating focus target.
- `KnowledgeCard.vue` displays a localized title, three icon points and a
  labelled play action. Hover/focus requests preloading of that player family;
  foreground loading and failure states are announced beside the action, with
  retry and reload controls. It has no account or playback side effects.
- `KnowledgeHomeCard.vue` is the new-member invitation. Its brief decorative
  entrance stops and respects reduced motion. CTA hover/focus warms the route
  and the first educational player without opening a modal.
- `knowledgePlayers.ts` shares pending/successful imports between intent and
  foreground loading. Educational, billing and routing players stay separate;
  failed imports are discarded and optional warming respects data saving,
  slow connections, and document visibility.
- `useKnowledgePlayer.ts` opens only after loading succeeds, bounds the wait
  to 30 seconds, and ignores results after a newer selection or page disposal.
  Its loading/error state is local and never changes account data.
- `knowledgeTopics.ts` inventories cards and derives their runtime labels.
- `knowledgeLessonChapters.ts` inventories localized chapter keys, icons,
  three point icons and semantic scene stages. Existing lessons have eight
  2.8-second beats; the first subscription tutorial has six 3.2-second steps.
- `KnowledgePlayer.vue` supplies the shared player with visible captions,
  points and explanations. Paired beats continue one 3D scene without resetting
  its clock. Sources are directly available below the transport.
- `KnowledgeScene.vue` composes the shared Three.js renderer, localized scene
  labels, exact illustrative payment arithmetic and simulated pointer actions.
- `knowledgeSceneLabels.ts` maps the visual stage to its localized role.
- `KnowledgeSubscriptionAction.vue` demonstrates balance, alternative billing
  modes, optional routing, official clients, subscription import/refresh and
  connection. Its controls are illustrations and never touch the clipboard.
- `knowledgeSources.ts` holds static primary-source references. Provider
  policies come from the product owner and repository, not carrier marketing.
  External references open only through a user's deliberate action.

## Shared animation library

`components/common/DemoPlayer.vue` owns the accessible Nuxt UI modal, one
`useIllustrationPlayback` clock, transport, focus restoration and Telegram Back
ownership. Explore's `BillingDemo` and DIY's `RoutingDemo` reuse that same
player. Their existing billing models remain unchanged; the Knowledgebase
opens those components directly rather than copying their explanations.

Scenes present `{ step, phase, reducedMotion, description }` without account effects.
Use `DemoStage` for the labelled canvas, `DemoThreeScene` for procedural 3D objects
and clock-driven packets, `DemoPointer` for desktop clicks/mobile taps,
and `illustrationTimeline` for interpolation. No independent timers, CSS loops,
API calls, live account reads, tracking, or writes belong in scenes. Hidden
documents suspend the clock. Pause/seek stop all clock-driven motion. Reduced
motion shows a complete static chapter with manual navigation available.

To add a lesson, register its metadata, build a focused scene, add matching
English/Chinese card/caption/description keys, cite factual sources, and extend
the hosted `knowledgebase` browser scenarios. Keep scenes under 200 lines where
practical. Every chapter keeps its three short icon points and concise detail
visible in the film; do not restore an explanation disclosure. Keep all material
facts when shortening copy. Avoid guarantees about IP acceptance, current gas prices, unsupported
service behavior, logging policies, or unseen deployment settings.

No backend endpoints, contracts, database schema or persisted preferences are
introduced. A funded traffic account does not need a separate traffic-plan
purchase. Importing a subscription requires saving/updating it in a supported
Mihomo/Xray client, selecting an accessible node and starting its connection;
node selection alone does not route device traffic.

The route uses the existing idle-preload path. Player chunks load
on relevant hover/focus or an explicit play action, not on every route preload.
