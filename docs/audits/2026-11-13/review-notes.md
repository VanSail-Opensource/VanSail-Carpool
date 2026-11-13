# Module review notes and checkpoint evidence

These are the review handoffs retained from the earlier and resumed passes of PR #147. They preserve the reasoning behind confirmed fixes, assessed candidates, retirement decisions and recovery boundaries. Each section is a historical checkpoint note: statements such as “pending CI,” intermediate file totals and handoff ownership describe that checkpoint, not the final PR status. The consolidated audit report and exact current-byte file-review ledger supersede intermediate coverage claims. Final validation and the merged revision are recorded in the PR.

Evidence filenames mentioned inside a handoff identify its original reviewer ledger or temporary analysis record. The complete current-file proof has been consolidated into `file-review.jsonl`; local analysis paths are not required to use the delivered audit.

## Earlier verified review checkpoints

### root-findings.md

#### Root audit findings, resumed from d90b6516

##### Confirmed and fixed in working tree

- Payment create preflight can become stale before generic item claim, allowing a checkout after a cancellation already committed or immutable expiry passed. Added database-specific admission (persistence agent) and required it in the billing worker. Reconcile terminal/recorded outcomes, fail definite never-attempted canceled/expired orders, never retry processing external attempts. Preserve local built-in idempotent quote recovery. Worker uses the service clock; hosted tests cover cancellation, elapsed expiry and concurrent terminal item during preflight. Existing failure fixture counts the new claim.
- Backup delete now rechecks current administrator under a transaction (persistence agent). HTTP maps its explicit authorization sentinel to403 FORBIDDEN and API documents the before-filesystem check. Hosted HTTP regression added.
- Valid Telegram authentication encountering account/session persistence failure was mapped to401 INVALID_TELEGRAM_DATA through a retired error branch. Only real invalid auth/known unavailable accounts remain401; unknown persistence/context failures503 AUTH_UNAVAILABLE with retry header and no cookie changes. Removed never-produced accounts.ErrUpstreamUnavailable.
- Local-only agreement completion retained obsolete upstream proxy/username failure cases and502 XNODE_UNAVAILABLE defaults; continuation mapped all DB failures409 state conflict. New503 ONBOARDING_UNAVAILABLE identifies retryable local failures. Removed unused ErrUsernameUnavailable and obsolete agreement provider identity mapping; kept active entitlement identity conflict sentinel. API responses and hosted tests updated.
- Single-space community membership refresh wrote both group/channel flags using a stale request user snapshot, clobbering newer evidence for the other space. Narrow atomic selected-space DB update (persistence agent); accounts interface/callers/fakes and stale snapshot regression updated.
- USDT scanner cursor decoding could allocate arbitrarily large persisted JSON and Solana discovery could accumulate unbounded per-account histories. Bound serialized cursors to 2 MiB, preflight graph depth/array/node counts before typed decoding, and cap Solana account histories at4096 while retaining existing histories. Bound persistence loading/saving before mutation (persistence agent); oversize scans fail closed without replacing the cursor or committing partial evidence. Added hosted malformed-input, capacity, and persistence rollback regressions.
- TRON/TON pagination stored an address index or offset without binding it to the recipient list. A configuration/watch-address change could continue pagination in a different recipient and miss transfers. Bind pagination to a digest of the ordered canonical recipients; on a changed/legacy scope restart the same time window from its beginning and rely on immutable transfer identities for deduplication. Reject negative/impossible cursor windows instead of panicking or silently skipping work. Added provider-response regressions.

##### Validation checkpoint

- Go1.27.4 go vet ./... passed after first payment integration (subsequent edits require recheck).
- Node26 npm run typecheck, npm run lint and npm run build passed before latest accounts/API contracts; build emitted existing chunk-size advisory only.
- No local automated test suite executed; hosted CI required per repository AGENTS.md.

##### Coverage

Manual exact-byte file ledger in root-ledger.jsonl. Current review includes billing, purchaseops, hourly, hourlybilling, providerops, entitlements, accounts Go sources/tests. New and changed files need current-hash ledger refresh before audit completion. Report is intermediate and explicitly not a full coverage claim.

### persistence-findings.md

#### Persistence and infrastructure audit findings

Audit baseline: d90b6516af2461223f0d906b7db29c2abd338c1a. Coverage ledger is persistence-ledger.jsonl. Only fully inspected source or explicitly generator-verified artifacts receive reviewed status.

##### Fixed: canceled queue backlog blocks available capacity

With two provider workers and a one-slot backlog, one active key A and one queued key A fill the queue even though the second worker is idle. Cancellation of queued A previously did not wake the scheduler, so unrelated key B could not enter until active A completed. Added cancellation wake callbacks, explicit callback disposal and cleared removed backing-array references. Regression: upstreamqueue/cancellation_capacity_test.go.

##### Fixed: stale administrator can delete backups

Backup deletion previously trusted the HTTP authorization snapshot and staged filesystem mutation before opening its metadata transaction. An administrator demoted/blocked/admitted to cleanup while waiting behind an export could still delete recovery data. The transaction now locks and validates the current actor before file staging, and locks the exact backup row until metadata/audit commit. Regression: backup/delete_actor_test.go. HTTP sentinel mapping is owned by root agent.

##### Fixed: checkout creation lacks atomic current-order admission

The generic provider item claim only changes queued to processing. A canceled/expired order or account cleanup could win after billing preflight but before provider call admission. Added payment-specific transactional claim that locks and rechecks operation, item, payment and owner; expired creating orders are terminalized, canceled/foreign/non-live targets cannot acquire external attempts, and only built-in idempotent reservations can resume processing. The claim counts lock/retry waiting toward the supplied deadline. Database regression: database/payment_create_attempt_test.go; worker integration is owned by root agent.

##### Fixed: recovery fingerprint missed identity sequence drift

Recovery inspection fingerprinted only three sequence summaries and normalized every uncalled sequence to zero. Seven other owned identity sequences could drift without changing the fingerprint, and changed first-use values were invisible. The report preserves its legacy sequence summary and additionally fingerprints every inspected-table owned sequence plus the standalone proxy sequence, including exact last value, called flag, start/increment/min/max/cache/cycle options. Goose history is excluded. Regression: postgres/recovery_sequences_test.go exercises unchanged table contents with sequence-only drift, called state and increment changes. Recovery still requires stopped writers because PostgreSQL sequences are not MVCC state.

##### Fixed: single-space membership refresh overwrote independent facts

Parent identified a stale request snapshot replacing the other Telegram space while persisting one refreshed membership check. Added UpdateCommunityMembership with an exact group/channel allowlist and one atomic UPDATE RETURNING, preserving the independently refreshed other-space value and onboarding state. Database regression: community_membership_test.go; accounts integration is owned by root agent.

##### Fixed: recovery contract omitted durable outbox claim fence

The schema contract and recovery table digest omitted outbox_jobs.claim_generation despite its authoritative attempt-fencing role. Corrected schema metadata (no applied migration changes) and added postgres/recovery_outbox_test.go: only the fence changes the overall/outbox digest, other rows and sequence states remain equal, and dropping the fence fails schema verification.

##### Fixed: stablecoin review used lexical creation timestamps

USDTMatchingEvidence filtered variable-width RFC3339Nano text directly against order creation, which could hide valid same-second receiving evidence or surface an earlier event for review. The candidate bound is now deliberately broad and exact parsed instants decide inclusion. Settlement keeps its independent exact financial checks. Regression: usdt_evidence_boundary_test.go.

##### Fixed: unbounded persisted stablecoin cursor

Parent confirmed that retained Solana account history could grow without a total bound. Source limits/validation are owned by parent; persistence validates the shared2MiB value/128-byte hash bounds before any batch mutation and bounds SQL read materialization of retained oversized state. Rejected batches preserve the existing cursor and all unprocessed payment obligations. Regression: usdt_cursor_bounds_test.go.

##### Validation

No local automated tests run. Source formatted with Go 1.27.4; targeted go vet for backup, upstreamqueue, database and postgres passed. Reviewed retry-adapter generator regenerated all 181 wrappers with zero changes. Hosted CI is required.

##### Review status

Infrastructure packages and database editor fully inspected. Database application families, migration/recovery connector and most database tests remain in progress; no blanket clean claim.

##### Automatic renewal timestamp boundary (fixed; hosted regression pending)

`DueAutoRenewals`, `MarkAutoRenewalFailed`, renewal availability and failure
projection compared variable RFC3339Nano text directly. A fractional expiry could
be selected at the preceding whole second and the failure path expired that paid
term early; the inverse representation deferred an already-due renewal. The same
ordering error could report a false failure despite a later successor. Indexed
outer candidate bounds now feed exact Go time comparisons, failed renewal updates
lock and verify the exact current expiry, and supersession uses the same exact
comparison. Removed the redundant second failure write that repeated the first
write's predicate. New database regressions cover inclusive/exclusive nanosecond
boundaries, availability, and successor failure suppression.

##### Payment expiry and resumption timestamp boundary (fixed; hosted regression pending)

Both global expiry paths directly compared RFC3339Nano strings, which could close
a live fractional-deadline checkout at the earlier whole second or keep an expired
whole-second checkout visible. New checkouts and the expiry scanner now share a
row-locked candidate scan and exact comparison before redacting/expiring only due
orders. Checkout resumption filters exact expiry and chooses exact creation order.
Regressions exercise both mutation entrypoints and successive resumption deadlines.

##### Stored draw validation before financial admission (fixed; hosted regressions pending)

Instant draws and raffle joins computed worst-case loss from stored reward JSON
before validating all prizes; only the selected reward was validated later. A
malformed unselected prize therefore could allow a charged draw with undefined
probability/loss terms. Raffle settlement expanded stored stock before checking
aggregate bounds. Fresh draw/raffle admission and actual settlement now validate
the full stored definition first, while prior committed receipts and retirement
refunds remain replayable. Added malformed unselected reward, reservation,
aggregate-stock settlement and correction/replay recovery regressions.

##### Repeated raffle reserve revision cannot commit (fixed; hosted regression pending)

`reconcileRaffleReservationsTx` reused one immutable ledger reference per ticket
for every reserve adjustment. The unique financial reference prevented the second
valid reserve-changing definition edit. References now include the durable draw
revision, preserving one adjustment receipt per ticket and accepted revision.
Regression checks successive increases and release with exact ledger conservation.

##### Activity entitlement boundary and queued-extension ordering (fixed; hosted regressions pending)

Draw eligibility, group-message reward eligibility and subscription extensions
used lexical timestamp predicates. They could grant rewards before a fractional
start, reject a live fractional expiry, or extend an expired term. Queued extension
shifts could skip a later fractional successor or move an earlier whole-second
term. A shared current-purchase selector now parses exact candidate windows, and
both immediate and stored extension paths share an exact queued successor shift.
Regressions cover paid reward eligibility and both inverse queue boundary cases.

##### Malformed stored coupon financial shape (fixed with root domain change)

SQL CHECK constraints already reject nonpositive coupon value/cap, but allow
percentage values above10000, percentage caps on fixed coupons, and empty discount
mode. Persistence previously passed these values straight to discount calculation;
attached renewal silently converted some invalid modes/values to a full-price
charge. Root hardened the calculator; the attached adapter now preserves its
failure, and automatic-renewal planning reports discount unavailable. Added
persistence regressions demonstrating unchanged wallet/purchases/coupon uses.

##### Exact member controls, refund queue and recovery timestamps

Active-control conflict detection compared variable-precision UTC strings, skipping still-paid fractional overlaps or treating expired whole-second terms as live. Queued selection, refund successor selection, rebasing and live-receipt recovery cast timestamps to PostgreSQL microsecond precision, which conflated terms or boundary instants separated by nanoseconds. Shared canonical UTC SQL normalization now retains all nine fractional digits and deterministic C collation, without rewriting historical evidence. Native compile/statistics helpers reuse the extracted implementation. New member-control regression coverage checks fractional overlap and 1 ns queue selection/inclusion; the existing recovery projection test now checks 1 ns future starts and expiry. Local automated tests were not run; hosted CI is the validation path.

##### Administrative entitlement exact time and accepted duration

Admin bulk extension selection/commit, combo replacement, inventory display status and member active facets used variable-precision text comparisons. Manual grant overlap checks used microsecond casts. These now use exact paid-window selection or canonical nine-digit SQL ordering. One-nanosecond overlap is rejected while exact adjacency remains valid. New manual grant `purchased_term_seconds` used `time.Sub`, saturating at about 292 years; checked Unix-second/nanosecond subtraction now retains the exact accepted interval for representable UTC RFC3339 years. Original-result replay now reads the retained immutable duration field, preserving historical missing or already recorded values instead of recomputing or backfilling them. Bulk minute extensions also reject year overflow before writing unreadable timestamps. Hosted regressions cover all changed decisions; targeted static vet passed before the final facet assertion addition and will be checked with the next checkpoint.

##### Hourly legacy exclusion and native administrative intent

Hourly legacy conflict/view predicates now preserve 1 ns paid expiry instead of rounding through PostgreSQL timestamps. Native command and firewall-exemption creation now recheck current administrator authority inside the common provider-operation transaction, after immutable replay; native owner supplies demotion/replay tests and related fixtures.

##### Affiliate configuration and transaction recovery

An all-disabled stored tier configuration passed raw SQL checks but caused the member overview to index an empty slice. Stored tier definitions are now validated before progress projection or payment commission calculation; administrator raw configuration reads remain available for ordinary revision-checked repair. Duplicate affiliate settlement previously swallowed a unique violation, leaving PostgreSQL's transaction aborted; targeted `ON CONFLICT(invited_user_id) DO NOTHING` now safely replays the original receipt without duplicate credit. Referral pagination now returns empty beyond the known page count before calculating an overflowing SQL offset. Hosted regression coverage demonstrates fail-closed payment rollback, normal configuration repair, exact replay and maximum-integer paging.

