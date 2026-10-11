# IP Lookup

## Responsibility and entry points

This module owns queued public-IP reputation checks, compact source-attributed
reports, owner-bound quotes, and provider normalization. `Service` accepts
canonical public IPv4/IPv6 addresses; database repositories own transactional
ledger/quota settlement and shared cache selection. `Worker` runs in the existing
provider-operation dispatcher. Each provider adapter enters its bounded
`upstreamqueue.Queue` before outbound HTTP; there is no second job lane.

Public contracts live in `types.go` and `report_types.go`; `config.go` validates
settings and hashes the frozen configuration. `worker.go` owns recovery and early
stopping. `verdict.go` folds transient evidence; `refusals.go` contains refusal
policy. `geo.go`, `provider_geo.go`, and `provider_sources.go` select coherent
location data and preserve enrichment lineage. `report_codec.go` is the sole
report persistence boundary. `retry_policy.go`, `worker_retry.go`, and
`live_retry.go` own background repair of partial results.

- `address.go` canonicalizes public IPs; `service.go` signs quotes and exposes read-only cache previews.
- `registry.go` describes provider capabilities; `provider_http.go` executes bounded fixed-endpoint requests through the existing queue.
- `provider_parse.go`, `provider_insights.go`, and `provider_scamalytics.go` normalize documented responses without storing provider snapshots.
- `provider_test.go`, `provider_http_test.go`, and `worker_test.go` cover provider variants, queued transport and durable recovery in hosted CI.

## Data and recovery

`ProviderResult` is transient. Persist reports with `EncodeReport`, restore with
`DecodeReport`, and expose `PublicReport` to members. Never serialize individual
provider payloads as a report. The compact public projection contains contacted
database IDs/statuses, positive refusal items and evidence, one network type,
country/city/coordinate pair, ASN/name, selected-field sources, and four nullable
MaxMind Insights values: `ip_risk_snapshot`, `static_ip_score`, `user_count`, and
`user_type`. Missing values remain absent/nullable; explicit zeroes are preserved.
Region, ISP, provider-wide scores, and negative per-database flags are not stored.

While processing, `_checkpoint` retains only stage IDs/statuses/attempt markers,
aggregate risk coverage, majority votes, and the winning location/comparison
metadata. Before each provider call the attempted marker is durable. Completed
stages never replay; interrupted attempted stages become unavailable. Persisted
partial and processing reports keep `_checkpoint` so repair can resume; encoded
terminal reports drop it. `RecordProvider` folds a stage exactly once and marks
the per-stage `complete` flag used to recompute aggregate completeness on repair.

Legacy payloads are compacted on decoding/migration. Completed provider evidence
is folded into the checkpoint, preserving stage markers without new requests.
Old terminal status, verdict, reasons, policy version, and refund decision stay
frozen. Historical failed results remain ineligible for cache. Provider URLs,
credentials, and raw upstream errors never enter report snapshots.

## Ordering and verdict

The administrator's `providers` array is the check sequence; omitted known
providers are appended disabled. `geolocationOrder` is an independent complete
permutation, defaulting to IP2Location, IPAPI, MaxMind, Scamalytics, AbuseIPDB.
IPQS remains available for risk checks and network voting, outside location
selection. Pending work uses its frozen configuration and matches adapters by ID.

Positive per-IP abuse, datacenter, VPN, proxy, Tor, report count, or applicable
fraud-score evidence refuses the IP and stops subsequent providers. Scamalytics
low risk does not refuse solely for a positive score; medium/high/very-high do.
Explicit risk flags always refuse. MaxMind `user_count > 5` refuses and records
the actual count; 5 passes that check. Its other requested Insights values are
informational. Missing optional subscription fields remain unknown.

IPAPI network classification uses `is_datacenter`, `is_satellite`, and `is_mobile`
in that precedence. All three explicit false means residential; absent flags
stay unknown. Organization/ASN type never overrides these flags. One normalized
vote is counted per contacted provider. A type must exceed half the known votes;
ties and plurality without a strict majority stay unknown. Supporting source
IDs are retained. A suitable verdict requires complete configured risk coverage
and a residential/mobile majority; it does not promise service-specific access.

