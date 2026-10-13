# Compensation components

- `CompensationConfigCard.vue` edits the revisioned nullable observation policy, within a flat configuration workspace that stacks on smaller browsers.
- `CompensationEventCard.vue` presents recipient-safe outage facts and operation state; long node and squad labels wrap within the card.
- `CompensationReviewModal.vue` keeps snapshots read-only while collecting minutes and reason.
- `format.ts` owns basis-point and observed-duration display conversion.
- `format.test.ts` covers exact factor formatting and duration flooring.
- `statusFilter.ts` maps the non-empty Nuxt UI selection sentinel to the API's empty status query.
- `statusFilter.test.ts` prevents empty select items from reintroducing a render failure.
- Configuration geometry, event status, and review workflow transitions are local and reduced-motion safe; the panel does not animate as a whole.

The browser panel separates configuration from its event table, including frozen
recipient counts, captured policy values, ineligibility reasons, and durable
operation states. Review keeps the existing recipient-safe snapshot semantics.