##### IP lookup quota and execution precision
- Paid quota eligibility cast purchase bounds to PostgreSQL microseconds, losing the last paid nanosecond or admitting future quota. Exact canonical UTC ordering now governs both quote and transactional debit; hosted tests assert the actual cash/quota result.
- Lease claim/heartbeat/publication likewise rounded retained deadlines. Exact stored deadlines now compare against exact supplied clocks or formatted database wall clock. Hosted coverage proves no early takeover, live final-nanosecond heartbeat, and expiry rejection.
- Oldest shared-run and free-read snapshot ordering now compare exact instants instead of variable precision text/coarse casts.
- API token throttle now preserves its exact five-minute boundary; active list priority compares exact token expiry.

##### Notification and coupon projection consistency
- Expiry reminders and successor suppression used lexical variable-precision stamps; traffic period dedupe rounded reset nanoseconds. Exact comparisons now retain each paid window and reset period. Typed hourly funding warning clocks are floored, preserving exact ordering against microsecond stored boundaries.
- Administrative extension notices saturated at time.Duration's 292-year range; exact purchased-second arithmetic now reports the full authorized duration.
- Coupon overview excluded committed zero-credit multiplier redemptions even though they consumed configured use limits. The count now follows the same balance-redemption classification as admission.

##### Retention precision and live execution recovery
- Generic cleanup used lexical created/updated/expiry comparisons and rounded IP lease deadlines. It could delete terminal scan metadata before TTL, compact detail before its grace boundary, or refund an IP request with a live final-nanosecond lease. All affected gates now compare exact canonical UTC instants, with widened indexed raw bounds for large age scans.
- Rollup insertion and source deletion use matching exact predicates, preserving totals through boundary retries. Existing uncertainty/foreign-key/immutable-receipt protections remain intact.
- Extracted shared member combo-conflict expression for native repair scheduler so early indefinitely blocked owners do not consume the entire candidate page.

###### Outbox scheduling, recovery and administrative deletion

Raw variable-precision timestamp comparisons could claim not-yet-due jobs or recover a processing heartbeat newer than the stale cutoff. Admission/recovery now combine widened indexed candidates and exact canonical UTC comparisons; ordering uses the same precision. Direct job deletion now locks and rechecks current administrator authority, keeping existing financial/recovery and audit gates. Failure strings now preserve valid UTF-8 at their 500-byte cap and replace unsupported null bytes, so a long multilingual or malformed provider error cannot prevent durable completion. Hosted regressions are in outbox_boundaries_test.go; database go vet passed, automated tests were not run locally.

###### Account lifecycle and private-message time boundaries

Onboarding completion and preference mutation could write new state after a cleanup fence, and legacy provisioning used raw text expiry. These now require active owners and exact paid boundaries, with closed-account and nanosecond regressions in existing tests. PM inferred read status, latest route/page ordering and webhook deduplication retention also used variable-precision lexical timestamps; all now compare exact canonical UTC values, covered in telegram_pm_boundaries_test.go.

###### Diagnostics claim and financial chronology

Connection scan creation now rejects inactive owners after read-only replay, and its queued-to-processing claim rechecks lifecycle under owner/scan locks plus exact immutable request expiry. Ineligible requests become explicit terminal receipts; the worker must honor that returned terminal state before remote I/O (parent integration requested). Ledger pages and refund unwind selection now order exact UTC instants: a whole-second queued term cannot precede a newer fractional term in newest-first cancellation. New hosted regressions are connection_scan_admission_test.go and finance_chronology_test.go.

The connection-scan admission clock now includes actual lock/retry wait duration, with a blocked-owner regression; its obsolete generated wrapper was retired in favor of the existing complete-transaction helper. All 8 final administrative pagination sources were re-reviewed by reporting owner and their reviewed hashes reconciled.

###### Stablecoin matching candidate starvation

The bounded matching-evidence query filtered exact order creation only after its 100-row SQL limit. A page of older whole-second events could sort ahead of a later fractional valid payment and hide it. Exact eligibility now runs before the cap, and exact chronological sorting keeps the useful bounded result. A 101-transfer hosted regression reproduces this case. The final inline group-message dedupe cutoff was also aligned with exact retention ordering.

###### Pricing summary source recovery and compaction (coordinated native/pricing fix)

Wall-clock summary watermarks could hide backdated or concurrent source facts. Rebuilding after partial raw compaction also replaced complete historical metrics with incomplete data; nonlinear metrics such as median cannot be repaired from old aggregates alone. Migration50 makes these states durable: unsealed summaries retain complete source and become dirty; sealed metrics retain their original values, and later source changes mark them stale. Earlier potentially compacted cohorts migrate conservatively to sealed/stale, and recent cohorts become dirty for full rebuild. Raw source, wallet, contracts and financial receipts are unchanged.

Source INSERT/UPDATE statements acquire a shared advisory gate before source tuples; maintenance takes a nonblocking exclusive gate and never queues ahead of financial writers. Actual material row triggers preserve billing operations while marking derived state. Conflict-ignored inserts, no-op writes and retention deletions do not mark summaries. A database guard forbids sealed metric/watermark/scope rewrites, unsealing and stale clearing; dirty groups cannot seal without clearing pending recomputation. Added populated upgrade, state/replay and lock-order/snapshot regressions. Native owner supplies matching summary/retention publication logic; root supplies exported state fields. Targeted database/postgres vet passed; hosted SQL/regressions remain pending.

Peer review refined renewal invalidation to compare actual eligibility and zero/one settled contribution. Equivalent AUTHORIZED/DEFERRED_HEALTH/READY_TO_SETTLE/EXPIRED changes, non-exclusion reasons and non-null authorization refreshes no longer mark unchanged metrics stale. The hosted trigger regression now proves this and still proves cancellation invalidation.

### xnode-findings.md

#### X-Node audit findings

##### Fixed: prepaid continuity stopped after one successor

`continuityUntil` in `internal/xnode/access_window.go` walked purchase history once in returned order, but the production database returns newest-first records. With multiple contiguous queued prepaid renewals, the helper extended only through the earliest successor and skipped later prepaid terms. Both native users and metered probes use this expiry. The helper now sorts a private copy chronologically before extending the chain, preserving caller-owned compilation snapshots. Added `access_window_test.go` cases for production descending order, unordered batched reads, ascending order, unchanged input order, and gaps containing cancelled/failed/expired terms. No local automated tests executed; hosted regressions are required.

##### Fixed: deferred upgrade lease expired before the native execution budget

`HandleProviderOperation` imposed a twelve-minute parent deadline even though `update.install` and `update.rollback` each have a fifteen-minute execution deadline, after a separate thirty-second preflight and delivery window. A valid long-running upgrade was necessarily cancelled early and stored as an uncertain operation. The deferred lease now derives its timeout from the longest supported command plus those separate windows and publication overhead. `operation_deadline_test.go` captures the real deferred outbox-claim context to verify the update/certificate command budgets survive dispatch. Existing exact transaction/readiness recovery behavior remains in force. No local automated tests executed.

##### Fixed: compilation could publish a candidate after its paid-access boundary

`cachedConfiguration` captured time before its first storage read and reused that timestamp for cache eligibility and final fencing. Storage latency or compilation spanning a paid term boundary could reuse/return the old candidate. It now checks the service clock after the epoch lookup, queries the final epoch using current time, and refuses to publish when any captured access/refresh boundary elapsed. `compile_boundary_test.go` advances the injected clock during the initial cached lookup, compilation and final epoch lookup; it requires a conflict without replacing the previous cache or delaying immediate recompile. No local automated tests executed.

##### Fixed: standalone service names could occupy private hourly namespaces

The admin named-service validator rejected PAYG and metered-probe prefixes but allowed `member-hourly-` names. The compiler classifies such a name as an enrollment-owned identity and omits standalone admission, leaving a successfully created but unusable service. Case variants also bypassed service-side checks while SQL scoped inventory excludes private prefixes case-insensitively. Validation now reserves all private prefixes without case sensitivity, before credentials or periods are minted. The existing no-state regression now covers hourly, PAYG, probe and mixed-case names. This is an administrator misconfiguration/recovery issue, not an ordinary member privilege escalation. No local automated tests executed.

##### Fixed: profile connection totals overflowed across owned principals

`UserNodeMetrics` summed per-principal sampled connection counts in int64 even though its public field is an exact decimal string. Valid telemetry for multiple owned primary/PAYG/hourly principals could overflow negative and clear `currentUse` when rates were zero. The profile projection now uses big.Int like the exact accounting/total-connection paths. A regression invokes the full owner-scoped method with two valid max-int64 principal samples and requires `18446744073709551614` with current activity retained. No local automated tests executed.

##### Fixed: legacy client mappers could copy server transport secrets

Normal endpoint conversion derives REALITY public keys and strips server TLS certificate/key material, but the legacy client mapper also received the original native inbound as `metadata.rawInbound`. Its documented non-host copy operation can select entire nested settings, so an imported/admin mapper copying `streamSettings` could distribute server private material to member clients. The mapper now receives an explicit public metadata projection and copies only the current member's final account options and sanitized client transport. Native server settings and arbitrary top-level fields are omitted; source documents are untouched. Regression coverage exercises TLS and REALITY object-copy operations, current member/public-key retention, metadata secret exclusion, arbitrary-field and whole-host-inbound access denial, and source immutability. Existing native validator already rejects managed account lists; the sanitizer additionally defends that boundary without depending on earlier validation. No local automated tests executed.

##### Fixed: routing loop context proofs omitted shared snippets

Routing context analysis read only the saved base configuration and included snippet IDs, but not snippet content, in the graph dependency digest. Actual node compilation expands shared outbound and routing snippets. A structurally cyclic graph could consequently pass the conservative context proof while an untracked proxy fallback was hidden in a shared snippet; same-ID snippet edits also escaped dependency-change detection. Context analysis now expands the same saved snippets and hashes the resulting document. Candidate config/snippet updates are installed temporarily in the request-local compile read cache so nested endpoint and routing proofs observe the same hypothetical content as native compilation, with previous cache state restored afterward. Tests cover shared fallback cycle rejection, valid one-way fallback, same-ID content changes, read errors, source immutability and candidate cache restoration. No local automated tests executed; targeted static vet started.

##### 8. Retained standalone terms admitted before their start

`servicePeriod` checked expiry but omitted the encrypted authoritative StartedAt. Retained/imported future terms could compile and expose native credentials early. Added one captured-clock half-open start/end predicate, an exact-nanosecond unit regression and a real-store compilation/subscription regression. No stored terms or accounting evidence change. Hosted execution pending.

##### 9. Unused retired importer template repair

`RepairImportedTemplates` had no repository caller; its sole references were its definition and an obsolete README description. Removed the internal package exported function/file and updated the module contract. This eliminates the retired importer mutation surface while preserving all readers of saved native templates and historical encrypted data. No runtime behavior or financial records change.

##### 10. Native SQL time comparisons diverged from exact entitlement time

Access repair and native member growth compared variable RFC3339Nano strings lexically, which reverses whole-second/fractional ordering. Compile-version selection cast paid terms to PostgreSQL timestamps, rounding away nanoseconds. Repair now shares exact parsed active-purchase selection and exact queued lineage; native SQL boundary projections normalize retained fractional widths to nine digits with C collation without rewriting stored timestamps. Added1ns, whole/fractional and equivalent-precision hosted regressions.

##### 11. Repair could cross lifecycle and closure generations or erase retired evidence

The older RepairXNodeAccessPeriod adapter had no captured owner-generation lock and reopened by status alone. It also cleared every retired node confirmation. The native caller now carries its observed generation; storage locks the active owner and shared purchase writer, validates target ownership, compares closure_generation, and resets only live admission barriers while preserving permanent removal proof. Hosted cases cover purging/purged/stale generation, changed closure and retired/live barrier distinction.

###### Additional exact native timestamp boundaries

The same review found native account-purge IP receipts compared expiry identity after PostgreSQL microsecond rounding. Proof for a control ending one nanosecond earlier could be reused for a changed block. Both pending-control and final-purge predicates now compare normalized exact stamps; the hosted IP recovery regression requires a new receipt after a one-nanosecond change. Automatic lease unbans likewise preserve nanosecond lease eligibility, and certificate recovery chooses the actual latest receipt across whole/fractional-second ordering.

Firewall sweeps compared exemption deadlines lexically, releasing a paid grant before its true fractional deadline. Sweep expiry and seven-day operational retention now use exact normalized UTC timestamps while keeping historical paid amounts and original deadline text immutable. Legacy ban listing and cursor search use the same chronological order with generation tie-breaking, including equal instants represented with different fractional widths. Existing hosted tests now cover each boundary. Targeted `go vet ./internal/xnode ./internal/platform/database` completed successfully after these edits; no local automated tests executed.

##### 12. Fresh native administrator intents lacked transactional role revalidation

The shared provider operation admission gate omitted `xnode_command` and `xnode_firewall_exemption`. HTTP administrator checks can become stale during native preflight or concurrent demotion; durable command or firewall mutation admission previously accepted that stale authority. The persistence owner added both kinds to the existing current-admin transaction gate. Replay lookup remains ahead of fresh-authority validation, preserving recovery of already accepted intent. Native fixtures now establish real administrator actors; regressions demote the actor, reject fresh generic/specialized command and positive/zero-hour firewall requests, retain prior replay, and prove rejected requests leave no durable mutation. No new role semantics or caller approval flow introduced.

###### Remaining native temporal selectors

The group-retirement membership query rounded the last nanosecond of 72-hour PAYG credit away. It now retains that exact credit boundary. Native notification candidate selection compares stored purchase nanoseconds against a single formatted UTC database-clock instant. The global settings post-audit deadline fence reads the database clock and compares it to the original time.Time deadline. PAYG node costs preserve exact ledger timestamp bounds, while typed capture-minute history rounds lower bounds upward and upper bounds downward so driver microsecond conversion cannot widen the requested interval. Hosted regression cases were added to the existing group, cost and history files.

##### 13. Ineligible PAYG owners could starve bounded recovery

The periodic missing-tuple repair selected the first 64 onboarded owners without checking account lifecycle or bot blocking, despite the provisioner refusing those states. A persistent group of purging/purged/blocked accounts could repeatedly fill the candidate batch ahead of valid members, with no tuple ever being created to advance the scan. Candidate selection now matches active unblocked owner and non-removing/non-retired node admission. `xnode_payg_jobs_test.go` seeds 64 ineligible lexical predecessors, proves the active owner's repair is queued, and verifies repeat scans coalesce without unavailable-owner work. No financial evidence or provisioning authority is weakened.

