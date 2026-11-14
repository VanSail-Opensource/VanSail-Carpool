# Community components

The page presents only the membership destinations and refresh control, without
the former readiness summary or quick-start guide. Destinations share available
desktop width and stack on mobile.

- `CommunityPage.vue` orchestrates canonical membership loading, activation refreshes, localized feedback, and per-space invite actions.
- `CommunityMembershipRows.vue` is the presentational two-row group and channel access surface; confirmed membership takes precedence over eligibility status.
- `CommunityPage.test.ts` covers loading, error fallback, and canonical row projection; `CommunityMembershipRows.test.ts` covers Joined precedence and per-space join intent.
- `CommunityMembershipRows.vue` animates only membership state and layout changes, leaving the page composition static.

The page reserves AppShell navigation clearance and leaves the single main landmark to the shell.