Contacted provider errors continue the sequence and are cached as partial,
inconclusive results. They never set `RefundRequired`, including an all-provider
outage or an outage before a later refusal. Actual infrastructure failures before
any provider attempt are distinguished using `ReportAttempted` and settled by
the database recovery path. Old refunded outcomes remain unchanged.

## Background retry of partial results

After a partial result reaches the member, the database schedules the first
repair job through the shared outbox when the frozen policy allows it. The wait
applies before the first retry and between retries (`available_at = now +
delay`), bounded by `MaxAttempts` (default 3, max 10) and `DelaySeconds`
(default 60, max 86400); a stored config without `retry` keeps the enabled
defaults. `Worker.HandleReportRetryOutbox` re-runs only error/processing/queued
stages of the frozen cache row — refusal-skipped, disabled, and successful
stages never replay — using the report's frozen credentials. Repairs update
the shared cache row in place under a `status='partial'` guard (conflict or a
disappeared row stops the chain silently) without touching receipts, billing,
or `completed_at`, so newer paid refreshes keep winning cache selection.

`LiveService.HandleLiveRetryOutbox` applies the same policy shape to partial
live snapshots: only failed sections refetch (a topology failure cascades to
graph/block/ASN sections), the merged snapshot replaces the frozen row, and
the chain continues while sections stay failed. Every outbound call still
flows through the existing upstream queues; repair jobs never bypass them.

- `retry_policy_test.go` covers policy defaults, attempt bounds, failed-stage selection, checkpoint repair and payload deduplication in hosted CI.
- `worker_retry_test.go` covers delayed report repair chains, exhausted policies, missing reports, healthy reports and conflicting cache writes in hosted CI.
- `live_retry_test.go` covers section repair planning, topology dependency cascades, PTR recovery, delayed retries and terminal/conflicting snapshot handling in hosted CI.

## Location selection

Each location candidate is one coherent source bundle; country, city, and
coordinates are never mixed across databases. Both coordinates must be finite
and within latitude/longitude bounds; zero is valid. City/coordinate candidates
take precedence over country-only fallback. Scamalytics enrichment sources are
named explicitly, such as `scamalytics:maxmind_geolite2`; AbuseIPDB geolocation
and network evidence is attributed to its documented IPinfo enrichment.

The following five bands are **application comparison policy**, not a claim that
different vendors' metrics are equivalent:

| Band | Explicit numeric confidence | Radius-only fallback |
| --- | --- | --- |
| VERY_HIGH | 95–100 | 0–10 km |
| HIGH | 80–94 | >10–50 km |
| MEDIUM | 60–79 | >50–200 km |
| LOW | 30–59 | >200–1,000 km |
| VERY_LOW | 0–29 | >1,000 km |

IPAPI native labels keep their band. MaxMind city confidence applies only to
city-bearing candidates; country confidence applies only to country-only data.
Use radius-derived bands when relevant explicit confidence is unavailable.
Known quality outranks missing quality. Within a band, smaller available radius
wins (missing radius sorts last), then administrator provider priority. This is
a total comparison so check order cannot change the winner. Explicit anycast
and IP2Location's capital fallback coordinates are VERY_LOW; an absent city is
never invented. The map must depict approximate IP location rather than a home.

## Cache, quote, and billing contracts

Quotes expire after two minutes and bind the owner, canonical requested IP,
frozen settings, and observed price/allowance. `QuoteResponse` adds response-only
`cachedReport` and `cacheMatch` (`none`, `exact`, `subnet`); signing/submission uses
only the embedded `Quote`. Cache previews do not create checks or consume quota.
Member receipts separately retain `requestedIP` and the original report's IP.

Reports have no automatic expiry. Exact-IP cache matches win; otherwise IPv4
can reuse a completed report in the same /24. IPv6 stays exact-only. Cache previews are free and offer the existing signed refresh price.
Refreshes from exact or IPv4 /24 previews target the requested IP, update quality
and realtime data, charging once per idempotent command using the configured refresh quota quantity or TXB fallback. Configuration changes
do not rewrite cached policy/verdict. Fresh lookups consume TXB or an allowance
snapshotted at first successful purchase activation; polling never charges.

