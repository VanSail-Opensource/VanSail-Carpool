# Member Knowledgebase

Nine visual tutorials are available to every onboarded member, including
accounts without a purchase. `/knowledgebase` is a thin route; Around TX links
to it on both screen sizes and new members receive a separate Home invitation.

## Entry and lifecycle

- `KnowledgePage.vue` composes cards and lazily opens one selected guide,
  retaining the initiating focus target.
- `KnowledgeCard.vue` shows a localized title, three icon points and a labelled
  play action. Hover/focus warms its player; loading/failure states are announced
  beside the action, with retry and reload controls.
- `KnowledgeHomeCard.vue` is the new-member tutorial invitation. Its brief
  decorative entrance respects reduced motion; intent warms the route/player.
- `knowledgePlayers.ts` shares pending/successful imports between intent and
  foreground loading. Educational, billing and routing players stay separate;
  failed imports are discarded and warming respects data saving and visibility.
- `useKnowledgePlayer.ts` bounds loading to 30 seconds and ignores superseded
  results or disposed pages. It never changes account state.
- `knowledgeTopics.ts` inventories cards and derives their runtime labels.
- `knowledgeLessonChapters.ts` defines 64 educational beats: subscription and
  payment have 12 each; networks, sharing, rules and why have 10 each. Each lasts
  3.2 seconds and is a single-purpose instruction with three short icon points.
- `KnowledgePlayer.vue` maps localized chapters directly onto the shared clock,
  with captions, essential detail and points inside the film. Sources are below
  the transport and open only after a user's deliberate action.
- `knowledgeSources.ts` holds reviewable primary references. Product policies
  come from the repository/owner rather than carrier marketing.

## Native interface scenes

`KnowledgeScene.vue` is a thin composition surface over `DemoStage`; it labels
all visuals as examples and passes deterministic `step`, `phase` and
`reducedMotion` props to six focused scenes:

- `KnowledgeSubscriptionScene.vue`: wallet credit, alternative billing modes,
  accessible nodes, optional routing and confirmed save, official clients,
  compatibility, private-link import/update, node selection and connection.
- `KnowledgePaymentScene.vue`: mock invoice/network/address/deadline, exchange
  deduction modes, exact decimal arithmetic, separate wallet gas, confirmation,
  credited balance and a support report that excludes secrets.
- `KnowledgeNetworkScene.vue`: optimized entry, international/broadband exits,
  an animated route, illustrative latency comparison and service checks.
- `KnowledgeSharingScene.vue`: carrier sharing versus controlled member use,
  one public/node IP, member and connection limits, service signals and support.
- `KnowledgeRulesScene.vue`: allowed app activity, private subscription, rejected
  abuse, capacity controls and reporting through a clear policy interface.
- `KnowledgeWhyScene.vue`: billing choices, project maintenance, illustrative
  monitoring/diagnosis, recovery, privacy boundaries and a support ticket.

`DemoWindow` provides shared mock application chrome; `DemoFlow` draws native
SVG connections with icon nodes. Values, counters, bars, traces, checks and
packets change from the player's clock. There are no 3D resources, WebGL/GPU
requirements, remote textures, independent timers or uncontrolled CSS animation.

## Shared player and extension contract

`components/common/DemoPlayer.vue` owns the accessible Nuxt UI modal, one
`useIllustrationPlayback` clock, transport, focus restoration and Telegram Back
ownership. Explore's `BillingDemo` and DIY's `RoutingDemo` use the same player;
the Knowledgebase opens those components directly. Together the nine tutorials
have 91 beats. Hidden documents suspend motion; pause and seek freeze/redraw all
visual state. Reduced motion shows a complete static pose with manual chapters.

To add a tutorial, register metadata, build a focused deterministic scene, add
matching English/Chinese keys, cite factual sources and extend the hosted
`knowledgebase` scenarios. Keep source files under 200 lines. Split complicated
instructions into additional concise beats; do not hide details in a disclosure.
Scenes never make requests, read live accounts, touch the clipboard or mutate
preferences. All apparent controls are illustrative and non-interactive.

Payment examples distinguish deducted withdrawal fees from separately charged
fees. With deduction, input = exact invoice amount + the provider's current fee;
with separate charging, input = invoice amount. Always verify the provider's
received amount equals the invoice. Provider withdrawal fees are not necessarily
raw chain gas, and the sample fee is not a current price quote. A self-custody
wallet must retain the chain's native gas/resources separately.

A funded traffic account needs no separate traffic-plan purchase. Subscription
setup requires importing/updating the private URL in a compatible client,
selecting an accessible node and starting its connection. IP reputation and
third-party service acceptance are never guaranteed. Example graphs do not claim
live performance or an uptime SLA. No backend contract, schema or stored account
state is changed by this module.
