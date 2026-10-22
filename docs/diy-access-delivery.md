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
- [x] Firewall: global policy, master aggregation, node exemptions, whitelists.
- [x] DIY: tabs, split profiles, conditional fields, atomic shared-quota billing.
  Full Go vet, Vue typecheck, lint, structure/i18n and diff checks passed.
  Chrome DevTools checked 390px/1365px rendering, loading, empty split hosts,
  populated profiles, creation/removal, tab draft retention, stream-one field
  visibility, error recovery, Chinese revision conflicts, insufficient funds,
  stale quotes and exact-key lost-response replay (one charge). No console or
  fixture request errors. Hosted billing/split regressions await final CI.
- [ ] Runtime and delivery: incremental reload, signed agent, panel pin, terminal CI.

Preserve credentials, subscription tokens, exact accounting, purchase history and
independent browser sessions. Automatic cleanup never consumes DIY allowance.

## Final audit and dependency refresh

The DIY route remains `/settings/diy`, with its only member entrance on Settings.
Chrome DevTools checked the entrance and rendered DIY tabs at 390px/1365px; the
Subscription page has no duplicate entrance. Upgraded Nuxt UI 4.12, Vue 3.5.44,
Pinia 4.1, Vite 8.3.5, Zod 4.6.7 and compatible direct/transitive npm packages.
TypeScript remains 5.9.3 because openapi-typescript 7.13 requires TypeScript 5.x;
no peer constraints were bypassed. Node 26.8.1 typecheck, lint, structure and
production build passed; npm audit reports no known vulnerabilities.

Go 1.27.3 fixes GO-2026-6809 in the agent and panel builds. The panel also updates
RoaringBitmap 2.30, Chi 5.5, TON utilities 1.19.1, netipx and SQLite 1.61/libc 1.79.
The Xray dependency stays aligned with the signed agent's native validator.
Local Go static analysis and module integrity passed; automated suites remain
hosted-only. Temporary local workspace overrides are excluded from delivery.

The audit confirmed atomic allowance/balance/settings/routing/receipt writes,
exact-request retry replay, legacy receipt revision compatibility, inactive field
filtering (including inherited modes), current-reference split exports and free
cleanup. Invalid combined profiles cannot remove ordinary hosts. Whitelist drafts
show unsaved changes separately from acknowledged protection. The agent audit
retains port ACLs for protected addresses and removes volatile protection from
rollback snapshots. Hosted Debian fixtures exercise silent TCP/UDP IPv4/IPv6
drops in both probe modes, first count deltas and master-only ban deadlines.
