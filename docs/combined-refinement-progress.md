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
XHR/fetch errors. Static vet, frontend typecheck, targeted ESLint, structure/i18n
checks and regenerated contracts passed. Hosted Container
[39088610536](https://github.com/VanSail-Opensource/VanSail-Carpool/actions/runs/39088610536)
passed at `4591c598`, including all race shards and image publication.

## Work still required

Runtime repair priority: X-Node `f2238bc` fixes installer/self-restart locking and
includes the two existing firewall import-format edits; `38d52a2` stages valid
listener targets before binding and joins activation shutdown. Hosted Quality
[39088271127](https://github.com/VanSail-Opensource/X-Node/actions/runs/39088271127)
passed, including race, binaries, vulnerability and disposable Debian checks.
`337c0ec` adds durable nftables/UFW ownership with Debian-on/off and Ubuntu-UFW
VM coverage. Latest audited code includes `1214722`. Quality 39090449636 passed
race/schema, binaries, vulnerability and both Debian VM jobs; Ubuntu hit UFW and
rollback deadlines. `893190b` adds bounded aggregate fixture time and safe command
timing diagnostics. `b2bf392` separates machine-readable subprocess stdout from
stderr diagnostics. `812e12d` restores iptables-nft tables after native snapshots. The subsequent Ubuntu fixture passed takeover/rollback/forwarding but hung on its reboot SSH channel. `5be64d2` bounds reboot channels/probes and enables SSH keepalives. Replacement Quality and signed v2.0.7 delivery remain pending.
Panel `4591c598` separates ready-agent liveness from sampled activity eligibility,
bounds failed compile/maintenance retries, and corrects the hosted deployment
hold to target the actual source configuration change. The continuing CPU audit
removes repeated capability/document work from unchanged syncs, separates command
errors from configuration rejection, ignores obsolete failures, and paces retry
of rejected state. Immediate negative ACKs cannot be overwritten by the send
return path. The hosted CPU profile now also exercises actual cached sync calls.
Container logs
showed no error according to the user; no production CPU profile was available.

- Stablecoin package implemented: USDT/USDC/both mode; pinned Circle native identities and queued precision probes; token-specific readiness/cursors/watchers; migration 085 preserving USDT progress and existing evidence IDs; exact-first integer matching for both tokens; selected-payment evidence settlement versus courtesy credit; permanent audited dismissal. Hosted Container [39112454263](https://github.com/VanSail-Opensource/VanSail-Carpool/actions/runs/39112454263) passed at `113595a5`, including corrected migration-boundary and memory-soak fixtures.
  Static evidence: generated contracts, frontend typecheck, targeted ESLint, structure/i18n, production Vite build and targeted Go vet passed. Chrome DevTools MCP exercised 1440px/light/English and an emulated 390px/dark/Chinese viewport: initial/loading/populated/empty/error/retry, signed mode/probe/save/dismiss requests, keyboard evidence selection, distinct courtesy/settlement choices, failed-load Save blocking, failed-save draft retention and contained layouts. Fixtures use constructed data and do not contact chains. Hosted regressions cover migration, token/precision isolation, exact/tolerance/ambiguity boundaries and courtesy/settlement/dismissal races. No local automated suites ran.
- Native admin package implemented: native-only durable admin scans, per-profile node activity and seven-day exact/reconciled accounting, selected-profile TXB adjustment, browser-only TX Statistics, memory-only bounded machine/user charts, grouped API token creation, and strict multiplier validation. Master transport liveness remains independent of migration ownership while activity requires native ownership, fresh ready snapshots and capabilities. Hosted regressions cover token receipt scope, unavailable nodes, precision, stale loads, chart retirement and exact counters. Static vet/typecheck/ESLint/structure checks passed. Chrome DevTools MCP used constructed 1440px/light/English and 390px/dark/Chinese fixtures for loading/populated/empty/error/retry, stale profiles, logout cleanup, scans, adjustment requests, permissions, one-time secrets, keyboard controls and multiplier boundaries; successful flows had no console errors or layout overflow. Hosted final results pending.
- Subscription package implemented: ordered bounded Go regex rules target exact template IDs; explicit format precedes regex, built-in client detection and real Base64 fallback. Existing native links and retained assignment references remain compatible. Stash has a distinct preset/format, sing-box remains exposed, and generated Clash/Stash REALITY hosts include hybrid-key support. Existing encrypted policy/revisions and generated API contracts are extended. Hosted regressions cover precedence, syntax, exact template versus assignment, compatibility and rendering; UI regression covers aborted stale previews. Chrome DevTools MCP checked desktop light/English and 390px dark/Chinese: loading, populated, empty, failed load/retry, revision-error draft retention, ordered rules, invalid Go lookahead, explicit Base64, real Go Stash YAML preview, retained assignment writes, keyboard selection, public/member format links, credential omission and console/layout state. Successful flows had no console errors; constructed fixtures establish no production rollout.
- Final hosted panel regressions and signed release verification/pinning remain required. The user explicitly skipped live node preflight and upgrades. Remnawave ownership remains unchanged.


## 2026-10-20 firewall and panel repair

Detailed checklist: [firewall-routing-repair-progress.md](firewall-routing-repair-progress.md).
Signed X-Node v2.0.10 at `d7b719423668bd2e423cffb58a37961636376596` passed
Quality 39403623381 and signed release 39405357352. Manifest/version and all
amd64/arm64 agent/CLI hashes verified with the pinned production trust key.
Panel pin is `v1.0.2-0.20261020094941-d7b719423668`; full hashes are recorded in
internal/xnode/README.md. Local automated test suites remain prohibited and were
not run. Panel delivery/browser/static evidence is tracked in the checklist.
