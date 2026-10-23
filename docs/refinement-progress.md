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
- [x] Complete static review and Chrome DevTools validation without local suites.
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
checks passed. Local automated suites were not run. Terminal hosted regressions
and image publication remain the delivery gate below.

The pinned X-Node revision is `5367fe1202201587e74ddea1293ae26c8be28e5b`,
matching signed release `v2.0.13`. The pinned release verifier accepted the
version-bound manifest signature, and both GitHub-published amd64/arm64 SHA-256
digest records matched that manifest. Manifest SHA-256:
`1f2a6020f02569eeeaf37ce1a567dba8f5e56db6d24204c6e7afa778f0db02b4`.
This checks published artifacts and protocol compatibility; no live fleet
deployment was performed. Agent source-set retirement uses the existing signed
protocol and waits for each enabled, connected firewall member's acknowledgment.

Final audit also covers reverse split-pair name collisions, FIFO enqueue ties,
standalone queue targets without member owners and historical migration fixtures.
