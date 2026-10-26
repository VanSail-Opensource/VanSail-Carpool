# Internationalization

- `generated.ts` merges generated locale-domain modules.
- `../../locales/*/recovery.json` owns the global render-recovery boundary copy.
- `../../locales/*/payment-crypto.json` owns localized BEPUSDT selection and payment-instruction copy.
- `../../locales/*/admin-compensation.json` owns reviewed node-outage compensation copy.
- `index.ts` exposes locale state and translation helpers, and synchronizes the document language and localized title.
- `index.test.ts` covers interpolation, locale persistence, and recovery copy registration.

- `paymentKeys.test.ts` ? Hosted checks for dotted crypto rail keys and ordinary nested lookup.
