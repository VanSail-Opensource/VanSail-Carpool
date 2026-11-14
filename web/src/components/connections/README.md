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

`ConnectionSummary.vue` derives the returned address count, configured lease
duration and snapshot time from the current page. Filtered counts are labelled
as matches; neither retained observations nor grants imply a currently open
connection. `ConnectionsPage.vue` separates the searchable address inventory
from the grant form: mobile places the form before the list, while wide screens
keep it alongside the paginated results. Refresh retains visible data and shows
progress; initial failures retain an explicit retry action. Empty, stale and
incomplete results remain distinct. `ConnectionObservationList.vue` renders
selectable IPv4/IPv6 addresses with labelled observation and lease timestamps.
