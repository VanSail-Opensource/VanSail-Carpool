# DIY and access delivery

Ordered implementation checklist. Local automated suites are prohibited; hosted
CI and Chrome DevTools provide execution evidence.

- [x] Paid access: shared eligibility, queued recovery, subscription diagnostics.
  Static Go vet passed; hosted regressions added, execution pending final CI.
- [x] Telegram identity: canonical IDs, resumable encrypted migration, onboarding.
  Static Go vet, Vue typecheck and targeted lint passed. Chrome DevTools checked
  mobile/desktop agreement rendering, legacy-step recovery, loading/empty/error,
  retry, completion and Chinese conflict handling; no console/request errors.
  Migration and credential-bound agent rekey regressions await hosted CI.
- [ ] Firewall: global policy, master aggregation, node exemptions, whitelists.
- [ ] DIY: tabs, split profiles, conditional fields, atomic shared-quota billing.
- [ ] Runtime and delivery: incremental reload, signed agent, panel pin, terminal CI.

Preserve credentials, subscription tokens, exact accounting, purchase history and
independent browser sessions. Automatic cleanup never consumes DIY allowance.
