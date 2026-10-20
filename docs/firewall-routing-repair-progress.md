# Firewall, commerce, assets and routing repair

Delivery evidence for the 2026-10-20 request. No local automated suites ran.
Chrome fixtures establish UI behavior; production debugging was read-only.

| Work | Implementation and validation |
| --- | --- |
| Live diagnosis | Initial read-only snapshot: 18 synchronized nodes, no firewall sync error, largest active-ban count 2,483; country filtering enabled. No host SSH/process/profile access supplied. Metrics payload about 1.67 MB, including up to 500 logs per node. |
| Agent capacity, retries and lookup | Versioned 1,048,576-entry ban sets, atomic legacy cleanup, master priority, bounded source refreshes, startup trimming of old durable overflow, enforcement backoff and sorted resident/sparse lookups. Hosted race, schema, security, architecture and both Debian host checks passed. |
| Source memory and storage | Ten million IP/CIDR records: 17,756 KiB peak RSS, 5,373,952-byte peak heap, 1,289,079-byte sparse index, 5.80 seconds. Disposable-host million-prefix RSS grew 7,442,432 bytes; urgent control, low-space recovery and 2 GiB admission checks passed. |
| Signed release | v2.0.11, source `153cf0d10bbc895f8a05cf4aa7b2fb8b02be2007`. [Quality 39441871106](https://github.com/VanSail-Opensource/X-Node/actions/runs/39441871106) and [release 39444134348](https://github.com/VanSail-Opensource/X-Node/actions/runs/39444134348) passed. Production-trusted manifest signature/version and all amd64/arm64 agent/CLI hashes independently verified. Full hashes are in internal/xnode/README.md. |
| Panel pin | `v1.0.2-0.20261020144243-153cf0d10bbc`; public protocol/native API compatibility checked through Go static/build validation. |
| Add-on overrides and duplicate facts | Either non-NULL override, including [], rejects before pricing or financial writes. Paid facts are checked independently; charged replay remains idempotent. Hosted financial/HTTP regressions passed. Chrome checked English/Chinese quote and commit rejection, normal success, loading/populated/empty/catalog-error states and unchanged rejected fixture additions. |
| Writable process scratch | TMPDIR resolves to DATA_DIR/tmp before app bootstrap; explicit SQLite/TMP/TEMP overrides remain supported. Asset create/write/seek errors are safe unavailable diagnostics; policy causes are sanitized. Hosted bootstrap/storage regressions passed. Chrome saved the constructed country-filter form with configured asset URLs and checked localized policy failure behavior. |
| Selected GeoSite expansion | Only selected template IDs resolve assets. Unrelated templates do not block Add/preview. Selected missing/unavailable assets return specific localized errors; invalid data remains distinct and verified stale refresh data remains usable. Hosted selected/stale/deployment/HTTP regressions and Chrome flows passed. |
| Routing tree and details | One tree combines You, ordered node/host hops and administrator-named template leaves. Details show original selectors, intersections/exclusions and semantic route status. Bounded IDs, endpoint isolation, error-state leaves, node filtering and keyboard activation are covered. Desktop/mobile Chrome checked readable details and contained layouts. |
| Access-loss route cleanup | Owner, plan revision and compile revision fence pruning; purchase-boundary crossing defers it. Inaccessible current/previous choices and superseded operational customizations are removed; remaining choices disable for review. Immutable evidence/accounting remain. Normal master synchronization removes runtime rules/private peers, including reconnects. Hosted database/deployment regressions passed. |
| Connections | Only nodes with observed active user IPs render. Scan metadata still tracks all targets. Chrome checked mixed active/empty/unavailable nodes, loading, empty and failed scans. |
| Endpoint deletion | Existing DELETE API, named action, busy guard and empty state. Chrome success removes the row; failure retains rows and shows a localized error. |
| Live logs and metrics | Selected-node memory-only logs, pause/refresh/close/session cleanup, stale/offline/empty/error states and decimal dropped counters. Summary polling omits fleet log payloads and avoids overlapping slow reads. Charts/sections mount lazily; native virtualization displays about 17 of 6,464 CPU rows with correct end scrolling. Chrome checked live updates, stopped pause/close polling and desktop/mobile layouts. Hosted lifecycle/buffer regressions passed. |
| Validation and delivery | Go vet/build/module integrity, frontend typecheck, ESLint, structure/i18n and production build passed. Chrome covered 1440px/light/English and 390px/dark/Chinese without application console errors or unexplained request failures. [Panel regression/image workflow 39440106230](https://github.com/VanSail-Opensource/VanSail-Carpool/actions/runs/39440106230) passed at b4526fa8, including all race shards, memory/CPU checks, 383 frontend tests and both images. Final dependency-pin validation is gated by the main Container workflow and reported in the delivery response. |

## Hosted failure repairs

The generated API files were regenerated after the final OpenAPI ordering edit.
The routing fixture now uses a fingerprint accepted by the operation contract.
Repeated subtest names exposed SQLite URI fragment aliasing: literal paths now
escape hash/percent/space characters, with file-location and isolation coverage.
Override cases retain every no-debit/no-ledger/no-adjustment assertion.

The reboot harness waits for a new boot identity, successful completed firewall
execution and an agent PID. It handles an inactive successful oneshot and the
constructed agent awaiting master configuration. It never infers live proxy
readiness from systemd state. Full reboot/uninstall checks and original deadlines
remain; both Debian ownership modes passed at the signed release commit.

## Delivery boundaries

No direct production setting, balance, route or node-version writes were issued.
Fleet upgrade and post-deployment CPU/kernel-cardinality checks require deployment;
the initial healthy field snapshot did not reproduce the earlier capacity failure.
The 256 MiB inventory-processing budget excludes kernel sets and independent
Xray/session/dynamic-master-state workloads. Existing hidden paid add-ons are not
retroactively reconciled. Existing plans whose digest included unused templates
can encounter one routing_dependencies_changed transition on revalidation.