## Extension, references, and validation

Add providers through `Registry` and a queued `Provider` adapter, then extend
normalization, contracts, and localization. Keep vendor capability/omitted-field
semantics documented. HTTP responses are bounded, fixed-endpoint, and redacted.
Settings, vaulted secrets, combo quotas, and audit are committed atomically.

- [AbuseIPDB](https://docs.abuseipdb.com/): 90-day report window; no verbose comments.
- [Scamalytics v3](https://docs.scamalytics.com/ip-fraud-risk-api/v3/): EU `api12`, account username, tier placeholders, enrichment lineage.
- [IPAPI](https://ipapi.is/developers.html): keyed flags, accuracy bands, ASN organization, VPN/provider names.
- [MaxMind Insights](https://dev.maxmind.com/geoip/docs/web-services/responses/): omitted false anonymizer flags, city/country confidence, radius, exact Insights fields.
- [IPQS](https://www.ipqualityscore.com/documentation/proxy-detection-api/response-parameters): strictness 3 and unknown unavailable connection type.
- [IP2Location](https://www.ip2location.io/ip2location-documentation): usage codes, coordinates/capital fallback, ASN name, proxy provider, tier-specific fields.

Hosted fixtures cover HTTP queue/authentication boundaries, provider variants,
five/six-user refusal thresholds, confidence/radius boundaries, order-independent
location selection, strict majority, compact serialization, legacy recovery,
and continuing API errors without refunds. Local automated suites are prohibited.
Static vetting is separate from hosted test and frontend browser evidence.

`geo_test.go` checks quality boundaries and all check-order permutations;
`compact_report_test.go` checks strict majority, MaxMind count boundaries, and
legacy compaction; `config_order_test.go` checks administrator order validation;
`provider_provenance_test.go` checks enrichment lineage and refusal evidence.

## Live network details

`live_service.go` owns the `GET /api/v1/ip-lookup/details` workflow,
member admission (one active request, six starts/minute), cancellation, and a
60-second total deadline. `live_types.go` defines the public projection and queue
source identifiers; these sources are not reputation providers. New realtime evidence is embedded in the combined report. Public reads serve saved evidence; quoted checks and refreshes update quality and realtime phases together. Realtime evidence never changes suitability policy. Requested IPs are canonical
public addresses, even when reputation previews reuse a neighboring IP.

- `live_transport.go` owns bounded, fixed-origin, redacted HTTP through existing `upstreamqueue` workers. The 15-second per-call budget applies only to HTTP execution once a job is dispatched; queue wait is bounded by the caller's flight deadline, so slower shared pacing cannot starve tail calls.

Live source queues are process-shared by all concurrent lookups and paced per source in `internal/app/ip_lookup.go`: RIPEstat and CAIDA at 600 ms (two in-flight), Cloudflare Radar at 800 ms (one in-flight), BGPKIT/RouteViews/InternetDB at 500 ms, and reputation providers plus DNS at 250 ms. Queue throughput is one request per interval regardless of worker count; workers only cap simultaneous in-flight requests. Worst-case cold lookups stay inside the 60-second flight budget at the cost of higher latency.
- `live_bgp.go` selects the longest observed prefix, uses original paths transiently, collapses visual prepends into edges, supports multiple origins, and bounds paths/nodes. RIPE Looking Glass falls back to RouteViews, then the explicitly dated RIPE BGP snapshot.
- `live_relationships.go` and `live_relationship_bulk.go` use bounded, paginated CAIDA observed-neighborhood queries and optional Cloudflare fallback. Only existing observed pairs are annotated, both endpoint orientations are handled, partial valid data is preserved, and indirect/unavailable relationships remain observed adjacency.
- `live_names.go` caches only successful ASN names, with seven-day expiry and 4,096-entry LRU eviction. Keyed IPAPI bulk, BGPKIT, and bounded RIPE holder lookups fill missing names. Collector exchange scope is observation context, never an invented physical hop.
- `live_asn.go` selects registration from RIPE RIR `lod=2`, routing totals from RIPE/RouteViews, and dated relationship degrees from CAIDA/Cloudflare.
- `live_asn_prefetch.go` overlaps independent ASN reads through a lookup-local request memo.
- `live_requests.go` coalesces identical queued HTTP reads within a lookup and tracks transient failures.
- `live_pipeline.go` emits immutable section snapshots and progress over nine fixed logical phases; topology arrives before contextual enrichment finishes. `live_tasks.go` derives one `ProgressTask` per phase from each finished section's `LiveMeta` (observed emptiness is success, unconfigured credentials are skipped, mixed ASN sources are partial), and every progress event carries the ordered per-task list.
- `live_events.go` defines progress/section/complete/error stream events and optional refresh behavior. JSON clients keep the same response projection.
- `live_cache.go` bounds concurrent shared work to 64 flights; completion evidence lives in SQLite. Free reads match exact IP then IPv4 /24 without expiry or credential invalidation. `RefreshRun` is worker-only and uses a separate durable run identity. Legacy internal live flights cancel when their last subscriber leaves; fresh combined work belongs to the application runtime.
- `live_classification.go` reads CAIDA's authoritative dated `dataset.clique` and bulk ASN names. The API's per-ASN `cliqueMember` flag incorrectly marks non-members true and is never used; absent/invalid membership lists stay unknown, without degree-based guesses.
- `progress.go` projects private durable checkpoints into check progress without exposing unfinished verdicts or changing persistence. Check progress appends one provider task per checkpoint stage; the realtime refresh phase of a paid check surfaces as a single trailing `live` task (`ip_lookup_live_check.go`), since per-section progress stays in the in-memory stream and is never persisted per section.
- `live_cache_test.go` covers persistent reads, rejected free refreshes, attempted outage caching, pre-attempt exclusion and early streaming in hosted CI.
- `live_ip.go` supplies keyed contextual IPAPI scores, distinct AbuseIPDB reported addresses in the masked IPv4 /24 or IPv6 /64 (30 days), exact decimal address capacity, and queued PTR lookups. Missing data remains nullable; block-plan restrictions never silently narrow the subnet.
- `live_traffic.go` requests Cloudflare Radar's BOT_CLASS, DEVICE_TYPE, and IP_VERSION summaries for the ASN over seven days; returned Other device values and actual date windows are preserved.
- `live_test.go` and `live_http_test.go` provide hosted regressions for prefix/origin/path preservation, name-only cache bounds, admission, queue enforcement, credentials, unknown/zero values, restrictions, and Cloudflare windows.

Cloudflare credentials use the existing vaulted settings map under
`cloudflare_radar`, independent of ordered risk-provider configuration. Blank
means keep; explicit clear removes it. No token or source payload reaches public
reports, audit events, or errors. Public adapters can return partial/unavailable
sections independently; external datasets retain their own observation dates.
`live_shodan.go` fetches InternetDB associated hostnames and observed open ports through the Shodan source queue. Expected no-information responses are empty, failures stay separate, invalid/out-of-range values are excluded, and no vulnerability/tag verdict is added. InternetDB is a weekly dataset without per-host observation timestamps. See https://internetdb.shodan.io/ for API and commercial-use licensing terms.
`live_shodan_test.go` covers sorted/deduplicated InternetDB hostnames and ports, unobserved addresses, provider outages, mismatched IPs, absent fields and invalid port ranges in hosted CI.
`live_integration_test.go` exercises the complete queued response projection with multiple origins, original prepends, source fallback, zero/unknown fields, live neighboring-IP requests, name-only cache behavior and credential-free public JSON in hosted CI.
Topology limits are 300 nodes, 1,500 edges and 1,000 original paths; omitted paths are disclosed. InternetDB retains at most 256 hostnames and 2,048 ports and marks larger observations partial.
- `live_snapshot.go` emits cached completion and classifies partial realtime coverage.
- `live_compact.go` is the public and storage boundary: retain the versioned observed-hop/T1 projection and deduplicated reachability/hop summaries; original AS paths never enter streamed or persisted output.
- `live_snapshot_test.go` covers compaction and lightweight repository fixtures in hosted CI.
- `worker_live.go` owns durable refresh-phase claim, attempt and snapshot-reference recovery. Complete quality stages remain checkpointed before realtime begins; joined receipts settle together only after both phases finish.
- `worker_lock.go` serializes shared report work with reference-counted locks, preventing joined jobs from overwriting checkpoints or duplicating external attempts.
- `worker_live_test.go` verifies realtime refresh after early quality refusal and no replay of completed phases in hosted CI.
- `live_shared_test.go` verifies shared subscriber cancellation and dataset membership despite false-positive per-ASN flags in hosted CI.
- `live_relationship_bulk_test.go` covers direct-pair pagination, reversed endpoints, unknown indirect links, absent clique lists and preservation of valid partial GraphQL data.

## Persistent realtime contracts

`LiveRepository` supplies cache reads and normalized snapshot storage. Public
`LiveDetails.ip` identifies the request; `sourceIP` identifies the original
observation and `cacheMatch` exposes exact/subnet reuse without changing source
timestamps. Snapshot IDs and `LiveStage` recovery markers stay private.
`GET details?refresh=true` returns `IP_LOOKUP_REFRESH_REQUIRED`; only the signed
quote/check workflow may force new observations. Interrupted work is not cached;
attempted source errors are cacheable partial evidence. Persistence failures
are retried through the existing durable lane, never through a second queue.

## Routing projection and registry enrichment, version 2

- `live_graph_projection.go` freezes per-observation ASN/edge counts, collapses prepends and retains three observed hops plus observed T1 endpoints. Original paths stay transient; compact route summaries preserve the real length of omitted segments. Repeated reads preserve the versioned projection unchanged.
- `live_transit_context.go` retains non-T1 intermediates toward the first observed T1, including peer connections that carry the origin's route, plus classified nearby upstreams. It excludes non-T1 collector branches beyond T1 and unrelated classified peers/customers. Missing T1 classification retains observed fallback evidence. A T1 origin shows only observed T1 context.
- `live_relationship_bulk.go` annotates retained observed pairs with dated per-edge provenance. Work covers at most 32 ASNs ordered by hop distance, observation count and ASN, using four bounded pages each and the existing flight deadline. Source failures remain explicit and cannot invent connections.
- `live_tier1.go` reuses dated CAIDA clique evidence for already-observed T1-to-T1 peer links. T1 ASNs are excluded from neighborhood expansion and name fallback calls; their bulk names, observations and graph nodes remain visible. `live_tier1_test.go` checks zero T1 follow-up requests, continued non-T1 classification and unknown-classification fallback in hosted CI.
- `live_contacts.go` fetches origin abuse emails from RIPEstat, validates/deduplicates them and composes at most eight origin panels. `live_websites.go` batches origin website reads from PeeringDB through a fixed-source queue paced at three seconds. A bounded process-local one-minute request memo honors duplicate-query limits; only safe HTTP(S) links are published.
- `live_transfer.go` fetches RIPEstat history for the exact source IP, retains normalized resource/holder/registry/date fields, sorts newest first and distinguishes no transfers from failed coverage. The snapshot JSON holds these fields without mirroring a registry dataset.
- `live_ip.go` derives the AbuseIPDB query from the address itself: IPv4 /24 and IPv6 /64, with a 30-day window. Routing failure does not suppress the check. Legacy mismatched block observations are suppressed without a new request. PTR values are trimmed before trailing-dot removal and deduplication.
- `live_graph_projection_test.go`, `live_transit_context_test.go` and `live_metadata_test.go` cover path/count preservation, the three requested IP examples, peer intermediates, irrelevant branches, source validation and coalescing in hosted CI.

Older completed projections are never upgraded by reads or background jobs; an
explicit paid refresh supplies version 2. New source errors retain the existing
bounded repair policy. Transfer history adds a ninth streaming phase; phase totals
and per-source availability remain independent of the quality verdict.

## RPKI quality policy, residential-v4

`rpki_types.go` defines compact normalized origin/ROA evidence. `rpki_lookup.go`
resolves the actual RIS covering prefix and validates each distinct observed origin
through the RIPE queue with two workers. `worker_rpki.go` owns a separate durable
checkpoint after reputation stages and before final aggregation. Attempt markers
are persisted before dispatch; interrupted or completed work is not replayed.
The evidence lives in report JSON, and the private stage lives in its checkpoint.

An invalid origin ASN or prefix length adds a refusal with ASN/prefix evidence.
Valid, no-ROA, unrouted and unavailable results are neutral for suitability;
unavailable coverage marks the report partial so the existing outbox can repair it.
`rpki_repair.go` refetches failed frozen pairs without erasing successful invalid
evidence. Provider order, coverage, charges, quota and refund accounting are
unchanged. Older reports with no RPKI checkpoint keep their original policy.
`rpki_lookup_test.go` and `worker_rpki_test.go` cover validation, mismatches,
multi-origin policy, durable ordering, repair, interruption and codec boundaries.

Public references: [RPKI validation](https://stat.ripe.net/docs/data-api/api-endpoints/rpki-validation),
[transfer history](https://stat.ripe.net/docs/data-api/api-endpoints/transfer-history),
[abuse contacts](https://stat.ripe.net/docs/data-api/api-endpoints/abuse-contact-finder),
and [PeeringDB query limits](https://docs.peeringdb.com/howto/work_within_peeringdbs_query_limits/).

## Background combined reports

Fresh operations execute independently of HTTP viewers. `background.go` owns application lifecycle, bounded admission, recovery, one-minute leases and 15-second heartbeats. `cache_key.go` defines IPv4 /24 and exact IPv6 cache identity. `enhanced_types.go` and `enhanced_progress.go` define compact evidence and the identity-free progress projection. `worker_enhanced.go` sequences proxy, short live enrichment, full probes and the gated subnet phase. `live_block_gate.go` defers AbuseIPDB block requests until the requested address passes.

`proxy_config.go` validates administrator-named inventories and resolver/port defaults. `proxy_parser.go` extracts endpoint hosts; `proxy_encoding.go` handles URI, structured configuration and encoded subscriptions. `proxy_pool.go` coalesces inventories for 30 minutes; `proxy_transport.go` bounds downloads and pins public DNS answers to prevent private-network redirects/rebinding; `proxy_dns.go` distributes queued DNS queries across configured public resolvers. Lists and subscription secrets are never persisted.

`probe_runner.go` checkpoints two concurrent ports per operation; `probe_dispatch.go` sends every network action through shared queue workers. `probe_handshake.go` observes SOCKS/HTTP and bounded Shadowsocks reactions; `probe_tls.go` records TLS feasibility and the Reality DNS heuristic. `probe_trojan.go`, `probe_trojan_request.go` and `probe_trojan_observation.go` retain all five research profiles, 12,000 streamed headers and 150/595/605-second observation milestones. `subnet.go` checks proxy membership, one AbuseIPDB block, then separate IPAPI requests, short-circuiting each positive address. Neighbor findings never refuse an otherwise suitable requested IP.

The public live-details endpoint serves saved combined report evidence only; the internal live service keeps legacy adapters for recovery compatibility. New reports embed live data and never create independent live snapshots. All fresh and refresh work uses the signed billing operation. A positive refresh quota cost uses the configured number of allowance units before TXB fallback; zero preserves TXB-only refreshes. Cache replacement detaches old report references while retaining receipts and quota/ledger history.

Hosted tests: `proxy_parser_test.go` covers endpoint formats, encoded input and URL boundaries; `probe_trojan_test.go` simulates full timing without wall-clock sleeps and verifies the complete streamed request; `enhanced_verdict_test.go` covers gating, suspected findings and neighbor isolation. Local automated suites remain prohibited.

`subnet_test.go` uses a queued synthetic block response to verify source order, per-address short-circuiting and requested-IP response reuse without external requests in hosted CI.

`worker_provider_context.go` folds reusable IPAPI context and starts inventory assessment after the first reported country, keeping the provider executor small.
