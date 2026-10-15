# X-Node administration

`XNodeWorkspace.vue` composes the independent browser console's nodes, global
policy, configurations/snippets, access groups/endpoints and live metrics.
`useXNodeWorkspace.ts` owns parallel initial reads, saves and visible-page metric
polling. `XNodeNodes.vue` edits one assigned profile and node utilities and exposes
write-only enrollment tokens. `XNodeOperationalSettings.vue` composes DNS/firewall
with `XNodeDDNSSettings.vue` and `XNodeCertificateSettings.vue`.

`XNodeGlobalSettings.vue` edits fleet policy and shared DNS providers.
`XNodeConfigurations.vue` handles one base per node, ordered shared snippets,
local JSON feedback and debounced pinned-core validation.
`XNodeJsonEditor.vue` is the shared native textarea wrapper for JSON inputs.
`XNodeAccessGroups.vue` manages node/inbound access and subscription endpoints.
`XNodeMetrics.vue` renders fleet and detailed collector snapshots.

Typed clients are in `api/xnode.ts`; all copy comes from `xnode.json` locales.
Configuration and enrollment reads bypass response persistence. No component
initializes Telegram SDK controls. Metric rates convert bytes/second to decimal
Mbps; backend byte accounting retains exact integers.

- `XNodeMigration.vue` provides the source preview/import and per-node handover/rollback workflow.
- `useXNodeMigration.ts` loads migration state, protects stale requests and tracks durable operation receipts.
- `XNodeMigrationFinalization.vue` collects the final source backup reference and exposes staged export and verified bridge completion through operation receipts.