##### 14. Legacy cursor ownership could cross native nodes

The legacy single-report cursor lock and the batch cursor adapter did not compare the retained cursor's node_id with the authenticated current node. A reused agent identifier could acknowledge a foreign cursor's earlier sequence or advance its journal under the wrong node. Both adapters now reject mismatched stored node ownership before replay or billing; batch preload excludes those entries from financial locks/reads. The existing cursor regression covers single, batch and legacy rate paths with both zero and already-advanced foreign sequences, requiring the original owner/digest/sequence and usage to remain unchanged.

##### 15. Migrated sample aliases omitted canonical owner equality

The retained old-proxy sample path accepted an alias that mapped to the expected native principal and registered period node, but did not check that the alias user_id belonged to that principal. Aggregate compatibility already required this relationship. Sample attribution now joins the canonical identity with matching owner as well; a foreign-owner retained alias is durably discarded with zero quota charge, while valid migrated samples still retain original period evidence. The existing alias regression now exercises this case with otherwise valid node attribution.

##### 16. Conflicted access-repair owners could monopolize the bounded scan

Access repair selected 128 candidates before checking pending operations, current temporary bans and other combo-control ownership. A fixed first page could therefore prevent all later eligible users from recovering. The shared existing combo authority expression now filters before LIMIT, while the transactional and worker checks remain. Fresh credential repair also skips bot-blocked owners that the credential writer cannot provision; complete existing credentials continue to qualify for paid-period recovery. Tests seed 128 permanent predecessors for review, ban and incomplete blocked credentials, and verify later eligible recovery plus deduplication. Exact purchase start/end comparisons in candidate selection preserve the final paid nanosecond and reject early or expired windows.

###### Final native temporal checks

Period finalization now compares the exact retained ended_at instant, preserving equivalent timestamp text while rejecting a one-nanosecond-different closure request. Unmanaged routing cleanup uses exact cutoff chronology. Raw usage rows, loss coverage and PAYG capture-minute history share inclusive microsecond ceil/floor bounds so nanosecond request parameters cannot widen the interval during PostgreSQL conversion. Hourly lifecycle retirement checks a newer prepaid purchase against an exact formatted database wall clock instead of rounding the paid deadline. Existing hosted regression files cover the deterministic boundaries; no local automated suites executed.

### frontend-findings.md

#### Frontend audit findings

##### F-WEB-001: moderation payload breaks strict generated contract

Confirmed in hosted CI at d90b6516. Fixed `AdminUserPM.vue` to construct a concrete blocked/muted union branch. No schema relaxation. Static validation and hosted CI pending.

##### F-WEB-002: old owner HTTP errors can invalidate the new session UI

A late 401/ADMIN_REQUIRED emitted global expiry unconditionally, clearing certificate attempt keys and suspending telemetry/log observers after identity switch. ONBOARDING_REQUIRED/CAPTCHA_REQUIRED likewise triggered new owner bootstrap. Added a distinct identity epoch to the shared cache session and captured it for JSON, blob and streaming request readers. Stale requests still reject to their caller but cannot emit cross-owner session events. Ordinary mutations do not suppress current owner expiry. Added 17 hosted regression cases.

##### F-WEB-003: unbound IP lookup completion payload

JSON fallback accepted a mismatched IP, and SSE only bound the envelope, not completed data. Both now require the exact requested IP before completion callbacks. Three regression cases exercise both transports.

##### F-WEB-004: unused random native-command key fallback

Removed the optional random key default after proving both current callers provide retained keys. This makes idempotency ownership explicit and prevents future retries from silently generating distinct attempts.

##### F-WEB-005: generic administrator mutations outlive their page

`useAdminSection.perform` admitted overlapping writes and started list refreshes after unmount. It also let reads begun before a write overwrite state while the mutation ran. Added single-flight/disposal guards and retired pre-mutation reads; confirmed writes still remain successful when only refresh fails. Three hosted regressions cover duplicates, ordering, disposal and partial read failure.

##### F-WEB-006: late Emby and coupon view results

Emby initial/refresh reads had no sequencing, allowing an old provisioning snapshot to replace a newer active account. Both Emby and coupon results published after their view disposed. Emby now uses latest-read/disposal fences; coupon disposal returns no grant and suppresses stale feedback. Added hosted out-of-order/retirement regressions and documented lifecycle.

##### F-WEB-007: Released Telegram BackButton stays visible

The last native BackButton owner removed its handler but replaced the button reference before hiding it, leaving an inert host navigation control after page teardown. Release now detaches and hides the previous host button before switching owners. Added a hosted regression covering stacked owners, reactive visibility, and final teardown.

##### F-WEB-008: Catalog editors allowed pending target/draft replacement

Catalog combo/squad fields remained editable during writes, and parent editor/confirmation events could replace or close the reviewed target while a request was pending. Combined with success-driven closure this could discard a new draft or close a different target. Both forms now disable their fields during writes, the squad handler rejects pending reentry, and the workspace gates editor/hide opening, cancellation and submission on the owning section state. Hosted regression tests cover one reviewed save/hide request and blocked target dismissal.

##### F-WEB-009: Settings enable stage could outlive its administrator workspace

Dependent disable/configure/enable writes await prior stages, then read current HTTP session at dispatch. A disposed previous administrator workspace could therefore send a later enable stage after identity replacement. Each write now rechecks owner-scope disposal; aggregate success feedback is suppressed after disposal and settings secrets are cleared. Admin Emby connection loads/saves now fence obsolete results and clear private drafts on teardown. Added hosted regressions for delayed configuration completion and delayed credential-save completion after unmount.

##### Retained legacy fixture-only module

`AdminEntitlementsPanel.vue` has no production route: `/admin/entitlements` redirects to users, and only `fixtures/ValidityAudit.vue` imports it. Retained as explicit fixture coverage of generic entitlement inventory expiry. No production permission decision relies on this component.

##### F-WEB-010: Affiliate configuration lost concurrent drafts and versions

Affiliate reads had no generation/disposal guard and a save unconditionally replaced the current tiers. A delayed cached refresh could roll a confirmed version back, and edits made during a pending save were discarded by its reply. Reads and writes now share a generation; saves snapshot the submitted tiers and only replace an unchanged draft. New confirmed versions are retained, reloads do not reapply old cache over edits, and disposal clears owner state. Three hosted regressions cover these races.

##### F-WEB-011 IP lookup configuration and credential lifetime
Confirmed administrator IP provider/fee/quota fields stayed editable during atomic saves and response hydration could erase newer secret drafts. Locked the complete form while pending; snapshot payloads, block concurrent load/save, fence owner changes, and clear credential drafts on change/disposal. Added three hosted regression cases. Maintenance trigger also now disables confirmation entry for pending-review/partial operation receipts, with a pending-review regression.

##### F-WEB-012 Pricing intervention result and recovery
Confirmed a successful manual intervention was reported as save failure when the following optimistic-version read failed; the original reason and consumed version remained. Confirmed success now clears the reason and version, emits refresh independently, and treats read failure separately. Disposed/replaced editors suppress follow-up requests and parent events. Added two hosted regressions for read failure after success and workspace disposal.

##### F-WEB-013 Pending administrator financial review dialogs
Confirmed entitlement editing and refund/balance amounts remained editable while writes were pending, and default native overlay close controls remained active despite nondismissible flags. Locked reviewed terms, amounts, reasons and close controls for pending entitlement, manual grant, payment credit and provider commands; submit handlers reject reentry. Added pending entitlement and balance adjustment hosted tests.

##### F-WEB-014 Native node-save snapshots
Node editor passed live reactive draft/settings references into the shared mutation runner, unlike adjacent fixed configuration/group editors, and kept fields/closing available while pending. Captured plain cloned payloads before dispatch, added local submission guard plus whole-form and overlay pending lock, and suppress late state after disposal. Added hosted deferred-runner test. Corrected two stale X-Node README claims (bound-node removal and retired DNS resolver controls).

##### F-WEB-015 — Closed funding sheets retained active financial observers

`BalancePaymentSheet` previously stopped only the payment target poll. A pending
create receipt could still trigger an order lookup and native Telegram invoice
after the sheet closed; a pending paid callback could later affect a reopened
sheet. `usePaymentOrder.suspend()` now invalidates order lifecycle and command
observer generations, stops target polling and clears the paid timer, retaining
unresolved idempotency keys for safe reopening. Sheet preparation is generation
fenced and handles initially open state. Coupon configuration locks its code and
provider selection during a write and drops outcomes after disposal. Hosted
regression cases cover late accepted creates with original key reuse, late paid
lookups/timers and disposed coupon success; no local suites executed.

##### F016 — Shared IP progress polling could repeatedly abort a slow refresh

`useSharedIPProgress` left an existing timer live when a manual read began, and an aborted earlier read could schedule another timer in `finally`. A replacement request taking longer than the polling interval could therefore be aborted by obsolete polling work. Reads now cancel their prior timer before starting and only an unaborted current request schedules the next read. Added a hosted regression that overlaps two manual refreshes with a pending timer and verifies a slow replacement completes before polling resumes. No local automated tests executed.

##### F017 — Add-on checkout allowed editing the selection during a submitted purchase

The add-on checkout Back action stayed enabled while purchase submission was pending. Returning to selection and editing changed the fingerprint, causing the correctly accepted response to be ignored and leaving the parent balance/purchase view unrefreshed. Disable Back and guard the parent transition during activation prompts and purchase submission. Added a hosted component regression for pending and released transitions. Explicit dialog cancellation retains the existing generation retirement and request-key recovery behavior.

##### F018 — Native member audit fixture still implemented retired raw routing APIs

`detector-retirement-audit.ts?native` assigned a removed `templates` method and returned raw customization documents from the current endpoint/template `routing` API. The real routing editor consequently received no `plan` and could not validate current behavior. Replaced these retired mocks with the current `RoutingView`, preview and versioned save receipt; completed subscription readiness fields. Added a desktop/mobile hosted scenario verifying the exact choice/revision/key. The member API's obsolete `XNodeCustomization` alias is removed; the administrator service-routing component and its existing regression import the same active type from the administrator API owner. Supplied missing required navigation gating props in two other fixtures.

F018 follow-up: removed stale fixture README instructions and the orphan `shared-error` receipt branch for retired handover/finalization paths, after whole-web reference search found no remaining consumer. Current native configuration, migration-compatibility and command fixtures remain.

##### F019 — Stablecoin fixture actions escaped constructed adapters

`stablecoin-audit.ts` mocked configuration, status and transfer reads, but the visible server check and permanent-dismiss controls retained real `usdtApi.check` / `dismiss` methods. With development proxy/auth context those actions could reach a configured server, contradicting the fixture's isolation contract. Added exact typed constructed probes and idempotent dismissal state, explicit probe/action failures, recoverable failed-dismiss trace, corrected retired credit-dialog documentation, and a desktop/mobile hosted browser regression that clicks both actions while the suite denies any API/provider network access. No local test suite executed.

##### F020 — Retired onboarding copy and current outage localization

Whole-source reference review found the core locale still advertised the retired mandatory community-join and permanent-username reservation flow, including an obsolete automatic-refund claim. Only the CAPTCHA fixture used its heading; changed that constructed post-verification heading to the active agreement title and removed the 22 unreferenced former-flow keys from both language catalogs. Retained current setup-progress/agreement/finish/error keys and historic error-code translations. Added bilingual AUTH_UNAVAILABLE and ONBOARDING_UNAVAILABLE messages to match the current retryable backend outages.

F020 language follow-up: English routing eligibility now describes current access (including hourly/PAYG), matching existing Chinese and actual endpoint authorization. Chinese gross-price label now says price before discounts; removed the unsupported tax-inclusive claim.

##### F021 — Retired onboarding styles and missing live skeleton animation

The full CSS pass and whole-web consumer search confirmed that the retired community invitation/username validation selectors and three old animation definitions have no remaining source, fixture or script consumers. Removed those dead selectors/keyframes. The active `SkeletonBlock` wrapper still uses `.skeleton-block::after`, which referenced an undefined `skeleton-pulse`; added the small opacity keyframe used by that existing rule. Existing reduced-motion policy remains in force. Static production build validates the stylesheet; no local automated test was run for this reversible style cleanup.

##### F022 — Font license provenance checksum clarification

The Noto standalone notice had a single trailing space normalized while its JSON `sha256` correctly described the original embedded upstream license. Added an explicit local checksum and normalization explanation, and corrected the README's byte-for-byte claim. Font binaries are unchanged.

##### F023 — Stablecoin probe ownership and disposed workspace

The token/network selector stayed editable during a server probe, but its result was an unscoped string. A delayed verification could therefore appear beside a different token rail. Probe results now retain their captured rail and the page only displays matching feedback. Configuration replacement/save invalidates pending probes. Disposed workspaces clear private config drafts and reject all subsequent API entry points. Extended existing hosted composable tests for config/probe ordering and disposal, and the isolated browser fixture scenario switches USDT to USDC to verify the old message disappears.

### pricing-database-findings.md

#### Pricing database audit findings

Scope: all 112 transferred baseline pricing database files (8,616 baseline lines), plus every new file added by this review. Baseline source and regression files have all been fully read. Generated retry wrappers and `pricing_basket.go` / `pricing_coupon_inputs.go` are covered by the persistence owner. The exact current-byte ledger is `pricing-database-ledger.jsonl`; changed files are re-recorded after final review.

##### Implemented corrections

###### 1. Current paid purchase and renewal scheduling lost timestamp precision

Pricing-owned timestamps use fixed nine-digit precision, but purchase history uses variable RFC3339Nano precision. Raw text comparisons made the whole-second expiry sort after later fractions. Checkout preparation, coupon scope, catalog prices, member offers and personal renewal decisions could continue using an expired purchase or omit a still-paid last nanosecond. The renewal scheduler had the same issue at the seven-day and 72-hour cutoffs.

