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
- [x] Review intended diff, commit/push main, and verify terminal hosted CI and
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
checks passed. Local automated suites were not run.

Chrome DevTools also exercised browser-admin combo edits with an empty description
and zero price, invalid traffic with retained drafts, token creation/deletion and
stale-list protection, plus native node populated/empty/loading/error states.
Desktop and mobile layouts had no document overflow or console errors in the
successful flows. Browser admin roots did not initialize the Telegram SDK.

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

Delivery evidence: commit `2893fb74e0f2b2486fb595a13117a397f76165c8`
completed [hosted workflow 39978903821](https://github.com/VanSail-Opensource/VanSail-Carpool/actions/runs/39978903821)
successfully, including all eight race shards, coverage aggregation, frontend
contracts/quality, Go quality/security, CPU profile, the 256 MiB memory soak and
production-image validation. Both linux/amd64 and linux/arm64 images published.
The multi-platform manifest has signed provenance and digest
`sha256:51ba03c6c2994c58425379640f40666d288c45b64435c9f5be35eb5288b040f1`.
This document-only completion checkpoint preserves that validated implementation;
its own push workflow is monitored before the final delivery report.
