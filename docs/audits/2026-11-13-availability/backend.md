# Persistence, business and input audit

Baseline: `985958f9bea54bd6145f27dd0f5b09ab0841609e` (merged comprehensive audit
PR #147). This follow-up reviews the requested availability/efficiency domains
against the retained full-file audit, then re-reviews every changed implementation,
contract and regression. It is not a second claim that every repository file was
read in this follow-up. Final hosted CI evidence belongs in the delivery PR.

## Findings and changes

| ID | Finding | Result and evidence |
|---|---|---|
| DB-01 | A process-wide database mutex was held across unrelated PostgreSQL calls. A blocked member/node row could delay IP worker heartbeats, PM receipts and browser logout. | Removed the global lock from statement-/transaction-owned mutations. Dedicated locks protect only the DIY cursor, cipher reference and group-fact flush. Existing row locks, unique constraints, compare-and-set predicates and transaction retries remain authoritative. `write_isolation_test.go` holds a real member row lock while an independent heartbeat and logout must finish. It also verifies exactly one winner for competing item starts/completions. |
| DB-02 | A full buffered group-message snapshot issued as many as 4096 INSERT round trips while holding that shared mutex. | Insert at most 512 facts per statement in one transaction, sorted by deduplication key. Up to eight statements persist a full buffer. Failed later batches roll back the whole snapshot; arrivals during a flush remain buffered. `activity_group_batch_test.go` tests 1025 records in three statements, injected second-batch failure/recovery and concurrent arrival. |
| DB-03 | Expired sessions, browser links, completed jobs and administrator reviews were deleted one row per query; node events populated temporary tables one ID at a time. | Batch already-selected exact expired IDs in groups of 512. Preserve RFC3339Nano parsing, live-bearer boundaries, unsettled-operation protection and the outer backup-gated transaction. Node events no longer create/populate/drop a temporary ID table. `retention_batch_test.go` protects three statements for 1025 expired sessions and a bearer with one nanosecond remaining. |
| DB-04 | Purchase histories and administrative inventory hydrated squad ownership through one query per purchase. | Use the existing authoritative ownership CTE over up to 256 purchase IDs per query. Full and split snapshots, explicit empty snapshots, paid fallback, activity rewards, deduplication and group ordering survive. `purchase_squad_batch_test.go` covers two queries for 257 purchases and both real list entrypoints. |
| DB-05 | Provider item mutations and member preference/membership changes updated a row, then read it through another snapshot. | `UPDATE ... RETURNING` returns the exact admitted/completed record and removes the second database round trip. A competing item transition cannot substitute its state for the winning statement's receipt. |
| VAL-01 | JSON validation first decoded into maps, dropping repeated keys. The later typed Go decoder merges repeated nested objects, so an earlier invalid field could survive although validation only saw the later object. | Validate every token and string, reject exact/escaped duplicate keys, and bound nesting before constructing a document tree. Case-sensitive map keys remain valid, while all case variants are checked before typed decoding. `json_tokens_test.go` reproduces the merge behavior and covers depth/trailing input. |
| RET-01 | The retired node multiplier accepted arbitrary values silently and returned a constant compatibility property; node writes accepted ignored server-owned fields. | Deleted `node_decode.go` and `Node.Multiplier`; strict writes allow only id/name/countryCode/providerName/enabled/configId. Obsolete multiplier and agent/revision/time fields are rejected. Historical fixed-point accounting and current 1x saves remain unchanged. `httpapi/xnode_node_write_test.go`, `native_policy_test.go`, and `node_multiplier_retired_test.go` cover the boundary. |
| RET-02 | Unused internal compatibility/fixture entrypoints remained. | Removed `PruneTransientRecords` and its generated retry adapter, `ListAllPurchases` and its unused interface/stub, the `ListProviderOperationItems` alias, and direct `RecordGroupMessageFact`. Their consumers now use the canonical existing interfaces; there is no replacement queue or persistence model. |
| RET-03 | Node DNS and DDNS exposed retired compatibility fields; the custom DDNS decoder also silently ignored unknown live inputs. | Removed public DNS mode/upstreams/legacy marker and singleton DDNS host aliases. Strict live settings use redirect and plural binding arrays. A bounded encrypted-settings reader preserves established per-node DNS as a private explicit native default and canonicalizes former stored DDNS scalars only when arrays were omitted. Ordinary saves drop retired keys; reads do not require a write, explicit native servers win, and shared configuration documents remain unchanged. `node_dns_test.go`, `node_settings_history_test.go` and `httpapi/xnode_settings_write_test.go` distinguish stored recovery from current requests. |
| RET-04 | The original administrative database-row PUT remained as an explicit compatibility alias without a production client. | Removed the handler, route and obsolete schema. Current typed POST review/apply routes retain all review hashes, confirmation and scope checks; the removed alias already enforced those checks, so no direct-write bypass is claimed. `database_admin_retirement_test.go` asserts the retired PUT is rejected before reaching database services. |
| AVAIL-01 | Previously accepted but unusable DDNS jobs would fail the stricter native contract and block compilation of unrelated users or a full reconnect. | Validate each stored DDNS job once in a pure compiled projection, omitting only invalid jobs while preserving encrypted source, host bindings and last accepted addresses. Strict live writes remain rejected. Only the current peer's accepted master candidate publishes `XNODE_DDNS_JOB_INVALID` after the compile epoch fence; manual/preflight and obsolete compiles do not mutate live diagnostics. The existing memory-only node log ring and master logger limit warnings to eight identifiers and once per node per minute. Unattempted manual runs fail input preflight; attempted work retains receipt replay. `ddns_compile_test.go` covers source preservation, bounded diagnostics, obsolete epochs/peers and replay; `ddns_compile_integration_test.go` covers paid users, authorization revocation, reconnect, valid siblings and strict writes. |

## Areas reviewed without a new storage model

- **Business/financial authority:** purchase admission and replay, immutable
  requested fingerprints, current-owner/lifecycle locks, quote version checks,
  coupon application, original renewal contract amounts, committed ledger facts,
  failed/uncertain provider outcomes and refund boundaries were traced against
  `billing.go`, purchase preparation, coupon grants, payment checkout, provider
  items and retention protections. No newly confirmed wallet or refund bypass was
  found beyond the corrected input/receipt boundary defects above. No invented
  charge, lost receipt, relaxed expiry or forced terminal outcome is introduced.
- **Feature availability under DB pressure:** authentication-token handling,
  member updates, native node mutations, PM content/topic recovery, IP lookup
  heartbeat/progress, provider item admission/completion, group ingestion and
  maintenance retention were reviewed. Independent writes now reach PostgreSQL
  independently; database-wide outages still return errors and preserve existing
  queue/receipt recovery. This does not claim service without its authoritative DB.
- **Unnecessary storage:** live node telemetry, source IPs and provider channel
  discovery remain process-local; stored information remains current desired
  state, encrypted secrets, necessary recovery/financial facts and bounded
  analytical/operational data. No table, column, persistent cache or copied
  external dataset was added. Existing exact cleanup selects are batched;
  historical accounting and idempotency receipts are not disposable cache data.
- **Input domain coverage:** shared HTTP method/path/query/header and JSON
  validation; structured node write fields; provider method selection; money and
  original checkout amount/currency equality; timestamp precision; purchase
  identity/quote/coupon validation. Routing/domain/protocol grammars are covered
  by the separate native-control audit.
- **Retirement boundaries:** the public administrative entitlement inventory and
  cancellation APIs remain distinct current capabilities despite removal of an
  unused UI fixture. Historical payment method spellings, accepted payment/renewal
  evidence, immutable migrations and pending job kinds retain real recovery
  consumers; deleting them would make accepted paid work unavailable. The removed
  paths have no such authority or consumer.
- **Independent review of controller changes:** inspected outbox admission and
  shutdown joining, per-node enrollment gates and report lanes. Corrected a
  newly introduced ordering race: generic state errors now retain the control
  lane, while errors for pending commands use the independent receipt lane.
  `master_results_isolation_test.go` holds an applied-state database ACK, proves
  command receipts still progress, then verifies a later state rejection cannot
  be erased by that older ACK.

## Verification

Allowed local static work: code and call-site review, SQL/control-flow/concurrency
review, `gofmt`, generated retry-adapter reconciliation, `go vet` and `go build`.
Repository policy forbids local automated suites; none were run. Regression tests
are authored for the hosted Go/race/PostgreSQL checks. Final-head hosted results
must pass before merge and release.

The change reduces defined query counts and removes an identified scheduling
bottleneck; no production throughput, latency or database-size benchmark is
claimed. Retention retains its existing recovery transaction and scans; batching
bounds each transfer, not the total number of historical candidates in a full
maintenance pass. New uncommitted native traffic remains subject to the explicit
upstream volatile-stream accounting contract.

Invalid stored DDNS is unavailable until an administrator corrects the original
settings. Omitted jobs do not produce fabricated agent execution receipts or
scheduled success entries; the existing node log view carries the explicit
diagnostic. This isolates utility failure from native availability without
discarding the administrator's source configuration.

### Retention concurrency cross-check

The selected-ID batch deletes retain the production SERIALIZABLE transaction
boundary: the PostgreSQL connector upgrades writable default-isolation
transactions, and CompactAndPrune retries serialization/deadlock failures. A
concurrent browser-session renewal changes the selected row and forces a stale
retention deletion to retry; if deletion wins, the renewal cannot restore it.
Browser-link and review expiries are immutable, consumed reviews never reopen,
and completed outbox jobs have no production transition back to live work; the
batch job delete also rechecks done status. No extra lock, persistence or
renewal/deletion exemption was added.