All affected purchase selectors now use the shared exact normalized timestamp expression and deterministic ID tie breaks. The renewal candidate query retains coarse index bounds followed by exact inclusive/exclusive predicates. `pricing_purchase_boundaries_test.go` covers whole/fractional paid boundaries and scheduler edges; `pricing_coupon_boundaries_test.go` verifies expired purchases cannot lend combo coupon eligibility.

###### 2. Response-model support included observations omitted from training

The native compute adapter trims the oldest observations to the 2,048-row and 256 KiB limits. Model diagnostics and personal-price support were computed from the original full sample slice. Old discarded extreme prices could therefore authorize extrapolation outside the actual training range and inflate support counts.

`pricing_models.go` now narrows the sample suffix to the native result's verified sample count before deriving support, coupon support, and diagnostics. New `pricing_model_support_test.go` forces byte-budget trimming and checks stored support against the actual accepted compute task, including the holdout exclusion.

###### 3. PAYG prepaid capacity was wrong at the exact credit-window cutoff

`pricing_optimization_inputs.go` compared variable `credit_started_at` text to a fixed-precision cutoff. A whole-second paid credit at the excluded boundary could consume sellable capacity, while its final live nanosecond could be classified incorrectly. Both committed-capacity queries now compare normalized exact instants. `pricing_capacity_test.go` verifies cutoff exclusion and cutoff-plus-one-nanosecond inclusion, committed capacity, and resulting demand/sales.

###### 4. Receipt analytics and member features crossed their declared time windows

The ledger timestamp format differs from pricing timestamps. Product actuals, product forecast input receipts, and the summary's authoritative net revenue now use coarse index candidates plus exact `[from, until)` predicates. The summary also excludes interventions and consumed discounts at or after its declared as-of time.

Member `renewals30` used only a lower bound, so concurrent or future-dated evidence could enter a feature with an earlier watermark. Its query now counts only automatic-renewal ledger rows within the exact lower bound and inclusive as-of instant.

Regression coverage was added to `pricing_product_receipts_test.go`, `pricing_summary_payment_test.go`, and `pricing_personal_freshness_test.go`. The receipt test verifies both the displayed total and the durable native compute task's first and last daily samples.

###### 5. Export filtering and pagination used lexical rather than chronological dates

Raw wallet/purchase timestamps could be included outside requested scope or excluded inside it. The same lexical sort could produce incorrect chronological keyset order, particularly when equal instants were stored with different fractional precision.

Timestamp dataset filters now use exact normalized instants with coarse index bounds. Dataset page ordering and every cursor comparison use the same normalized expression; raw source timestamps are preserved in returned/exported records. Date-only accounting keys and integer nanosecond keys retain their native representations. `pricing_dataset_keys.go` holds the shared key helpers. Expanded `pricing_dataset_page_test.go` covers whole seconds, one nanosecond, equivalent fractional encodings and exclusive upper bounds.

###### 6. Related export metadata crossed product types

User-offer and node-binding relations compared only product IDs. A combo and an addon/PAYG group sharing an ID could bring unrelated product metadata into a scoped export. `pricing_export_relations.go` now binds the full product kind plus ID, and only combos use combo membership relations. The hosted dataset-page regression covers both user and node scopes.

###### 7. Export source watermark ignored current boot-scoped native receipts

The manifest only read legacy `xnode_report_cursors`, although current native receipts advance `xnode_traffic_streams`. It also used lexical `MAX`. The watermark now includes both durable cursor families and picks the exact chronological maximum. The existing full export/privacy/cancellation regression now verifies a boot-stream-only latest receipt appears in the manifest.

###### 8. Retired pricing helpers had no production callers

Removed unused `pricing_cost_evidence.go`; the active optimizer already captures retained cost version evidence during its actual input read. Removed the unused `PricingPAYGBatch` wrapper, which always reset pagination to the beginning. The application's cursor-preserving `PricingPAYGBatchAfter` path remains the only batch entry point.

###### 9. Derived summaries could lose delayed outcomes and compacted history

Validation and calibration summaries used processing wall-clock time as coverage proof after separate source reads. A concurrent/backdated outcome could be treated as already summarized. Calibration also needed to notice a previously recorded outcome becoming mature later, and the cleaner deleted 1,000-row source subsets while leaving the original aggregate eligible for reconstruction. Rebuilding from the remaining subset would silently erase the already compacted contribution.

The coordinated correction adds `dirty`, `sealed`, and `stale` state to both derived summary tables. The persistence owner supplies migration 50, source-write triggers, immutable sealed-summary guards, and upgrade/locking regressions. This review supplies the Go publication, actual-write, and retention protocols:

- Source writers hold shared transaction gates, including forecast creation before its feature/entity/batch/value writes. Summary publication and source-only compaction try an exclusive gate without queuing behind financial transactions. Busy work defers to a later pass. ReadCommitted transactions acquire the gate before reading source, preventing a snapshot captured before a wait from hiding committed outcomes. Only an actual new forecast insertion changes cohort state; conflict-ignored replay stays clean.
- Changed unsealed evidence marks its cohort dirty. Publication reads the whole available cohort, replaces its buckets atomically, and records actual source/maturity watermarks. Calibration also checks per-bucket source counts, so another row's later timestamp cannot hide newly mature observations. Existing empty cohorts and newly resolved all-excluded cohorts receive an explicit zero-sample marker: later eligibility corrections remain discoverable and the completed source can eventually finish retention.
- A complete clean cohort is sealed before the first bounded source deletion. A cohort containing an unevaluated forecast cannot be sealed, and each late forecast still requires an actual before purge even if its older cohort was already sealed. Purchase offer retention now spans the full 90-day rebuild horizon and aligns its cutoff to a complete UTC day.
- Once sealed, original metrics cannot be rewritten or reopened. Late material source changes remain writable and mark the aggregate stale. Replays and unchanged forecast re-evaluations preserve clean sealed metrics. The source and aggregate are deliberately separate: correcting an actual must never be blocked by an already compacted analytical projection.
- Summary identity survives its nominal one-year expiry while source rows remain to be purged, so the remaining bounded batches keep their eligibility. Financial reservation release and policy expiry execute first, separately from the analytics lock section, so an analytical failure cannot indefinitely prevent expiry or create financial/source lock inversion.

`pricing_summary_recovery_test.go` covers backdated actuals, empty and aged dirty cohorts, maturation under nonmonotonic source timestamps, unchanged re-evaluation, and sealed-history preservation. `pricing_summary_retention_test.go` covers an active source writer, a 1,001-row partial purge, source correction after compaction, eventual metadata release, an initially excluded cohort, missing forecast evaluation evidence, and financial expiry despite an injected analytical failure. `pricing_forecast_admission_test.go` exercises the real source/feature gate interleaving with sealing, replay, and late unevaluated retention. Existing pre-fix cohorts that may already have lost source rows are conservatively preserved as sealed and stale; lost historical source cannot be reconstructed honestly.

###### 10. Expiry closure could overwrite a concurrently committed purchase outcome

`ClosePricingOffers` selected outcome-NULL IDs in a subquery but did not recheck the current target row after waiting for a purchase transaction. A successful purchase committed during that wait could be overwritten with nonpurchase. The UPDATE now includes an outer `outcome IS NULL` compare-and-set guard. The hosted two-transaction regression in `pricing_worker_state_test.go` waits for the real PostgreSQL lock and verifies the original paid outcome and observation time survive closure.

##### Findings being resolved with the other owners

- Global/model-fallback `ExecutePricingModelAction` reads at most 512 dynamic products, has no continuation, then reports success. Root owns the durable handler boundary; paging is being coordinated.
- Root owns the dataset registry and must expose `dirty`, `sealed`, and `stale` alongside both summary datasets and update `raw/offers` retention metadata from 60 to 90 days before the recovery change is published. The exact integration changes and state semantics have been sent to root.

##### Checked and not a defect

`ApplyCatalogPricing` mutates catalog slice elements passed by value, but the native `Catalog` hydration path allocates fresh combo, addon and included-product slices for each request before `CatalogForUser` calls it. No shared member-cache mutation was demonstrated; no change is needed for that ownership pattern.

##### Verification

No local automated tests have been run, in accordance with repository instructions. Regression sources are for hosted CI. Go formatting completed, and `go vet ./internal/platform/database ./internal/xnode` exited 0 after the final source-admission, recovery, material-change, retention, and independent-expiry regressions were added. The independent Go recovery peer review is clear: its three concrete findings were fixed and covered by meaningful hosted regression sources. Runtime validation remains hosted-only.

Final exact-byte reconciliation: 542 current native files, 236 current transferred native database files, and 120 current pricing files have matching ledger SHA-256 values, with no missing or changed reviewed paths. All 112 transferred pricing baseline files are covered; its ledger contains 121 entries including nine new files and one explicitly retired file. Two retired files have explicit removed entries across the three ledgers. The migration and its peer-owned regression fixtures have their own persistence-owner coverage and are not double-counted here.

### reporting-database-findings.md

#### Reporting database audit

All 31 transferred baseline files were read in full. Six added source/regression files were also reviewed; the ledger records the current SHA-256 for all 37 files. No local automated test suites were executed. Go formatting, database-package vet and diff whitespace checks passed. Hosted regression execution is pending the parent PR checkpoint.

##### R001 — Snapshot JSON exact-number preservation — fixed

`SaveStatisticsPartition` decoded objects into `map[string]any`, coercing numeric tokens to float64 before storage. For example, `9007199254740993` became `9007199254740992`. It now uses `map[string]json.RawMessage`, retaining exact nested integer/decimal values while preserving object validation, the existing payload limit and compact marshaling. `statistics_snapshots_test.go` verifies large signed/unsigned values, nested arrays, decimal precision, timestamp recovery and malformed/non-object rejection.

##### R002 — Exact report timestamp boundaries — fixed

Product/admin reports compared variable-width RFC3339Nano TEXT lexically, which orders fractional seconds before a whole-second Z suffix. Usage breakdown cast retained nanosecond TEXT to PostgreSQL timestamps, which round to microseconds. Reports now reuse the shared nine-digit C-collated `purchaseTimestampOrderSQL`/`purchaseTimestampOrder` comparison, with coarse indexed candidate bounds for half-open admin ranges. Typed native rows use `nativeUsageTimeBounds` to ceil inclusive lower bounds and floor inclusive upper bounds. Receipt, purchase expiry, active-member ordering, lifecycle and closed-period predicates preserve their original precision without rewriting immutable facts.

Added `statistics_time_bounds.go`, `statistics_time_bounds_test.go`, `product_statistics_time_bounds_test.go` and `usage_breakdown_time_bounds_test.go`. Tests cover whole/fractional/nanosecond boundaries, agreement between amounts/distributions, active subscription state, all usage receipt kinds, typed coverage bounds and lifecycle admission.

##### R003 — Wide aggregate averages — fixed

PostgreSQL `SUM(bigint)` can exceed int64 while its average remains representable. Scanning both SUM and COUNT directly into int64 rejected valid retained data and prevented statistics refresh. The average now parses canonical nonnegative big integers, performs exact half-up rounding, and checks the final int64 result. Extended `statistics_activity_cash_test.go` includes wide sums/counts, empty inputs, negative source rejection and an overflowing final rounded average.

##### R004 — Coherent multi-query report snapshots — fixed

Product statistics counted users and buyers separately, so a concurrent membership change could produce a conversion above 100 percent. Catalog/draw totals and distributions also used independent snapshots during cancellation or catalog changes. Product and multi-query Combo/Squad/LuckyDraw reporting now run in read-only repeatable-read transactions. Single-query game/native active-member reads already have one statement snapshot. The obsolete database-size wrapper was removed during the transaction refactor.

`statistics_read_snapshot_test.go` deterministically holds the next relation after a public report establishes its first snapshot, changes membership or cancels a purchase, and verifies that the first response remains consistent while the next sees committed changes. All wait and mutation paths use bounded contexts.

##### Reviewed contracts retained

Current owner/generation/lifecycle checks, immutable purchase terms, independent traffic and receipt aggregates, signed settled draw rewards, exact rational allocation, shared display caps, explicit unknown legacy estimates and retained loss-only closed-period coverage remain intact. PAYG account generation is an admission fence rather than a persisted account column; this was confirmed with the native audit owner. Reports retain their documented live-catalog compatibility semantics where no immutable selection snapshot exists. Database README ownership remains with the persistence agent, who received all six added filenames and updated contracts.

### reporting-pagination-findings.md

#### Administrator timestamp pagination follow-on

Eight transferred source files were fully read, plus two added test/fixture files. All current hashes are recorded in `reporting-pagination-ledger.jsonl`. Database Go vet, formatting and diff checks passed; automated regression execution remains hosted only.

##### P001 — Chronological keyset and capped recent selection — fixed

RFC3339Nano storage uses variable fractional width. Direct TEXT ordering incorrectly puts a whole-second `Z` value after later fractional values in the same second; shortening fractional values also misorders adjacent instants. The five administrator cursor-page methods now normalize both the source timestamp and the validated canonical cursor timestamp through `purchaseTimestampOrderSQL`. Descending `(timestamp,id)` order and exclusive tuple seeks use identical semantics. Existing opaque cursor encoding, filter binding, owner/target scoping, search escaping and row limits remain intact.

Recent user refunds, open administrator operations, owner operations, audit events, users and backups now use the same exact newest-first order. ID ties were added where absent so capped selections are deterministic. Stored financial/audit/backup records are not rewritten.

`admin_timestamp_fixture_test.go` independently seeds whole-second, one-nanosecond, half-second and equal-instant facts. `admin_timestamp_pages_test.go` traverses all five paged APIs one item at a time and checks complete chronological order, tie order, cursor progress and no skipped/repeated rows; six recent projections verify the actual newest records are selected before applying limits.

The related wallet ledger keyset and refund unwind ordering were reported to the persistence owner, who retained responsibility for those separate files. Existing exact active-status predicates in transferred user/entitlement source were reviewed and preserved.


## Resumed review and final integration findings

### resume-root-findings.md

#### Resumed root audit findings

