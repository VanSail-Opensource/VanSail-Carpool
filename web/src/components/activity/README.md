# Activity components

Dynamic game and lucky-draw collections use local Motion layout/presence
boundaries. Routine refreshes do not stagger the whole activity surface;
selection and pending feedback remain short and haptic-aware.

- `ActivityPage.vue` composes community activity states in one phone column and a balanced two-column desktop grid.
  It overrides inherited header/toolbar margins, uses compact section gaps and
  content-sized empty states. Message progress is part of its parent section,
  avoiding a second padded card. Check-in receipts omit the duplicate success
  icon and tighten body/footer spacing while retaining 44px actions.
- `DailyCheckInCard.vue`, `GroupMessageRewardPanel.vue`, `BetGamesPanel.vue`, and `LuckyDrawPanel.vue` implement member activities with selection, confirmation, and authoritative outcome haptics; `GroupMessageRewardPanel.vue` presents message count, reward amount, progress, and claimed/available state in one card.
- Member lucky-draw listing and playback remain instant-only; `ActivityResultContent.vue` presents resolved random reward content returned by the server.
- `ActivityResultDialog.vue` hosts check-in and bet receipts; `ActivityResultContent.vue` renders shared authoritative reward and balance details, including the selected lucky-draw prize.
- `draw/README.md` specifies the seven result-only presentation modes, focused draw modal, local artwork, and retry/reduced-motion behavior.
- `BetSuccessFireworks.vue` renders a bounded, non-interactive success burst and honors reduced motion.
- `feedback.ts` centralizes result classification and Telegram notification mapping.
- `ActivityResultDialog.test.ts`, `BetSuccessFireworks.test.ts`, and `feedback.test.ts` cover result feedback boundaries and daily check-in reward rendering.
- `gameIcons.ts` maps server-owned icon keys to external Iconify names.
- `ActivityResultDialog.vue`, `DailyCheckInCard.vue`, `GroupMessageRewardPanel.vue`, and `BetSuccessFireworks.vue` own completion-only Motion feedback; particle trajectories remain CSS and reduced motion uses a single success marker.

- `GroupBoostPanel.vue` presents active personal count and multiplier, or replaces the check-in and group reward blocks with localized boost guidance and a live refresh. Props own verified state; its refresh event is handled by `ActivityPage.vue` through `useActivity`.
- `GroupBoostPanel.test.ts` covers required, unavailable, boosted and refreshing states plus link and refresh actions.
