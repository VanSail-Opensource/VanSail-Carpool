# Activity admin

Lucky-draw prize drafts use memory-only client keys for stable input identity. The
persisted prize ID remains the API identity; the client key is never sent to or
stored by the backend.

- `AdminActivitySettings.vue` edits the calendar timezone, daily check-in reward range, and group-message reward configuration from Settings; its typed save API is coordinated by the parent Settings action.
- `AdminLuckyDrawEditor.vue` composes the browser side editor for paid draws and exact probability or stock totals.
- `AdminDrawPrizeRow.vue` presents prize inputs and catalog selectors through typed change events.
- `drawDraft.ts` maps saved configuration to editor state and serializes quantized rewards.
- `AdminDrawForecast.vue` reads current-catalog income, expense, and break-even estimates.

- `useActivityWorkspace.ts` loads cached games, draws, and live catalog options; owns table tabs, selection, exact-minor-unit game writes, revisioned draw edits, publish/delete actions, and resource statistics loaders. `AdminActivityPanel.vue` composes this state into tables and side editors.

Save failures are shown inside the game/draw editor while preserving its draft.
Removal dialogs retain failed targets, announce errors inside the modal, and
clear a previous target's error when a new removal opens. Removed game copy
explains that settled bets and balance history remain retained.
Chrome DevTools inspection on 2026-11-09 covered EN/Chinese at 390px/1440px,
light/dark, loading/populated/empty/error states, game/draw removal and retry,
keyboard confirmation/Escape/focus restoration, and announced modal failures.
Browser fixtures emitted no Telegram calls or backend mutations; console checks
were clean. Settled financial retention is covered by hosted backend regressions.
Extension points belong in the typed draft serializer or this feature composable;
route and bootstrap code remain composition surfaces.

Traffic reset is absent from reward selectors and current write contracts. Old
prize definitions cannot serialize as new awards. Historical result projections
retain their discriminator separately from the supported reward-write type.

`AdminActivitySettings.vue` propagates its typed boolean save result to the aggregate Settings action. `drawDraft.ts` parses configured prize ticks from decimal strings before converting to the numeric API representation, rejecting malformed precision and unsafe integer conversion. Server prize limits remain authoritative. `drawDraft.test.ts` covers fractional-cent/exponent input, exact signed ticks and unsafe renewal serialization.

Workspace reads and mutations share a request generation, preventing a delayed
cached inventory from replacing a saved game/draw. Confirmed server responses
update the local row before refresh; refresh errors identify read failure rather
than asking the operator to replay a completed write. Editor fields, dismissal
and repeat submissions lock during pending work; disposal retires completions.
`useActivityWorkspace.test.ts` covers these boundaries. `AdminDrawForecast.test.ts`
covers selected-draw isolation and explicit read retry. Hosted browser scenarios
exercise the real controls, retained errors and recovery on mobile and desktop.

Successful removals update local configuration before a refresh: games and
instant draws leave the list; raffles retain a cancelled, disabled projection.
A successful publication acknowledgement advances only a still-draft row to
publishing and never downgrades a newer state. Read failure does not expose a
repeat publication or cancellation action; Refresh reconciles retained facts.