This pass continues PR #147 from 5f064e7892282c592bca76ab692da294e7d9ed3b,
preserving the recovered uncommitted audit work. Earlier exact-hash evidence is
reused only for unchanged bytes. Newly read root modules include request
authentication, connections, Emby, every integration adapter, bot commands,
notifications, outbox payloads and Telegram private-message delivery/recovery.
Other source, infrastructure and reference scopes have separate reviewer ledgers.

##### Global replay-evidence capacity

The HMAC nonce cache bounded each session to 512 live nonces but did not bound
the number of sessions. It could grow without a process-wide limit. The cache now
caps total live nonces at 32768 and refuses new evidence before allocating a
session when full. It never evicts live replay protection to make space. Duplicate
live nonces still return the replay error at capacity. Expired entries reclaim
their exact accounting; cleanup is time-bounded instead of scanning the entire
cache on every rejected request. The unused last-use field was removed.

The new replay_capacity_test.go covers global capacity, no session allocation on
refusal, retained replay evidence, expired replacement and concurrent callers.

##### Connection execution admission and capabilities

StartConnectionScan can authoritatively reject an expired or unavailable owner
and return a terminal receipt without an error. The worker formerly continued
to native/provider execution. It now requires the returned processing state and
captures the admission time after the potentially slow owner lookup. Existing
provider IDs and terminal outcomes never trigger another start. The unblock
worker likewise stops when its item claim returns a different state. Signed
handle validation rejects oversized inputs before parsing and uses bounded
token splitting.

admission_test.go exercises late owner reads, both refusal causes, replay and a
competing terminal unblock claim. Concurrent scan-poll row locking and original
administrator authority propagation were separately coordinated with platform
and service reviewers and are included in the combined findings.

##### Bounded BEPusdt amount parsing

The provider response amount comparator passed untrusted strings directly to
big.Rat.SetString. Short exponent strings could request an enormous integer
expansion. Response decimals now have a 64-byte maximum and a strict decimal
grammar with at most two exponent digits before arbitrary-precision parsing.
Ordinary provider scientific notation remains accepted; rational fractions and
extreme exponent forms are rejected. The request/callback amount contract is
unchanged. amount_response_test.go covers provider response boundaries and exact
normal comparisons.

##### Retired Emby service entrypoints

Whole-code call-site review proved direct Service.Setup and
Service.RetryProvisioning were used only by their own tests. Production setup,
preferences, password changes and reviewed retries already enter through
OperationService, which owns immutable intent, receipt and financial admission.
The redundant entrypoints, interface members and fake methods were removed.
Server-price/password-context/choice validation tests now exercise the shared
preparation that the live durable path uses. The existing legacy provisioning
outbox remains active so accepted historical jobs can recover.

##### Telegram private-message phase admission and repair

A claim update and subsequent item read can observe another process's completed
state. The PM worker now requires processing status before entering a provider
phase and leaves a competing result untouched; the dispatcher reloads the parent
receipt and preserves any unfinished continuation.

Profile repair appended a changing verification timestamp after a profile had
already been truncated to the transport limit. The integration's final truncation
could remove that timestamp, make the edit unchanged, and leave no returned
chat/thread evidence to authorize repair. The probe is now prefixed and bounded
together with the body. worker_boundaries_test.go covers changed claim states
and repair of an overlong escaped profile while retaining the changing probe.

Command targeting was traced across modules: PM routing defers slash commands,
and HTTP validates an addressed bot suffix before command dispatch. No wrong-bot
financial-command bypass was found. Shared formatter whitespace/bold handling
was reviewed and fixed by the small-module reviewer.

##### Documentation and hosted fixture corrections

Notification guidance no longer advertises the retired automatic-reset scanner
or obsolete outage fields. Retained paid-reset completion/refund formats remain
available for historical events. A broken bot-command inventory sentence was
completed; root module architecture and recovery inventories were read.

Hosted checkpoint da319b3a exposed fixture prerequisites that static analysis
cannot execute. Affiliate corruption fixtures now append a malformed version
instead of attempting to mutate immutable historical tiers. Access-repair
fixtures complete onboarding before asserting paid-window and closure behavior;
the lower-level period-restore fixtures retain their separate scope. These
corrections preserve production immutability and eligibility guards, as well as
the original exact-time and retained-accounting assertions.

##### Verification

The immutable checkpoint tree 8724736d4a86054b6bf296e29cf4fccec0d3fd17 passed
whole-repository go vet, go build and static structure/workflow checks. Frontend
generation, typecheck, lint and production build passed. Git transport lacked a
write credential, so the connected GitHub API published the identical verified
tree as commit da319b3a25fbf903ae5be93218f1072cdbc29d55 with an expected-head lease.

Hosted run 43411561457 passed actual desktop/mobile browser recovery, production
image validation, Go quality/security, native CPU profiling and the 256-MiB
application memory soak. Its frontend schema-test and database/native fixture
failures were investigated individually, corrected without weakening business
assertions, and require the next hosted run. Subsequent targeted vet/build and
diff/format checks passed. No local automated suite was run, in accordance with
AGENTS.md. Final merge remains gated on every applicable hosted check for the
exact final head, with source coverage reconciled again after any correction.

### resume-services-findings.md

#### Resumed service and API audit

##### Confirmed fixes

- Payment profile readiness inspected only the first matching account. A disabled or healthy early account hid later enabled broken accounts; a profile-list outage could fall back to disabled legacy settings. Aggregate every enabled account, deduplicate diagnostics, and retain explicit unavailable-profile evidence. Hosted tests cover both ordering cases and storage failures.
- Removed unexported `validateHTTPOrHTTPSURL` and `validateEZPayType`: repository-wide references were only their own tests; all real endpoint validation is HTTPS and payment channels use the billing validator. Removed obsolete connectivity README inventory.
- Global/non-product model recalibration silently processed only 512 products and restarted the same prefix on failure. Existing model-action outbox jobs now checkpoint one target at a time using exact claim generation, attempts and previous payload, process at most 4 per pass, and defer pending compute/resource waits without consuming delivery attempts. Hosted tests cover 513 targets, partial progress and stale claims. No new queue or schema.
- Pricing analytical export descriptors now state 90-day raw offer retention and expose `dirty`, `sealed`, `stale` on both summary datasets, so exports distinguish incomplete, compacted and stale metrics.
- Added missing HTTP test inventory entries responsible for initial hosted `audit:structure` failure.

- Authenticated member/unified subscription adapters discarded the supplied account generation before capturing current native read authority. A stale request starting after account revival could obtain new credentials. Both app adapters and the direct native HTTP subscription handler now check the original ID/generation and active/unblocked state before and after projection, returning zero output on mismatch. Hosted tests cover stale, purged, blocked, revived-current, and generation changes during projection.
- Removed unused duplicate `proxyProvider` statistics entry points after repository-wide reference review; active native statistics retain the shared traffic helper and no queued migration facade remains in that path.
- Telegram Stars refund callbacks accepted a missing charge identifier because the shared validation helper permits omission for pre-checkout. The refund branch now requires an exact, nonempty retained provider charge and paid/refunded state before touching the wallet. Hosted tests prove absent/mismatched proof leaves funds unchanged; existing receipt repair coverage remains.
- Shared administrator error mapping returned raw errors and could disclose private SQL/integration details. It now emits stable public text and distinguishes typed database failures and canceled/deadline/unavailable connections; hosted tests check redaction and statuses.

##### Assessed candidates and coordination

- Telegram commands already validate any addressed bot suffix against cached bot identity before group/PM command dispatch; no parser broadening was needed.
- Raw native access flag release alone does not bypass active administrator bans: independent owner/generation purchase-period validation and restricted-user compiler/export checks remain. No unsupported setter redesign was introduced.
- Administrator connection scan authority review was transferred back to this agent and completed; the confirmed fix and accepted-intent distinction are recorded below.

##### Review progress

All current owned files are covered by exact SHA256 manual-review proof: admin 72, app 125, HTTP 206, plus the 4 transferred pricing implementation/dataset files (407 total, including 150 test files and 3 module documents). This includes tests, module documents and authored fixes. No local automated test suites executed. Focused go vet ./internal/app ./internal/httpapi passed after the final native HTTP authority fence; broader package vet passed previously. Focused build of admin/app/httpapi/pricing/database also passed, and git diff --check is clean. Parent checkpoint 3f8c4311 passed whole-repository vet/build and structure checks. Hosted regression execution remains the parent CI gate.

##### Follow-up cross-module findings and CI repairs

- Confirmed administrator scan authority was lost after HTTP middleware. RequestAdmin/PollAdmin now take the original actor; pre/post result checks require the same generation and current active, unblocked administrator role. Fresh SQL admission locks actor/owner, verifies actor role/generation and atomically adds actor audit attribution with scan/outbox. Hosted tests cover stale authorization before admission, demotion during sensitive IP projection, exact transport propagation, and audit-failure rollback/retry. No schema change. As agreed with parent, previously accepted read-only diagnostic intent continues after requester demotion; all later result reads require current authority.
- Confirmed Emby safeFailure was only truncation, and retry/pending-review persistence accepted raw non-password provider errors. Stable errors now cover those paths while preserving errors.Is classification and refund/review state. HTTP projects safe account-status text for old rows as well, preventing historical URL/token/response diagnostics from reaching members or administrators. Hosted regressions preserve retry, terminal refund and ambiguous-review behavior.
- Repaired hosted CI helper mismatch for coupon_uses, activity_draw_results and activity_raffle_tickets by adding three explicit static COUNT queries to assertRowCount. Malformed coupon/draw tests retain all no-debit/no-consumption/recovery assertions.
- Final follow-up go vet and go build of connections, emby, httpapi and database pass. git diff --check is clean; no local automated suites ran.

- Emby account reads treated any connection/settings or setup-price read failure as configured:false with price zero. The adapter now tags actual invalid pricing with existing ErrInvalidSetup; missing/invalid setup retains the disabled response, while real storage failures return safe 503 EMBY_UNAVAILABLE before upstream work or successful-state publication. Hosted regressions exercise success, connection/price outage and recovery, plus missing/invalid settings and valid free/exact-price setup. Final focused app/httpapi vet/build passes.
- Final reconciliation re-read the six current authentication/onboarding/community files changed by parallel owners; all 407 owned current paths have exact-hash review proof.

##### Subscription integration peer review

Independent review found no gap in the cumulative pre-expansion template allowance: escaped insertion bytes are counted before expanded arrays, and all Xray profiles share an allowance including remarks and separators. Both canonical TLS legs and legacy mapped outputs retain fixed-off verification, with member ownership unable to override that setting.

Confirmed one download-fingerprint inheritance gap: a new or partial download leg inherits an Xray-only primary fingerprint during canonical projection, but the previous omission lookup read only raw child fields and left unsupported inherited values in Clash/Stash. Following parent/platform handoff, endpoint_values.go now resolves that single inherited field through the canonical copied projection. Seven additional hosted scenarios verify raw/typed parent inheritance, explicit child precedence, inactive legs, current security selection, native semantics and source immutability. Final subscription vet/build and diff whitespace checks pass; no local suites ran.

##### Telegram worker and formatting integration peer review

Read the post-checkpoint worker claim guard, repair timestamp prefix and Markdown whitespace truncation changes, their hosted regressions, and the surrounding queue, SQL state transitions, dispatcher completion, profile formatter and Telegram transport. No confirmed actionable flaw found in these reviewed changes. A non-processing item result stops remote work without receipt overwrite; the dispatcher reloads the durable receipt so nil does not acknowledge unfinished work. Repair edits require exact message/chat/thread proof and the changing timestamp prefix survives both limit passes. Markdown truncation preserves complete escape/link tokens and avoids synthesizing a whitespace-only bold entity. Static go vet of telegrampm/telegramformat passed. No source edits or local automated suites.

##### Hosted Stars refund fixture correction

Hosted run 43415231195 for source checkpoint 4feed117 rejected the three new Stars refund proof cases during setup: their request fingerprint was 14 characters, below the existing 16-character provider operation minimum. Corrected only that fixture value. Final source review preserves every missing/mismatched-charge case, the 409 REFUND_MISMATCH response and unchanged 1000-minor wallet balance. Production admission and refund validation remain unchanged. Static go vet ./internal/httpapi and git diff --check passed; hosted execution is required to confirm the corrected fixture. No local automated suites ran.

##### Manual-grant PostgreSQL deadlock peer review

Hosted foundation job 131851459282 exposed a repeated owner SHARE-to-NO-KEY-UPDATE conversion cycle in concurrent manual grants; whole-transaction retries already existed and could exhaust. Independently traced the service/API callers, immutable replay, lock helper callers, balance/outbox/notification boundaries and PostgreSQL connector. Platform fixed fresh grant admission with strongest-required account locks in deterministic ID order while retaining shared distinct-actor authority and unchanged replay/lifecycle validation. Read all final changed source and the controlled concurrency regression: same-owner, self and reciprocal administrator attempts reject 40P01 directly, allow ordinary 40001 before public recovery, preserve exact effects, and leave unrelated owners concurrent. No actionable flaw found. Review was read-only; no local automated suites ran.

### resume-platform-findings.md

#### Resume audit: platform, native control plane, deployment and subscription

##### Scope and exact coverage

Repository: `<repository>`.
Resumed from PR #147 at `5f064e7892282c592bca76ab692da294e7d9ed3b` while preserving all previously uncommitted work. The parent subsequently checkpointed the first batch at `3f8c43113592147f9bb779597761e8dfa71062e4`.

Owned scope comprises `internal/platform/**`, `internal/xnode/**`, `internal/subscription/**`, `deploy/**`, `.github/**`, `scripts/**`, `cmd/**`, Dockerfile, Compose example, Makefile, ignore rules and Go dependency manifests. Pricing action continuation source was coordinated with the API/services owner, who implemented that change and owns the app/dataset integration.

The final coverage reconciliation at the time of this report contains **2,056 extant owned files**: **1,876 reviewed files** and **180 generator-verified transaction adapters**. There are **zero missing or stale exact-hash records**. Every preexisting source proof was carried forward only when SHA-256 matched the current bytes. These counts include source, tests, schemas, configuration and documentation; they are not a claim that all files were newly reread in this resumed session.

