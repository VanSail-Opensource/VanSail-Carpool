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
Extension points belong in the typed draft serializer or this feature composable;
route and bootstrap code remain composition surfaces.
