# Internationalization

- `../../locales/*/usage-breakdown.json` owns all-mode traffic/cost labels,
  original-term estimate limitations, unallocated costs and coverage disclosures;
  `generated.ts` registers both language domains.

- `../../locales/*/hourly.json` owns Charge by hour member/admin terms, consent,
  eligibility, cooldown and conflict copy; `generated.ts` registers both locales.

- `generated.ts` merges generated locale-domain modules.
- `../../locales/*/recovery.json` owns the global render-recovery boundary copy.
- `../../locales/*/payment-crypto.json` owns localized BEPUSDT selection and payment-instruction copy.
- `index.ts` exposes locale state and translation helpers, and synchronizes the document language and localized title.
- `index.test.ts` covers interpolation, locale persistence, and recovery copy registration.
- `accountRecovery.test.ts` checks that verified partial-cleanup sign-in explains
  administrator review in both locales without claiming invalid identity.

- `paymentKeys.test.ts` ? Hosted checks for dotted crypto rail keys and ordinary nested lookup.
