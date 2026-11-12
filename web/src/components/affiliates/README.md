# Affiliate Centre components

- `AffiliateCentrePage.vue` coordinates the member route and Telegram BackButton.
- `AffiliateSummary.vue` presents the server-built link and commission metrics with copy-intent feedback.
- `AffiliateTierProgress.vue` renders current and next tier progress accessibly.
- `AffiliateReferralList.vue` renders Telegram first/last names and one status per fixed five-row server page, emitting semantic selection feedback only when the server page changes.
- `affiliates-01.css` keeps phone layouts single-column below 480px and preserves the shared bottom-navigation clearance.
- `AffiliateSummary.test.ts` covers link copying and discovery-disabled controls.
- `AffiliateReferralList.test.ts` covers pending rows and server page events.
- `AffiliateTierProgress.test.ts` covers progress and top-tier states.
- Referral rows, changed summary values, and tier-completion feedback own their local reduced-motion-safe transitions.
- Failed page refreshes keep the previous rows and show their error; a later
  page response cannot replace a more recent selection.
