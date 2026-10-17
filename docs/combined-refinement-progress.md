# Combined refinement delivery record

## Delivered agent foundation

- Panel CI fixture repair: `c9b5ccdb` on `main`.
- X-Node signed release: `v2.0.6`, source `fa867d98c0a48915bd04ffc5c15b327a8ff24360`.
- Hosted Quality: [39047630061](https://github.com/VanSail-Opensource/X-Node/actions/runs/39047630061), all jobs successful.
- Signed release: [39048159251](https://github.com/VanSail-Opensource/X-Node/actions/runs/39048159251), successful.
- Production-trusted manifest signature and published amd64/arm64 agent/CLI hashes independently verified.

The agent now samples active flows without an idle grace period, keeps hard handler/accounting reservations separate, supports owner-scoped peer authentication, rejects agent GeoIP assets/selectors, accepts resolved non-Reality source filters and shared domain blocks, and schedules geosite refreshes from completion with bounded retries. Existing CLI distribution-gate removal is included in the signed binaries.

## Panel native-settings package

Implements queued, bounded country-prefix resolution; native-only sampled activity decisions; service-user global-limit exemptions; effective Abuse overlays with exact exemptions; typed DNS/provider/firewall/binding/endpoint forms; optional per-user node bandwidth; guarded settings reads; and experience-injected operation polling. Node/rate SQL constraints migrate to allow zero without changing settled accounting. Client subscription documents retain independent GeoIP validation.

Hosted Container [39068953668](https://github.com/VanSail-Opensource/VanSail-Carpool/actions/runs/39068953668) passed at `a074c580`; its only predecessor failure was corrected in the migration regression assertion.

Static evidence: targeted Go vet, frontend typecheck, changed-module ESLint, structure/localization checks and generated API contracts. No local automated suites were run. Hosted regressions cover the new accounting, upgrade, selector, activity and request-scope boundaries.

Chrome DevTools MCP used an isolated browser because the configured shared profile was occupied. Constructed fixtures covered 1440px/light and 390px/dark/Chinese layouts, initial/loading/populated/empty states, load retry, failed-save draft retention, stale node responses, zero multiplier, bandwidth, port lists, provider secret masking, domain JSON errors, duplicate bindings, endpoint fields, keyboard switch control and admin command polling. Successful affected flows had no console or request errors. Mock results do not establish production fleet rollout.

## Managed routing package

Implements typed admin presets, purchased inbound/template/outbound choices, a
shared ordered matcher for compilation/preview/path projection, conservative
conditional loop proofs, private transit credentials, encrypted legacy conversion
evidence, audited durable deployment barriers and dependency revalidation.
Unsafe or uncertain server contexts fail with explicit diagnostics. Expanded
geosite sets stay in bounded memory; member projections contain references.

Chrome DevTools MCP covered desktop light and mobile dark/Chinese, separate
Google/AI paths, destination highlighting, superseded previews, interactive node
filtering, keyboard selection, ordering/deletion, same-node exclusion, matching
loop refusal and disjoint cycles, member receipt polling, failed-load retry,
failed-save draft retention and typed admin preset saves. Mobile uses a vertical
graph with readable node targets. Successful affected flows had no console or
XHR/fetch errors. Static vet, frontend typecheck, targeted ESLint, structure/i18n checks and regenerated contracts passed. Hosted routing regressions remain pending until delivery.

## Work still required

- USDC mode/scanners/cursors/proofs/settlement, courtesy matching and immutable review dismissal.
- Native admin scans/profile metrics and seven-day graph, TX Statistics browser migration, remaining metric tables/charts and API Access/control refinements.
- Subscription regex precedence/preview, preserved references, base64 compatibility, Stash/sing-box selection and REALITY hybrid-key output.
- Final hosted panel regressions and sequential enabled-node preflight/upgrade verification. Remnawave ownership remains unchanged.
