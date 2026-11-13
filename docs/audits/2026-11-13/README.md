# Complete repository audit — 13 November 2026

This audit continues [PR #147](https://github.com/VanSail-Opensource/VanSail-Carpool/pull/147), including its earlier verified fixes and recovered work. The review covers the whole repository snapshot, including application code, native control, persistence, migrations, tests, fixtures, frontend, API contracts, build/release tools, deployment/recovery instructions, and retained reference assets.

**Coverage: 4,819 current files accounted for, with zero missing or stale file hashes.** The [machine-readable coverage summary](coverage.json) and [per-file review ledger](file-review.jsonl) are the authoritative inventory. The ledger records each path, exact SHA-256, size, classification, review method and substantive review note. The source snapshot digest is `a951c88f5c170139e8a2398ae3853b22286b16aa7a1e0cbb1931eabdde7ce953`.

The report and its evidence files under `docs/audits/` are audit outputs and are excluded from their own hash inventory. Downloaded dependency source trees and local build caches are outside the repository scope. Dependency manifests, lock graphs, checksums, pinned native contracts and the repository's supply-chain workflows are within scope.

## How every file was accounted for

| File category | Current files |
|---|---:|
| Application and supporting source | 2,289 |
| Unit, integration and regression tests | 1,389 |
| Generated source (including 180 transaction adapters) | 182 |
| SQL migrations and schema contracts | 50 |
| OpenAPI contracts | 283 |
| Fixtures and browser checks | 147 |
| Configuration, manifests and inventories | 141 |
| Generated metadata and dependency lock graph | 2 |
| Module, operational and development documentation | 273 |
| Retained upstream contracts and reference material | 24 |
| Application/static assets | 22 |
| Reference assets | 17 |
| **Total** | **4,819** |

Handwritten code and tests were read in their module context and traced across callers, persistence and provider boundaries. Earlier review evidence was carried forward only when the current SHA-256 was identical. Changed files and new regressions received fresh review. This avoids presenting unchanged files as newly read during the resumed session.

Generated transaction adapters are a separate evidence category: the generator was read completely, every generated path was inventoried, and regeneration of all 180 adapters produced identical bytes. Generated API types and response schemas were checked against the authoritative OpenAPI documents and complete generated deltas. The npm lockfile was parsed as a complete graph, checking resolution, integrity, links, install scripts, engines and overrides. Large upstream schema/reference files and binary/static assets retain structural, contract and provenance review notes instead of fictitious line-by-line executable-code claims. Eleven design-oriented guides received complete inspection of their code, configuration, commands and inline-code inventories; their remaining aesthetic prose was structurally classified.

The resumed starting head was `5f064e7892282c592bca76ab692da294e7d9ed3b`. The first resumed checkpoint was published as `da319b3a25fbf903ae5be93218f1072cdbc29d55`, with the same tree as the locally validated snapshot. The comparison base of PR #147 is recorded in `coverage.json`. Deleted paths are retained there as an explicit retirement inventory.

## Financial correctness and payment recovery

The central financial rule is that accepted intent, exact money and retained settlement evidence must survive retries, timeouts, account changes and partial provider results. Several confirmed defects violated that rule at a boundary rather than in the main arithmetic path.

| Area | Confirmed issue | Corrected behavior |
|---|---|---|
| Checkout execution | Preflight could become stale before claiming an external creation, after cancellation or immutable expiry. | Database admission rechecks the current order and owner before provider work. Terminal results replay; uncertain external attempts are reconciled without blind creation. |
| Payment accounts | Checkout/callback identity and credentials could become detached from the durable selected account; profile-read failures could fall back to unrelated settings. | Orders bind immutable payment-account identity, credentials remain while referenced by evidence, and configuration failures fail closed. Readiness examines every enabled matching account. |
| Telegram Stars refunds | The generic event validator allowed an absent charge ID for pre-checkout, and the refund path reused that allowance. | Refund callbacks require a nonempty exact retained provider charge and a paid/refunded order before wallet reversal. |
| Purchase, renewal and add-on arithmetic | Overflow, stale replay terms and duration inference could alter admitted amounts or proration. | Exact integer boundaries are checked before charging; accepted fingerprints and purchased-term evidence govern replay. Historical unknown duration is not manufactured. |
| Paid-time boundaries | Lexical comparisons of variable-precision timestamps disagreed with chronological order at fractional starts/expiries. | Financial, entitlement, refund, renewal, report and retention selectors compare exact instants with their declared inclusive/exclusive boundaries. |
| Coupon and activity data | Malformed retained discounts or prize definitions could reach financial admission; repeated raffle revisions could lose coherent reservation evidence. | Validate the entire retained financial shape before fees, sampling, reservation or settlement. Replays retain immutable classifications and revisions. |
| Affiliate recovery | Malformed tier configuration could look valid; conflict replay could leave PostgreSQL transactions aborted. | Business paths fail closed, administrator repair remains available, and receipt replay leaves the enclosing transaction usable without duplicate commission. |
| Administrator grants | Concurrent matching requests first held shared owner locks, then all tried to upgrade them; PostgreSQL could exhaust the existing transaction retries in a repeated deadlock cycle. | Acquire actor/owner locks in stable ID order with the required owner strength from the start. Keep a distinct actor shared, preserve immutable replay, and verify self-grants, reciprocal administrators, unrelated-owner progress and exact durable effects. |
| Stablecoin scanning | Persisted cursors and histories could grow without bounds; recipient changes could reuse an unrelated offset and skip evidence. | Bound JSON structure/size and histories, bind pagination to the canonical recipient list, and restart the same window with immutable transfer deduplication when scope changes. |
| Ledger contracts | Seventeen active or retained entry kinds were absent from the generated enum. | Valid retained ledger pages restore through the real response contract; unknown kinds remain rejected. |

Relevant module contracts: [billing](../../../internal/billing/README.md), [purchase operations](../../../internal/purchaseops/README.md), [hourly billing](../../../internal/hourlybilling/README.md), [database](../../../internal/platform/database/README.md), [stablecoins](../../../internal/usdt/README.md), and [API schemas](../../../api/components/schemas/README.md).

## Pricing models, analytics and incomplete evidence

Global/model pricing actions silently stopped after an arbitrary first 512 products. They now advance bounded keyset pages using the existing durable job. Each completed target checkpoints its cursor under the currently held attempt and claim generation, comparing the expected previous payload. A pass processes at most four targets; pending compute or resource pressure keeps the continuation without consuming a delivery attempt. No second queue or schema was introduced.

Displayed-offer recording had an independent 512-setting cap. A displayed product outside that unrelated inventory prefix lost its offer evidence. Recording now reads settings for the actual shown IDs, bounded by the existing 64-product response. The regression spans 513 products and checks replay deduplication.

Derived summaries could lose delayed outcomes, remain apparently current after source changes, or be recomputed from already compacted history. Migration `00050_pricing_summary_recovery.sql` and its owning transactions add explicit dirty/sealed/stale semantics, source invalidation, admission coordination and retention sealing. Sealed history is preserved; incomplete retained raw data does not become a falsely fresh model metric. Dataset exports describe the 90-day minimum raw-offer retention window; cleanup additionally requires settled offer outcomes and sealed calibration evidence, so records may remain longer.

Reporting reads now preserve exact JSON numbers, wide aggregate arithmetic and a coherent snapshot across related queries. Their declared time windows are enforced chronologically. Null persisted statistics documents are unavailable evidence, allowing an independent valid partition and authoritative refresh to survive. The audit does not equate zero observations with failed collection.

Relevant contracts: [pricing](../../../internal/pricing/README.md), [statistics](../../../internal/statistics/README.md), and [database summary recovery](../../../internal/platform/database/README.md).

## Authorization, credentials and enterprise boundaries

Original authenticated authority must remain attached to an operation even when a later read sees a newer account. Both subscription adapters and the direct native subscription endpoint now check the caller's original owner ID and account generation before and after projection. Purged, blocked or revived accounts cannot inherit a stale request's credential access; mismatches discard the whole response.

Fresh administrator connection scans previously reached persistence without the actor. Admission now carries the original administrator ID/generation, validates current role/lifecycle/block state under transaction locks, and commits scan metadata, outbox work and attribution together. Polling rechecks authority both before and after sensitive IP projection. An already accepted read-only diagnostic may finish after a later demotion, consistently with accepted-command semantics; disclosure still requires current authority.

Backup deletion and other fresh administrative intents recheck current authority at the durable boundary. Cross-domain database and backup operations require the appropriate administrator and native-control scopes. Generic database operations cannot mutate protected billing, cleanup, usage-history or recovery evidence tables.

Error privacy was corrected at both write and read boundaries. Shared administrator failures no longer return raw SQL or integration diagnostics. Emby provisioning's former `safeFailure` only truncated errors, so private URLs, provider bodies or reflected credentials could persist and then appear in account views. New failures use safe text, while projections also sanitize historical rows. Typed retry, refund and review decisions remain intact; sanitized retry wrappers preserve their underlying error classification. Emby configuration storage outages now produce a safe unavailable response instead of a successful disabled configuration with a zero price; typed incomplete settings retain their intended setup behavior.

These controls do not create a new organizational tenancy model. Ordinary administrator-read access includes paged financial/pricing datasets. The single-use browser export grant gates a bulk archive and is not a confidentiality boundary against those existing administrator reads. The review records this design distinction rather than mislabeling it as an exploit.

Relevant contracts: [accounts](../../../internal/accounts/README.md), [administration](../../../internal/admin/README.md), [connections](../../../internal/connections/README.md), [Emby](../../../internal/emby/README.md), and [HTTP API](../../../internal/httpapi/README.md).

## Native access, subscription rendering and resource limits

Native access fixes preserve paid continuity across multiple accepted successors, keep execution leases aligned with their real queue budget, and prevent publishing a compiled candidate after its paid boundary. Standalone service names cannot occupy private hourly namespaces. Access repair rechecks current lifecycle, purchase and closure generations while preserving retired-node accounting evidence. Bounded repair scans exclude ineligible/conflicted owners before they can monopolize the page.

Large connection totals retain exact integer transport across owned principals. Legacy source/alias comparisons enforce canonical owner and node equality. Routing-loop proofs include the shared snippets that affect the compiled document. These changes preserve native authorization and immutable accounting as the authorities; UI projections and remembered legacy IDs do not grant access.

The [pinned native protocol](../../../reference/Upstream/X-Node/protocol.md#volatile-boot-traffic-streams) deliberately keeps new, uncommitted traffic in bounded process memory. Reconnects can replay it while the process survives, but process restart or buffer overflow can lose that usage. Restoring an older database can also omit reports that the agent already acknowledged and retired. The master retains surviving committed usage and loss/gap evidence; settlement uses those facts and can therefore underbill or produce a larger refund. This is a material billing/recovery tradeoff of the current upstream contract, not a guarantee of lossless metering. The audit preserves that explicit policy and never manufactures missing usage or charges from a gap marker. Durable legacy batches, command receipts and committed accounting have separate recovery guarantees.

Small subscription templates could repeat insertion markers enough times to allocate a large expanded tree before the final 8-MiB check. Preflight accounting now includes every repeated proxy/name insertion and JSON escaping before expansion. Selectable Xray profiles share the same allowance. The regression uses a template that is valid with a small preview inventory but excessive with the larger live inventory.

The dynamic tuning catalog populated only the main TLS fingerprint, making every explicit download fingerprint invalid. Both legs now use current choices and independent client compatibility checks. Download compatibility examines the materialized download leg, so unsupported fingerprints inherited from the primary leg are also omitted without changing saved settings or native output. Legacy format mappers could restore insecure TLS after endpoint policy projection; the secure policy is enforced on the final ephemeral client document, including the independent download leg, without rewriting saved encrypted mapper/source documents.

The HMAC replay cache had a per-session cap but no process-wide bound. It now limits total live nonces to 32768, refuses capacity before allocating a new session and never evicts live replay protection. Expired entries reclaim accounting. BEPusdt response amount comparison now bounds decimal length and exponent grammar before arbitrary-precision parsing, preventing extreme short exponent strings from requesting enormous allocation.

Relevant contracts: [X-Node](../../../internal/xnode/README.md), [subscription rendering](../../../internal/subscription/README.md), [request authentication](../../../internal/requestauth/README.md), and [BEPusdt](../../../internal/integrations/bepusdt/README.md).

## Durable recovery, notifications and user interfaces

Workers now respect authoritative admission results. A connection scan refused by persistence cannot proceed to provider work, and a competing terminal item cannot be overwritten by unblock or private-message execution. Concurrent scan polls lock the row before checking monotonic progress, so an older result cannot overwrite newer progress or completed success. The shared dispatcher reloads durable completion rather than treating a nil handler return as proof of an external outcome.

Telegram private-message recovery preserves topic, profile and delivered-message references under ambiguous responses. A changing profile-repair probe now survives truncation, allowing Telegram's returned chat/thread evidence to authorize the binding. Shared Markdown truncation avoids manufacturing empty bold entities from whitespace-only prefixes. PM routing defers slash commands; the HTTP command layer validates addressed bot suffixes before financial or administrator dispatch.

IP lookup repair no longer turns malformed traffic or null prefix responses into successful empty observations. Completion streams retain partial/error/skipped task states. Retained route summaries are validated before traversal, preventing malformed hop vectors from panicking recovery while preserving valid sibling evidence and repair eligibility.

Frontend corrections bind asynchronous outcomes to the current identity, request generation and component lifetime. Old HTTP failures cannot expire a new user's session. Pending administrator writes keep the reviewed target and draft stable. Confirmed writes remain successful when only a following refresh fails. Closing a payment sheet retires its observers and callbacks while retaining unresolved idempotency keys for safe reopening. IP progress polling no longer lets an obsolete timer repeatedly abort a slow replacement request. Provider probe feedback stays attached to its original rail, and the last Telegram BackButton owner hides and detaches the native button.

Private-message moderation had a separate parent-lifetime defect: both failed and queued commands requested a full profile reload, unmounting the component that owned the retained replay key, error and receipt. Failed commands now keep that state, while accepted commands refresh flags quietly and retain the component through polling. A failed quiet read preserves the last valid profile and blocks ordinary profile edits until recovery. The hosted browser fixture exercises lost responses, replay of the same key, failed refresh, recovered flags and terminal polling.

Browser fixtures were updated to current typed native routing contracts and isolated provider adapters. Constructed stablecoin checks/dismissals can no longer escape to real configured API actions. The stablecoin review drawer now owns its styles after teleporting outside the page container, and its empty state remains visible outside horizontal table overflow. Retired onboarding copy/styles and an undefined active skeleton animation were corrected. The hosted browser suite exercises desktop and mobile recovery behavior with screenshots and rejects provider/API network escape.

Relevant contracts: [provider operations](../../../internal/providerops/README.md), [private messaging](../../../internal/telegrampm/README.md), [notifications](../../../internal/notifications/README.md), [IP lookup](../../../internal/iplookup/README.md), and [frontend](../../../web/README.md).

## Retired code and deliberately retained compatibility

Confirmed unused direct Emby setup/retry entrypoints, raw credential-bearing native routing-template projection, duplicate provider statistics entrypoints, unused administrator validators, unreachable IP topology processing, unused aggregate failure state and formatter bookkeeping were removed. Earlier PR #147 retirements also remove direct payment execution paths, the standalone entitlement drain, obsolete importer repair and retired frontend APIs/copy/styles where call-site review proved no production consumer.

Historical financial records, migrations and acknowledged job kinds are retained where existing accepted work depends on them. Legacy provisioning delivery, paid reset completion/refund reconciliation, old notification tombstones, terminal receipts and immutable schema history are recovery contracts. The retained entitlement panel is explicitly a fixture consumer rather than an active production route. Transitive dependency checksum entries are not treated as live runtime modules merely because their names resemble a retired backend.

## Deployment and recovery procedure

Deployment examples now point at this repository's GHCR image and preserve an explicit image override. Root setup instructions no longer reference a nonexistent environment file, use the current Go toolchain declared by the module, explain the embedded PostgreSQL durable mount, and distinguish restricted runtime credentials from migration credentials. Native protocol reference snapshots match the pinned module; older signed-release verification is clearly historical.

Seven copied development-guide files also contained demonstrably broken examples. Corrections cover successful worker completion, cancellation-aware channel reads, the constraints import, a fuzz invariant at integer overflow, Vue attribute reactivity and typed SFC syntax, and Telegram component callback setup/cleanup. These are reference-code corrections; project permission and test policies remain intact. Ten obsolete PHP payment examples remain quarantined as non-executable text, with their insecure historical behavior documented rather than reintroduced into the application.

Use the existing [PostgreSQL recovery runbook](../../../deploy/postgres/recovery.md) and [schema command](../../../cmd/db-schema/README.md). The operational sequence remains:

1. Preserve the failed database/WAL, a verified backup, secured verification evidence and the original configuration master key. Recovery needs both database evidence and the key for encrypted settings/credentials.
2. Block authoritative ingress and stop the application, scheduler, outbox and native master. Preserve provider callbacks through the reviewed ingress mechanism. Preserve live agents and their state directories; avoid agent restart, upgrade or re-enrollment before reconciling surviving traffic and the separate durable obligations. Verify that the old database writers are stopped.
3. Restore the logical or encrypted physical/PITR backup into a fresh isolated target. Verify the recovered baseline against evidence from the same schema and selected recovery point, including authoritative table and identity-sequence digests, financial facts and role boundaries.
4. Apply any required forward migrations using migration-role credentials, then verify the resulting schema and financial invariants. Do not rewrite historical migrations or infer missing financial facts.
5. Reconcile uncertain provider effects, surviving traffic reports under their original identities, and separate durable agent journals. Use authenticated commit-before-ACK processing and preserve loss evidence instead of inventing missing bytes. Re-establish archiving on the promoted timeline and take a fresh full backup before normal traffic resumes.
6. Resume one application/master and its existing outbox/operation continuations. Succeeded/failed/compensated facts replay exactly; ambiguous provider outcomes stay under review until authoritative reconciliation. A failed notification or stale analytical summary is not permission to repeat a charge.

Migration 50 changes analytical recovery metadata and guards; it preserves financial/source facts. A sealed/stale summary is an explicit limitation of retained history, not a request to fabricate a fresh metric. Rolling back a deployed application must respect the current forward schema and recovery runbook rather than overwriting the only surviving database.

Hosted restoration validates the repository procedures with synthetic fixtures. Production readiness still depends on the selected backup storage, separately protected recovery credentials and master key, measured archive latency, and restore drills. This audit does not establish a production RPO or RTO.

## Verification and release gate

Repository policy prohibits local automated suites. Local verification therefore used formatting, `go vet`, `go build`, module checksum verification, complete generated-contract checks, frontend typecheck/lint/production build, shell syntax and static structure/workflow checks. New behavioral regressions run in hosted CI.

Checkpoint `da319b3a` passed hosted production-image validation, Go quality/security, native CPU profiling, the 256-MiB application memory soak, and actual desktop/mobile Chromium recovery. The browser job recorded 142 scenarios and 312 screenshots. Its frontend schema-test and database/native fixture failures were diagnosed individually: strict OpenAPI composition validation was preserved, immutable affiliate history was not weakened, account fixtures gained their required onboarding evidence, valid telemetry stayed within the native per-node limit, and count helpers gained explicit static queries. The omitted add-on array persistence boundary was also corrected.

Checkpoint `4feed117` passed all 900 frontend tests, Go quality/security, seven race shards, image/CPU/memory gates and both logical and physical/PITR restoration steps. It exposed the real administrator-grant lock conversion described above, a short fingerprint in the new Stars-refund fixture, and an overly strict PM browser assertion during an animated status transition. The grant lock order was corrected with controlled contention regressions; the fixture and browser checks were corrected without dropping their financial or retained-state assertions. All applicable checks must run again for the completed head.

**The merge gate is the complete set of applicable hosted checks on the final PR head**, available in [PR #147 checks](https://github.com/VanSail-Opensource/VanSail-Carpool/pull/147/checks). An earlier successful checkpoint is not evidence for later source changes. Final CI results and the exact merged revision are recorded in the PR, avoiding a self-referential commit hash in this report.

Hosted validation includes all eight race shards and coverage aggregation; frontend contracts, tests and dependency audit; desktop/mobile browser recovery; PostgreSQL business/concurrency contracts, logical export/restore and encrypted physical/PITR recovery; security checks; production image verification; native CPU checks; and the bounded-memory soak. No local suite, live customer billing transaction or real third-party delivery is claimed as validation.

## Evidence and limits

The [review notes](review-notes.md) preserve module-level findings from the earlier and resumed passes, with checkpoint-specific progress and validation statements labeled as historical notes. `file-review.jsonl` is the final current-byte reconciliation and supersedes intermediate coverage counts in those notes.

This is a complete repository-file audit with targeted fixes and hosted regression gates, not a claim that every possible runtime defect has been eliminated. External provider behavior, every downstream client version and private production configuration are not exhaustively reproduced by repository fixtures. Uncertain external outcomes remain governed by durable reconciliation. The report distinguishes confirmed defects, corrected fixture assumptions, retained compatibility and assessed candidates that did not establish a bypass.
