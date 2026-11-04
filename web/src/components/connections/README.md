# Member connections

`ConnectionsPage.vue` composes `useMemberConnections`, `ConnectionLeaseForm` and
`ConnectionObservationList`. State and API work belong to the composable; children
receive typed props and emit grant requests. The paginated GET returns native
owner observations across combo/PAYG principals, deduplicated by canonical IP.
No node identity is rendered. Search fences old replies and preserves pager data.

POST `/api/v1/subscription/connections/leases` accepts only `{ ip }`. Eligibility,
public-address validation, capacity and duplicate expiry are enforced by X-Node.
Leases and owner associations are volatile. The original lease start survives
renewals; owner-specific last-observed stays empty until actual telemetry.
Unavailable, stale, incomplete, empty, loading and error states remain explicit.

Existing Nuxt UI forms and pagination provide keyboard/touch interaction. The
member route remains separate from the browser admin root. Browser fixtures use
`fixtures/member-connections-audit.html`; admin scans and unblock dialogs remain
under administrator modules. Hosted regressions own backend behavior checks.

`ConnectionLeaseForm.vue` accepts a canonical IP. `ConnectionObservationList.vue` displays observations and lease times. `useMemberConnections.ts` owns private paging, cancellation and grant refresh.
