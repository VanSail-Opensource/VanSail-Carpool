# Actions supply chain

External action references are pinned to complete upstream commit IDs. The table records the refs resolved on 2026-11-08. Annotated tags are peeled to their commit, not pinned to the tag object. Action manifests and commit metadata were checked in their canonical repositories; Docker login's credential entry point and the attestation wrapper's transitive action binding were reviewed.

| Action | Resolved ref | Commit |
| --- | --- | --- |
| actions/checkout | v7 | [3d3c42e5aac5ba805825da76410c181273ba90b1](https://github.com/actions/checkout/commit/3d3c42e5aac5ba805825da76410c181273ba90b1) |
| actions/setup-node | v7 | [949feb2413d6458794dcd2491c4babbbce0c15c1](https://github.com/actions/setup-node/commit/949feb2413d6458794dcd2491c4babbbce0c15c1) |
| actions/setup-go | v7 | [b7ad1dad31e06c5925ef5d2fc7ad053ef454303e](https://github.com/actions/setup-go/commit/b7ad1dad31e06c5925ef5d2fc7ad053ef454303e) |
| actions/upload-artifact | v7 | [cf430e030ddbb5b0abf93d22962f4752f3646cd9](https://github.com/actions/upload-artifact/commit/cf430e030ddbb5b0abf93d22962f4752f3646cd9) |
| actions/download-artifact | v8 | [9000827ccba6bdab643e8b6fd33ac0654aef8333](https://github.com/actions/download-artifact/commit/9000827ccba6bdab643e8b6fd33ac0654aef8333) |
| actions/attest-build-provenance | v4 | [4d101475d8b20a2381f78447822ac1eab6504dd8](https://github.com/actions/attest-build-provenance/commit/4d101475d8b20a2381f78447822ac1eab6504dd8) |
| docker/setup-buildx-action | v4 | [f87e5991a6d7451dcb8d9637bfbc97413f497069](https://github.com/docker/setup-buildx-action/commit/f87e5991a6d7451dcb8d9637bfbc97413f497069) |
| docker/login-action | v4 | [8aa5831fb8d48d8f76594c7ddde61ae58019f550](https://github.com/docker/login-action/commit/8aa5831fb8d48d8f76594c7ddde61ae58019f550) |
| docker/metadata-action | v6 | [dc802804100637a589fabce1cb79ff13a1411302](https://github.com/docker/metadata-action/commit/dc802804100637a589fabce1cb79ff13a1411302) |
| docker/build-push-action | v7 | [c3c9e263c25d99ce0380d002d59b67737d91b0dc](https://github.com/docker/build-push-action/commit/c3c9e263c25d99ce0380d002d59b67737d91b0dc) |
| dorny/paths-filter | v4 | [ceb8a2b8f2d89434be7ff52d3de7ec3738c5cc9d](https://github.com/dorny/paths-filter/commit/ceb8a2b8f2d89434be7ff52d3de7ec3738c5cc9d) |

The attestation wrapper pins `actions/attest` to `508db95dd578ae2727ebd6217d5ba78e4fbda05d` (v4.2.1); its manifest runs the bundled Node 24 entry point, with no further composite action references.

Workflow defaults grant contents read only. The change detector additionally reads Actions and pull requests for its publication baseline and path filter. Platform builders can write GHCR packages; only manifest publication can obtain an OIDC token and write GitHub attestations. Dependabot's existing merge job owns its contents/pull-request write grants explicitly. Checkouts do not persist credentials into the worktree or Git configuration. Artifact downloads use the current-run runtime credentials; no cross-run GitHub token is supplied.

`web/scripts/workflow-policy.mjs` checks reviewed repository identities, full SHA refs, credential persistence and per-job write grants. Container publication must depend on successful quality and cannot be triggered by pull requests. `check-workflows.mjs` reads the real YAML files without contacting GitHub or executing repository jobs. It runs through `npm run audit:structure`; hosted `npm run test:workflows` covers mutable/short/unreviewed refs, broad defaults, misplaced writes, persisted credentials and quality/PR bypasses. Local automated suites remain prohibited by AGENTS.md.

When updating an action, resolve and inspect the commit in its canonical upstream repository, inspect its manifest and any composite dependencies, preserve the full SHA and readable version comment, and update this review record. New action repositories or write-bearing jobs require an explicit policy extension. Runtime packages and signed X-Node releases have separate compatibility and delivery gates.

This implements the full-SHA and least-privilege measures in [GitHub's secure-use guidance](https://docs.github.com/en/actions/reference/security/secure-use).