All 53 deployment/CI/entrypoint/dependency files and all 68 original subscription files were read completely. Three new subscription files and independent platform/native integration rereads are recorded in `resume-platform-ledger.jsonl`. `resume-platform-coverage.json` records the per-file current hash and proof source for the entire owned scope. The adapter generator itself was read in full and regenerated all 180 wrappers with **zero byte changes**; `platform-generator-check.json` retains that result. Those wrappers remain explicitly classified as generator-verified rather than falsely described as newly manually reviewed source.

##### Confirmed defects fixed

###### Concurrent diagnostic polls could regress a completed scan

`retryAttempt_UpdateConnectionScan` read the row without locking, checked monotonic status/progress against that snapshot, then executed an unconditional update. Two concurrent polls could both pass the check, after which a delayed poll could overwrite a terminal success or newer progress.

The authoritative read now locks the exact scan row with `FOR UPDATE`. Existing provider identity, terminal-state and monotonic-progress checks therefore run after any competing writer commits. The path does not acquire an owner lock after the scan lock, preserving the existing lock-order boundary.

`connection_scan_concurrency_test.go` holds a real competing PostgreSQL update until the delayed poll is waiting, then releases it. The regression covers both newer in-progress evidence and a completed success, including provider ID and completion facts. Hosted execution is required.

###### Displayed dynamic offers beyond 512 settings were silently omitted

`RecordPricingExposures` bounded each displayed response to 64 combos but fetched an arbitrary first 512 enabled product settings across the whole inventory. A legitimate displayed dynamic combo outside that unrelated settings page never produced offer evidence.

The query now selects enabled settings for the actual bounded shown IDs with `ANY($1::text[])`. Work remains bounded by the response's 64-combo limit. The existing per-offer dynamic-setting checks and exposure replay deduplication remain authoritative.

`pricing_exposure_inventory_test.go` creates 513 enabled dynamic products, records bounded displayed batches twice, and asserts one retained offer per shown product without replay duplication. Hosted execution is required.

###### Small templates could allocate large intermediate expanded inventories

The previous 8-MiB checks counted each proxy/name once during preparation and checked the serialized output after expansion. A small administrator template could repeat `SubscriptionNames` thousands of times. A one-endpoint save preview would pass, while a large live owner inventory could allocate a large expanded tree before the final byte check.

`template_budget.go` accounts for every insertion using serialized value sizes before allocating expanded arrays. The estimate includes JSON escaping and conservatively retains original marker bytes. All selectable Xray profiles share the same allowance. Existing marker grammar, depth, endpoint count, intermediate document and final serialization limits remain.

`template_budget_test.go` covers repeated escaped names, a valid small preview that amplifies with a larger live inventory, and cumulative Xray profiles. The regression asserts the preflight error; it does not claim a measured production memory bound or a local execution result.

###### Download TLS fingerprint selection was permanently invalid

The generated catalog declares `downloadFingerprint` as an enum whose choices are populated dynamically. The population switch only handled the main `fingerprint` key, so every explicit download fingerprint failed save validation. Format compatibility also checked only the main leg.

Both keys now receive the current pinned/native and independent-client choice set. Per-client download fingerprint compatibility mirrors the main leg, allowing supported values and omitting unsupported optional choices while preserving usable peers and saved administrator intent.

`endpoint_fingerprint_test.go` covers a populated field catalog, pinned native stream validation, structured native URI preservation, unknown-value rejection and optional per-client download omission. Hosted execution is required.

###### Legacy client mappers could bypass endpoint secure TLS policy

Endpoint projection correctly changed inherited `allowInsecure:true` to the documented secure false policy. For sing-box, Mihomo and Stash, legacy format mappers ran afterward and could restore `tls.insecure` or `skip-cert-verify`, including on the independent XHTTP download leg. Diagnostics and the actual client document could disagree.

The endpoint renderer now applies the same fixed-off verification policy after format mapping and typed administrator projection. The separate member allowlist cannot override TLS verification. Only the ephemeral exported client document changes; saved encrypted source and mapper documents remain intact.

The extended `endpoint_tls_policy_test.go` covers all four document formats and both YAML legs, including source mutation isolation. Hosted execution is required.

###### Deployment sample and command inventory retained retired references

The root Compose example still referenced the former `txyyddss/remna-user-panel` publication, while the release workflow publishes the current repository's lower-case GHCR path. The sample now defaults to `ghcr.io/vansail-opensource/vansail-carpool:latest`, with an explicit `VANSAIL_IMAGE` override. Existing volume/project identities were preserved.

The command README contained broken fragments linking retired `dbinventory` and `datamigrate` packages. It now describes the actual server and `db-schema migrate|verify|fingerprint` entrypoints, including the original master key and quiesced writer requirement for comparing recovery evidence. Missing database renewal shard and forecast admission inventory entries were added.

The legacy native `Service.RoutingTemplates` helper was also retired. It returned raw credential-bearing outbound maps, but whole-code call-site review found no mounted handler or production consumer; only an old duplicate ACL assertion invoked it. Actual member routing uses typed `OwnedRoutingPlan` metadata. The existing ACL regression remains against live `MemberProxies` inventory and still covers owned/foreign allow and exclude modes plus invalid policy rejection.

##### Recovery and pricing integration review

The pending migration `00050_pricing_summary_recovery.sql` was read completely together with the calibration/evaluation publication, admission locks, forecast actuals, retention sealing and upgrade regression sources. It conservatively labels summaries whose raw history may already have been compacted as sealed/stale; it does not manufacture fresh metrics from incomplete history or modify financial/source facts. The database guard prevents sealed metric/scope/watermark replacement, unsealing and stale clearing. Material source changes invalidate currentness, while no-op/conflict replay preserves it. Financial expiry and override cleanup remain outside the optional summary lock gate.

The API/services owner's pricing action continuation was independently reviewed. It retains a sorted `(product_type, product_id)` cursor in the existing outbox payload, checkpoints each completed target using current status/attempt/claim-generation/previous-payload fencing, processes at most four targets per pass and returns the existing wait continuation for pending computation or resource pressure. `CompleteOutboxJob` fences attempt and generation without comparing the old payload, so cursor checkpointing is compatible with completion. No additional queue or schema was introduced. The API owner also updated dataset retention/state exports and the full-job app handoff.

The original authenticated subscription snapshot gap was confirmed and sent to the API/services owner: native `ReadSubscription` anchors lifecycle state at the start of its own read, but the app adapters previously discarded the authenticated caller's `AccountGeneration`. The API owner added pre/post current-owner checks bound to that original authenticated snapshot. The native fence continues to protect identity/credential/period and owner state across retries during the read. That app change is separately owned and evidenced by its author.

##### Deployment, dependency and supply-chain conclusions

The reviewed workflows pin external actions to commits, keep normal CI read-only, and scope publishing permissions to release jobs. The Dependabot `pull_request_target` flow does not check out pull-request code and revalidates author, draft/state, live head and checks before a match-head merge. Change-selection scripts fail closed when trustworthy history is unavailable. Race manifests partition the discovered test/package inventory rather than relying on an incomplete fixed package list.

Embedded startup uses the unprivileged runtime user, protected persistent data/credential paths and private PostgreSQL sockets. The reviewed recovery workflow and runbooks restore into isolated inactive targets, preserve the original encryption master key, compare retained table/sequence fingerprints and financial invariants, and prohibit overwriting the only surviving live database. These are source-level findings; no local Docker restore exercise was run in this session.

`go.mod` has no live SQLite persistence backend. Legacy SQLite-related `go.sum` entries belong to transitive module/test metadata; deleting them merely because of their names would not retire runtime code. Dependency checksum verification succeeded.

##### Validation and precise limits

- Transaction adapter regeneration: all 180 existing adapters unchanged.
- `go vet ./internal/platform/... ./internal/xnode`: passed.
- `go vet ./internal/platform/database ./internal/app ./internal/httpapi`: passed after correcting the new scan fixture's float64 progress field.
- `go mod verify`: passed (`all modules verified`).
- `go vet ./internal/subscription ./internal/xnode ./internal/platform/...`: passed with the new subscription regressions.
- `go vet ./internal/xnode ./internal/platform/database ./internal/app ./internal/httpapi ./internal/subscription`: passed after legacy routing-helper retirement and current cross-owner integration edits.
- `git diff --check`: passed.
- Shell syntax checking for all embedded startup, PostgreSQL initialization/verification and hosted recovery shell scripts: passed.
- No local automated test suite was run, per `AGENTS.md`; hosted tests and the parent's final CI/merge gates remain required.

The generic retired-detector release flag setter was examined after an API-owner concern. Its `false` write is not itself bound to the earlier manual-ban observation, but active manual bans independently block every native compiler/export path, and authoritative purchase activation/restoration checks the current ban and owner generation. No access bypass was established from that raw flag race. It was reported back as an unconfirmed candidate rather than presented as a fixed or exploitable defect. Non-Xray client version/platform compatibility remains limited to the independent documented adapters and hosted structural tests, not execution in every client runtime.

##### Hosted checkpoint corrections

Three hosted failures were reconciled with authoritative contracts: the profile fixture now sums two principals to MaxInt64 (a larger single-node report is intentionally invalid); the provisioning fixture completes onboarding while active before closing its owner; and combo replacement normalizes omitted add-ons to an explicit empty JSON array, asserted by the paid-window regression. The live service already normalizes the slice, so the latter is a persistence-boundary robustness fix, not a demonstrated HTTP exploit.

##### Recovery runbook follow-up: volatile traffic and durable journals

The PostgreSQL runbook formerly said to keep agent journals durable and reconcile unacknowledged journals without distinguishing their durability. The pinned agent at `41f04f945454` keeps new `agent:boot:sequence` traffic reports solely in bounded process memory; only pre-upgrade `agent:sequence` traffic journals are persisted and drained on upgrade. Durable utility command receipts and hourly byte-credit obligations are separate from that raw traffic stream. `traffic.go` ACK retirement removes current queue heads without filesystem writes and persists only legacy retirement. `traffic_memory.go` creates a fresh stream and empty queue at process start.

Narrowly corrected `deploy/postgres/recovery.md`: preserve running agent processes and state directories, avoid restarting/upgrading/re-enrolling agents before reconciliation, use original identities and restored cursors for authenticated commit-before-ACK replay, and retain separate durable obligations. Documented that process restart or buffer pressure may lose uncommitted samples, while an older database restore may lack already acknowledged reports retired on the agent. Preserving failed database/WAL and gap/loss evidence is necessary; no missing bytes are invented and no settled financial facts are rewritten. The existing pinned policy can underbill or refund more after traffic loss. No runtime, permission, testing, or settlement policy changed.

Validation: complete runbook/source contract review, correct relative Markdown section link, and `git diff --check -- deploy/postgres/recovery.md` pass. No automated suite was run. Final SHA-256: `6784c4102933a22b169b50c938004e1c1aacd3d877628442408733a981d98aba`. Exact proof appended to `resume-platform-ledger.jsonl` and coverage entry refreshed.

##### Hosted concurrency follow-up: manual entitlement grant lock conversion

Foundation job `131851459282` in run `43415231195` failed `TestConcurrentManualGrantReplaysOneOriginalResult` after 126.34 seconds with PostgreSQL `40P01`. The hosted server trace shows competing `SELECT account_generation FROM users WHERE id=$1 FOR NO KEY UPDATE` statements waiting on one another. Complete source review confirmed that each fresh grant first acquired actor `FOR SHARE` and actor/owner `FOR SHARE`, then attempted the incompatible owner upgrade. Eight concurrent callers repeatedly formed the same conversion cycle until the existing bounded transaction retry exhausted; retry ownership itself was already correct.

The targeted helper now takes the distinct actor shared lock and owner serialization lock in stable ID order before the shared lifecycle check. A self-grant obtains the owner lock before actor validation, and reciprocal administrator grants use the same order. Actor eligibility and active-owner checks, immutable accepted replay, fingerprints, overlap prevention, zero-charge semantics, notifications and audit effects are unchanged. The generic owner helper, other modules and retry budget remain unchanged.

Added a controlled PostgreSQL contention regression for the same owner, self-grants and reciprocal administrators. A separate transaction holds shared account locks until both attempts are visibly waiting; single attempts must not return `40P01`, while expected serializable `40001` may use the unchanged public retry path. Exact replay results and one purchase/audit/sync job per accepted owner remain asserted. An additional blocked-owner scenario requires an unrelated owner under the same actor to complete, guarding against an unnecessary exclusive actor lock. The original eight-request concurrent replay regression is untouched.

Go1.27.4 `go vet ./internal/platform/database`, `go build ./internal/platform/database`, and targeted `git diff --check` passed. No local automated suite ran; hosted execution remains pending. Final hashes are recorded in `manual-grant-lock-order-evidence.json` and the appended platform ledger records.

Independent API/services peer review of the final helper and controlled contention regressions passed without actionable issues. Source hashes are unchanged from the recorded static validation.

### resume-frontend-findings.md

#### Resumed frontend and API audit

##### Scope and proof

Repository: `VanSail-Opensource/VanSail-Carpool`, continuing PR #147 at
`5f064e7892282c592bca76ab692da294e7d9ed3b` with the recovered dirty worktree preserved.

The current owned inventory contains **1,477 files: 1,189 under `web/` and 288 under
`api/`**. The exact-byte reconciliation records 312 current reviews (full reads or
complete changed-byte reviews over verified whole-file baselines) and 1,165 files retaining an exact SHA-256 match to the earlier
audit proof. There are **zero unreviewed or stale owned files** at reconciliation.
The inherited records retain their original scope and rationale; they are not
represented as new manual reads. See `resume-frontend-coverage-summary.json`,
`resume-frontend-manifest.json`, and `resume-frontend-ledger.jsonl`.

All 288 API files were read completely, including the 285 previously unreviewed
files and three authentication/onboarding contracts reread at reconciliation. The root registry,
parameter and response definitions, every schema shard, every endpoint shard and
their documentation were checked for authentication boundaries, input/output
alignment, immutable evidence, retired entrance behavior, idempotency and
recovery semantics. Existing frontend fixes F-WEB-001 through F-WEB-023 remain
preserved in the previous frontend findings document and their original hashes.

