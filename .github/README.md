# Hosted delivery

`workflows/ci.yml` owns hosted frontend, Go, PostgreSQL, race, memory and image quality gates. `workflows/container.yml` builds both image architectures after quality and publishes their combined manifest with provenance. `workflows/postgresql.yml` contains reusable recovery and database gates. `workflows/dependabot-auto-merge.yml` retains the existing verified-bot merge workflow without checking out pull-request code.

External actions use reviewed commit IDs and checkouts discard credentials. Read grants for change detection and write grants for publication/merging belong to their specific jobs. `../web/scripts/workflow-policy.mjs` and its CLI enforce these boundaries; negative regressions run only in hosted Actions. Extension and pin-review details are in `../docs/actions-security.md`.
