# Member Knowledgebase

Eight short visual lessons are available to every onboarded member, including
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
- `knowledgeTopics.ts` inventories cards, chapter IDs, icons and durations.
- `KnowledgePlayer.vue` maps five educational scenes to the common player,
  with short captions, screen-reader descriptions and optional explanations.
- `KnowledgeNetworkScene.vue` illustrates China entry, international transit,
  local ISP exits and why performance and IP reputation are distinct.
- `KnowledgePaymentScene.vue` separates exact invoice receipt from network
  gas and exchange withdrawal deductions; it never constructs a real payment.
- `KnowledgeSharingScene.vue` distinguishes widespread CGNAT from a node's
  dedicated public IP, bounded member sharing and service-side acceptance.
- `KnowledgeRulesScene.vue` passes normal-use icons through a policy gate and
  stops scraping, account factories, resharing, torrents, spam and attacks.
- `KnowledgeWhyScene.vue` illustrates payment choice, maintained source code,
  service quality, privacy and direct bot support.
- `knowledgeSources.ts` holds static primary-source references. Provider
  policies come from the product owner and repository, not carrier marketing.
  External references open only through a user's deliberate action.

## Shared animation library

`components/common/DemoPlayer.vue` owns the accessible Nuxt UI modal, one
`useIllustrationPlayback` clock, transport, focus restoration and Telegram Back
ownership. Explore's `BillingDemo` and DIY's `RoutingDemo` reuse that same
player. Their existing billing models remain unchanged; the Knowledgebase
opens those components directly rather than copying their explanations.

Scenes present `{ step, phase, reducedMotion, description }` without effects.
Use `DemoStage` for the labelled canvas, `DemoNode` for positioned icon tiles,
`DemoFlow` for clock-driven packets, `DemoPointer` for desktop clicks/mobile taps,
and `illustrationTimeline` for interpolation. No independent timers, CSS loops,
API calls, live account reads, tracking, or writes belong in scenes. Hidden
documents suspend the clock. Pause/seek stop all clock-driven motion. Reduced
motion shows a complete static chapter with manual navigation available.

To add a lesson, register its metadata, build a focused scene, add matching
English/Chinese card/caption/description keys, cite factual sources, and extend
the hosted `knowledgebase` browser scenarios. Keep scenes under 200 lines where
practical. Explanations and sources stay in a disclosure so the film remains
icon-led. Avoid guarantees about IP acceptance, current gas prices, unsupported
service behavior, logging policies, or unseen deployment settings.

No backend endpoints, contracts, database schema or persisted preferences are
introduced. The route uses the existing idle-preload path. Player chunks load
on relevant hover/focus or an explicit play action, not on every route preload.
