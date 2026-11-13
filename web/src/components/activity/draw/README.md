# Lucky-draw presentation

The draw coordinator sends one idempotent request before displaying a prize. Presenters receive the server result and can only animate toward it; they never choose a reward.

- `selection.ts` defines the seven display styles, Random's six animated choices, bounded prize preview, exact grid/slot stopping steps, and reward celebration policy.
- `useDrawExperience.ts` owns the request-to-receipt state machine, retry, live reduced-motion path, and completion watchdog. Grid, wheel, and slot still require the center control or lever under reduced motion; their watchdog starts only after that interaction.
- `DrawExperienceModal.vue` hosts one accessible draw and receipt surface. Its Reveal result action requests the scratch presenter's animation.
- `DrawCelebration.vue` teleports several CSS firework bursts across the viewport for positive rewards. Reduced motion suppresses them.
- `presenters/README.md` maps the individual visual modes.
- `selection.test.ts` checks Random coverage without Simple, bounded previews, selected-prize insertion, exact stopping for every preview size, and celebration eligibility.
- `useDrawExperience.test.ts` checks request, retry, reduced-motion, and stalled-animation flows.
  It also verifies that a delayed result cannot restore a disposed presentation.
  Disposal clears retained draw/result state before older requests can finish.