##### F-API-RESUME-001 — Valid financial ledger kinds excluded from generated contracts

**Severity:** Medium correctness and recovery defect.

`LedgerKind` did not list 17 categories that active financial writers emit or
that retained historical evidence still contains. These include add-on debits,
administrator/member refunds, Telegram deductions, group-message rewards, raffle
fee/reservation/refund settlements, affiliate rewards, IP lookup/refresh/refund
entries and historical traffic reset charges/compensations.

The incomplete enum propagated into both the generated TypeScript `LedgerEntry`
type and the generated ledger response validator. An otherwise valid ledger
page with any of these entries failed optional cached-response compatibility
and was discarded before restore. Live responses remain authoritative; this
finding does not assert that the live accounting transaction was rejected.

**Evidence:** Read the financial `insertLedgerTx` writer call sites and dynamic
kind selection in the database package, including `billing_purchase_addons.go`,
`admin_entitlement_refund.go`, `member_refund_commit.go`,
`member_reset_compensation.go`, `retention_operation_compensation.go`,
`notification_admin_finance.go`, the raffle/affiliate settlement writers and
`ip_lookup_commands.go`/`ip_lookup_refunds.go`. Read the response generator,
validator and cache restore consumers. The existing `questionnaire_reward`
category is intentionally retained rather than removed as retired code.

**Fix:** Expanded the authoritative enum and regenerated `generated.ts` and
`responseSchemas.json`. Complete structural/token comparisons with the prior
reviewed generated artifacts show only the intended enum expansion. Updated
schema and frontend-contract documentation.

**Regression:** `web/src/api/contracts/responses.test.ts` now accepts every newly
declared category and retained questionnaire evidence through the real
`GET /api/v1/ledger` route, then rejects an unknown category. The negative case
also protects against accidentally testing an unknown route whose compatibility
check deliberately defaults to true. Test execution is delegated to hosted CI.

##### Existing CI failure diagnosis

Fetched the frontend quality logs for job `131727155476`, workflow run
`43377417373`. Its failure was the repository structure audit requiring README
entries for files already included at `5f064e7`: frontend identity/Emby/Telegram
back-button tests and backend backup/authentication/onboarding/native boundary
tests. Existing recovered documentation changes already address these entries.

##### F-API-RESUME-002 — Schema test helper weakened strict composition

**Severity:** Medium regression-gate correctness; the OpenAPI contract itself was correct.

Hosted job `131839125206` in workflow run `43411561457` failed the PM moderation
input-boundary assertion. Zod 4.6.7 converts `allOf` to object intersections;
its intersection implementation filters unknown-key issues unless both branches
reject that key. A strict base combined with a passthrough `anyOf` branch therefore
accepted `{blocked: true, unexpected: true}` in the test helper.

The bounded helper now evaluates each `allOf`, `anyOf`, and `oneOf` branch against
the original input independently. Base `additionalProperties: false` remains
strict; both valid booleans are allowed and at least one is required. Added
malformed-sibling cases and assertion context. Existing bulk-duration and
callback composition assertions now use the same helper. No runtime contract
or generated type was weakened to satisfy the test.

##### F-WEB-RESUME-001 — PM parent reload discarded unresolved command ownership

**Severity:** Medium recovery and operator-feedback defect.

The earlier `AdminUserPM.vue` proof retained an unresolved candidate concerning
receipt reset. The complete follow-up traced `useDurableCommand`,
`useOperationReceipt`, browser-owned receipt loading, the actual profile parent,
and durable PM moderation/repair storage. The explicit reset of an accepted
pending-review receipt is intentional: active queued/processing work is blocked,
operator repair remains available, and durable server evidence is preserved.
This candidate is now closed with a final ledger entry, not merely inherited.

That trace exposed a separate real defect: `moderate()` emitted `changed` after
every attempt. The parent performed a nonquiet load that removed the profile
and unmounted the command owner. An ambiguous failed request lost its retained
idempotency key and error, and an accepted queued request lost receipt polling.

The fix emits no reload on failed submission. Accepted active moderation and
terminal outcomes request a quiet parent refresh, preserving immediate committed
flag updates and the mounted receipt owner. A failed quiet read retains the
last-good profile while displaying the read error; ordinary profile mutations
remain blocked until read recovery. Terminal outcomes are emitted once.

`AdminUserPM.test.ts` covers ambiguous retries, accepted receipt polling and
explicit pending-review repair using the real command composable.
`useAdminUserProfile.test.ts` covers failed quiet reload and recovery. The new
`PMRecoveryAudit.vue` / `browser-pm-recovery.mjs` exercise desktop/mobile lost
response, exact key reuse, retained failed profile refresh, accepted flags and
terminal release in hosted Chromium. The fixture transport is synthetic.

##### F-WEB-RESUME-002/003 — Transfer evidence layout and empty-state clipping

**Severity:** Low operator-interface correctness.

Visual inspection of the first checkpoint's actual hosted screenshots showed
concatenated amount/network/date evidence in the side panel, and a clipped empty
message after dismissal of the last transfer on mobile. Production-source tracing
confirmed that evidence CSS required the `.usdt-admin` ancestor even though the
slideover teleports outside it; the empty message also lived inside a table whose
horizontal scroll position survived row removal.

`USDTReviewItem.vue` now owns its responsive evidence/form styles. Obsolete
ancestor-dependent evidence rules were removed. `USDTReview.vue` renders its
localized empty state outside the table overflow. The existing hosted browser
scenario retains failure/retry screenshots and now verifies actual toolbar layout
and empty-message viewport bounds. No transfer, credit or dismissal semantics changed.

##### Static validation

- `npm run generate:api`: passed; generated 202 response shape contracts.
- `npm run typecheck`: passed.
- `npm run lint`: passed with zero warnings from ESLint.
- `npm run audit:structure`: final pass, including localization, icon inventory,
  documentation inventory and workflow policy checks.
- `npm run build`: final pass (3,852 modules; existing chart/map/ELK chunk-size warnings only).
- No local automated test suites were run, following `AGENTS.md` section 10.

Hosted Chromium job `131839125015` / run `43411561457` validated the first
checkpoint `da319b3a`: 142 passing scenarios, zero recorded console/page/request
issues and 312 screenshots. The platform reviewer downloaded artifact
`14028974886` (SHA-256 `e9fcb41130f2a9f99d03e56955542f9ea0f879ff3487566fbfa2c427e53bc714`)
to `browser/hosted-da319b3a/` and inspected eight states; this reviewer inspected
the two transfer-review screenshots and traced the confirmed layout defects above.
The expanded 144-scenario run `43415231195` at `4feed117` passed all 142 existing
scenarios, including both stablecoin layout assertions. Artifact `14031488647`
was downloaded and its SHA-256 verified. Eight actual desktop/mobile PNGs were
inspected: stablecoin failure and empty completion, plus PM lost-response and
held-receipt/read-error states. The stablecoin layout corrections are visible.
Both new PM scenarios stopped on a raw count of one info notice while Queued and
Processing coexist during the keyed animation. The harness now waits for the
processing label and departure of the outgoing notice while retaining every
replay-key, single-mutation, flag and terminal assertion. The screenshots also
exposed a fixture-only `common.retry` key, corrected to existing `common.tryAgain`
in both PM fixture shells. The later PM assertions need the next hosted run.
Full details: `browser/hosted-4feed117-review.md`.

No successful local browser result is claimed: shared-Chrome loopback was
isolated, the narrow server escalation was automatically rejected, and ordinary
local Chromium was blocked by OS socket permissions. No local automated suite
was run.

##### Limitations

Hosted regressions, integrated browser fixtures, final branch verification and
merge are parent-owned. Static review and exact file coverage do not establish
absence of every possible defect. Uncertain provider behavior remains guarded
by the repository's durable-operation/reconciliation design rather than being
declared safe by this audit.

### resume-iplookup-findings.md

#### Resumed IP lookup audit

##### Coverage and proof

Transferred ownership: every file under `internal/iplookup/`, continuing PR #147.
All 97 files present at transfer were read completely, including Go source,
regressions and the directory guide. Four newly authored regression files and
every changed source byte were then reread. The current inventory is **101 files:
63 source, 37 test, one documentation file**, totaling 303,937 bytes and 8,825
lines. Exact SHA-256 reconciliation has **zero missing or stale files**.

Proof: `resume-iplookup-manifest.json`, `resume-iplookup-ledger.jsonl`, and
`resume-iplookup-coverage-summary.json`. The ledger records actual reads, not
inferences from filenames or grep matches. Related persistence files were read
to trace durable snapshot status/retry boundaries; their source ownership remains
with the parent/platform reviewer.

##### F-IP-RESUME-001 — Invalid traffic observations suppressed repair

**Severity:** Medium correctness/recovery.

`trafficSummary` marked malformed recognized measurements partial, but then
replaced that state with empty when none of the measurements was usable.
Missing/null/wrong-shaped `summary_0` documents also became empty. `LivePartial`
and `liveRetryPlanOf` treat empty as a valid observed result, so this lost
coverage could be stored as succeeded and excluded from automatic repair.

**Fix:** Reject missing or non-object summaries as unavailable, preserve partial
when every recognized value is malformed, keep valid sibling observations, and
only classify an actual empty successful summary as empty. Existing legitimate
zero values remain valid. No suitability or financial verdict is changed.

**Regression:** `live_traffic_test.go` covers malformed, mixed, missing, null,
wrong-shaped, empty and zero responses and checks both snapshot classification
and retry planning.

##### F-IP-RESUME-002 — Null prefix fallback asserted no announcement

**Severity:** Medium data correctness.

Go accepts JSON null into a nil string slice without returning a decoding error.
The RouteViews fallback counted that slice as zero prefixes and, after both
families, could publish `announced=false` although no count was observed.

**Fix:** Require a non-nil decoded list before recording prefix counts or source
success. An explicit empty array still records observed zero; unavailable data
remains unknown and the existing partial primary-source coverage remains intact.

**Regression:** `live_asn_test.go` covers null, object, empty-array and actual
IPv4/IPv6 prefix results through the queued provider transport and ASN projection.

##### F-IP-RESUME-003 — Completion events replaced degraded tasks with success

**Severity:** Medium status correctness.

`liveComplete` unconditionally emitted success for every phase for both fresh
and cached results. In addition, ASN task projection ignored source partial
states and traffic failures. This contradicted the source availability encoded
in the same response. The member UI currently drops task details after final
settlement; the defect is the streaming contract, not a claimed persistent UI
failure.

**Fix:** Derive completion tasks from final typed sections through the existing
task projection, preserving error codes, partial coverage and skipped optional
sources. Completion still reports the full completed count. ASN aggregation now
includes partial metadata and traffic degradation without treating an omitted
optional credential as a failure. A missing legacy transfer section is skipped.

**Regression:** `live_events_test.go` exercises fresh then persisted-cache
completion with a failing Shodan source, malformed traffic, missing optional
credential, observed-empty PTR and empty transfer history. It also covers
partial/all-failed/mixed ASN metadata and optional/failed traffic aggregation.

##### F-IP-RESUME-004 — Damaged retained graph could panic a repair worker

**Severity:** Medium recovery robustness.

Stored projection-version-2 route summaries were trusted after JSON decoding.
`relationshipTargets` and `pruneTransitContext` indexed Hops by ASN position
without validating vector lengths. A malformed retained document, such as two
ASNs and no hop entries, could panic during a graph repair. This finding requires
malformed stored state; the normal bounded live writer constructs valid vectors.

**Fix:** Validate route vectors at the compact public/persistence boundary and
repair traversal boundaries. Keep valid evidence, omit malformed vectors, retain
existing source errors, and mark lost coverage partial/truncated with a stable
error code so topology repair remains eligible. Hop totals obey the existing
128-ASN observation bound and cannot overflow. The original passed snapshot is
not mutated; its financial result and immutable IP identity are untouched.

**Regression:** `live_projection_recovery_test.go` covers missing/extra/zero/
negative/overflowing hops and zero ASNs, preserving valid siblings, idempotent
projection, direct repair consumers and recovery eligibility.

##### F-IP-RESUME-005 — Confirmed unused bookkeeping and unreachable legacy code

**Severity:** Low maintainability.

`liveRequests.failed` had only writes and no readers anywhere in the package.
Its extra lock and stale aggregate were removed; per-request errors and durable
attempt evidence remain unchanged. `compactLegacyTopology` contained a path
processing loop even though its sole caller routes every nonempty path list to
the current projection first. That unreachable loop and its now-unused tier map
were removed. Stored legacy links and their deterministic ordering remain.

No new mirror tests were added for these inert removals; existing projection and
shared-request regressions remain the hosted behavior gate.

##### Other boundaries reviewed

- Owner-bound expiring quote signatures, request admission, queue capacity,
  fixed destinations, redirect denial and provider response size bounds.
- Durable attempt markers, interrupted operation handling, fresh execution
  leases, heartbeat cancellation, fenced writes and frozen repair revisions.
- Exact-IP cache identity, paid refresh separation, shared-flight cancellation,
  immutable stream snapshots, section recovery and credential redaction.
- Dated classification evidence, observed-versus-inferred relationships, bounded
  graph projections, fallback metadata, historical report compaction and ratings.
- Retired inventory/active scan/subnet behavior stays retired; terminal financial
  evidence remains historical rather than recomputed by this audit.

##### Validation and remaining integration

- `gofmt` applied to changed Go files.
- `go vet ./internal/iplookup`: passed with the retained Go toolchain.
- `go build ./internal/iplookup`: passed.
- `git diff --check`: passed.
- All new files are described in the directory README.
- Local automated test suites were **not run**, as required by `AGENTS.md`.
- Hosted regression execution and final branch checks are parent-owned.

The first checkpoint's hosted Chromium job `131839125015` / run `43411561457`
passed all 142 scenarios with no recorded console/page/request issues. Its 312
screenshots were retrieved, and the platform reviewer inspected the retained IP
snapshot/owner-replacement states. The source checkpoint `4feed117` now runs the
expanded hosted suite under `43415231195`; current backend regressions and final
browser review await that run. No successful local browser result is claimed:
shared-Chrome loopback was isolated, the narrow server escalation was
automatically rejected, and ordinary local Chromium was blocked by OS socket
permissions. Local automated suites were not run.

