# Squad profile components

- `profile.ts` owns localized profile type labels, icons, and ISO country options.
- `CarrierLogo.vue` renders the bundled China Telecom, China Unicom, and China Mobile route marks with accessible labels.
- `SquadProfileFacts.vue` renders typed ISP, upstream, carrier, port, and location facts for default and member-facing layouts.
- `SquadProfileSummary.vue` renders compact profile headings, optional generated facts, caller-owned name-prefix, name-tag, heading-meta, and facts slots, and safe extra Markdown. Its member presentation keeps the squad name and caller-owned state tags on independently wrapping tracks, combines localized type and port speed beneath the squad name, and keeps China route carrier marks without abbreviation prefixes.
- `profile.test.ts` checks locale-aware ISO country options.
- `SquadProfileSummary.test.ts` checks generated facts, caller-owned state tags, unlimited ports, and Markdown.

The summary is a pure projection of the typed API profile. It does not invent
provider data or persist a second generated description.
Member facts keep configured country/region visible independently of node inventory. Bandwidth stays in the member summary heading; route/carrier tokens wrap within narrow cards.
