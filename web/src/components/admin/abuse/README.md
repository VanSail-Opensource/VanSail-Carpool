# Abuse administration UI

- `AbusePolicyCard.vue` edits the revisioned global QPS and continuous-streak policy, validates operational bounds, and emits the complete policy for saving.
- `AbusePunishmentLadder.vue` presents an escalation table and independently saved side editor per detector action.
- `AbuseRulesCard.vue` manages the create/edit lifecycle for RE2 domain reasons, selects newly created rules after reload, and keeps a compact remote-ID whitelist.
- `AbuseNodesCard.vue` presents the node table, key actions, last-report timestamps, and compact QPS summary.
- `AbuseRecordsCard.vue` presents paginated detector table records with the linked Remnawave username.
- `AbusePolicyCard.test.ts` verifies localized streak loading, bounds, wheel-safe numeric input, and complete-policy submission.

The panel composes policy, escalation, rules/whitelist, nodes, and record tabs.
Mutation failures are shown inside open rule/escalation side editors while the
revisioned drafts remain available. The browser owns appearance and navigation.