### resume-small-modules-findings.md

#### Resumed small-module audit

##### Scope and coverage

Owned modules: `currencydisplay`, `maintenance`, `onboarding`, `preferences`,
`routingmatch`, `squadprofile`, `statistics`, `telegramformat`, `usageaccounting`,
`validation`, and `webui` under `internal/`.

Every one of the 58 original files was read completely. The newly added
statistics regression and every modified file were then read in full at their
final hashes. Current inventory: **59 files, 131,967 bytes, 3,912 lines**,
comprising 32 source files, 15 test files, 11 directory guides and one explicit
non-executable embed placeholder. Exact SHA-256 reconciliation has **zero
missing or stale files**. There were no inherited-proof skips in this scope.

Proof is retained in `resume-small-modules-manifest.json`,
`resume-small-modules-ledger.jsonl`, and
`resume-small-modules-coverage-summary.json`.

##### F-SMALL-RESUME-001 — JSON null looked like a fresh zero statistics partition

**Severity:** Medium data availability and recovery correctness.

`decodePartition` accepted JSON `null` into a zero-valued Go struct. Hydration
then assigned the persisted timestamp to that empty value, making unavailable
cache evidence appear to be an available zero snapshot. A valid sibling
partition did not distinguish this case from an observed empty aggregate.

The decoder now rejects a whitespace-delimited JSON null document as unavailable
before unmarshalling. Independent partition hydration keeps the healthy sibling;
the next authoritative refresh can repair the rejected partition normally.
Malformed payload handling and the existing generation guard against delayed
initial reads remain intact.

`null_partition_test.go` checks both remote and database null payloads, missing
freshness on the invalid partition, retained sibling freshness, a single stale
marker and successful authoritative recovery. The existing corrupt-cache fixture
accepts an explicit corruption payload for this regression. Tests await hosted CI.

##### F-SMALL-RESUME-002 — Truncation could create an empty Telegram bold entity

**Severity:** Medium delivery/recovery robustness at the message boundary.

`telegramformat.Limit` counted whitespace as content in an open bold span. If a
long dynamic label began with enough whitespace to reach the cutoff, the final
`TrimSpace` removed the span's apparent content and appending the closing marker
produced `**`, which is invalid MarkdownV2 markup. This could force fallback or
prevent a delivery/recovery message from using its intended formatting.

Bold content tracking now requires a non-whitespace rune. A cutoff inside a
whitespace-only prefix returns the last complete safe token rather than creating
an empty entity. Existing escape-pair, mention and normal-bold truncation rules
are preserved. The confirmed unused `trailingBackslashes` helper was removed
after package-wide reference tracing; no live caller depended on it.

`markdown_test.go` adds spaces, tabs, line breaks and ideographic spaces, checking
the expected safe result and retained length bound. The parent independently
fixed the PM repair worker's appended verification-timestamp overflow; this
shared formatter change remains compatible with that repair.

##### Other reviewed boundaries

- Currency conversion uses exact integer arithmetic, clamps unavailable rates
  through the existing callers and preserves display-only behavior.
- Maintenance keeps durable claims and backup gating; both real callers already
  apply the 45-minute execution deadline. No speculative timeout rewrite was made.
- Onboarding content remains locale-paired, bounded and normalized; retired
  duration overrides do not re-enter the publication contract.
- Notification preferences preserve historical delivery categories and fail
  closed for unknown policy values.
- Routing selectors are bounded; port parsing sorts and merges overlapping
  ranges before intersection, resolving the suspected interval-growth issue.
  Existing regex, graph, expansion and traversal budgets remain authoritative.
- Squad profiles retain typed discriminated normalization and existing country,
  feature-count and text limits without calling a provider directly.
- Usage/refund calculations retain exact proportions and saturation; no billing
  cadence or immutable accounting policy was changed speculatively.
- Request validation retains path/query/header, JSON depth and text boundaries.
  Embedded UI preload copies its bytes; returned filesystem metadata exposes no
  mutable backing data through `Sys`.

##### Validation

`gofmt` was applied to changed Go files. `go vet` and `go build` passed for all
11 owned modules with the retained Go toolchain. The later frontend build also
completed against the embedded UI destination. The code and regressions were
reviewed statically; local automated test suites were not run under `AGENTS.md`
section 10. Hosted regression execution, final branch checks and merge are
parent-owned. No unresolved source-review candidate remains in this scope.

### resume-reference-findings.md

#### Reference, documentation and copied guidance audit

Reviewed on 2026-11-13. This is the child audit handoff for PR #147; the root audit manifest and final hosted checks determine the delivery head. No local automated suite, application deployment, live provider mutation, node upgrade or payment call was performed.

##### Exact scope and coverage

`resume-reference-coverage.json` covers every current file under `.agents/`, `.codex/`, `.dsh/`, `docs/` excluding the root-owned `docs/audits/` folder, `reference/`, and root `README.md` / `AGENTS.md`.

- 169 current files, 169 matching SHA-256 evidence records, zero missing or stale hashes.
- 139 files were completely read, including all operating/recovery documentation, all archived PHP code, all configuration, and 51 copied guidance/configuration files.
- 30 files have an explicit structural classification: 18 inert SVG assets, one complete Emby OpenAPI parse/contract review, and 11 design-oriented guidance documents. For those 11 guides, all headings, every fenced code/configuration/command block, and every inline code-bearing line were inspected. Their remaining aesthetic prose was classified rather than hand-read line by line. This classification does not count them as manually reviewed production source.
- Every executable snippet/configuration found in the owned tree was inspected. The 10 retired PHP examples remain `.php.txt` archives, excluded from the production image; they are not live payment handlers.
- `resume-reference-ledger.jsonl` contains the per-file hash, classification, status, rationale and evidence. It contains 170 versioned records for 169 unique paths.

##### Confirmed operating-guidance corrections

###### PostgreSQL startup and recovery

Root README incorrectly pointed to a missing `.env.example`, omitted the external PostgreSQL connection/migration-role requirement, and described persistent application storage without the embedded database volume. It now directs operators to a protected environment file and the existing PostgreSQL runbooks, and identifies both required embedded volumes. Version statements follow the actual module/package manifests.

The current web-module documentation still advertised a browser `RESTORE <filename>` upload, staged file swap and application restart workflow. That workflow is retired. The text now describes authenticated PostgreSQL archive management and offline restoration into a separate volume with workers and the master stopped, linked to `deploy/postgres/recovery.md`.

No startup or recovery script was changed in this documentation pass. The runtime/entrypoint/recovery implementation was already reviewed in the platform audit.

###### Native ownership and account lifecycle

The onboarding guide still described reserved service usernames, a remote-provider 404 proof and automatic refunds/restarted usernames. Current agreement onboarding instead uses verified Telegram identity and the existing queued native provisioning path. The corrected guide preserves paid and identity facts on conflicts, distinguishes historical compatibility helpers, and links to the actual account lifecycle.

The administration guide incorrectly said node assignments were never persisted locally. Native X-Node owns durable groups and node bindings; merchandising metadata is not authoritative membership. The corrected description reflects that ownership.

The native commerce delivery record now distinguishes retired rollover-credit behavior from retained historical settlement evidence. The module index removes nonexistent documents and points native integration ownership to `internal/xnode/README.md`.

###### Dependency and release evidence

The X-Node reference README was tied to an older source revision despite the current module pin `v1.0.2-0.20261112141734-41f04f945454`. The selected module's origin is `41f04f945454282e3bfd6dde7e638bd29cfb9b0f`. All four copied public documents were compared to that exact cached module; `certificates.md` and `compute-pool.md` were already identical, while `protocol.md` and `hourly-account-limits.md` were refreshed byte for byte.

The protocol correction is operationally material: uncommitted traffic is a volatile boot-local stream, not restart-durable traffic. Committed accounting and gap evidence remain separately durable. The hourly reference adds the pinned core's static compiled hourly-format and rollback compatibility requirements.

Historical v2.0.25 signed-artifact verification remains explicitly historical. A Go source pin does not prove that a deployed or newly published binary has been verified. The security and integration documentation now makes the same distinction.

The Xray source-availability notice now points to the selected source `a4a3dff5b20c2faae567b742c172c7b95e75c083`; the retained license text exactly matches the selected module's LICENSE. No new licensing interpretation is asserted.

###### Historical records

Nine older progress/validation documents now begin with an explicit historical notice: combined refinement, Explore/member validation, panel improvements, refinement, BAPO 5.1, compute browser validation, compute progress, hourly redesign, and PAYG/DIY/reporting/DDNS progress. Recorded commit IDs, measurements, evidence and pending checks remain intact; they are not silently rewritten to claim current success.

The incremental Xray document labels its v2.0.12 baseline as historical. The dated 2026-10-18 incident retains its actual temporary mitigation and diagnostic commands, while making clear that those commands are not the current fleet runbook or authorization to restart a node.

The tooling document's Linux MCP example now uses `npx` arguments directly instead of incorrectly reusing Windows `cmd /c` wrapper arguments.

##### Reference structural review

###### Emby OpenAPI

`emby-reference-structure.json` records a complete JSON parse with duplicate-key detection and traversal of every value/reference. The imported document declares OpenAPI 3.0.1 and API version 4.1.1.0, with 356 paths, 445 operations, 201 schemas and 2,611 reference occurrences (206 unique). It has no unresolved or external references and no duplicate keys. Maximum structural depth is 10.

The consumed user creation/list/get/delete, password, policy, selectable-media-folder and parental-rating contracts and their relevant schemas were manually inspected against the maintained Emby adapter. The document's historical sample server is not an authorized integration target. New `reference/Upstream/Emby/README.md` records the available version information, explicitly unknown original retrieval provenance, supported adapter boundary and reference-only status. No claim is made that all repeated schema prose was hand-read.

###### SVG assets

`svg-reference-structure.json` records full XML parsing and attribute/CSS inspection for all 18 SVGs. No script, event-handler attribute, foreignObject, iframe/object/embed, or executable external reference was found. Two old public W3C doctypes contain no internal entities. Plasma's embedded image is an inline PNG; its entire base64 payload was decoded and inspected for type/dimensions/hash, not treated as a network link. Numeric path coordinates are classified as image data.

###### Retired payment samples

All 10 archived PHP examples were completely read. Their limitations include disabled TLS verification, unescaped display/error data, client-chosen amounts and incomplete callback owner/amount/deduplication verification. Those examples were already quarantined as non-executable text; no current runtime import or image path revives them. Existing archive notices remain. The BEPusdt references disagree on acknowledgement text, which is already handled by the maintained adapter's explicit configured behavior rather than guessing from an obsolete sample.

##### Corrected copied-code examples

These changes affect repository reference material, not installed personal/plugin skills or production endpoints. Repository permission and hosted-test policies were not modified.

1. `.agents/skills/golang-pro/SKILL.md`: the single worker now closes its result channel so a successful pipeline completes instead of waiting for its timeout. The documented ownership/buffering requirement is explicit.
2. `.agents/skills/golang-pro/references/concurrency.md`: channel processing, fan-in and both pipeline stages now select on cancellation while waiting to receive, not only while sending.
3. `.agents/skills/golang-pro/references/generics.md`: `constraints` is identified/imported from `golang.org/x/exp/constraints`, with the standard `cmp.Ordered` alternative noted. It is not a nonexistent standard package.
4. `.agents/skills/golang-pro/references/testing.md`: the illustrative positive-addition fuzz invariant excludes signed integer overflow, where its original monotonicity assertion was false.
5. `.agents/skills/vue-best-practices/references/component-fallthrough-attrs.md`: render-time getters replace computed values that would cache Vue's nonreactive attrs indefinitely.
6. `.agents/skills/vue-best-practices/references/sfc.md`: the sanitization example declares `lang="ts"` for its typed props.
7. `.agents/skills/telegram-mini-app/SKILL.md`: reusable `.ts` modules are plain TypeScript instead of invalid SFC wrappers; Vue bridge refs are per invocation; back-button registration occurs after mounting and removes the exact registered callback; Svelte retains and unregisters its callback on teardown.

The guides still contain intentionally bad examples and incomplete illustrative application boundaries. Their snippets are not certified turnkey production applications, deployment instructions or evidence that every optional library/version works with this project. The copied aesthetic guidance does not override the project's current product, authorization, accessibility, data or test policies.

##### Validation and browser evidence

- The four extracted, modified Go examples pass static `go vet`, with only the missing illustrative imports/types supplied for compilation. They were not executed. Extraction copies and results are under `reference-static/`.
- The changed TypeScript and Vue SFC examples pass TypeScript transpilation / Vue compiler syntax inspection; `reference-static/typescript.json` records each block. This is static syntax validation, not browser behavior.
- Scoped `git diff --check` passes. The final aggregate repository manifest is owned by the parent audit.
- Hosted browser run 43411561457 at `da319b3a25fbf903ae5be93218f1072cdbc29d55` produced artifact 14028974886 (`frontend-browser-recovery`): 142 passing scenarios, zero reported issues, 312 screenshots. The downloaded ZIP SHA-256 is `e9fcb41130f2a9f99d03e56955542f9ea0f879ff3487566fbfa2c427e53bc714`.
- Eight actual desktop/mobile PNGs were visually inspected; detailed provenance, observed states and limitations are in `browser/hosted-evidence.md`. Final-head PM/stablecoin screenshot review is handed to `frontend_contracts` for run 43415231195 at source checkpoint `4feed1176034d3b30d5e35702626542cd014d74a`.
- Local Chromium never reached a page because its process-singleton socket creation was denied by the environment. No local browser success is claimed, no rejected escalation was repeated, and no local browser suite ran. Hosted evidence is kept distinct from production rollout evidence.

##### Handoff

Source changes are stable. Root owns the final docs/audits report, exact-head CI verification, commit/push and merge. No production recovery rehearsal, live provider validation, signed current-release binary validation or fleet upgrade is claimed by this reference pass.
