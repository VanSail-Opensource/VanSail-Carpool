# Community components

The member page presents two responsive destination cards, a compact membership
summary, and a single refresh action. Group conversations and official updates
have distinct icons and short topic cues. Cards share desktop width and stack on
mobile. The footer explains the Telegram handoff and offers Explore to members
without join eligibility.

- `CommunityPage.vue` composes canonical loading, cached display, recoverable errors,
  the destination cards and per-space invite actions. It has an explicit empty/error
  state and announces initial loading; refresh remains reachable in every state.
- `CommunityMembershipRows.vue` renders the two cards and emits only the selected
  eligible join intent. Confirmed membership takes precedence over lapsed access.
  Initial entrances honor reduced motion; there are no persistent decorative loops.
- `CommunityPage.test.ts` covers loading, error fallback, and canonical row projection.
- `CommunityMembershipRows.test.ts` covers Joined precedence and per-space join intent.

`useCommunityMembership` owns Telegram activation refreshes and API calls. Session
snapshots render for up to 60 seconds, but join controls remain disabled until a
fresh membership response succeeds. Errors preserve useful display data while
preventing stale eligibility from opening an invite. The existing backend remains
authoritative for access and invite creation. All text comes from the paired locales.

AppShell owns navigation clearance and the single main landmark.
