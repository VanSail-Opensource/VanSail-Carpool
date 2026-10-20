# Firewall, commerce, assets and routing repair

Delivery checklist for the 2026-10-20 request. No local automated suites run.
Chrome fixtures establish UI behavior, not production mutations or rollout.

| Work | Status and evidence |
| --- | --- |
| Live diagnosis | Read-only metrics: 18 synchronized nodes; no sync error; largest active-ban count 2,483. Country filtering is currently enabled. No host SSH/process/profile access supplied. Metrics payload about 1.67 MB with up to 500 logs per node. |
| Agent capacity, retries, source lookup | Implemented and pushed `81b483f`; initial hosted race/schema/binary/vulnerability and 10-million-record RSS gate passed. Peak heap 5,431,296 bytes, non-race scale duration 5.77 seconds. |
| Agent host regressions and signed release | Signed v2.0.10 at `d7b719423668bd2e423cffb58a37961636376596`; Quality [39403623381](https://github.com/VanSail-Opensource/X-Node/actions/runs/39403623381) and release [39405357352](https://github.com/VanSail-Opensource/X-Node/actions/runs/39405357352) passed, including both disposable Debian hosts. Ten-million-record peak RSS 17,152 KiB; heap 6,324,224 bytes; 5.78 seconds. Manifest/version and all four binary hashes verified. |
| Legacy durable-cap follow-up | `32f84d3` adds startup trimming/persistence regression for old source/master overflow. Hosted Quality, corrected signed v2.0.11 and final panel pin are pending. |
| Panel pin | Updated go.mod/go.sum to `v1.0.2-0.20261020094941-d7b719423668`; full Go vet/build and module verification passed. Hashes in internal/xnode/README.md. |
| Add-on overrides and duplicate facts | Implemented with unchanged financial facts and charged replay regressions. Targeted vet passed. Chrome checked English/Chinese messages, no rejected fixture purchase, successful normal addition, loading/populated and 1440px/light + 390px/dark layouts without console errors or overflow. Empty, catalog-error and commit-time override fixtures also checked; rejected quote/commit creates no fixture addition. |
| Writable process scratch | Implemented TMPDIR under DATA_DIR/tmp, safe asset storage classification and policy warning causes; targeted vet passed. Hosted storage/bootstrap regressions pending. |
| Selected-template GeoSite expansion | Implemented; targeted vet passed. Chrome Add succeeds with an unrelated GeoSite template and missing asset; selected missing/unavailable URLs show localized specific errors and disable Save. Existing stale-cache and deployment regressions extended. |
| Routing tree/template details and status | Implemented shared-prefix tree with administrator template names, original settings modal, semantic status, node filtering and bounded IDs. Desktop/mobile Chrome checked details and error-state leaves. |
| Route removal after node-access loss | Implemented owner/plan/compile-revision fencing, purchase-boundary deferral, disabled remaining graph, operational-choice cleanup and native compile invalidation. Static vet passed; hosted database/deployment regressions required. |
| Connections show only observed active-IP nodes | Implemented. Chrome mixed populated/empty/unavailable nodes, loading, empty and failed scans checked at desktop/mobile sizes. |
| Subscription endpoint deletion | Implemented existing DELETE API action, busy guard and empty state. Chrome success removes row; failure retains both rows and shows localized error; mobile contained. |
| Realtime node logs and metrics performance | Implemented selected-node volatile logs, pause/refresh/close/session cleanup, stale/offline/empty/error states, decimal dropped counters, summary metric reads, lazy sections and native table virtualization. Chrome checked updates, pause/close stops polling, no hidden tables/charts and responsive modal. Large-table Chrome check: 6,464 CPU rows render about 17 rows in a bounded viewport and scroll to the final entries at 1440px/light and 390px/dark. |
| Final static checks, Chrome, panel push and hosted CI | Go vet/build/module integrity, typecheck, ESLint, structure/i18n and production build passed. Final edits pass static checks, production build and Chrome interaction checks with no application console errors or unexplained request failures. Panel commit/push/hosted CI pending. |

Production settings, balances, routes and node versions have not been mutated.
The 256 MiB source-processing target excludes kernel-set memory and independent
Xray/session/dynamic-master-state workloads; signed delivery remains gated on
all hosted checks. Existing hidden paid add-ons are not retroactively reconciled.


## Hosted failure repair

Panel Container 39408501412 stopped on generated API ordering drift, a short
operation fingerprint in the new routing fixture, and SQLite filename collisions
from URI fragments in repeated subtest names. The repair regenerates the final
contract, validates the fixture fingerprint, gives cases descriptive names, and
escapes literal SQLite paths with a regression for hash/percent/space filenames.
All financial assertions and override cases remain enabled. No local suites ran.

X-Node Quality 39408307643 passed tests, schema, binaries, vulnerability and the
large-firewall host; nftables-off passed its main assertions but lost its final
SSH channel after reboot. `55df0a4` waits for the new boot identity plus active
firewall/agent units before that session. Full host assertions and deadlines
remain. Replacement Quality is pending before signed v2.0.11 and final pinning.
