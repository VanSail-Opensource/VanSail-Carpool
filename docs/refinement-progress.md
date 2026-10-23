# Native retirement and DIY refinement

This checklist records the implementation and delivery gates for the agreed
refinement. Work proceeds in order; each group is audited before the next.

- [x] Remove the retired source integration, retire bridge artifacts, and preserve
  immutable history and exact native accounting through forward migrations.
- [x] Allow independent queued work to progress with bounded execution while
  retaining account/node ordering, durable leases, and IP-check serialization.
- [x] Keep proxy inventories in panel IP Lookup and clear published node sources.
- [x] Delete revoked API-token records and purge earlier revoked rows.
- [x] Repair combo submission and verify retained drafts and successful edits.
- [x] Prune DIY contracts and saved overrides; correct client compatibility,
  XHTTP multiplex policy, MPTCP, range editing, and split-profile names.
- [x] Simplify DIY tabs and controls; verify localized feedback and layouts.
- [ ] Complete static review and Chrome DevTools validation without local suites.
- [ ] Review intended diff, commit/push main, and verify terminal hosted CI and
  compatibility with the pinned signed X-Node release artifacts.

Native API names replace retired provider-specific names without wire aliases.
Historical migrations and immutable accounting/audit facts remain upgrade
evidence. Runtime code must not contact the retired source.

DIY validation used Chrome DevTools at 390px/dark and 1440px, including English
and Chinese, reduced motion, loading, no-host states, failed load/retry,
failed Save with retained drafts, valid and unfinished ranges, split references,
Mihomo randomized fingerprint compatibility and the fixed-off MPTCP control.
Range/exact values reached the shared Save as typed min/max objects. Selection,
press and success/error haptic events were observed with a constructed SDK host.
No console errors or failed preview requests remained in the successful flows.

Static evidence: Go vet, frontend types/lint, structure/localization and diff
checks passed. Local automated suites were not run. Hosted regressions and
signed native release evidence remain delivery gates below.
