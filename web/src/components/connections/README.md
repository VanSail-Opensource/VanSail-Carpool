# Connection components

`FirewallExemption.vue` purchases a fleet-wide exact-public-IP exemption through
`api/firewall.ts`. It shows the quoted TXB price and duration, preserves retry
idempotency, and reports durable queued delivery separately from actual host state.
This is independent of existing per-user session IP unblock controls.

- `ConnectionsPage.vue` composes scans, focused dialogs, and cross-refresh after terminal mutations; active blocks remain hidden until the connection scan completes.
- `ConnectionScanStatus.vue` presents the stable starting and polling surface with localized progress, a reduced-motion-safe radar, and no scan mutations.
- `ConnectionNodeList.vue` presents provider nodes, recent IP addresses, and one explicit block command per signed handle.
- `ConnectionBlockList.vue` presents loading, error, empty, active, expiry, and unblock states with precise retry and open feedback.
- `ConnectionBlockDialog.vue` owns block confirmation plus shared-IP and three-day disclosures; `ConnectionUnblockDialog.vue` owns early-removal confirmation. Close is soft, status checks are light, unblock confirmation is rigid, and destructive block confirmation remains heavy.
- `ConnectionBlockDialog.test.ts` and `ConnectionUnblockDialog.test.ts` cover native Back ownership, warnings, expiry, and confirmation.
- `types.ts` defines the UI-only selected connection target passed from the list to the confirmation surface.
- `ConnectionScanStatus.vue`, `ConnectionNodeList.vue`, `ConnectionBlockList.vue`, `ConnectionBlockDialog.vue`, and `ConnectionUnblockDialog.vue` own reduced-motion-safe scan, list, and workflow transitions. The continuous radar stays CSS-owned.

The member node list displays only scan nodes with at least one observed active
user IP. Unavailable or empty nodes stay in scan progress metadata but never
produce empty connection cards. A scan with no observed IPs uses the existing
empty state. Native-admin browser fixture `view=connections` includes mixed
active/empty/unavailable nodes; `empty`, `slow`, and `error` cover other states.
